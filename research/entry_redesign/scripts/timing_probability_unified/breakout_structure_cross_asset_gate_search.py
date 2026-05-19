"""Walk-forward gate search on cross-asset current-shape events.

Research-only. This searches for symbol-local structure-quality gates after
the ETH `low_eff_low_atr` gate failed to become a positive BTC signal. Gates
are intentionally small and interpretable; thresholds are calibrated only on a
trailing train window, then evaluated on the next calendar month.
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
    BARS_CACHE_DIR,
    EVAL_END,
    EVAL_START,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _evaluate_events,
)
from timing_probability_unified.breakout_structure_cross_asset_validation import (  # noqa: E402
    _load_symbol_1s_bars,
    _markdown_table,
    _output_tag,
)

logger = logging.getLogger(__name__)


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


GATE_SPECS: list[GateSpec] = [
    GateSpec("baseline_model_advance", "no extra gate", ()),
    GateSpec("low_eff_q20", "lowest 20% 300s efficiency", (GateCondition("eff_300s", "<=", quantile=0.20),)),
    GateSpec("low_eff_q30", "lowest 30% 300s efficiency", (GateCondition("eff_300s", "<=", quantile=0.30),)),
    GateSpec("low_atr_q40", "lowest 40% 24h ATR percentile", (GateCondition("signal_atr_percentile", "<=", quantile=0.40),)),
    GateSpec(
        "low_eff_low_atr_q20_q40",
        "lowest 20% efficiency plus lowest 40% ATR percentile",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("signal_atr_percentile", "<=", quantile=0.40),
        ),
    ),
    GateSpec(
        "low_eff_low_atr_q30_q50",
        "lowest 30% efficiency plus lowest 50% ATR percentile",
        (
            GateCondition("eff_300s", "<=", quantile=0.30),
            GateCondition("signal_atr_percentile", "<=", quantile=0.50),
        ),
    ),
    GateSpec("high_speed_q60", "top 40% side-normalized 300s speed", (GateCondition("speed_300s_atr", ">=", quantile=0.60),)),
    GateSpec("high_speed_q80", "top 20% side-normalized 300s speed", (GateCondition("speed_300s_atr", ">=", quantile=0.80),)),
    GateSpec(
        "low_eff_high_speed_q20_q60",
        "low efficiency plus high side-normalized speed",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("speed_300s_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec("low_rf_q40", "lower RF probability", (GateCondition("rf_probability", "<=", quantile=0.40),)),
    GateSpec("high_rf_q60", "higher RF probability", (GateCondition("rf_probability", ">=", quantile=0.60),)),
    GateSpec("level_far_q60", "level far from signal open", (GateCondition("level_to_signal_open_atr", ">=", quantile=0.60),)),
    GateSpec("level_near_q40", "level near signal open", (GateCondition("level_to_signal_open_atr", "<=", quantile=0.40),)),
    GateSpec("wick_touch_ext_le_0", "touch-second close has not extended beyond level", (GateCondition("touch_extension_atr", "<=", value=0.0),)),
    GateSpec(
        "late_touch_q40",
        "not too early in signal bar",
        (GateCondition("pre_touch_seconds", ">=", quantile=0.40),),
    ),
    GateSpec(
        "side_sma_slope_up_q60",
        "closed-bar SMA5 slope aligns with trade side",
        (GateCondition("side_sma5_slope_atr", ">=", quantile=0.60),),
    ),
    GateSpec(
        "side_sma_gap_up_q60",
        "previous close is on the favorable side of SMA5",
        (GateCondition("side_sma5_gap_atr", ">=", quantile=0.60),),
    ),
    GateSpec(
        "low_eff_side_slope_q20_q60",
        "low efficiency plus favorable SMA5 slope",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("side_sma5_slope_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec(
        "low_atr_side_gap_q40_q60",
        "low ATR percentile plus favorable SMA5 gap",
        (
            GateCondition("signal_atr_percentile", "<=", quantile=0.40),
            GateCondition("side_sma5_gap_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec("ctx4h_side_up_q60", "prior 4h return aligns with side", (GateCondition("ctx4h_side_return_atr", ">=", quantile=0.60),)),
    GateSpec("ctx12h_side_up_q60", "prior 12h return aligns with side", (GateCondition("ctx12h_side_return_atr", ">=", quantile=0.60),)),
    GateSpec(
        "low_eff_ctx4h_side_up_q20_q60",
        "low efficiency plus favorable prior 4h return",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("ctx4h_side_return_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec(
        "low_eff_ctx12h_side_up_q20_q60",
        "low efficiency plus favorable prior 12h return",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("ctx12h_side_return_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec(
        "low_eff_low_atr_ctx4h_up",
        "low efficiency plus low ATR plus favorable prior 4h return",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("signal_atr_percentile", "<=", quantile=0.40),
            GateCondition("ctx4h_side_return_atr", ">=", quantile=0.60),
        ),
    ),
    GateSpec(
        "low_eff_low_atr_ctx12h_up",
        "low efficiency plus low ATR plus favorable prior 12h return",
        (
            GateCondition("eff_300s", "<=", quantile=0.20),
            GateCondition("signal_atr_percentile", "<=", quantile=0.40),
            GateCondition("ctx12h_side_return_atr", ">=", quantile=0.60),
        ),
    ),
]


def _month_range(start: str, end_exclusive: str) -> list[pd.Timestamp]:
    end = pd.Timestamp(end_exclusive, tz="UTC")
    return [
        month
        for month in pd.date_range(pd.Timestamp(start, tz="UTC"), end, freq="MS")
        if month < end
    ]


def _load_events(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    events = pd.read_csv(path)
    for column in ("touch_time", "signal_start", "signal_end"):
        if column in events.columns:
            events[column] = pd.to_datetime(events[column], utc=True)
    if "side_sma5_slope_atr" not in events.columns:
        side_sign = np.where(events["side"].astype(str) == "short", -1.0, 1.0)
        events["side_sma5_slope_atr"] = pd.to_numeric(events["prev_sma5_slope_atr"], errors="coerce") * side_sign
        events["side_sma5_gap_atr"] = pd.to_numeric(events["prev_sma5_gap_atr"], errors="coerce") * side_sign
    return events.reset_index(drop=True)


def _add_context_features(events: pd.DataFrame, bars_1s: pd.DataFrame) -> pd.DataFrame:
    if events.empty:
        return events.copy()
    bars_1h = (
        bars_1s.resample("1h")
        .agg({"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"})
        .dropna(subset=["open"])
    )
    index = bars_1h.index
    close = bars_1h["close"].to_numpy(dtype="float64", copy=False)
    high = bars_1h["high"].to_numpy(dtype="float64", copy=False)
    low = bars_1h["low"].to_numpy(dtype="float64", copy=False)

    ctx4_return: list[float] = []
    ctx12_return: list[float] = []
    ctx4_range: list[float] = []
    ctx12_range: list[float] = []
    for row in events.itertuples(index=False):
        signal_start = pd.Timestamp(getattr(row, "signal_start"))
        if signal_start.tzinfo is None:
            signal_start = signal_start.tz_localize("UTC")
        else:
            signal_start = signal_start.tz_convert("UTC")
        pos = int(index.searchsorted(signal_start, side="left"))
        last = pos - 1
        side_sign = -1.0 if str(getattr(row, "side")) == "short" else 1.0
        atr = float(getattr(row, "atr", np.nan))
        if last < 0 or not np.isfinite(atr) or atr <= 0:
            ctx4_return.append(np.nan)
            ctx12_return.append(np.nan)
            ctx4_range.append(np.nan)
            ctx12_range.append(np.nan)
            continue

        def ret(hours: int) -> float:
            start = last - hours
            if start < 0:
                return np.nan
            return float((close[last] - close[start]) / atr * side_sign)

        def rng(hours: int) -> float:
            start = last - hours + 1
            if start < 0:
                return np.nan
            return float((np.max(high[start : last + 1]) - np.min(low[start : last + 1])) / atr)

        ctx4_return.append(ret(4))
        ctx12_return.append(ret(12))
        ctx4_range.append(rng(4))
        ctx12_range.append(rng(12))

    out = events.copy()
    out["ctx4h_side_return_atr"] = ctx4_return
    out["ctx12h_side_return_atr"] = ctx12_return
    out["ctx4h_range_atr"] = ctx4_range
    out["ctx12h_range_atr"] = ctx12_range
    return out


def _materialize_gate(spec: GateSpec, train_events: pd.DataFrame) -> tuple[tuple[str, str, float], ...]:
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
    return tuple(conditions)


def _apply_conditions(events: pd.DataFrame, conditions: tuple[tuple[str, str, float], ...]) -> pd.DataFrame:
    if not conditions:
        return events.copy().reset_index(drop=True)
    mask = np.ones(len(events), dtype=bool)
    for column, op, value in conditions:
        if not np.isfinite(value):
            mask &= False
            continue
        series = pd.to_numeric(events[column], errors="coerce")
        if op == "<=":
            mask &= (series <= value).fillna(False).to_numpy()
        elif op == ">=":
            mask &= (series >= value).fillna(False).to_numpy()
        else:
            raise ValueError(f"unsupported op: {op}")
    return events[mask].copy().reset_index(drop=True)


def _conditions_text(conditions: tuple[tuple[str, str, float], ...]) -> str:
    return " & ".join(f"{column} {op} {value:.12g}" for column, op, value in conditions) or "none"


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


def _evaluate_events_row(
    *,
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> tuple[dict[str, Any], pd.DataFrame]:
    matrix, trades = _evaluate_events(events, bars_cache)
    same = _matrix_metrics(matrix, "same_close_xslip0bps")
    adverse10 = _matrix_metrics(matrix, "next_adverse_xslip10bps")
    return {
        "events": int(len(events)),
        "trade_count": same["trade_count"],
        "same_close_calendar_sum": same["calendar_sum"],
        "same_close_worst_sm": same["worst_sm"],
        "same_close_neg_sm": same["neg_sm"],
        "adverse10_calendar_sum": adverse10["calendar_sum"],
        "adverse10_worst_sm": adverse10["worst_sm"],
        "adverse10_neg_sm": adverse10["neg_sm"],
    }, trades


def _aggregate(rows: list[dict[str, Any]], gate_column: str = "gate") -> pd.DataFrame:
    if not rows:
        return pd.DataFrame()
    df = pd.DataFrame(rows)
    out_rows: list[dict[str, Any]] = []
    for gate, group in df.groupby(gate_column):
        out_rows.append(
            {
                "gate": gate,
                "forward_months": int(group["forward_month"].nunique()),
                "forward_events": int(group["forward_events"].sum()),
                "trade_count": int(group["trade_count"].sum()),
                "same_close_calendar_sum": float(group["same_close_calendar_sum"].sum()),
                "same_close_worst_month": float(group["same_close_calendar_sum"].min()),
                "same_close_neg_months": int((group["same_close_calendar_sum"] < 0).sum()),
                "adverse10_calendar_sum": float(group["adverse10_calendar_sum"].sum()),
                "adverse10_worst_month": float(group["adverse10_calendar_sum"].min()),
                "adverse10_neg_months": int((group["adverse10_calendar_sum"] < 0).sum()),
            }
        )
    return pd.DataFrame(out_rows).sort_values(
        ["adverse10_calendar_sum", "same_close_calendar_sum"],
        ascending=[False, False],
    )


def _select_gate(train_rows: list[dict[str, Any]], min_train_trades: int) -> dict[str, Any]:
    eligible = [
        row
        for row in train_rows
        if row["gate"] != "baseline_model_advance"
        and row["train_trade_count"] >= min_train_trades
        and row["train_adverse10_calendar_sum"] > 0.0
    ]
    if not eligible:
        baseline = [row for row in train_rows if row["gate"] == "baseline_model_advance"]
        return baseline[0] if baseline else max(train_rows, key=lambda row: row["train_trade_count"])
    return max(
        eligible,
        key=lambda row: (
            row["train_adverse10_calendar_sum"],
            row["train_adverse10_worst_sm"],
            row["train_trade_count"],
        ),
    )


def _write_report(
    *,
    symbol: str,
    candidate_summary: pd.DataFrame,
    selected_summary: pd.DataFrame,
    split_rows: pd.DataFrame,
    diagnostics: dict[str, Any],
    output_path: Path,
) -> None:
    lines = [
        f"# Breakout Structure Cross-Asset Gate Search — {symbol}",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. All thresholds are trailing-window quantiles; forward rows are out-of-sample by month.",
        "",
        "## Candidate Aggregate",
        "",
        _markdown_table(
            candidate_summary,
            [
                "gate",
                "forward_months",
                "forward_events",
                "trade_count",
                "same_close_calendar_sum",
                "same_close_worst_month",
                "same_close_neg_months",
                "adverse10_calendar_sum",
                "adverse10_worst_month",
                "adverse10_neg_months",
            ],
        ),
        "",
        "## Selected Aggregate",
        "",
        _markdown_table(
            selected_summary,
            [
                "gate",
                "forward_months",
                "forward_events",
                "trade_count",
                "same_close_calendar_sum",
                "same_close_worst_month",
                "same_close_neg_months",
                "adverse10_calendar_sum",
                "adverse10_worst_month",
                "adverse10_neg_months",
            ],
        ),
        "",
        "## Split Decisions",
        "",
        _markdown_table(
            split_rows,
            [
                "forward_month",
                "selected_gate",
                "selected_conditions",
                "train_adverse10_calendar_sum",
                "train_trade_count",
                "same_close_calendar_sum",
                "adverse10_calendar_sum",
                "trade_count",
            ],
        ),
        "",
        "## Interpretation",
        "",
        "- Candidate aggregate applies every gate family to every forward month.",
        "- Selected aggregate chooses the best positive train gate each month; if no eligible gate is positive it falls back to baseline.",
        "- Promotion needs positive adverse10, acceptable worst month, and non-trivial trade count without relying on the selected fallback.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(
    symbol: str,
    train_months: int,
    min_train_trades: int,
    bars_cache_dir: Path,
    eval_start: pd.Timestamp,
    eval_end: pd.Timestamp,
    events_csv: Path | None = None,
) -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    tag = _output_tag(symbol, train_months, eval_start, eval_end)
    if events_csv is None:
        events_csv = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_events.csv"
    events = _load_events(events_csv)
    events = events[(events["touch_time"] >= eval_start) & (events["touch_time"] < eval_end)].copy().reset_index(drop=True)

    bars_1s = _load_symbol_1s_bars(symbol, bars_cache_dir, eval_start, eval_end)
    events = _add_context_features(events, bars_1s)
    bars_cache = _bars_cache_for_symbol(symbol, bars_1s)
    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_params)

    months = _month_range(eval_start.strftime("%Y-%m-%d"), eval_end.strftime("%Y-%m-%d"))
    forward_months = months[train_months:]

    candidate_rows: list[dict[str, Any]] = []
    split_rows: list[dict[str, Any]] = []
    selected_forward_rows: list[dict[str, Any]] = []
    selected_trades_parts: list[pd.DataFrame] = []

    try:
        for forward_start in forward_months:
            train_start = forward_start - pd.DateOffset(months=train_months)
            forward_end = forward_start + pd.DateOffset(months=1)
            forward_month = forward_start.strftime("%Y-%m")
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

            materialized = [(spec, _materialize_gate(spec, train_events)) for spec in GATE_SPECS]
            train_rows: list[dict[str, Any]] = []
            forward_by_gate: dict[str, tuple[dict[str, Any], pd.DataFrame, tuple[tuple[str, str, float], ...]]] = {}
            for spec, conditions in materialized:
                gated_train = _apply_conditions(train_events, conditions)
                train_metrics, _ = _evaluate_events_row(events=gated_train, bars_cache=bars_cache)
                train_row = {
                    "forward_month": forward_month,
                    "gate": spec.name,
                    "conditions": _conditions_text(conditions),
                    "train_events": train_metrics["events"],
                    "train_trade_count": train_metrics["trade_count"],
                    "train_adverse10_calendar_sum": train_metrics["adverse10_calendar_sum"],
                    "train_adverse10_worst_sm": train_metrics["adverse10_worst_sm"],
                }
                train_rows.append(train_row)

                gated_forward = _apply_conditions(forward_events, conditions)
                forward_metrics, forward_trades = _evaluate_events_row(events=gated_forward, bars_cache=bars_cache)
                forward_row = {
                    "forward_month": forward_month,
                    "gate": spec.name,
                    "conditions": _conditions_text(conditions),
                    "forward_events": forward_metrics["events"],
                    "trade_count": forward_metrics["trade_count"],
                    "same_close_calendar_sum": forward_metrics["same_close_calendar_sum"],
                    "same_close_worst_sm": forward_metrics["same_close_worst_sm"],
                    "same_close_neg_sm": forward_metrics["same_close_neg_sm"],
                    "adverse10_calendar_sum": forward_metrics["adverse10_calendar_sum"],
                    "adverse10_worst_sm": forward_metrics["adverse10_worst_sm"],
                    "adverse10_neg_sm": forward_metrics["adverse10_neg_sm"],
                }
                candidate_rows.append(forward_row)
                forward_by_gate[spec.name] = (forward_row, forward_trades, conditions)

            selected_train = _select_gate(train_rows, min_train_trades)
            selected_forward, selected_trades, selected_conditions = forward_by_gate[selected_train["gate"]]
            selected_forward_rows.append({**selected_forward, "gate": "walkforward_selected"})
            split_rows.append(
                {
                    "forward_month": forward_month,
                    "selected_gate": selected_train["gate"],
                    "selected_conditions": _conditions_text(selected_conditions),
                    **{key: value for key, value in selected_train.items() if key.startswith("train_")},
                    **{key: value for key, value in selected_forward.items() if key not in {"forward_month", "gate", "conditions"}},
                }
            )
            if not selected_trades.empty:
                selected_trades = selected_trades.copy()
                selected_trades["selected_gate"] = selected_train["gate"]
                selected_trades["forward_month"] = forward_month
                selected_trades_parts.append(selected_trades)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    candidate_df = pd.DataFrame(candidate_rows)
    split_df = pd.DataFrame(split_rows)
    selected_df = pd.DataFrame(selected_forward_rows)
    candidate_summary = _aggregate(candidate_rows)
    selected_summary = _aggregate(selected_forward_rows)
    selected_trades = pd.concat(selected_trades_parts, ignore_index=True, sort=False) if selected_trades_parts else pd.DataFrame()

    out_tag = f"{tag}_min{min_train_trades}"
    candidate_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_candidates.csv"
    split_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_splits.csv"
    selected_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_selected.csv"
    candidate_summary_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_candidate_summary.csv"
    selected_summary_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_selected_summary.csv"
    selected_trades_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_selected_trades.csv"
    diagnostics_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_diagnostics.json"
    report_path = OUTPUT_DIR / f"breakout_structure_cross_asset_gate_search_{out_tag}_report.md"

    candidate_df.to_csv(candidate_path, index=False)
    split_df.to_csv(split_path, index=False)
    selected_df.to_csv(selected_path, index=False)
    candidate_summary.to_csv(candidate_summary_path, index=False)
    selected_summary.to_csv(selected_summary_path, index=False)
    if not selected_trades.empty:
        selected_trades.to_csv(selected_trades_path, index=False)

    diagnostics = {
        "symbol": symbol,
        "events_csv": str(events_csv),
        "bars_cache_dir": str(bars_cache_dir),
        "eval_start": eval_start.isoformat(),
        "eval_end_exclusive": eval_end.isoformat(),
        "train_months": train_months,
        "min_train_trades": min_train_trades,
        "events": int(len(events)),
        "exec_params": {**saved_params, **replay_params},
        "gate_specs": [
            {
                "name": spec.name,
                "description": spec.description,
                "conditions": [condition.__dict__ for condition in spec.conditions],
            }
            for spec in GATE_SPECS
        ],
        "outputs": {
            "candidate_rows_csv": str(candidate_path),
            "split_rows_csv": str(split_path),
            "selected_rows_csv": str(selected_path),
            "candidate_summary_csv": str(candidate_summary_path),
            "selected_summary_csv": str(selected_summary_path),
            "selected_trades_csv": str(selected_trades_path),
            "report_md": str(report_path),
        },
        "runtime_seconds": time.time() - started,
    }
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(
        symbol=symbol,
        candidate_summary=candidate_summary,
        selected_summary=selected_summary,
        split_rows=split_df,
        diagnostics=diagnostics,
        output_path=report_path,
    )
    logger.info("written %s", report_path)


def _utc_timestamp(text: str) -> pd.Timestamp:
    ts = pd.Timestamp(text)
    return ts.tz_localize("UTC") if ts.tzinfo is None else ts.tz_convert("UTC")


def main() -> None:
    parser = argparse.ArgumentParser(description="Cross-asset walk-forward gate search")
    parser.add_argument("--symbol", default="BTCUSDT", choices=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--train-months", type=int, default=3)
    parser.add_argument("--min-train-trades", type=int, default=10)
    parser.add_argument("--bars-cache-dir", type=Path, default=BARS_CACHE_DIR)
    parser.add_argument("--eval-start", default=EVAL_START.isoformat())
    parser.add_argument("--eval-end", default=EVAL_END.isoformat())
    parser.add_argument("--events-csv", type=Path, default=None)
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(
        symbol=args.symbol,
        train_months=args.train_months,
        min_train_trades=args.min_train_trades,
        bars_cache_dir=args.bars_cache_dir,
        eval_start=_utc_timestamp(args.eval_start),
        eval_end=_utc_timestamp(args.eval_end),
        events_csv=args.events_csv,
    )


if __name__ == "__main__":
    main()
