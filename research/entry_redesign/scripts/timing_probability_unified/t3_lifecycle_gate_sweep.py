"""Lifecycle replay sweep for T3 quality gates.

This research-only tool keeps the stricter ``baseline_plus_t3`` +
``reentry_window`` lifecycle from ``t3_pre_touch_lifecycle.py``, but moves the
next search step from a single pre-touch cap to lifecycle-side T3 quality
filters. Filters are applied only to ``t3_swing`` breakout locks; original T2
locks keep the baseline path.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from dataclasses import asdict, dataclass
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
    DEFAULT_MONTHS,
    DEFAULT_SYMBOLS,
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _nested_sum_counts,
    _net_pnl_pct,
    _patched_replay_kwargs,
    _shape_attr,
)

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class LifecycleGateSpec:
    """One T3 lifecycle quality-gate candidate."""

    label: str
    filters: dict


@dataclass
class LifecycleGateSiloResult:
    """One fixed-calendar symbol-month replay result."""

    candidate: str
    filters: dict
    symbol: str
    month: str
    return_pct: float
    final_balance: float
    trades: int
    win_rate_pct: float
    max_dd_pct: float
    t2_trades: int
    t3_trades: int
    t2_net_pnl_pct: float
    t3_net_pnl_pct: float
    t3_rejects: int
    t3_reject_reasons: dict
    breakout_locks: dict
    entry_reasons: dict
    exit_reasons: dict
    elapsed_seconds: float


@dataclass
class LifecycleGateMetrics:
    """Fixed-calendar aggregate for one lifecycle gate candidate."""

    candidate: str
    filters: dict
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
    t3_rejects: int
    by_month: dict[str, float]
    by_symbol: dict[str, float]


@dataclass
class LifecycleGateDelta:
    """Delta from the first lifecycle gate candidate."""

    candidate: str
    baseline_candidate: str
    calendar_silo_sum_delta_pct: float
    worst_calendar_silo_delta_pct: float
    trade_count_delta: int
    t3_trade_delta: int
    t3_net_pnl_delta_pct: float
    t3_reject_delta: int
    negative_calendar_silos_delta: int


def compact_gate_specs() -> list[LifecycleGateSpec]:
    """Small default sweep for lifecycle-internal T3 filters."""
    return [
        LifecycleGateSpec("pre900", {"max_pre_touch_seconds": 900.0}),
        LifecycleGateSpec("pre600", {"max_pre_touch_seconds": 600.0}),
        LifecycleGateSpec("pre300", {"max_pre_touch_seconds": 300.0}),
        LifecycleGateSpec(
            "pre900_sep0p25",
            {"max_pre_touch_seconds": 900.0, "min_sma_atr_separation": 0.25},
        ),
        LifecycleGateSpec(
            "pre600_sep0p25",
            {"max_pre_touch_seconds": 600.0, "min_sma_atr_separation": 0.25},
        ),
        LifecycleGateSpec(
            "pre900_ext0p75",
            {"max_pre_touch_seconds": 900.0, "max_breakout_extension_atr": 0.75},
        ),
        LifecycleGateSpec(
            "pre600_ext0p75",
            {"max_pre_touch_seconds": 600.0, "max_breakout_extension_atr": 0.75},
        ),
        LifecycleGateSpec(
            "pre900_trend",
            {"max_pre_touch_seconds": 900.0, "trend": True},
        ),
        LifecycleGateSpec(
            "pre900_long_only",
            {"max_pre_touch_seconds": 900.0, "allowed_sides": ["long"]},
        ),
        LifecycleGateSpec(
            "pre900_short_only",
            {"max_pre_touch_seconds": 900.0, "allowed_sides": ["short"]},
        ),
        LifecycleGateSpec(
            "pre600_long_only",
            {"max_pre_touch_seconds": 600.0, "allowed_sides": ["long"]},
        ),
        LifecycleGateSpec(
            "pre600_short_only",
            {"max_pre_touch_seconds": 600.0, "allowed_sides": ["short"]},
        ),
    ]


def extended_gate_specs() -> list[LifecycleGateSpec]:
    """A still-bounded follow-up grid for combo checks."""
    specs = compact_gate_specs()
    specs.extend(
        [
            LifecycleGateSpec(
                "pre900_sep0p25_ext0p75",
                {
                    "max_pre_touch_seconds": 900.0,
                    "min_sma_atr_separation": 0.25,
                    "max_breakout_extension_atr": 0.75,
                },
            ),
            LifecycleGateSpec(
                "pre600_sep0p25_ext0p75",
                {
                    "max_pre_touch_seconds": 600.0,
                    "min_sma_atr_separation": 0.25,
                    "max_breakout_extension_atr": 0.75,
                },
            ),
            LifecycleGateSpec(
                "pre900_short_sep0p25",
                {
                    "max_pre_touch_seconds": 900.0,
                    "allowed_sides": ["short"],
                    "min_sma_atr_separation": 0.25,
                },
            ),
            LifecycleGateSpec(
                "pre600_short_sep0p25",
                {
                    "max_pre_touch_seconds": 600.0,
                    "allowed_sides": ["short"],
                    "min_sma_atr_separation": 0.25,
                },
            ),
            LifecycleGateSpec(
                "pre900_short_ext0p75",
                {
                    "max_pre_touch_seconds": 900.0,
                    "allowed_sides": ["short"],
                    "max_breakout_extension_atr": 0.75,
                },
            ),
            LifecycleGateSpec(
                "pre600_short_ext0p75",
                {
                    "max_pre_touch_seconds": 600.0,
                    "allowed_sides": ["short"],
                    "max_breakout_extension_atr": 0.75,
                },
            ),
        ]
    )
    return specs


def build_gate_specs(candidate_set: str) -> list[LifecycleGateSpec]:
    if candidate_set == "compact":
        return compact_gate_specs()
    if candidate_set == "extended":
        return extended_gate_specs()
    raise ValueError(f"unknown candidate set: {candidate_set}")


def filter_gate_specs(specs: list[LifecycleGateSpec], labels: list[str] | None) -> list[LifecycleGateSpec]:
    """Optionally keep a requested ordered subset of candidate labels."""
    if not labels:
        return specs
    lookup = {spec.label: spec for spec in specs}
    missing = [label for label in labels if label not in lookup]
    if missing:
        raise ValueError(f"unknown candidate labels: {', '.join(missing)}")
    return [lookup[label] for label in labels]


def run_gate_silo(
    *,
    spec: LifecycleGateSpec,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> LifecycleGateSiloResult:
    """Run one symbol-month lifecycle silo for a generic T3 gate."""
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
            t3_quality_filters=dict(spec.filters),
            quality_filter_shapes=["t3_swing"],
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t2_attr = _shape_attr(attribution, "original_t2")
    t3_attr = _shape_attr(attribution, "t3_swing")
    rejects = diagnostics.get("t3_quality_rejects", {})

    return LifecycleGateSiloResult(
        candidate=spec.label,
        filters=dict(spec.filters),
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        trades=int(summary["trades"]),
        win_rate_pct=round(float(summary["win_rate_pct"]), 6),
        max_dd_pct=round(float(summary["max_dd_pct"]), 6),
        t2_trades=int(t2_attr.get("trades", 0)),
        t3_trades=int(t3_attr.get("trades", 0)),
        t2_net_pnl_pct=round(_net_pnl_pct(attribution, "original_t2", initial_balance), 6),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        t3_rejects=_nested_sum_counts(rejects),
        t3_reject_reasons=rejects,
        breakout_locks=diagnostics.get("breakout_locks", {}),
        entry_reasons=summary.get("entry_reasons", {}),
        exit_reasons=summary.get("exit_reasons", {}),
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_gate_metrics(
    spec: LifecycleGateSpec,
    silos: list[LifecycleGateSiloResult],
    months: list[str],
    symbols: list[str],
) -> LifecycleGateMetrics:
    """Aggregate one candidate on a fixed month x symbol grid."""
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    for month in months:
        for symbol in symbols:
            row = lookup.get((symbol, month))
            if row is None:
                grid_rows.append((symbol, month, 0.0, 0, 0, 0, 0.0, 0.0, 0))
                continue
            grid_rows.append(
                (
                    symbol,
                    month,
                    row.return_pct,
                    row.trades,
                    row.t2_trades,
                    row.t3_trades,
                    row.t2_net_pnl_pct,
                    row.t3_net_pnl_pct,
                    row.t3_rejects,
                )
            )

    total_slots = len(months) * len(symbols)
    returns = [float(row[2]) for row in grid_rows]
    total_return = round(float(sum(returns)), 6)
    by_month = {
        month: round(float(sum(row[2] for row in grid_rows if row[1] == month)), 6)
        for month in months
    }
    by_symbol = {
        symbol: round(float(sum(row[2] for row in grid_rows if row[0] == symbol)), 6)
        for symbol in symbols
    }

    return LifecycleGateMetrics(
        candidate=spec.label,
        filters=dict(spec.filters),
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
        t3_rejects=int(sum(row[8] for row in grid_rows)),
        by_month=by_month,
        by_symbol=by_symbol,
    )


def compute_gate_deltas(metrics: list[LifecycleGateMetrics]) -> list[LifecycleGateDelta]:
    if not metrics:
        return []
    baseline = metrics[0]
    return [
        LifecycleGateDelta(
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
            t3_trade_delta=int(row.t3_trades - baseline.t3_trades),
            t3_net_pnl_delta_pct=round(row.t3_net_pnl_pct - baseline.t3_net_pnl_pct, 6),
            t3_reject_delta=int(row.t3_rejects - baseline.t3_rejects),
            negative_calendar_silos_delta=int(
                row.negative_calendar_silos - baseline.negative_calendar_silos
            ),
        )
        for row in metrics[1:]
    ]


def run_gate_sweep(
    *,
    specs: list[LifecycleGateSpec],
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> tuple[list[LifecycleGateSiloResult], list[LifecycleGateMetrics]]:
    all_silos: list[LifecycleGateSiloResult] = []
    metrics: list[LifecycleGateMetrics] = []

    for spec in specs:
        candidate_silos = []
        for symbol in symbols:
            for month in months:
                logger.info("Running lifecycle gate %s %s %s", spec.label, symbol, month)
                row = run_gate_silo(
                    spec=spec,
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    reentry_fill_policy=reentry_fill_policy,
                )
                candidate_silos.append(row)
                all_silos.append(row)
        metrics.append(compute_gate_metrics(spec, candidate_silos, months, symbols))

    return all_silos, metrics


def write_outputs(
    *,
    specs: list[LifecycleGateSpec],
    silos: list[LifecycleGateSiloResult],
    metrics: list[LifecycleGateMetrics],
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    deltas = compute_gate_deltas(metrics)
    payload = {
        "note": (
            "Research-only full lifecycle T3 quality gate sweep. Filters apply "
            "only to t3_swing breakout locks inside baseline_plus_t3; original_t2 "
            "is unchanged. This is a discovery step, not a live promotion."
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
    (output_dir / "t3_lifecycle_gate_sweep_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    rows = []
    for row in metrics:
        data = asdict(row)
        data["filters"] = json.dumps(row.filters, sort_keys=True)
        data["by_month"] = json.dumps(row.by_month, sort_keys=True)
        data["by_symbol"] = json.dumps(row.by_symbol, sort_keys=True)
        rows.append(data)
    pd.DataFrame(rows).to_csv(output_dir / "t3_lifecycle_gate_sweep_candidates.csv", index=False)

    lines = [
        "# T3 Lifecycle Gate Sweep",
        "",
        "Research-only full reentry-window lifecycle sweep. Missing symbol-months are counted as 0.0.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T3 Trades | T3 Net PnL | T3 Rejects | Filters |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for row in metrics:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.calendar_avg_symbol_month_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} "
            f"| {row.total_trades} | {row.t2_trades} | {row.t3_trades} "
            f"| {row.t3_net_pnl_pct:.6f}% | {row.t3_rejects} "
            f"| `{json.dumps(row.filters, sort_keys=True)}` |"
        )

    if deltas:
        lines.extend(
            [
                "",
                "## Delta Vs Baseline",
                "",
                "| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T3 Trade Delta | T3 PnL Delta | T3 Reject Delta | Neg Silo Delta |",
                "|---|---:|---:|---:|---:|---:|---:|---:|",
            ]
        )
        for row in deltas:
            lines.append(
                f"| `{row.candidate}` vs `{row.baseline_candidate}` "
                f"| {row.calendar_silo_sum_delta_pct:+.6f}% "
                f"| {row.worst_calendar_silo_delta_pct:+.6f}% "
                f"| {row.trade_count_delta:+d} "
                f"| {row.t3_trade_delta:+d} "
                f"| {row.t3_net_pnl_delta_pct:+.6f}% "
                f"| {row.t3_reject_delta:+d} "
                f"| {row.negative_calendar_silos_delta:+d} |"
            )

    lines.extend(
        [
            "",
            "## Silo Detail",
            "",
            "| Candidate | Symbol | Month | Return | Trades | T2 Trades | T3 Trades | T3 PnL | T3 Rejects | Reject Reasons |",
            "|---|---|---|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for row in silos:
        lines.append(
            f"| `{row.candidate}` | `{row.symbol}` | {row.month} "
            f"| {row.return_pct:.6f}% | {row.trades} | {row.t2_trades} "
            f"| {row.t3_trades} | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_rejects} "
            f"| `{json.dumps(row.t3_reject_reasons, sort_keys=True)}` |"
        )

    (output_dir / "t3_lifecycle_gate_sweep_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Lifecycle replay sweep for T3 quality gates")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_lifecycle_gate_sweep")
    parser.add_argument("--candidate-set", choices=["compact", "extended"], default="compact")
    parser.add_argument("--labels", nargs="+", default=None)
    parser.add_argument("--months", nargs="+", default=DEFAULT_MONTHS)
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

    specs = filter_gate_specs(build_gate_specs(str(args.candidate_set)), args.labels)
    silos, metrics = run_gate_sweep(
        specs=specs,
        months=[str(value) for value in args.months],
        symbols=[str(value) for value in args.symbols],
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
        months=[str(value) for value in args.months],
        symbols=[str(value) for value in args.symbols],
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )

    best = max(metrics, key=lambda row: row.calendar_silo_sum_pct)
    baseline = metrics[0]
    print(
        f"Best lifecycle gate: {best.candidate} "
        f"calendar_sum={best.calendar_silo_sum_pct:.6f}%, "
        f"delta={best.calendar_silo_sum_pct - baseline.calendar_silo_sum_pct:+.6f}%, "
        f"t3_pnl={best.t3_net_pnl_pct:.6f}%, "
        f"trades={best.total_trades}, t3_trades={best.t3_trades}"
    )
    print(f"Reports: {output_dir}/t3_lifecycle_gate_sweep_{{summary.json,report.md,candidates.csv}}")


if __name__ == "__main__":
    main()
