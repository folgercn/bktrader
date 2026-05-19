"""Walk-forward validation for breakout structure-quality gates.

Research-only. This validates the mined structure-quality gates without using
future data for thresholds or candidate selection. Each split calibrates gate
thresholds on a trailing train window, selects the best train gate, then
evaluates it on the next calendar month with same-close and adverse-fill stress.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    EVAL_END,
    EVAL_START,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _evaluate_events,
    _load_all_1s_bars,
)

logger = logging.getLogger(__name__)

BASE_EVENTS_PATH = OUTPUT_DIR / "breakout_shape_expansion_events_restrictive_0p5bps.csv"
BASE_TRADES_PATH = OUTPUT_DIR / "breakout_structure_quality_trades_baseline_touch_entry.csv"


@dataclass(frozen=True)
class GateCondition:
    column: str
    op: str
    quantile: float | None = None
    value: float | None = None


@dataclass(frozen=True)
class GateSpec:
    name: str
    description: str
    conditions: tuple[GateCondition, ...]


@dataclass(frozen=True)
class MaterializedGate:
    spec_name: str
    description: str
    conditions: tuple[tuple[str, str, float], ...]


GATE_SPECS: list[GateSpec] = [
    GateSpec(
        name="baseline_model_advance",
        description="no extra gate after current-shape timing advance",
        conditions=(),
    ),
    GateSpec(
        name="low_eff_q20",
        description="train-calibrated lower 20% pre-touch 300s efficiency",
        conditions=(GateCondition("eff_300s", "<=", quantile=0.20),),
    ),
    GateSpec(
        name="low_eff_low_atr_q20_q40",
        description="low efficiency plus low 24h ATR percentile",
        conditions=(
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("signal_atr_percentile", "<=", quantile=0.40),
        ),
    ),
    GateSpec(
        name="low_rf_slope_up_q40_q60",
        description="lower RF probability plus positive closed-bar SMA5 slope",
        conditions=(
            GateCondition("rf_probability", "<=", quantile=0.40),
            GateCondition("prev_sma5_slope_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec(
        name="level_far_sma_gap_q60_q80",
        description="level far from signal open plus prior close above side-normalized SMA5",
        conditions=(
            GateCondition("level_to_signal_open_atr", ">=", quantile=0.60),
            GateCondition("prev_sma5_gap_atr", ">=", quantile=0.80),
        ),
    ),
    GateSpec(
        name="wick_touch_ext_le_0",
        description="first touch is wick-led, close has not extended beyond level",
        conditions=(GateCondition("touch_extension_atr", "<=", value=0.0),),
    ),
    GateSpec(
        name="wick_late_q40",
        description="wick-led first touch and not too early in the signal bar",
        conditions=(
            GateCondition("touch_extension_atr", "<=", value=0.0),
            GateCondition("pre_touch_seconds", ">=", quantile=0.40),
        ),
    ),
]


def _load_events_and_trades() -> tuple[pd.DataFrame, pd.DataFrame]:
    if not BASE_EVENTS_PATH.exists():
        raise FileNotFoundError(f"{BASE_EVENTS_PATH} missing; run breakout_shape_expansion.py first")
    if not BASE_TRADES_PATH.exists():
        raise FileNotFoundError(f"{BASE_TRADES_PATH} missing; run breakout_structure_quality_gate_sweep.py first")

    events = pd.read_csv(BASE_EVENTS_PATH)
    trades = pd.read_csv(BASE_TRADES_PATH)
    for df in (events, trades):
        if "touch_time" in df.columns:
            df["touch_time"] = pd.to_datetime(df["touch_time"], utc=True)
    for column in ("signal_start", "signal_end"):
        if column in events.columns:
            events[column] = pd.to_datetime(events[column], utc=True)

    events = events[
        (events["touch_time"] >= EVAL_START)
        & (events["touch_time"] < EVAL_END)
        & (events["timing_prediction"] != "skip")
    ].copy()
    trades = trades[trades["selected_delay"] != "none"].copy()

    pnl = trades[["event_key", "weighted_pnl", "delay_pnl_pct", "position_size"]].copy()
    events = events.merge(pnl, on="event_key", how="inner")
    events["year_month"] = events["touch_time"].dt.strftime("%Y-%m")
    return events.reset_index(drop=True), trades.reset_index(drop=True)


def _month_range(start: str, end_exclusive: str) -> list[pd.Timestamp]:
    end = pd.Timestamp(end_exclusive, tz="UTC")
    return [
        month
        for month in pd.date_range(pd.Timestamp(start, tz="UTC"), end, freq="MS")
        if month < end
    ]


def _materialize_gate(spec: GateSpec, train_events: pd.DataFrame) -> MaterializedGate:
    conditions: list[tuple[str, str, float]] = []
    for condition in spec.conditions:
        if condition.value is not None:
            value = float(condition.value)
        elif condition.quantile is not None:
            series = pd.to_numeric(train_events[condition.column], errors="coerce").dropna()
            value = float(series.quantile(condition.quantile)) if not series.empty else np.nan
        else:
            raise ValueError(f"condition needs value or quantile: {condition}")
        conditions.append((condition.column, condition.op, value))
    return MaterializedGate(spec.name, spec.description, tuple(conditions))


def _apply_materialized_gate(events: pd.DataFrame, gate: MaterializedGate) -> pd.DataFrame:
    if not gate.conditions:
        return events.copy().reset_index(drop=True)
    mask = np.ones(len(events), dtype=bool)
    for column, op, value in gate.conditions:
        if not np.isfinite(value):
            mask &= False
            continue
        series = pd.to_numeric(events[column], errors="coerce")
        if op == "<=":
            mask &= (series <= value).fillna(False).to_numpy()
        elif op == ">=":
            mask &= (series >= value).fillna(False).to_numpy()
        else:
            raise ValueError(f"unsupported operator: {op}")
    return events[mask].copy().reset_index(drop=True)


def _monthly_metrics_from_pnl(events: pd.DataFrame) -> dict[str, Any]:
    if events.empty:
        return {
            "trade_count": 0,
            "calendar_sum": 0.0,
            "worst_sm": 0.0,
            "neg_sm": 0,
            "avg_pnl": 0.0,
        }
    monthly = events.groupby("year_month")["weighted_pnl"].sum()
    return {
        "trade_count": int(len(events)),
        "calendar_sum": float(monthly.sum()),
        "worst_sm": float(monthly.min()) if len(monthly) else 0.0,
        "neg_sm": int((monthly < 0).sum()),
        "avg_pnl": float(events["weighted_pnl"].sum() / len(events)),
    }


def _matrix_metrics(matrix: pd.DataFrame, scenario: str) -> dict[str, Any]:
    if matrix.empty:
        return {"calendar_sum": 0.0, "worst_sm": 0.0, "neg_sm": 0, "trade_count": 0}
    row = matrix[matrix["scenario"] == scenario]
    if row.empty:
        return {"calendar_sum": 0.0, "worst_sm": 0.0, "neg_sm": 0, "trade_count": 0}
    item = row.iloc[0]
    return {
        "calendar_sum": float(item["calendar_sum_gate_on"]),
        "worst_sm": float(item["worst_sm_gate_on"]),
        "neg_sm": int(item["neg_sm_count"]),
        "trade_count": int(item["trade_count_gate_on"]),
    }


def _evaluate_forward_events(
    *,
    gate_name: str,
    forward_month: str,
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> tuple[dict[str, Any], pd.DataFrame]:
    matrix, trades = _evaluate_events(events, bars_cache)
    same = _matrix_metrics(matrix, "same_close_xslip0bps")
    adverse0 = _matrix_metrics(matrix, "next_adverse_xslip0bps")
    adverse10 = _matrix_metrics(matrix, "next_adverse_xslip10bps")
    row = {
        "forward_month": forward_month,
        "gate": gate_name,
        "forward_events": len(events),
        "same_close_calendar_sum": same["calendar_sum"],
        "same_close_worst_sm": same["worst_sm"],
        "same_close_neg_sm": same["neg_sm"],
        "adverse0_calendar_sum": adverse0["calendar_sum"],
        "adverse0_worst_sm": adverse0["worst_sm"],
        "adverse0_neg_sm": adverse0["neg_sm"],
        "adverse10_calendar_sum": adverse10["calendar_sum"],
        "adverse10_worst_sm": adverse10["worst_sm"],
        "adverse10_neg_sm": adverse10["neg_sm"],
        "trade_count": same["trade_count"],
    }
    if not trades.empty:
        trades = trades.copy()
        trades["walkforward_gate"] = gate_name
        trades["forward_month"] = forward_month
    return row, trades


def _select_gate(train_rows: list[dict[str, Any]], min_train_trades: int) -> dict[str, Any]:
    eligible = [
        row
        for row in train_rows
        if row["train_trade_count"] >= min_train_trades
        and row["train_calendar_sum"] > 0.0
        and row["gate"] != "baseline_model_advance"
    ]
    if not eligible:
        baseline = [row for row in train_rows if row["gate"] == "baseline_model_advance"]
        if baseline:
            return baseline[0]
        return max(train_rows, key=lambda row: (row["train_calendar_sum"], row["train_trade_count"]))
    return max(
        eligible,
        key=lambda row: (
            row["train_calendar_sum"],
            row["train_worst_sm"],
            row["train_trade_count"],
        ),
    )


def _aggregate_forward(rows: list[dict[str, Any]], gate_column: str = "gate") -> pd.DataFrame:
    if not rows:
        return pd.DataFrame()
    df = pd.DataFrame(rows)
    out_rows: list[dict[str, Any]] = []
    for gate, group in df.groupby(gate_column):
        out_rows.append(
            {
                "gate": gate,
                "forward_months": int(group["forward_month"].nunique()),
                "trade_count": int(group["trade_count"].sum()),
                "same_close_calendar_sum": float(group["same_close_calendar_sum"].sum()),
                "same_close_worst_month": float(group["same_close_calendar_sum"].min()),
                "same_close_neg_months": int((group["same_close_calendar_sum"] < 0).sum()),
                "adverse0_calendar_sum": float(group["adverse0_calendar_sum"].sum()),
                "adverse0_worst_month": float(group["adverse0_calendar_sum"].min()),
                "adverse0_neg_months": int((group["adverse0_calendar_sum"] < 0).sum()),
                "adverse10_calendar_sum": float(group["adverse10_calendar_sum"].sum()),
                "adverse10_worst_month": float(group["adverse10_calendar_sum"].min()),
                "adverse10_neg_months": int((group["adverse10_calendar_sum"] < 0).sum()),
            }
        )
    return pd.DataFrame(out_rows).sort_values(
        ["adverse10_calendar_sum", "same_close_calendar_sum"],
        ascending=[False, False],
    )


def _conditions_text(gate: MaterializedGate) -> str:
    return " & ".join(f"{column} {op} {value:.12g}" for column, op, value in gate.conditions) or "none"


def _markdown_table(df: pd.DataFrame, cols: list[str]) -> str:
    if df.empty:
        return "_empty_"
    view = df[cols].copy()

    def fmt(value: Any) -> str:
        if isinstance(value, (float, np.floating)):
            if not np.isfinite(value):
                return ""
            return f"{float(value):.6f}"
        if isinstance(value, (int, np.integer)):
            return str(int(value))
        if pd.isna(value):
            return ""
        return str(value)

    rows = [[fmt(value) for value in row] for row in view.to_numpy()]
    widths = [
        max(len(str(col)), *(len(row[idx]) for row in rows)) if rows else len(str(col))
        for idx, col in enumerate(view.columns)
    ]
    header = "| " + " | ".join(str(col).ljust(widths[idx]) for idx, col in enumerate(view.columns)) + " |"
    sep = "| " + " | ".join("-" * widths[idx] for idx in range(len(widths))) + " |"
    body = [
        "| " + " | ".join(row[idx].ljust(widths[idx]) for idx in range(len(widths))) + " |"
        for row in rows
    ]
    return "\n".join([header, sep] + body)


def _write_report(
    *,
    split_rows: pd.DataFrame,
    candidate_forward: pd.DataFrame,
    selected_summary: pd.DataFrame,
    candidate_summary: pd.DataFrame,
    diagnostics: dict[str, Any],
    output_path: Path,
) -> None:
    lines: list[str] = []
    lines.append("# Breakout Structure Walk-Forward Validation")
    lines.append("")
    lines.append(f"Generated: {pd.Timestamp.utcnow().isoformat()}")
    lines.append("")
    lines.append(
        "Scope: research-only, ETHUSDT 1h, current production shape `restrictive_0p5bps`, "
        "model-advance events only. Thresholds are calibrated on trailing train windows."
    )
    lines.append("")
    lines.append("## Selected-Gate Aggregate")
    lines.append("")
    lines.append(
        _markdown_table(
            selected_summary,
            [
                "gate",
                "forward_months",
                "trade_count",
                "same_close_calendar_sum",
                "same_close_worst_month",
                "same_close_neg_months",
                "adverse10_calendar_sum",
                "adverse10_worst_month",
                "adverse10_neg_months",
            ],
        )
    )
    lines.append("")
    lines.append("## Candidate Forward Aggregate")
    lines.append("")
    lines.append(
        _markdown_table(
            candidate_summary,
            [
                "gate",
                "forward_months",
                "trade_count",
                "same_close_calendar_sum",
                "same_close_worst_month",
                "same_close_neg_months",
                "adverse10_calendar_sum",
                "adverse10_worst_month",
                "adverse10_neg_months",
            ],
        )
    )
    lines.append("")
    lines.append("## Split Decisions")
    lines.append("")
    lines.append(
        _markdown_table(
            split_rows,
            [
                "forward_month",
                "selected_gate",
                "selected_conditions",
                "train_calendar_sum",
                "train_worst_sm",
                "train_trade_count",
                "same_close_calendar_sum",
                "adverse10_calendar_sum",
                "trade_count",
            ],
        )
    )
    lines.append("")
    lines.append("## Interpretation")
    lines.append("")
    lines.append("- `Candidate Forward Aggregate` applies each gate family with train-calibrated thresholds to every forward month.")
    lines.append("- `Selected-Gate Aggregate` simulates a realistic selector: choose the best positive train gate, then trade only the next month.")
    lines.append("- This is stricter than the prior in-sample quality-gate sweep; a gate that survives here is worth model/retrain work, not immediate live promotion.")
    lines.append("")
    lines.append("## Diagnostics")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str))
    lines.append("```")
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(train_months: int = 3, min_train_trades: int = 20) -> None:
    start = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    events, _trades = _load_events_and_trades()
    logger.info("loaded model-advance events=%d", len(events))

    bars_1s = _load_all_1s_bars("ETHUSDT")
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)

    original_exec_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_exec_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_exec_params)

    split_rows: list[dict[str, Any]] = []
    train_candidate_rows: list[dict[str, Any]] = []
    candidate_forward_rows: list[dict[str, Any]] = []
    selected_forward_rows: list[dict[str, Any]] = []
    selected_trades_parts: list[pd.DataFrame] = []

    months = _month_range("2025-06-01", "2026-05-01")
    forward_months = months[train_months:]

    try:
        for forward_start in forward_months:
            train_start = forward_start - pd.DateOffset(months=train_months)
            forward_end = forward_start + pd.DateOffset(months=1)
            forward_month = forward_start.strftime("%Y-%m")
            logger.info("split forward=%s train=%s..%s", forward_month, train_start.date(), forward_start.date())

            train_events = events[
                (events["touch_time"] >= train_start)
                & (events["touch_time"] < forward_start)
            ].copy()
            forward_events = events[
                (events["touch_time"] >= forward_start)
                & (events["touch_time"] < forward_end)
            ].copy()
            if train_events.empty or forward_events.empty:
                continue

            materialized = [_materialize_gate(spec, train_events) for spec in GATE_SPECS]
            train_rows_for_split: list[dict[str, Any]] = []
            for gate in materialized:
                gated_train = _apply_materialized_gate(train_events, gate)
                train_metrics = _monthly_metrics_from_pnl(gated_train)
                train_row = {
                    "forward_month": forward_month,
                    "gate": gate.spec_name,
                    "conditions": _conditions_text(gate),
                    "train_calendar_sum": train_metrics["calendar_sum"],
                    "train_worst_sm": train_metrics["worst_sm"],
                    "train_neg_sm": train_metrics["neg_sm"],
                    "train_trade_count": train_metrics["trade_count"],
                    "train_avg_pnl": train_metrics["avg_pnl"],
                }
                train_rows_for_split.append(train_row)
                train_candidate_rows.append(train_row)

            selected_train_row = _select_gate(train_rows_for_split, min_train_trades)
            selected_gate = next(gate for gate in materialized if gate.spec_name == selected_train_row["gate"])

            for gate in materialized:
                gated_forward = _apply_materialized_gate(forward_events, gate)
                forward_row, _ = _evaluate_forward_events(
                    gate_name=gate.spec_name,
                    forward_month=forward_month,
                    events=gated_forward,
                    bars_cache=bars_cache,
                )
                forward_row["conditions"] = _conditions_text(gate)
                candidate_forward_rows.append(forward_row)
                if gate.spec_name == selected_gate.spec_name:
                    selected_forward_rows.append(forward_row)

            selected_events = _apply_materialized_gate(forward_events, selected_gate)
            selected_eval, selected_trades = _evaluate_forward_events(
                gate_name=selected_gate.spec_name,
                forward_month=forward_month,
                events=selected_events,
                bars_cache=bars_cache,
            )
            if not selected_trades.empty:
                selected_trades_parts.append(selected_trades)

            split_rows.append(
                {
                    "forward_month": forward_month,
                    "train_start": train_start.strftime("%Y-%m"),
                    "train_end_exclusive": forward_start.strftime("%Y-%m"),
                    "selected_gate": selected_gate.spec_name,
                    "selected_conditions": _conditions_text(selected_gate),
                    "train_calendar_sum": selected_train_row["train_calendar_sum"],
                    "train_worst_sm": selected_train_row["train_worst_sm"],
                    "train_trade_count": selected_train_row["train_trade_count"],
                    **{
                        key: value
                        for key, value in selected_eval.items()
                        if key not in {"forward_month", "gate"}
                    },
                }
            )
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(original_exec_params)

    split_df = pd.DataFrame(split_rows)
    train_candidate_df = pd.DataFrame(train_candidate_rows)
    candidate_forward_df = pd.DataFrame(candidate_forward_rows)
    selected_forward_df = pd.DataFrame(selected_forward_rows)
    selected_summary = _aggregate_forward(
        [{**row, "gate": "walkforward_selected"} for row in selected_forward_rows]
    )
    candidate_summary = _aggregate_forward(candidate_forward_rows)
    selected_trades = pd.concat(selected_trades_parts, ignore_index=True) if selected_trades_parts else pd.DataFrame()

    diagnostics: dict[str, Any] = {
        "base_events_path": str(BASE_EVENTS_PATH),
        "base_trades_path": str(BASE_TRADES_PATH),
        "events": len(events),
        "train_months": train_months,
        "min_train_trades": min_train_trades,
        "forward_months": [month.strftime("%Y-%m") for month in forward_months],
        "base_share": BASE_SHARE,
        "exec_params": {**original_exec_params, **replay_exec_params},
        "runtime_seconds": time.time() - start,
    }

    run_tag = f"train{train_months}m_min{min_train_trades}"
    split_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_splits.csv"
    train_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_train_candidates.csv"
    candidate_forward_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_candidate_forward.csv"
    selected_forward_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_selected_forward.csv"
    selected_summary_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_selected_summary.csv"
    candidate_summary_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_candidate_summary.csv"
    selected_trades_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_selected_trades.csv"
    diagnostics_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_diagnostics.json"
    report_path = OUTPUT_DIR / f"breakout_structure_walkforward_{run_tag}_report.md"

    split_df.to_csv(split_path, index=False)
    train_candidate_df.to_csv(train_path, index=False)
    candidate_forward_df.to_csv(candidate_forward_path, index=False)
    selected_forward_df.to_csv(selected_forward_path, index=False)
    selected_summary.to_csv(selected_summary_path, index=False)
    candidate_summary.to_csv(candidate_summary_path, index=False)
    if not selected_trades.empty:
        selected_trades.to_csv(selected_trades_path, index=False)
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(
        split_rows=split_df,
        candidate_forward=candidate_forward_df,
        selected_summary=selected_summary,
        candidate_summary=candidate_summary,
        diagnostics=diagnostics,
        output_path=report_path,
    )

    logger.info("written %s", report_path)
    logger.info("written %s", selected_summary_path)
    logger.info("written %s", candidate_summary_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="Breakout structure quality walk-forward validation")
    parser.add_argument("--train-months", type=int, default=3)
    parser.add_argument("--min-train-trades", type=int, default=20)
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(train_months=args.train_months, min_train_trades=args.min_train_trades)


if __name__ == "__main__":
    main()
