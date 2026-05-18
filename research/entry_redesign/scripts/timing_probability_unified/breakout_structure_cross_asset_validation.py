"""Cross-asset validation for the breakout structure-quality gate family.

Research-only. This script checks whether the `low_eff_low_atr_q20_q40`
structure family that improved the ETH pretouch timing candidate also has
out-of-sample signal on another symbol. It does not modify live defaults and
writes symbol-specific outputs so ETH artifacts are not overwritten.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified import breakout_shape_expansion as shape_expansion  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    CANONICAL_EVENTS_CSV,
    BARS_CACHE_DIR,
    EVAL_END,
    EVAL_START,
    MODEL_PATH,
    OUTPUT_DIR,
    ShapeVariant,
    _bars_cache_for_symbol,
    _evaluate_events,
    _load_all_1s_bars,
    _resample_1h,
    apply_frozen_model,
    detect_variant_events,
    live_quality_filter,
)

logger = logging.getLogger(__name__)

RESTRICTIVE_VARIANT = ShapeVariant(
    name="restrictive_0p5bps",
    mode="restrictive",
    bps=0.5,
    description="current production: prev2 must be separated by +0.5bps",
)
GATES = {
    "baseline_model_advance": (),
    "low_eff_low_atr_q20_q40": (
        ("eff_300s", "<=", 0.20),
        ("signal_atr_percentile", "<=", 0.40),
    ),
}


def _load_model() -> dict[str, Any]:
    with MODEL_PATH.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def _load_symbol_1s_bars(
    symbol: str,
    bars_cache_dir: Path,
    eval_start: pd.Timestamp,
    eval_end: pd.Timestamp,
) -> pd.DataFrame:
    files = sorted(bars_cache_dir.glob(f"{symbol}_*_1s.pkl"))
    if not files:
        raise FileNotFoundError(f"no 1s bars cache for {symbol} under {bars_cache_dir}")

    dfs = []
    for path in files:
        df = pd.read_pickle(path)
        if df.index.tz is None:
            df.index = df.index.tz_localize("UTC")
        else:
            df.index = df.index.tz_convert("UTC")
        dfs.append(df[["open", "high", "low", "close", "volume"]])
        logger.info("loaded %s rows=%d", path.name, len(df))

    bars = pd.concat(dfs).sort_index()
    bars = bars[~bars.index.duplicated(keep="first")]
    bars = bars[(bars.index >= eval_start - pd.Timedelta(hours=24)) & (bars.index < eval_end)]
    logger.info(
        "combined %s rows=%d range=%s..%s",
        symbol,
        len(bars),
        bars.index.min(),
        bars.index.max(),
    )
    return bars


def _month_range(start: str, end_exclusive: str) -> list[pd.Timestamp]:
    end = pd.Timestamp(end_exclusive, tz="UTC")
    return [
        month
        for month in pd.date_range(pd.Timestamp(start, tz="UTC"), end, freq="MS")
        if month < end
    ]


def _materialize_gate(gate: str, train_events: pd.DataFrame) -> tuple[tuple[str, str, float], ...]:
    conditions: list[tuple[str, str, float]] = []
    for column, op, quantile in GATES[gate]:
        series = pd.to_numeric(train_events[column], errors="coerce").dropna()
        value = float(series.quantile(quantile)) if not series.empty else np.nan
        conditions.append((column, op, value))
    return tuple(conditions)


def _conditions_text(conditions: tuple[tuple[str, str, float], ...]) -> str:
    return " & ".join(f"{column} {op} {value:.12g}" for column, op, value in conditions) or "none"


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


def _evaluate_forward(
    *,
    symbol: str,
    gate: str,
    forward_month: str,
    conditions: tuple[tuple[str, str, float], ...],
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> tuple[dict[str, Any], pd.DataFrame]:
    matrix, trades = _evaluate_events(events, bars_cache)
    same = _matrix_metrics(matrix, "same_close_xslip0bps")
    adverse10 = _matrix_metrics(matrix, "next_adverse_xslip10bps")
    row = {
        "symbol": symbol,
        "forward_month": forward_month,
        "gate": gate,
        "conditions": _conditions_text(conditions),
        "forward_events": int(len(events)),
        "same_close_calendar_sum": same["calendar_sum"],
        "same_close_worst_sm": same["worst_sm"],
        "same_close_neg_sm": same["neg_sm"],
        "adverse10_calendar_sum": adverse10["calendar_sum"],
        "adverse10_worst_sm": adverse10["worst_sm"],
        "adverse10_neg_sm": adverse10["neg_sm"],
        "trade_count": same["trade_count"],
    }
    if not trades.empty:
        trades = trades.copy()
        trades["gate"] = gate
        trades["forward_month"] = forward_month
    return row, trades


def _aggregate(rows: list[dict[str, Any]]) -> pd.DataFrame:
    if not rows:
        return pd.DataFrame()
    df = pd.DataFrame(rows)
    out_rows: list[dict[str, Any]] = []
    for gate, group in df.groupby("gate"):
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


def _canonical_coverage(symbol: str, events: pd.DataFrame, eval_start: pd.Timestamp, eval_end: pd.Timestamp) -> dict[str, Any]:
    if not CANONICAL_EVENTS_CSV.exists():
        return {"canonical_events": 0, "overlap_keys": 0}
    canonical = pd.read_csv(CANONICAL_EVENTS_CSV)
    canonical = canonical[canonical["symbol"] == symbol].copy()
    if canonical.empty:
        return {"canonical_events": 0, "overlap_keys": 0}
    canonical["touch_time"] = pd.to_datetime(canonical["touch_time"], utc=True)
    canonical = canonical[(canonical["touch_time"] >= eval_start) & (canonical["touch_time"] < eval_end)].copy()
    if canonical.empty:
        return {"canonical_events": 0, "overlap_keys": 0}
    canonical["signal_start"] = pd.to_datetime(canonical["signal_start"], utc=True)
    events = events.copy()
    events["signal_start"] = pd.to_datetime(events["signal_start"], utc=True)
    canonical_keys = set(zip(canonical["signal_start"].dt.strftime("%Y-%m-%dT%H:%M:%S%z"), canonical["side"].astype(str)))
    event_keys = set(zip(events["signal_start"].dt.strftime("%Y-%m-%dT%H:%M:%S%z"), events["side"].astype(str)))
    return {
        "canonical_events": int(len(canonical)),
        "canonical_start": pd.to_datetime(canonical["touch_time"], utc=True).min().isoformat(),
        "canonical_end": pd.to_datetime(canonical["touch_time"], utc=True).max().isoformat(),
        "current_shape_model_advance_events": int(len(events)),
        "overlap_keys": int(len(canonical_keys & event_keys)),
        "canonical_coverage_rate": float(len(canonical_keys & event_keys) / len(canonical_keys)) if canonical_keys else 0.0,
    }


def _write_report(
    *,
    symbol: str,
    summary: pd.DataFrame,
    split_rows: pd.DataFrame,
    diagnostics: dict[str, Any],
    report_path: Path,
) -> None:
    lines = [
        f"# Breakout Structure Cross-Asset Validation — {symbol}",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. This checks whether the ETH-derived structure family generalizes cross-asset; no live defaults are changed.",
        "",
        "## Aggregate",
        "",
        _markdown_table(
            summary,
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
        "## Split Rows",
        "",
        _markdown_table(
            split_rows,
            [
                "forward_month",
                "gate",
                "forward_events",
                "conditions",
                "same_close_calendar_sum",
                "adverse10_calendar_sum",
                "trade_count",
            ],
        ),
        "",
        "## Interpretation",
        "",
        "- `baseline_model_advance` is the full current-shape model-advance pool.",
        "- `low_eff_low_atr_q20_q40` uses only trailing 3-month symbol-local quantiles, then trades the next calendar month.",
        "- A cross-asset pass would require adverse10 improvement with acceptable worst-month and fewer negative months than the baseline.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    report_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _output_tag(symbol: str, train_months: int, eval_start: pd.Timestamp, eval_end: pd.Timestamp) -> str:
    if eval_start == EVAL_START and eval_end == EVAL_END:
        return f"{symbol.lower()}_train{train_months}m"
    end_inclusive = eval_end - pd.Timedelta(seconds=1)
    return f"{symbol.lower()}_{eval_start.strftime('%Y%m')}_{end_inclusive.strftime('%Y%m')}_train{train_months}m"


def run(
    symbol: str = "BTCUSDT",
    train_months: int = 3,
    bars_cache_dir: Path = BARS_CACHE_DIR,
    eval_start: pd.Timestamp = EVAL_START,
    eval_end: pd.Timestamp = EVAL_END,
) -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    model = _load_model()
    saved_shape_state = {
        "BARS_CACHE_DIR": shape_expansion.BARS_CACHE_DIR,
        "EVAL_START": shape_expansion.EVAL_START,
        "EVAL_END": shape_expansion.EVAL_END,
    }
    shape_expansion.BARS_CACHE_DIR = bars_cache_dir
    shape_expansion.EVAL_START = eval_start
    shape_expansion.EVAL_END = eval_end
    bars_1s = _load_symbol_1s_bars(symbol, bars_cache_dir, eval_start, eval_end)
    bars_1h = _resample_1h(bars_1s)
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
    try:
        raw_events, detector_diagnostics = detect_variant_events(symbol, bars_1s, bars_1h, RESTRICTIVE_VARIANT)
        quality_events = live_quality_filter(raw_events)
        events = apply_frozen_model(quality_events, model)
        events = events[events["timing_prediction"] != "skip"].copy().reset_index(drop=True)
        events["touch_time"] = pd.to_datetime(events["touch_time"], utc=True)

        months = _month_range(eval_start.strftime("%Y-%m-%d"), eval_end.strftime("%Y-%m-%d"))
        forward_months = months[train_months:]
        split_rows: list[dict[str, Any]] = []
        trades_parts: list[pd.DataFrame] = []
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

            for gate in GATES:
                conditions = _materialize_gate(gate, train_events)
                gated_forward = _apply_conditions(forward_events, conditions)
                row, trades = _evaluate_forward(
                    symbol=symbol,
                    gate=gate,
                    forward_month=forward_month,
                    conditions=conditions,
                    events=gated_forward,
                    bars_cache=bars_cache,
                )
                split_rows.append(row)
                if not trades.empty:
                    trades_parts.append(trades)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)
        shape_expansion.BARS_CACHE_DIR = saved_shape_state["BARS_CACHE_DIR"]
        shape_expansion.EVAL_START = saved_shape_state["EVAL_START"]
        shape_expansion.EVAL_END = saved_shape_state["EVAL_END"]

    split_df = pd.DataFrame(split_rows)
    summary = _aggregate(split_rows)
    trades = pd.concat(trades_parts, ignore_index=True, sort=False) if trades_parts else pd.DataFrame()

    tag = _output_tag(symbol, train_months, eval_start, eval_end)
    events_path = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_events.csv"
    split_path = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_splits.csv"
    summary_path = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_summary.csv"
    trades_path = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_trades.csv"
    diagnostics_path = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_diagnostics.json"
    report_path = OUTPUT_DIR / f"breakout_structure_cross_asset_{tag}_report.md"

    events.to_csv(events_path, index=False)
    split_df.to_csv(split_path, index=False)
    summary.to_csv(summary_path, index=False)
    if not trades.empty:
        trades.to_csv(trades_path, index=False)

    diagnostics = {
        "symbol": symbol,
        "model_version": model.get("version"),
        "model_features": model.get("feature_names"),
        "variant": RESTRICTIVE_VARIANT.__dict__,
        "train_months": train_months,
        "bars_cache_dir": str(bars_cache_dir),
        "eval_start": eval_start.isoformat(),
        "eval_end_exclusive": eval_end.isoformat(),
        "raw_events": int(len(raw_events)),
        "quality_events": int(len(quality_events)),
        "model_advance_events": int(len(events)),
        "detector_diagnostics": detector_diagnostics,
        "canonical_coverage": _canonical_coverage(symbol, events, eval_start, eval_end),
        "base_share": BASE_SHARE,
        "exec_params": {**saved_params, **replay_params},
        "outputs": {
            "events_csv": str(events_path),
            "splits_csv": str(split_path),
            "summary_csv": str(summary_path),
            "trades_csv": str(trades_path),
            "report_md": str(report_path),
        },
        "runtime_seconds": time.time() - started,
    }
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(symbol=symbol, summary=summary, split_rows=split_df, diagnostics=diagnostics, report_path=report_path)

    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="Cross-asset validation for breakout structure gate")
    parser.add_argument("--symbol", default="BTCUSDT", choices=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--train-months", type=int, default=3)
    parser.add_argument("--bars-cache-dir", type=Path, default=BARS_CACHE_DIR)
    parser.add_argument("--eval-start", default=EVAL_START.isoformat())
    parser.add_argument("--eval-end", default=EVAL_END.isoformat())
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(
        symbol=args.symbol,
        train_months=args.train_months,
        bars_cache_dir=args.bars_cache_dir,
        eval_start=pd.Timestamp(args.eval_start, tz="UTC") if pd.Timestamp(args.eval_start).tzinfo is None else pd.Timestamp(args.eval_start).tz_convert("UTC"),
        eval_end=pd.Timestamp(args.eval_end, tz="UTC") if pd.Timestamp(args.eval_end).tzinfo is None else pd.Timestamp(args.eval_end).tz_convert("UTC"),
    )


if __name__ == "__main__":
    main()
