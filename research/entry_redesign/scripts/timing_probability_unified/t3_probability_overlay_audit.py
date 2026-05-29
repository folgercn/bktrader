"""Audit frozen probability-model scores on strict T3 lifecycle trades.

Research-only. This does not change the lifecycle replay. It first collects the
current T2-disabled + T3 60m strict lifecycle trades, then scores matching T3
touch events with the frozen ``data/pretouch_model.json`` timing/RF model. The
output is a trade-level attribution table that tells us whether probability
scores are useful as a T3 filter/sizer before we wire them into a replay gate.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, _SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    MAX_EFF_300S,
    MODEL_PATH,
    SPEED_THRESHOLD,
    apply_frozen_model,
)
from timing_probability_unified.t2_lifecycle_context_sizing import EXTENDED_MONTHS  # noqa: E402
from timing_probability_unified.t3_event_generator import T3EventConfig, generate_t3_events  # noqa: E402
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    DEFAULT_SYMBOLS,
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _patched_replay_kwargs,
)

logger = logging.getLogger(__name__)

OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)
T3_60M_EXIT_OVERRIDES = {"min_hold_seconds_before_sl": 3600.0}
ENTRY_METADATA_COLUMNS = [
    "side",
    "signal_start",
    "signal_bar_index",
    "breakout_level",
    "breakout_pre_touch_seconds",
    "breakout_extension_atr",
    "level_to_signal_open_atr",
    "atr",
    "atr_percentile",
    "ctx4h_side_return_atr",
    "ctx12h_side_return_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
]


def _run_t3_floor_ledger(
    *,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> pd.DataFrame:
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)
    with _patched_replay_kwargs(symbol):
        ledger, _ = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=T3_REENTRY_SIZE_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"max_pre_touch_seconds": 900.0},
            quality_filter_shapes=["t3_swing"],
            shape_sizing_filters={"allowed_sides": []},
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=0.0,
            sizing_filter_fail_action="skip_lock",
            t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
            reentry_fill_policy=reentry_fill_policy,
        )
    return ledger


def _pair_t3_trades(
    ledger: pd.DataFrame,
    *,
    symbol: str,
    month: str,
    initial_balance: float,
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    if ledger.empty:
        return rows
    open_entry = None
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue
        if str(open_entry.get("breakout_shape_name", "")) != "t3_swing":
            open_entry = None
            continue

        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        notional = float(open_entry.get("notional", 0.0))
        gross_value = (
            side_mult * (exit_price - entry_price) / entry_price * notional
            if entry_price > 0.0 and notional > 0.0
            else 0.0
        )
        fee_value = notional * 0.002
        entry_time = pd.Timestamp(open_entry["time"])
        exit_time = pd.Timestamp(row["time"])
        item = {
            "symbol": symbol,
            "month": month,
            "entry_time": entry_time,
            "exit_time": exit_time,
            "hold_seconds": float((exit_time - entry_time).total_seconds()),
            "entry_type": str(open_entry["type"]),
            "entry_reason": str(open_entry["reason"]),
            "exit_reason": str(row["reason"]),
            "entry_price": entry_price,
            "exit_price": exit_price,
            "notional_pct": notional / initial_balance * 100.0,
            "gross_pnl_pct": gross_value / initial_balance * 100.0,
            "fee_pct": fee_value / initial_balance * 100.0,
            "net_after_fee_pct": (gross_value - fee_value) / initial_balance * 100.0,
        }
        for column in ENTRY_METADATA_COLUMNS:
            item[column] = open_entry.get(column, np.nan)
        rows.append(item)
        open_entry = None
    return rows


def collect_t3_trades(
    *,
    symbols: list[str],
    months: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    for symbol in symbols:
        for month in months:
            logger.info("Collecting T3 floor trades %s %s", symbol, month)
            ledger = _run_t3_floor_ledger(
                symbol=symbol,
                month=month,
                timeframe=timeframe,
                initial_balance=initial_balance,
                reentry_fill_policy=reentry_fill_policy,
            )
            rows.extend(_pair_t3_trades(ledger, symbol=symbol, month=month, initial_balance=initial_balance))
    return pd.DataFrame(rows)


def build_scored_t3_events(symbols: list[str], *, structure_mode: str = "strict_current") -> pd.DataFrame:
    with Path(MODEL_PATH).open("r", encoding="utf-8") as fh:
        model = json.load(fh)
    parts = []
    for symbol in symbols:
        logger.info("Generating/scoring T3 events %s", symbol)
        bars = _load_window_bars(
            symbol,
            pd.Timestamp("2025-06-01", tz="UTC"),
            pd.Timestamp("2026-05-01", tz="UTC"),
        )
        events = generate_t3_events(
            bars,
            T3EventConfig(
                symbol=symbol,
                max_pre_touch_seconds=900.0,
                structure_mode=structure_mode,
            ),
        )
        if events.empty:
            continue
        scored = apply_frozen_model(events, model)
        scored["model_version"] = str(model.get("version", ""))
        parts.append(scored)
    if not parts:
        return pd.DataFrame()
    return pd.concat(parts, ignore_index=True)


def enrich_trades_with_scores(trades: pd.DataFrame, events: pd.DataFrame) -> pd.DataFrame:
    if trades.empty or events.empty:
        return trades.copy()
    trade_frame = trades.copy()
    event_frame = events.copy()
    trade_frame["signal_start_ts"] = pd.to_datetime(trade_frame["signal_start"], utc=True)
    event_frame["signal_start_ts"] = pd.to_datetime(event_frame["signal_start"], utc=True)
    event_frame["side"] = event_frame["side"].astype(str)
    trade_frame["side"] = trade_frame["side"].astype(str)
    feature_cols = [
        "symbol",
        "side",
        "signal_start_ts",
        "event_id",
        "touch_time",
        "timing_prediction",
        "rf_probability",
        "sizing_multiplier",
        "model_feature_imputations",
        "speed_300s_atr",
        "eff_300s",
        "pre_touch_seconds",
        "touch_extension_atr",
        "roundtrip_cost_atr",
        "model_version",
    ]
    feature_cols = [column for column in feature_cols if column in event_frame.columns]
    features = event_frame[feature_cols].drop_duplicates(["symbol", "side", "signal_start_ts"])
    return trade_frame.merge(
        features,
        on=["symbol", "side", "signal_start_ts"],
        how="left",
        validate="m:1",
        suffixes=("", "_model"),
    )


def _numeric_bucket(series: pd.Series, bins: list[float], labels: list[str]) -> pd.Series:
    numeric = pd.to_numeric(series, errors="coerce")
    bucketed = pd.cut(numeric, bins=bins, labels=labels, include_lowest=True, right=False)
    return bucketed.astype(object).where(bucketed.notna(), "missing").astype(str)


def _fixed_monthly_stats(frame: pd.DataFrame, months: list[str], symbols: list[str]) -> dict[str, Any]:
    if frame.empty:
        month_symbol = pd.Series(dtype=float)
    else:
        work = frame.copy()
        work["month_symbol"] = work["month"].astype(str) + "|" + work["symbol"].astype(str)
        month_symbol = work.groupby("month_symbol")["net_after_fee_pct"].sum()
    values = []
    by_month: dict[str, float] = {}
    by_symbol: dict[str, float] = {}
    for month in months:
        month_total = 0.0
        for symbol in symbols:
            value = float(month_symbol.get(f"{month}|{symbol}", 0.0))
            values.append(value)
            month_total += value
            by_symbol[symbol] = by_symbol.get(symbol, 0.0) + value
        by_month[month] = round(month_total, 6)
    return {
        "net_after_fee_pct": round(float(sum(values)), 6),
        "worst_symbol_month_pct": round(float(min(values, default=0.0)), 6),
        "negative_symbol_months": int(sum(1 for value in values if value < 0.0)),
        "by_month": by_month,
        "by_symbol": {symbol: round(value, 6) for symbol, value in by_symbol.items()},
    }


def _gate_mask(trades: pd.DataFrame, label: str) -> pd.Series:
    rf = pd.to_numeric(trades.get("rf_probability"), errors="coerce")
    timing = trades.get("timing_prediction", pd.Series("", index=trades.index)).astype(str)
    speed = pd.to_numeric(trades.get("speed_300s_atr"), errors="coerce").abs()
    eff = pd.to_numeric(trades.get("eff_300s"), errors="coerce")
    extension = pd.to_numeric(trades.get("touch_extension_atr"), errors="coerce").abs()
    side = trades.get("side", pd.Series("", index=trades.index)).astype(str)

    if label == "all_t3":
        return pd.Series(True, index=trades.index)
    if label == "matched_model_events":
        return rf.notna()
    if label == "timing_fast_or_slow":
        return timing.isin(["fast", "slow"])
    if label == "timing_fast":
        return timing == "fast"
    if label.startswith("rf_ge_"):
        threshold = float(label.removeprefix("rf_ge_").replace("p", "."))
        return rf >= threshold
    if label == "model_live_quality":
        return (speed >= SPEED_THRESHOLD) & (eff <= MAX_EFF_300S)
    if label == "model_live_quality_timing":
        return (speed >= SPEED_THRESHOLD) & (eff <= MAX_EFF_300S) & timing.isin(["fast", "slow"])
    if label == "ext_abs_le_0p05":
        return extension <= 0.05
    if label == "ext_abs_le_0p10":
        return extension <= 0.10
    if label == "side_long":
        return side == "long"
    if label == "side_short":
        return side == "short"
    raise ValueError(f"unknown gate label: {label}")


def summarize_gates(trades: pd.DataFrame, months: list[str], symbols: list[str]) -> pd.DataFrame:
    labels = [
        "all_t3",
        "matched_model_events",
        "timing_fast_or_slow",
        "timing_fast",
        "rf_ge_0p45",
        "rf_ge_0p50",
        "rf_ge_0p55",
        "rf_ge_0p60",
        "rf_ge_0p65",
        "rf_ge_0p70",
        "model_live_quality",
        "model_live_quality_timing",
        "ext_abs_le_0p05",
        "ext_abs_le_0p10",
        "side_long",
        "side_short",
    ]
    rows = []
    for label in labels:
        mask = _gate_mask(trades, label)
        view = trades[mask].copy()
        stats = _fixed_monthly_stats(view, months, symbols)
        pnl = pd.to_numeric(view.get("net_after_fee_pct"), errors="coerce").fillna(0.0)
        rows.append(
            {
                "gate": label,
                "trades": int(len(view)),
                "avg_trade_net_pct": round(float(pnl.mean()), 6) if len(view) else 0.0,
                "win_rate_pct": round(float((pnl > 0.0).mean()) * 100.0, 2) if len(view) else 0.0,
                **stats,
            }
        )
    return pd.DataFrame(rows).sort_values(
        ["net_after_fee_pct", "worst_symbol_month_pct", "trades"],
        ascending=[False, False, False],
    )


def summarize_buckets(trades: pd.DataFrame) -> pd.DataFrame:
    if trades.empty:
        return pd.DataFrame()
    rows = []
    bucket_specs = {
        "rf_probability": _numeric_bucket(
            trades["rf_probability"],
            [-np.inf, 0.35, 0.45, 0.50, 0.55, 0.60, 0.70, np.inf],
            ["<0.35", "0.35-0.45", "0.45-0.50", "0.50-0.55", "0.55-0.60", "0.60-0.70", ">=0.70"],
        ),
        "timing_prediction": trades["timing_prediction"].fillna("missing").astype(str),
        "side": trades["side"].astype(str),
        "exit_reason": trades["exit_reason"].astype(str),
        "speed_300s_abs": _numeric_bucket(
            pd.to_numeric(trades["speed_300s_atr"], errors="coerce").abs(),
            [-np.inf, 0.10, SPEED_THRESHOLD, 0.35, 0.50, np.inf],
            ["<0.10", f"0.10-{SPEED_THRESHOLD:.3f}", f"{SPEED_THRESHOLD:.3f}-0.35", "0.35-0.50", ">=0.50"],
        ),
        "touch_extension_abs": _numeric_bucket(
            pd.to_numeric(trades["touch_extension_atr"], errors="coerce").abs(),
            [-np.inf, 0.02, 0.05, 0.10, 0.20, np.inf],
            ["<0.02", "0.02-0.05", "0.05-0.10", "0.10-0.20", ">=0.20"],
        ),
    }
    for family, buckets in bucket_specs.items():
        frame = trades.copy()
        frame["_bucket"] = buckets
        for bucket, group in frame.groupby("_bucket", dropna=False):
            pnl = pd.to_numeric(group["net_after_fee_pct"], errors="coerce").fillna(0.0)
            rows.append(
                {
                    "family": family,
                    "bucket": str(bucket),
                    "trades": int(len(group)),
                    "net_after_fee_pct": round(float(pnl.sum()), 6),
                    "avg_trade_net_pct": round(float(pnl.mean()), 6),
                    "win_rate_pct": round(float((pnl > 0.0).mean()) * 100.0, 2),
                }
            )
    return pd.DataFrame(rows).sort_values(["family", "net_after_fee_pct"], ascending=[True, False])


def _markdown_table(df: pd.DataFrame, columns: list[str], limit: int | None = None) -> str:
    if df.empty:
        return "_empty_"
    view = df[columns].head(limit).copy() if limit is not None else df[columns].copy()

    def fmt(value: Any) -> str:
        if isinstance(value, (float, np.floating)):
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


def write_outputs(
    *,
    output_dir: Path,
    trades: pd.DataFrame,
    events: pd.DataFrame,
    gate_summary: pd.DataFrame,
    bucket_summary: pd.DataFrame,
    months: list[str],
    symbols: list[str],
    elapsed_seconds: float,
    structure_mode: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    trades_out = trades.copy()
    for column in ("entry_time", "exit_time", "signal_start", "signal_start_ts", "touch_time"):
        if column in trades_out.columns:
            trades_out[column] = pd.to_datetime(trades_out[column], utc=True, errors="coerce").astype(str)
    trades_out.to_csv(output_dir / "t3_probability_overlay_trades.csv", index=False)
    events.to_csv(output_dir / "t3_probability_overlay_scored_events.csv", index=False)
    gate_summary.to_csv(output_dir / "t3_probability_overlay_gate_summary.csv", index=False)
    bucket_summary.to_csv(output_dir / "t3_probability_overlay_bucket_summary.csv", index=False)

    payload = {
        "note": (
            "Research-only audit. Scores frozen data/pretouch_model.json on T3 events and joins them to "
            "strict T2-disabled + T3 60m lifecycle trades. Gate metrics are trade-level attribution, not "
            "yet a rerun lifecycle gate."
        ),
        "model_path": str(MODEL_PATH),
        "months": months,
        "symbols": symbols,
        "structure_mode": structure_mode,
        "trades": int(len(trades)),
        "matched_trades": int(pd.to_numeric(trades.get("rf_probability"), errors="coerce").notna().sum())
        if not trades.empty
        else 0,
        "events": int(len(events)),
        "elapsed_seconds": round(float(elapsed_seconds), 2),
        "top_gates": gate_summary.head(10).to_dict(orient="records"),
    }
    (output_dir / "t3_probability_overlay_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )

    lines = [
        "# T3 Probability Overlay Audit",
        "",
        "Research-only audit for using the frozen probability model as a T3 quality layer.",
        "",
        f"- Model: `{MODEL_PATH}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        f"- T3 structure mode: `{structure_mode}`",
        f"- Trades: {len(trades)}",
        f"- Scored events: {len(events)}",
        f"- Runtime: {elapsed_seconds:.2f}s",
        "",
        "## Gate Summary",
        "",
        _markdown_table(
            gate_summary,
            [
                "gate",
                "trades",
                "net_after_fee_pct",
                "avg_trade_net_pct",
                "win_rate_pct",
                "worst_symbol_month_pct",
                "negative_symbol_months",
            ],
            limit=16,
        ),
        "",
        "## Probability Buckets",
        "",
        _markdown_table(
            bucket_summary[bucket_summary["family"].isin(["rf_probability", "timing_prediction", "side"])],
            ["family", "bucket", "trades", "net_after_fee_pct", "avg_trade_net_pct", "win_rate_pct"],
            limit=40,
        ),
        "",
        "## Read",
        "",
        "- This is not a promoted gate yet; it is the evidence layer before rerunning lifecycle with skips/sizing.",
        "- A useful probability layer should improve T3 net contribution without relying on tiny trade counts.",
        "- If the best buckets lose against `all_t3`, the frozen T2 probability model is not portable to T3.",
    ]
    (output_dir / "t3_probability_overlay_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_scored_events_only(
    *,
    output_dir: Path,
    events: pd.DataFrame,
    symbols: list[str],
    elapsed_seconds: float,
    structure_mode: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    events.to_csv(output_dir / "t3_probability_overlay_scored_events.csv", index=False)
    payload = {
        "note": "Research-only scored T3 events export; lifecycle attribution was skipped.",
        "model_path": str(MODEL_PATH),
        "symbols": symbols,
        "structure_mode": structure_mode,
        "events": int(len(events)),
        "elapsed_seconds": round(float(elapsed_seconds), 2),
    }
    (output_dir / "t3_probability_overlay_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbols", nargs="+", default=DEFAULT_SYMBOLS)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--reentry-fill-policy", default="strict_next_second_cross")
    parser.add_argument(
        "--scored-events-only",
        action="store_true",
        help="Only generate/scored T3 event CSV for downstream overlay sizing; skip lifecycle attribution tables.",
    )
    parser.add_argument(
        "--structure-mode",
        choices=["strict_current", "prev3_dominates"],
        default="strict_current",
        help=(
            "T3 structure predicate for generated/scored events. strict_current preserves the "
            "historical prev3/prev2/prev1 ordering; prev3_dominates only requires prev3 to "
            "dominate prev2 and prev1."
        ),
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=OUTPUT_DIR / "t3_probability_overlay_extended",
    )
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    started = time.time()
    months = list(args.months)
    symbols = list(args.symbols)
    if args.scored_events_only:
        events = build_scored_t3_events(symbols, structure_mode=str(args.structure_mode))
        write_scored_events_only(
            output_dir=Path(args.output_dir),
            events=events,
            symbols=symbols,
            elapsed_seconds=time.time() - started,
            structure_mode=str(args.structure_mode),
        )
        logger.info("Wrote %s", args.output_dir)
        return 0
    trades = collect_t3_trades(
        symbols=symbols,
        months=months,
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )
    events = build_scored_t3_events(symbols, structure_mode=str(args.structure_mode))
    enriched = enrich_trades_with_scores(trades, events)
    gate_summary = summarize_gates(enriched, months, symbols)
    bucket_summary = summarize_buckets(enriched)
    write_outputs(
        output_dir=Path(args.output_dir),
        trades=enriched,
        events=events,
        gate_summary=gate_summary,
        bucket_summary=bucket_summary,
        months=months,
        symbols=symbols,
        elapsed_seconds=time.time() - started,
        structure_mode=str(args.structure_mode),
    )
    logger.info("Wrote %s", args.output_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
