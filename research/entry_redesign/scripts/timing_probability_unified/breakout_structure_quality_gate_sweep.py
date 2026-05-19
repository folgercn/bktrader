"""Breakout structure quality-gate sweep for ETH pretouch timing lead.

Research-only. This script tests a small set of interpretable structure-quality
filters discovered after the pure shape and post-touch confirmation sweeps. The
filters are intentionally reported as mined candidates, not live defaults.
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
    _neg_sm_count,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    compute_calendar_sum,
    compute_worst_sm,
)

logger = logging.getLogger(__name__)

BASE_EVENTS_PATH = OUTPUT_DIR / "breakout_shape_expansion_events_restrictive_0p5bps.csv"


@dataclass(frozen=True)
class QualityGate:
    name: str
    description: str
    conditions: tuple[tuple[str, str, float], ...]


QUALITY_GATES: list[QualityGate] = [
    QualityGate(
        name="baseline_touch_entry",
        description="current production-shape D0 lens; no extra structure-quality gate",
        conditions=(),
    ),
    QualityGate(
        name="low_eff_le_q20",
        description="pre-touch 300s efficiency in the lowest 20% of model-advance events",
        conditions=(("eff_300s", "<=", 0.7699153778815291),),
    ),
    QualityGate(
        name="low_eff_low_atr_pct",
        description="low 300s efficiency plus low 24h ATR percentile",
        conditions=(
            ("eff_300s", "<=", 0.7699153778815291),
            ("signal_atr_percentile", "<=", 0.2916666666666667),
        ),
    ),
    QualityGate(
        name="low_rf_slope_up",
        description="lower RF probability but closed-bar SMA5 slope already positive",
        conditions=(
            ("rf_probability", "<=", 0.58),
            ("prev_sma5_slope_atr", ">=", 0.0478745222904068),
        ),
    ),
    QualityGate(
        name="level_far_sma_gap_up",
        description="breakout level far from signal open and previous close above SMA5 for the side",
        conditions=(
            ("level_to_signal_open_atr", ">=", 0.4400237847225935),
            ("prev_sma5_gap_atr", ">=", 0.3480695487355221),
        ),
    ),
    QualityGate(
        name="wick_touch_ext_le_0",
        description="first touch is wick-led; touch-second close has not extended beyond the level",
        conditions=(("touch_extension_atr", "<=", 0.0),),
    ),
    QualityGate(
        name="wick_late",
        description="wick-led first touch and not too early in the signal bar",
        conditions=(
            ("touch_extension_atr", "<=", 0.0),
            ("pre_touch_seconds", ">=", 598.0),
        ),
    ),
]


def _load_base_events(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(
            f"{path} does not exist; run breakout_shape_expansion.py first"
        )
    events = pd.read_csv(path)
    for column in ("signal_start", "signal_end", "touch_time"):
        if column in events.columns:
            events[column] = pd.to_datetime(events[column], utc=True)
    events = events[(events["touch_time"] >= EVAL_START) & (events["touch_time"] < EVAL_END)].copy()
    return events.reset_index(drop=True)


def _apply_gate(events: pd.DataFrame, gate: QualityGate) -> pd.DataFrame:
    if not gate.conditions:
        return events.copy().reset_index(drop=True)
    mask = np.ones(len(events), dtype=bool)
    for column, op, value in gate.conditions:
        series = pd.to_numeric(events[column], errors="coerce")
        if op == "<=":
            mask &= (series <= value).fillna(False).to_numpy()
        elif op == ">=":
            mask &= (series >= value).fillna(False).to_numpy()
        else:
            raise ValueError(f"unsupported op: {op}")
    return events[mask].copy().reset_index(drop=True)


def _summarize_months(trades: pd.DataFrame) -> tuple[float, float, int, dict[str, float], float, float]:
    if trades.empty:
        return 0.0, 0.0, 0, {}, 0.0, 0.0
    gate_on = trades[trades["speed_gate_pass"] == True].copy()  # noqa: E712
    if gate_on.empty:
        return 0.0, 0.0, 0, {}, 0.0, 0.0
    gate_on["touch_time"] = pd.to_datetime(gate_on["touch_time"], utc=True)
    gate_on["year_month"] = gate_on["touch_time"].dt.strftime("%Y-%m")
    monthly = gate_on.groupby("year_month")["weighted_pnl"].sum()
    pre_2026 = gate_on[gate_on["touch_time"] < pd.Timestamp("2026-01-01", tz="UTC")]["weighted_pnl"].sum()
    post_2026 = gate_on[gate_on["touch_time"] >= pd.Timestamp("2026-01-01", tz="UTC")]["weighted_pnl"].sum()
    return (
        float(monthly.sum()),
        float(monthly.min()) if len(monthly) else 0.0,
        int((monthly < 0).sum()),
        {str(k): float(v) for k, v in monthly.items()},
        float(pre_2026),
        float(post_2026),
    )


def _summarize_gate(
    gate: QualityGate,
    events: pd.DataFrame,
    matrix: pd.DataFrame,
    trades: pd.DataFrame,
) -> dict[str, Any]:
    same_sum, same_worst, same_neg, _, pre_2026_sum, post_2026_sum = _summarize_months(trades)
    row: dict[str, Any] = {
        "variant": gate.name,
        "description": gate.description,
        "conditions": " & ".join(f"{c} {op} {v:.6f}" for c, op, v in gate.conditions) or "none",
        "quality_events": len(events),
        "model_advance_events": int((events["timing_prediction"] != "skip").sum()) if not events.empty else 0,
        "d0_traded_events": int((trades["selected_delay"] != "none").sum()) if not trades.empty else 0,
        "same_close_calendar_sum": same_sum,
        "same_close_worst_sm": same_worst,
        "same_close_neg_sm": same_neg,
        "same_close_pre_2026_sum": pre_2026_sum,
        "same_close_2026_sum": post_2026_sum,
    }
    if not matrix.empty:
        for scenario in ("next_adverse_xslip0bps", "next_adverse_xslip10bps"):
            view = matrix[matrix["scenario"] == scenario]
            if view.empty:
                continue
            prefix = "adverse0" if scenario == "next_adverse_xslip0bps" else "adverse10"
            row[f"{prefix}_calendar_sum"] = float(view.iloc[0]["calendar_sum_gate_on"])
            row[f"{prefix}_worst_sm"] = float(view.iloc[0]["worst_sm_gate_on"])
            row[f"{prefix}_neg_sm"] = int(view.iloc[0]["neg_sm_count"])
    return row


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
    summary: pd.DataFrame,
    adverse: pd.DataFrame,
    monthly: pd.DataFrame,
    diagnostics: dict[str, Any],
    output_path: Path,
) -> None:
    lines: list[str] = []
    lines.append("# Breakout Structure Quality-Gate Sweep — ETH Pretouch Timing Lead")
    lines.append("")
    lines.append(f"Generated: {pd.Timestamp.utcnow().isoformat()}")
    lines.append("")
    lines.append(
        "Scope: research-only, ETHUSDT 1h, current production shape "
        "`restrictive_0p5bps`, frozen `data/pretouch_model.json` `20260515_v1`."
    )
    lines.append(
        "These gates are mined candidates from the broad live-like event pool. "
        "They are evidence for the next validation step, not live defaults."
    )
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    summary_cols = [
        "variant",
        "quality_events",
        "model_advance_events",
        "d0_traded_events",
        "same_close_calendar_sum",
        "same_close_worst_sm",
        "same_close_neg_sm",
        "same_close_pre_2026_sum",
        "same_close_2026_sum",
        "adverse10_calendar_sum",
        "adverse10_worst_sm",
        "adverse10_neg_sm",
    ]
    lines.append(_markdown_table(summary, summary_cols))
    lines.append("")
    lines.append("## Gate Conditions")
    lines.append("")
    lines.append(_markdown_table(summary, ["variant", "conditions", "description"]))
    lines.append("")
    lines.append("## Adverse Fill Matrix")
    lines.append("")
    adverse_cols = [
        "variant",
        "scenario",
        "calendar_sum_gate_on",
        "worst_sm_gate_on",
        "neg_sm_count",
        "trade_count_gate_on",
    ]
    lines.append(_markdown_table(adverse, adverse_cols))
    lines.append("")
    lines.append("## Monthly PnL")
    lines.append("")
    if monthly.empty:
        lines.append("_empty_")
    else:
        pivot = (
            monthly.pivot_table(
                index="year_month",
                columns="variant",
                values="weighted_pnl",
                aggfunc="sum",
                fill_value=0.0,
            )
            .reset_index()
            .rename_axis(None, axis=1)
        )
        lines.append(_markdown_table(pivot, list(pivot.columns)))
    lines.append("")
    lines.append("## Interpretation")
    lines.append("")
    lines.append("- Pure shape expansion and post-touch confirmation did not turn the broad live-like pool positive.")
    lines.append("- Several structure-quality gates are positive even under next-second adverse 10bps stress, but the sample is small and thresholds were mined in-sample.")
    lines.append("- The most interesting next step is walk-forward validation/retraining around these features, not a live default change.")
    lines.append("")
    lines.append("## Diagnostics")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str))
    lines.append("```")
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run() -> None:
    start = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    base_events = _load_base_events(BASE_EVENTS_PATH)
    model_advance_events = base_events[base_events["timing_prediction"] != "skip"].copy().reset_index(drop=True)
    logger.info("loaded quality events=%d model-advance=%d", len(base_events), len(model_advance_events))

    bars_1s = _load_all_1s_bars("ETHUSDT")
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)

    original_exec_params = deepcopy(DEFAULT_EXEC_PARAMS)
    DEFAULT_EXEC_PARAMS.update(
        {
            "initial_stop_atr": 0.45,
            "breakeven_at_r": 0.8,
            "trail_start_r": 1.5,
            "trail_buffer_atr": 0.05,
            "max_hold_hours": 2.0,
        }
    )

    summary_rows: list[dict[str, Any]] = []
    adverse_rows: list[pd.DataFrame] = []
    monthly_rows: list[dict[str, Any]] = []
    diagnostics: dict[str, Any] = {
        "base_events_path": str(BASE_EVENTS_PATH),
        "quality_events": len(base_events),
        "model_advance_events": len(model_advance_events),
        "eval_start": EVAL_START.isoformat(),
        "eval_end_exclusive": EVAL_END.isoformat(),
        "base_share": BASE_SHARE,
        "exec_params": {k: DEFAULT_EXEC_PARAMS[k] for k in sorted(DEFAULT_EXEC_PARAMS)},
        "variants": {},
    }

    try:
        for gate in QUALITY_GATES:
            logger.info("=" * 72)
            logger.info("gate %s", gate.name)
            gated_events = _apply_gate(model_advance_events, gate)
            logger.info("%s events=%d", gate.name, len(gated_events))
            matrix, trades = _evaluate_events(gated_events, bars_cache)
            if not matrix.empty:
                matrix.insert(0, "variant", gate.name)
                adverse_rows.append(matrix)
            if not trades.empty:
                trades["structure_quality_gate"] = gate.name
                trades.to_csv(
                    OUTPUT_DIR / f"breakout_structure_quality_trades_{gate.name}.csv",
                    index=False,
                )
            gated_events.to_csv(
                OUTPUT_DIR / f"breakout_structure_quality_events_{gate.name}.csv",
                index=False,
            )
            _, _, _, monthly, _, _ = _summarize_months(trades)
            for month, weighted_pnl in monthly.items():
                monthly_rows.append(
                    {
                        "variant": gate.name,
                        "year_month": month,
                        "weighted_pnl": weighted_pnl,
                    }
                )
            summary_rows.append(_summarize_gate(gate, gated_events, matrix, trades))
            diagnostics["variants"][gate.name] = {
                "conditions": gate.conditions,
                "events": len(gated_events),
                "description": gate.description,
            }
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(original_exec_params)

    summary = pd.DataFrame(summary_rows)
    adverse = pd.concat(adverse_rows, ignore_index=True) if adverse_rows else pd.DataFrame()
    monthly = pd.DataFrame(monthly_rows)

    summary_path = OUTPUT_DIR / "breakout_structure_quality_summary.csv"
    adverse_path = OUTPUT_DIR / "breakout_structure_quality_adverse_matrix.csv"
    monthly_path = OUTPUT_DIR / "breakout_structure_quality_monthly.csv"
    diagnostics_path = OUTPUT_DIR / "breakout_structure_quality_diagnostics.json"
    report_path = OUTPUT_DIR / "breakout_structure_quality_report.md"

    summary.to_csv(summary_path, index=False)
    adverse.to_csv(adverse_path, index=False)
    monthly.to_csv(monthly_path, index=False)
    diagnostics["runtime_seconds"] = time.time() - start
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, adverse, monthly, diagnostics, report_path)

    logger.info("written %s", summary_path)
    logger.info("written %s", adverse_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="ETH pretouch breakout structure quality-gate sweep")
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run()


if __name__ == "__main__":
    main()
