"""Lifecycle replay sweep for T3-specific exit overrides.

This research-only tool keeps T2 unchanged and only applies executable exit
overrides to ``t3_swing`` positions. It is meant to test whether the T3 edge is
improved by late-hold caps or tighter trailing/stop settings after outcome
diagnostics showed entry-side no-trade filters were not the right lever.
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
    _net_pnl_pct,
    _patched_replay_kwargs,
    _shape_attr,
)

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class T3ExitSpec:
    """One T3-specific executable exit candidate."""

    label: str
    t3_exit_overrides: dict


@dataclass
class T3ExitSiloResult:
    """One fixed-calendar symbol-month replay result."""

    candidate: str
    t3_exit_overrides: dict
    symbol: str
    month: str
    return_pct: float
    final_balance: float
    total_trades: int
    t2_trades: int
    t3_trades: int
    t2_net_pnl_pct: float
    t3_net_pnl_pct: float
    t3_win_rate_pct: float
    t3_exit_reasons: dict
    elapsed_seconds: float


@dataclass
class T3ExitCandidateMetrics:
    """Fixed-calendar aggregate for one exit candidate."""

    candidate: str
    t3_exit_overrides: dict
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
    t3_win_rate_pct: float
    t3_exit_reasons: dict
    by_month: dict[str, float]
    by_symbol: dict[str, float]


@dataclass
class T3ExitCandidateDelta:
    """Delta from the first exit candidate."""

    candidate: str
    baseline_candidate: str
    calendar_silo_sum_delta_pct: float
    worst_calendar_silo_delta_pct: float
    trade_count_delta: int
    t3_trade_delta: int
    t3_net_pnl_delta_pct: float
    negative_calendar_silos_delta: int


def compact_exit_specs() -> list[T3ExitSpec]:
    """Small default search set for T3 exit behavior."""
    return [
        T3ExitSpec("baseline", {}),
        T3ExitSpec("t3_timecap_30m", {"max_hold_seconds": 1800.0}),
        T3ExitSpec("t3_timecap_60m", {"max_hold_seconds": 3600.0}),
        T3ExitSpec("t3_timecap_120m", {"max_hold_seconds": 7200.0}),
        T3ExitSpec("t3_min_hold_sl_5m", {"min_hold_seconds_before_sl": 300.0}),
        T3ExitSpec("t3_min_hold_sl_10m", {"min_hold_seconds_before_sl": 600.0}),
        T3ExitSpec("t3_min_hold_sl_12m", {"min_hold_seconds_before_sl": 720.0}),
        T3ExitSpec("t3_min_hold_sl_15m", {"min_hold_seconds_before_sl": 900.0}),
        T3ExitSpec("t3_min_hold_sl_20m", {"min_hold_seconds_before_sl": 1200.0}),
        T3ExitSpec("t3_min_hold_sl_30m", {"min_hold_seconds_before_sl": 1800.0}),
        T3ExitSpec("t3_min_hold_sl_45m", {"min_hold_seconds_before_sl": 2700.0}),
        T3ExitSpec("t3_min_hold_sl_60m", {"min_hold_seconds_before_sl": 3600.0}),
        T3ExitSpec("t3_trail_fast_0p3", {"delayed_trailing_activation": 0.3}),
        T3ExitSpec("t3_trail_tight_0p2", {"trailing_stop_atr": 0.2}),
        T3ExitSpec("t3_stop_0p04", {"stop_loss_atr": 0.04}),
    ]


def build_exit_specs(candidate_set: str) -> list[T3ExitSpec]:
    if candidate_set == "compact":
        return compact_exit_specs()
    raise ValueError(f"unknown candidate set: {candidate_set}")


def filter_exit_specs(specs: list[T3ExitSpec], labels: list[str] | None) -> list[T3ExitSpec]:
    """Optionally keep a requested ordered subset of candidate labels."""
    if not labels:
        return specs
    lookup = {spec.label: spec for spec in specs}
    missing = [label for label in labels if label not in lookup]
    if missing:
        raise ValueError(f"unknown candidate labels: {', '.join(missing)}")
    return [lookup[label] for label in labels]


def _merge_counts(dst: dict[str, int], src: dict) -> None:
    for key, value in src.items():
        dst[str(key)] = int(dst.get(str(key), 0)) + int(value)


def run_exit_silo(
    *,
    spec: T3ExitSpec,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> T3ExitSiloResult:
    """Run one symbol-month lifecycle silo with a T3 exit override."""
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)

    started = time.time()
    with _patched_replay_kwargs(symbol):
        ledger, _diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=T3_REENTRY_SIZE_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"max_pre_touch_seconds": 900.0},
            quality_filter_shapes=["t3_swing"],
            t3_exit_overrides=dict(spec.t3_exit_overrides),
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t2_attr = _shape_attr(attribution, "original_t2")
    t3_attr = _shape_attr(attribution, "t3_swing")

    return T3ExitSiloResult(
        candidate=spec.label,
        t3_exit_overrides=dict(spec.t3_exit_overrides),
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        total_trades=int(summary["trades"]),
        t2_trades=int(t2_attr.get("trades", 0)),
        t3_trades=int(t3_attr.get("trades", 0)),
        t2_net_pnl_pct=round(_net_pnl_pct(attribution, "original_t2", initial_balance), 6),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        t3_win_rate_pct=round(float(t3_attr.get("win_rate_pct", 0.0)), 6),
        t3_exit_reasons=dict(t3_attr.get("exit_reasons", {})),
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_exit_metrics(
    spec: T3ExitSpec,
    silos: list[T3ExitSiloResult],
    months: list[str],
    symbols: list[str],
) -> T3ExitCandidateMetrics:
    """Aggregate one candidate on a fixed month x symbol grid."""
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    exit_reasons: dict[str, int] = {}
    for month in months:
        for symbol in symbols:
            row = lookup.get((symbol, month))
            if row is None:
                grid_rows.append((symbol, month, 0.0, 0, 0, 0, 0.0, 0.0, 0.0))
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
                    row.t3_win_rate_pct * row.t3_trades,
                )
            )
            _merge_counts(exit_reasons, row.t3_exit_reasons)

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
    t3_trades = int(sum(row[5] for row in grid_rows))
    weighted_win = float(sum(row[8] for row in grid_rows))

    return T3ExitCandidateMetrics(
        candidate=spec.label,
        t3_exit_overrides=dict(spec.t3_exit_overrides),
        calendar_silo_sum_pct=total_return,
        calendar_avg_symbol_month_pct=round(total_return / total_slots, 6) if total_slots else 0.0,
        worst_calendar_silo_pct=round(float(min(returns, default=0.0)), 6),
        negative_calendar_silos=int(sum(1 for value in returns if value < 0.0)),
        traded_symbol_months=int(sum(1 for row in grid_rows if row[3] > 0)),
        flat_symbol_months=int(sum(1 for row in grid_rows if row[3] == 0)),
        total_trades=int(sum(row[3] for row in grid_rows)),
        t2_trades=int(sum(row[4] for row in grid_rows)),
        t3_trades=t3_trades,
        t2_net_pnl_pct=round(float(sum(row[6] for row in grid_rows)), 6),
        t3_net_pnl_pct=round(float(sum(row[7] for row in grid_rows)), 6),
        t3_win_rate_pct=round(weighted_win / t3_trades, 6) if t3_trades else 0.0,
        t3_exit_reasons=exit_reasons,
        by_month=by_month,
        by_symbol=by_symbol,
    )


def compute_exit_deltas(metrics: list[T3ExitCandidateMetrics]) -> list[T3ExitCandidateDelta]:
    if not metrics:
        return []
    baseline = metrics[0]
    return [
        T3ExitCandidateDelta(
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
            negative_calendar_silos_delta=int(
                row.negative_calendar_silos - baseline.negative_calendar_silos
            ),
        )
        for row in metrics[1:]
    ]


def run_exit_sweep(
    *,
    specs: list[T3ExitSpec],
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> tuple[list[T3ExitSiloResult], list[T3ExitCandidateMetrics]]:
    all_silos: list[T3ExitSiloResult] = []
    metrics: list[T3ExitCandidateMetrics] = []
    for spec in specs:
        candidate_silos = []
        for symbol in symbols:
            for month in months:
                logger.info("Running T3 exit sweep %s %s %s", spec.label, symbol, month)
                row = run_exit_silo(
                    spec=spec,
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    reentry_fill_policy=reentry_fill_policy,
                )
                candidate_silos.append(row)
                all_silos.append(row)
        metrics.append(compute_exit_metrics(spec, candidate_silos, months, symbols))
    return all_silos, metrics


def write_outputs(
    *,
    specs: list[T3ExitSpec],
    silos: list[T3ExitSiloResult],
    metrics: list[T3ExitCandidateMetrics],
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    deltas = compute_exit_deltas(metrics)
    payload = {
        "note": (
            "Research-only full lifecycle T3 exit sweep. Overrides apply only "
            "to t3_swing positions inside baseline_plus_t3; original_t2 is unchanged."
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
    (output_dir / "t3_lifecycle_exit_sweep_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    csv_rows = []
    for row in metrics:
        data = asdict(row)
        data["t3_exit_overrides"] = json.dumps(row.t3_exit_overrides, sort_keys=True)
        data["t3_exit_reasons"] = json.dumps(row.t3_exit_reasons, sort_keys=True)
        data["by_month"] = json.dumps(row.by_month, sort_keys=True)
        data["by_symbol"] = json.dumps(row.by_symbol, sort_keys=True)
        csv_rows.append(data)
    pd.DataFrame(csv_rows).to_csv(output_dir / "t3_lifecycle_exit_sweep_candidates.csv", index=False)

    lines = [
        "# T3 Lifecycle Exit Sweep",
        "",
        "Research-only full reentry-window lifecycle sweep. Overrides apply only to T3 positions.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Trades | T3 Net PnL | T3 Win Rate | T3 Exit Reasons | Overrides |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for row in metrics:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.calendar_avg_symbol_month_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} | {row.total_trades} "
            f"| {row.t3_trades} | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_win_rate_pct:.2f}% "
            f"| `{json.dumps(row.t3_exit_reasons, sort_keys=True)}` "
            f"| `{json.dumps(row.t3_exit_overrides, sort_keys=True)}` |"
        )

    if deltas:
        lines.extend(
            [
                "",
                "## Delta Vs Baseline",
                "",
                "| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T3 Trade Delta | T3 PnL Delta | Neg Silo Delta |",
                "|---|---:|---:|---:|---:|---:|---:|",
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
                f"| {row.negative_calendar_silos_delta:+d} |"
            )

    lines.extend(
        [
            "",
            "## Silo Detail",
            "",
            "| Candidate | Symbol | Month | Return | Trades | T3 Trades | T3 Net PnL | T3 Win Rate | T3 Exit Reasons |",
            "|---|---|---|---:|---:|---:|---:|---:|---|",
        ]
    )
    for row in silos:
        lines.append(
            f"| `{row.candidate}` | `{row.symbol}` | {row.month} "
            f"| {row.return_pct:.6f}% | {row.total_trades} | {row.t3_trades} "
            f"| {row.t3_net_pnl_pct:.6f}% | {row.t3_win_rate_pct:.2f}% "
            f"| `{json.dumps(row.t3_exit_reasons, sort_keys=True)}` |"
        )

    (output_dir / "t3_lifecycle_exit_sweep_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Lifecycle replay sweep for T3 exit overrides")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_lifecycle_exit_sweep")
    parser.add_argument("--candidate-set", choices=["compact"], default="compact")
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

    specs = filter_exit_specs(build_exit_specs(str(args.candidate_set)), args.labels)
    silos, metrics = run_exit_sweep(
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
        f"Best T3 exit candidate: {best.candidate} "
        f"calendar_sum={best.calendar_silo_sum_pct:.6f}%, "
        f"delta={best.calendar_silo_sum_pct - baseline.calendar_silo_sum_pct:+.6f}%, "
        f"t3_pnl={best.t3_net_pnl_pct:.6f}%, "
        f"t3_trades={best.t3_trades}"
    )
    print(f"Reports: {output_dir}/t3_lifecycle_exit_sweep_{{summary.json,report.md,candidates.csv}}")


if __name__ == "__main__":
    main()
