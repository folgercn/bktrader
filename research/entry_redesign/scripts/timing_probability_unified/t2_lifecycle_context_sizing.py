"""Lifecycle replay for research-only original_t2 context sizing.

This runner is the lifecycle bridge for the current T2 ``ctx4h_scaled025``
event-ledger idea. It keeps ``baseline_plus_t3`` and strict reentry fill
semantics, but applies an optional sizing multiplier to ``original_t2`` breakout
locks instead of rejecting them. That preserves the event-ledger contract:
context-pass events trade at full size; context-fail events can stay alive at
25% size.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path

import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, _SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    DEFAULT_SYMBOLS,
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _net_pnl_pct,
    _patched_replay_kwargs,
    _shape_attr,
)

logger = logging.getLogger(__name__)

EXTENDED_MONTHS = [
    "2025-06",
    "2025-07",
    "2025-08",
    "2025-09",
    "2025-10",
    "2025-11",
    "2025-12",
    "2026-01",
    "2026-02",
    "2026-03",
    "2026-04",
]


@dataclass(frozen=True)
class T2LifecycleSizingSpec:
    """One original_t2 sizing candidate."""

    label: str
    shape_sizing_filters: dict
    fail_multiplier: float
    t3_exit_overrides: dict = field(default_factory=dict)
    sizing_filter_fail_action: str = "scale"


@dataclass
class T2LifecycleSizingSilo:
    """One fixed-calendar symbol-month lifecycle result."""

    candidate: str
    shape_sizing_filters: dict
    fail_multiplier: float
    t3_exit_overrides: dict
    sizing_filter_fail_action: str
    symbol: str
    month: str
    return_pct: float
    final_balance: float
    total_trades: int
    t2_trades: int
    t3_trades: int
    t2_net_pnl_pct: float
    t3_net_pnl_pct: float
    t2_size_multiplier_attribution: dict
    t2_size_filter_fails: int
    t2_size_filter_reasons: dict
    reentry_fill_rejects: dict
    elapsed_seconds: float


@dataclass
class T2LifecycleSizingMetrics:
    """Aggregate fixed-calendar metrics for one sizing candidate."""

    candidate: str
    shape_sizing_filters: dict
    fail_multiplier: float
    t3_exit_overrides: dict
    sizing_filter_fail_action: str
    calendar_silo_sum_pct: float
    calendar_avg_symbol_month_pct: float
    worst_calendar_silo_pct: float
    negative_calendar_silos: int
    traded_symbol_months: int
    flat_symbol_months: int
    total_trades: int
    t2_trades: int
    t3_trades: int
    t2_net_pnl_pct: float
    t3_net_pnl_pct: float
    t2_size_multiplier_attribution: dict
    t2_size_filter_fails: int
    t2_size_filter_reasons: dict
    reentry_fill_rejects: dict
    by_month: dict[str, float]
    by_symbol: dict[str, float]


@dataclass
class T2LifecycleSizingDelta:
    """Delta from the first candidate."""

    candidate: str
    baseline_candidate: str
    calendar_silo_sum_delta_pct: float
    worst_calendar_silo_delta_pct: float
    trade_count_delta: int
    t2_trade_delta: int
    t2_net_pnl_delta_pct: float
    negative_calendar_silos_delta: int


def build_sizing_specs(candidate_set: str) -> list[T2LifecycleSizingSpec]:
    """Build bounded research-only original_t2 sizing candidates."""
    ctx4h_filters = {
        "max_atr_percentile": 40.0,
        "min_ctx_side_return_atr": 0.0,
        "ctx_return_lookback_bars": 4,
    }
    t3_60m = {"min_hold_seconds_before_sl": 3600.0}
    if candidate_set == "ctx4h_multiplier_sensitivity":
        specs = [
            T2LifecycleSizingSpec(
                f"original_t2_ctx4h_scaled{int(multiplier * 100):03d}_t3_min_hold_sl_60m",
                ctx4h_filters,
                multiplier,
                t3_60m,
            )
            for multiplier in (1.0, 0.5, 0.25, 0.1, 0.0)
        ]
        specs.append(
            T2LifecycleSizingSpec(
                "original_t2_ctx4h_skipfail_t3_min_hold_sl_60m",
                ctx4h_filters,
                0.0,
                t3_60m,
                "skip_lock",
            )
        )
        return specs
    if candidate_set != "compact":
        raise ValueError(f"unknown candidate set: {candidate_set}")
    return [
        T2LifecycleSizingSpec("strict_baseline", {}, 1.0),
        T2LifecycleSizingSpec(
            "original_t2_ctx4h_scaled025",
            ctx4h_filters,
            0.25,
        ),
        T2LifecycleSizingSpec(
            "original_t2_ctx12h_scaled025",
            {
                "max_atr_percentile": 40.0,
                "min_ctx_side_return_atr": 0.0,
                "ctx_return_lookback_bars": 12,
            },
            0.25,
        ),
        T2LifecycleSizingSpec(
            "original_t2_ctx4h_scaled025_t3_min_hold_sl_60m",
            ctx4h_filters,
            0.25,
            t3_60m,
        ),
    ]


def filter_sizing_specs(
    specs: list[T2LifecycleSizingSpec],
    labels: list[str] | None,
) -> list[T2LifecycleSizingSpec]:
    """Optionally keep a requested ordered subset."""
    if not labels:
        return specs
    lookup = {spec.label: spec for spec in specs}
    missing = [label for label in labels if label not in lookup]
    if missing:
        raise ValueError(f"unknown candidate labels: {', '.join(missing)}")
    return [lookup[label] for label in labels]


def _flat_counts(nested: dict) -> dict[str, int]:
    out: dict[str, int] = {}
    for side_counts in nested.values():
        for reason, count in dict(side_counts).items():
            out[str(reason)] = int(out.get(str(reason), 0)) + int(count)
    return out


def _size_bucket(multiplier: float) -> str:
    if multiplier >= 0.999:
        return "pass_full_or_unfiltered"
    if multiplier <= 0.001:
        return "fail_zero"
    return f"fail_scaled_{multiplier:.2f}"


def _empty_multiplier_bucket() -> dict:
    return {
        "trades": 0,
        "gross_pnl_pct": 0.0,
        "fee_pct": 0.0,
        "net_after_fee_pct": 0.0,
        "notional_pct": 0.0,
        "multiplier_sum": 0.0,
        "avg_multiplier": 0.0,
    }


def summarize_t2_size_multiplier_attribution(
    ledger: pd.DataFrame,
    initial_balance: float,
) -> dict[str, dict]:
    """Summarize original_t2 realized PnL by entry size multiplier."""
    if ledger.empty or "breakout_shape_name" not in ledger.columns:
        return {}

    buckets: dict[str, dict] = {}
    open_entry = None
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue
        if str(open_entry.get("breakout_shape_name", "")) != "original_t2":
            open_entry = None
            continue

        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        notional = float(open_entry.get("notional", 0.0))
        multiplier = float(open_entry.get("size_multiplier", 1.0))
        pnl_value = (
            side_mult * (exit_price - entry_price) / entry_price * notional
            if entry_price > 0.0 and notional > 0.0
            else 0.0
        )
        fee_value = notional * 0.002
        bucket = _size_bucket(multiplier)
        stats = buckets.setdefault(bucket, _empty_multiplier_bucket())
        stats["trades"] += 1
        stats["gross_pnl_pct"] += pnl_value / initial_balance * 100.0
        stats["fee_pct"] += fee_value / initial_balance * 100.0
        stats["net_after_fee_pct"] += (pnl_value - fee_value) / initial_balance * 100.0
        stats["notional_pct"] += notional / initial_balance * 100.0
        stats["multiplier_sum"] += multiplier
        open_entry = None

    rounded: dict[str, dict] = {}
    for bucket, stats in buckets.items():
        trades = int(stats["trades"])
        rounded[bucket] = {
            "trades": trades,
            "gross_pnl_pct": round(float(stats["gross_pnl_pct"]), 6),
            "fee_pct": round(float(stats["fee_pct"]), 6),
            "net_after_fee_pct": round(float(stats["net_after_fee_pct"]), 6),
            "notional_pct": round(float(stats["notional_pct"]), 6),
            "avg_multiplier": round(float(stats["multiplier_sum"]) / trades, 6) if trades else 0.0,
        }
    return rounded


def _merge_multiplier_attribution(rows: list[dict]) -> dict[str, dict]:
    merged: dict[str, dict] = {}
    for row in rows:
        for bucket, stats in row.items():
            dst = merged.setdefault(bucket, _empty_multiplier_bucket())
            trades = int(stats.get("trades", 0))
            dst["trades"] += trades
            dst["gross_pnl_pct"] += float(stats.get("gross_pnl_pct", 0.0))
            dst["fee_pct"] += float(stats.get("fee_pct", 0.0))
            dst["net_after_fee_pct"] += float(stats.get("net_after_fee_pct", 0.0))
            dst["notional_pct"] += float(stats.get("notional_pct", 0.0))
            dst["multiplier_sum"] += float(stats.get("avg_multiplier", 0.0)) * trades

    rounded: dict[str, dict] = {}
    for bucket, stats in merged.items():
        trades = int(stats["trades"])
        rounded[bucket] = {
            "trades": trades,
            "gross_pnl_pct": round(float(stats["gross_pnl_pct"]), 6),
            "fee_pct": round(float(stats["fee_pct"]), 6),
            "net_after_fee_pct": round(float(stats["net_after_fee_pct"]), 6),
            "notional_pct": round(float(stats["notional_pct"]), 6),
            "avg_multiplier": round(float(stats["multiplier_sum"]) / trades, 6) if trades else 0.0,
        }
    return rounded


def run_sizing_silo(
    *,
    spec: T2LifecycleSizingSpec,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> T2LifecycleSizingSilo:
    """Run one symbol-month lifecycle silo."""
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)

    started = time.time()
    with _patched_replay_kwargs(symbol):
        ledger, diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=T3_REENTRY_SIZE_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"max_pre_touch_seconds": 900.0},
            quality_filter_shapes=["t3_swing"],
            shape_sizing_filters=dict(spec.shape_sizing_filters),
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=float(spec.fail_multiplier),
            sizing_filter_fail_action=str(spec.sizing_filter_fail_action),
            t3_exit_overrides=dict(spec.t3_exit_overrides),
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t2_attr = _shape_attr(attribution, "original_t2")
    t3_attr = _shape_attr(attribution, "t3_swing")
    sizing_fails = _flat_counts(diagnostics.get("shape_sizing_filter_fails", {}))
    multiplier_attribution = summarize_t2_size_multiplier_attribution(ledger, initial_balance)

    return T2LifecycleSizingSilo(
        candidate=spec.label,
        shape_sizing_filters=dict(spec.shape_sizing_filters),
        fail_multiplier=float(spec.fail_multiplier),
        t3_exit_overrides=dict(spec.t3_exit_overrides),
        sizing_filter_fail_action=str(spec.sizing_filter_fail_action),
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        total_trades=int(summary["trades"]),
        t2_trades=int(t2_attr.get("trades", 0)),
        t3_trades=int(t3_attr.get("trades", 0)),
        t2_net_pnl_pct=round(_net_pnl_pct(attribution, "original_t2", initial_balance), 6),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        t2_size_multiplier_attribution=multiplier_attribution,
        t2_size_filter_fails=int(sum(sizing_fails.values())),
        t2_size_filter_reasons=sizing_fails,
        reentry_fill_rejects=dict(diagnostics.get("reentry_fill_rejects", {})),
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_metrics(
    spec: T2LifecycleSizingSpec,
    silos: list[T2LifecycleSizingSilo],
    months: list[str],
    symbols: list[str],
) -> T2LifecycleSizingMetrics:
    """Aggregate one candidate on a fixed month x symbol grid."""
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    sizing_reasons: dict[str, int] = {}
    reentry_rejects: dict[str, int] = {}
    multiplier_rows = []
    for month in months:
        for symbol in symbols:
            row = lookup.get((symbol, month))
            if row is None:
                grid_rows.append((symbol, month, 0.0, 0, 0, 0, 0.0, 0.0))
                continue
            grid_rows.append(
                (
                    symbol,
                    month,
                    row.return_pct,
                    row.total_trades,
                    row.t2_trades,
                    row.t3_trades,
                    row.t2_net_pnl_pct,
                    row.t3_net_pnl_pct,
                )
            )
            for reason, count in row.t2_size_filter_reasons.items():
                sizing_reasons[reason] = int(sizing_reasons.get(reason, 0)) + int(count)
            for reason, count in _flat_counts(row.reentry_fill_rejects).items():
                reentry_rejects[reason] = int(reentry_rejects.get(reason, 0)) + int(count)
            multiplier_rows.append(row.t2_size_multiplier_attribution)

    total_slots = len(months) * len(symbols)
    returns = [float(row[2]) for row in grid_rows]
    total_return = round(float(sum(returns)), 6)
    return T2LifecycleSizingMetrics(
        candidate=spec.label,
        shape_sizing_filters=dict(spec.shape_sizing_filters),
        fail_multiplier=float(spec.fail_multiplier),
        t3_exit_overrides=dict(spec.t3_exit_overrides),
        sizing_filter_fail_action=str(spec.sizing_filter_fail_action),
        calendar_silo_sum_pct=total_return,
        calendar_avg_symbol_month_pct=round(total_return / total_slots, 6) if total_slots else 0.0,
        worst_calendar_silo_pct=round(float(min(returns, default=0.0)), 6),
        negative_calendar_silos=int(sum(1 for value in returns if value < 0.0)),
        traded_symbol_months=int(sum(1 for row in grid_rows if row[3] > 0)),
        flat_symbol_months=int(sum(1 for row in grid_rows if row[3] == 0)),
        total_trades=int(sum(row[3] for row in grid_rows)),
        t2_trades=int(sum(row[4] for row in grid_rows)),
        t3_trades=int(sum(row[5] for row in grid_rows)),
        t2_net_pnl_pct=round(float(sum(row[6] for row in grid_rows)), 6),
        t3_net_pnl_pct=round(float(sum(row[7] for row in grid_rows)), 6),
        t2_size_multiplier_attribution=_merge_multiplier_attribution(multiplier_rows),
        t2_size_filter_fails=int(sum(sizing_reasons.values())),
        t2_size_filter_reasons=sizing_reasons,
        reentry_fill_rejects=reentry_rejects,
        by_month={
            month: round(float(sum(row[2] for row in grid_rows if row[1] == month)), 6)
            for month in months
        },
        by_symbol={
            symbol: round(float(sum(row[2] for row in grid_rows if row[0] == symbol)), 6)
            for symbol in symbols
        },
    )


def compute_deltas(metrics: list[T2LifecycleSizingMetrics]) -> list[T2LifecycleSizingDelta]:
    """Compute deltas from the first candidate."""
    if not metrics:
        return []
    baseline = metrics[0]
    return [
        T2LifecycleSizingDelta(
            candidate=row.candidate,
            baseline_candidate=baseline.candidate,
            calendar_silo_sum_delta_pct=round(
                row.calendar_silo_sum_pct - baseline.calendar_silo_sum_pct,
                6,
            ),
            worst_calendar_silo_delta_pct=round(
                row.worst_calendar_silo_pct - baseline.worst_calendar_silo_pct,
                6,
            ),
            trade_count_delta=int(row.total_trades - baseline.total_trades),
            t2_trade_delta=int(row.t2_trades - baseline.t2_trades),
            t2_net_pnl_delta_pct=round(row.t2_net_pnl_pct - baseline.t2_net_pnl_pct, 6),
            negative_calendar_silos_delta=int(
                row.negative_calendar_silos - baseline.negative_calendar_silos
            ),
        )
        for row in metrics[1:]
    ]


def run_sizing_validation(
    *,
    specs: list[T2LifecycleSizingSpec],
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> tuple[list[T2LifecycleSizingSilo], list[T2LifecycleSizingMetrics]]:
    """Run all sizing candidates."""
    all_silos: list[T2LifecycleSizingSilo] = []
    metrics: list[T2LifecycleSizingMetrics] = []
    for spec in specs:
        candidate_silos = []
        for symbol in symbols:
            for month in months:
                logger.info("Running T2 lifecycle sizing %s %s %s", spec.label, symbol, month)
                row = run_sizing_silo(
                    spec=spec,
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    reentry_fill_policy=reentry_fill_policy,
                )
                candidate_silos.append(row)
                all_silos.append(row)
        metrics.append(compute_metrics(spec, candidate_silos, months, symbols))
    return all_silos, metrics


def write_outputs(
    *,
    specs: list[T2LifecycleSizingSpec],
    silos: list[T2LifecycleSizingSilo],
    metrics: list[T2LifecycleSizingMetrics],
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
) -> None:
    """Write JSON and Markdown reports."""
    output_dir.mkdir(parents=True, exist_ok=True)
    deltas = compute_deltas(metrics)
    payload = {
        "note": (
            "Research-only lifecycle bridge for original_t2 context sizing. "
            "Sizing filters scale original_t2 reentry-window orders; they do not reject breakout locks."
        ),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "candidate_specs": [asdict(spec) for spec in specs],
        "candidates": [asdict(row) for row in metrics],
        "deltas_vs_first_candidate": [asdict(row) for row in deltas],
        "silos": [asdict(row) for row in silos],
    }
    (output_dir / "t2_lifecycle_context_sizing_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    lines = [
        "# T2 Lifecycle Context Sizing",
        "",
        "Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult | Fail Action | T3 Overrides |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---|---|",
    ]
    for row in metrics:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.calendar_avg_symbol_month_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} | {row.total_trades} "
            f"| {row.t2_trades} | {row.t2_net_pnl_pct:.6f}% "
            f"| {row.t3_net_pnl_pct:.6f}% | {row.t2_size_filter_fails} "
            f"| `{json.dumps(row.shape_sizing_filters, sort_keys=True)}` "
            f"| {row.fail_multiplier:.2f} "
            f"| `{row.sizing_filter_fail_action}` "
            f"| `{json.dumps(row.t3_exit_overrides, sort_keys=True)}` |"
        )

    if deltas:
        lines.extend(
            [
                "",
                "## Delta Vs Baseline",
                "",
                "| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T2 Trade Delta | T2 PnL Delta | Neg Silo Delta |",
                "|---|---:|---:|---:|---:|---:|---:|",
            ]
        )
        for row in deltas:
            lines.append(
                f"| `{row.candidate}` vs `{row.baseline_candidate}` "
                f"| {row.calendar_silo_sum_delta_pct:+.6f}% "
                f"| {row.worst_calendar_silo_delta_pct:+.6f}% "
                f"| {row.trade_count_delta:+d} "
                f"| {row.t2_trade_delta:+d} "
                f"| {row.t2_net_pnl_delta_pct:+.6f}% "
                f"| {row.negative_calendar_silos_delta:+d} |"
            )

    multiplier_rows = []
    for row in metrics:
        for bucket, stats in row.t2_size_multiplier_attribution.items():
            multiplier_rows.append(
                [
                    f"`{row.candidate}`",
                    f"`{bucket}`",
                    stats["trades"],
                    f"{stats['avg_multiplier']:.6f}",
                    f"{stats['gross_pnl_pct']:.6f}%",
                    f"{stats['fee_pct']:.6f}%",
                    f"{stats['net_after_fee_pct']:.6f}%",
                    f"{stats['notional_pct']:.6f}%",
                ]
            )
    if multiplier_rows:
        lines.extend(
            [
                "",
                "## T2 Size Multiplier Attribution",
                "",
                "| Candidate | Bucket | Trades | Avg Mult | Gross PnL | Fee | Net After Fee | Notional |",
                "|---|---|---:|---:|---:|---:|---:|---:|",
            ]
        )
        for row in multiplier_rows:
            lines.append("| " + " | ".join(str(value) for value in row) + " |")

    lines.extend(
        [
            "",
            "## Silo Detail",
            "",
            "| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |",
            "|---|---|---|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for row in silos:
        lines.append(
            f"| `{row.candidate}` | `{row.symbol}` | {row.month} "
            f"| {row.return_pct:.6f}% | {row.total_trades} | {row.t2_trades} "
            f"| {row.t2_net_pnl_pct:.6f}% | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t2_size_filter_fails} "
            f"| `{json.dumps(row.t2_size_filter_reasons, sort_keys=True)}` |"
        )

    (output_dir / "t2_lifecycle_context_sizing_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Lifecycle bridge for original_t2 context sizing")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t2_lifecycle_context_sizing")
    parser.add_argument(
        "--candidate-set",
        choices=["compact", "ctx4h_multiplier_sensitivity"],
        default="compact",
    )
    parser.add_argument("--labels", nargs="+", default=None)
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbols", nargs="+", default=DEFAULT_SYMBOLS)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument(
        "--reentry-fill-policy",
        choices=["historical", "strict_next_second_cross"],
        default="strict_next_second_cross",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    specs = filter_sizing_specs(build_sizing_specs(str(args.candidate_set)), args.labels)
    months = [str(value) for value in args.months]
    symbols = [str(value) for value in args.symbols]
    silos, metrics = run_sizing_validation(
        specs=specs,
        months=months,
        symbols=symbols,
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )
    output_dir = Path(args.output_dir)
    write_outputs(
        specs=specs,
        silos=silos,
        metrics=metrics,
        output_dir=output_dir,
        months=months,
        symbols=symbols,
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )

    best = max(metrics, key=lambda row: row.calendar_silo_sum_pct)
    baseline = metrics[0]
    print(
        f"Best T2 lifecycle sizing candidate: {best.candidate} "
        f"calendar_sum={best.calendar_silo_sum_pct:.6f}%, "
        f"delta={best.calendar_silo_sum_pct - baseline.calendar_silo_sum_pct:+.6f}%, "
        f"t2_pnl={best.t2_net_pnl_pct:.6f}%, "
        f"size_fails={best.t2_size_filter_fails}"
    )
    print(f"Reports: {output_dir}/t2_lifecycle_context_sizing_{{summary.json,report.md}}")


if __name__ == "__main__":
    main()
