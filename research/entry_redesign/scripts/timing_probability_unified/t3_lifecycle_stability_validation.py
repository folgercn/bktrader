"""Cross-calendar stability validation for T3 lifecycle contribution.

This research-only tool runs ``baseline_plus_t3`` + ``reentry_window`` on a
fixed month x symbol grid. Unlike the exit sweep, it does not change T3 exits
or entry gates. The default fill policy is intentionally strict so a breakout
lock cannot become a same-second ``re_p`` fill.
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


@dataclass
class T3StabilitySiloResult:
    """One fixed-calendar symbol-month lifecycle baseline replay."""

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
    reentry_fill_rejects: dict
    elapsed_seconds: float


@dataclass
class T3StabilityMetrics:
    """Aggregate fixed-calendar stability metrics."""

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
    reentry_fill_rejects: dict
    positive_t3_silos: int
    negative_t3_silos: int
    flat_t3_silos: int
    worst_t3_silo_pct: float
    t3_exit_reasons: dict
    by_month: dict[str, float]
    by_year: dict[str, float]
    by_symbol: dict[str, float]
    t3_by_month: dict[str, float]
    t3_by_year: dict[str, float]
    t3_by_symbol: dict[str, float]


def _merge_counts(dst: dict[str, int], src: dict) -> None:
    for key, value in src.items():
        dst[str(key)] = int(dst.get(str(key), 0)) + int(value)


def _year(month: str) -> str:
    return str(month)[:4]


def run_stability_silo(
    *,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    pre_touch_max: float,
    reentry_fill_policy: str,
) -> T3StabilitySiloResult:
    """Run one symbol-month lifecycle baseline silo."""
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
            t3_quality_filters={"max_pre_touch_seconds": float(pre_touch_max)},
            quality_filter_shapes=["t3_swing"],
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t2_attr = _shape_attr(attribution, "original_t2")
    t3_attr = _shape_attr(attribution, "t3_swing")

    return T3StabilitySiloResult(
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
        reentry_fill_rejects=dict(_diagnostics.get("reentry_fill_rejects", {})),
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_stability_metrics(
    silos: list[T3StabilitySiloResult],
    months: list[str],
    symbols: list[str],
) -> T3StabilityMetrics:
    """Aggregate fixed month x symbol stability metrics with zero-filled gaps."""
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    exit_reasons: dict[str, int] = {}
    reentry_rejects: dict[str, int] = {}
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
            for side_reasons in row.reentry_fill_rejects.values():
                _merge_counts(reentry_rejects, side_reasons)

    total_slots = len(months) * len(symbols)
    returns = [float(row[2]) for row in grid_rows]
    t3_returns = [float(row[7]) for row in grid_rows]
    total_return = round(float(sum(returns)), 6)
    t3_trades = int(sum(row[5] for row in grid_rows))
    weighted_t3_win = float(sum(row[8] for row in grid_rows))

    return T3StabilityMetrics(
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
        t3_net_pnl_pct=round(float(sum(t3_returns)), 6),
        t3_win_rate_pct=round(weighted_t3_win / t3_trades, 6) if t3_trades else 0.0,
        reentry_fill_rejects=reentry_rejects,
        positive_t3_silos=int(sum(1 for value in t3_returns if value > 0.0)),
        negative_t3_silos=int(sum(1 for value in t3_returns if value < 0.0)),
        flat_t3_silos=int(sum(1 for value in t3_returns if value == 0.0)),
        worst_t3_silo_pct=round(float(min(t3_returns, default=0.0)), 6),
        t3_exit_reasons=exit_reasons,
        by_month={
            month: round(float(sum(row[2] for row in grid_rows if row[1] == month)), 6)
            for month in months
        },
        by_year={
            year: round(float(sum(row[2] for row in grid_rows if _year(row[1]) == year)), 6)
            for year in sorted({_year(month) for month in months})
        },
        by_symbol={
            symbol: round(float(sum(row[2] for row in grid_rows if row[0] == symbol)), 6)
            for symbol in symbols
        },
        t3_by_month={
            month: round(float(sum(row[7] for row in grid_rows if row[1] == month)), 6)
            for month in months
        },
        t3_by_year={
            year: round(float(sum(row[7] for row in grid_rows if _year(row[1]) == year)), 6)
            for year in sorted({_year(month) for month in months})
        },
        t3_by_symbol={
            symbol: round(float(sum(row[7] for row in grid_rows if row[0] == symbol)), 6)
            for symbol in symbols
        },
    )


def run_stability_validation(
    *,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    pre_touch_max: float,
    reentry_fill_policy: str,
) -> tuple[list[T3StabilitySiloResult], T3StabilityMetrics]:
    silos = []
    for symbol in symbols:
        for month in months:
            logger.info("Running T3 lifecycle stability %s %s", symbol, month)
            silos.append(
                run_stability_silo(
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    pre_touch_max=pre_touch_max,
                    reentry_fill_policy=reentry_fill_policy,
                )
            )
    return silos, compute_stability_metrics(silos, months, symbols)


def write_outputs(
    *,
    silos: list[T3StabilitySiloResult],
    metrics: T3StabilityMetrics,
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    pre_touch_max: float,
    reentry_fill_policy: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "note": (
            "Research-only T3 lifecycle cross-calendar stability validation. "
            "This keeps baseline_plus_t3/reentry_window behavior unchanged and "
            "uses fixed month x symbol reporting with zero-filled gaps. The "
            "strict_next_second_cross fill policy rejects same-second reentry "
            "fills and requires a reclaim cross through re_p."
        ),
        "timeframe": timeframe,
        "t3_pre_touch_max": float(pre_touch_max),
        "reentry_fill_policy": reentry_fill_policy,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "metrics": asdict(metrics),
        "silos": [asdict(row) for row in silos],
    }
    (output_dir / "t3_lifecycle_stability_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    pd.DataFrame([asdict(row) for row in silos]).to_csv(
        output_dir / "t3_lifecycle_stability_silos.csv",
        index=False,
    )

    lines = [
        "# T3 Lifecycle Stability Validation",
        "",
        "Research-only cross-calendar lifecycle validation. No T3 exit or entry override is applied beyond the baseline T3 quality cap.",
        "",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Timeframe: `{timeframe}`",
        f"- T3 pre-touch max: `{pre_touch_max}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Aggregate",
        "",
        "| Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Trades | T3 Net PnL | T3 Win Rate | Positive/Negative/Flat T3 Silos | Worst T3 Silo |",
        "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        f"| {metrics.calendar_silo_sum_pct:.6f}% "
        f"| {metrics.calendar_avg_symbol_month_pct:.6f}% "
        f"| {metrics.worst_calendar_silo_pct:.6f}% "
        f"| {metrics.negative_calendar_silos} "
        f"| {metrics.total_trades} "
        f"| {metrics.t3_trades} "
        f"| {metrics.t3_net_pnl_pct:.6f}% "
        f"| {metrics.t3_win_rate_pct:.2f}% "
        f"| {metrics.positive_t3_silos}/{metrics.negative_t3_silos}/{metrics.flat_t3_silos} "
        f"| {metrics.worst_t3_silo_pct:.6f}% |",
        "",
        "## Reentry Fill Rejects",
        "",
        "| Reason | Count |",
        "|---|---:|",
    ]
    for reason, count in sorted(metrics.reentry_fill_rejects.items()):
        lines.append(f"| `{reason}` | {count} |")

    lines.extend(
        [
            "",
            "## By Year",
            "",
            "| Year | Calendar Sum | T3 Net PnL |",
            "|---|---:|---:|",
        ]
    )
    for year in metrics.by_year:
        lines.append(
            f"| {year} | {metrics.by_year[year]:.6f}% | {metrics.t3_by_year.get(year, 0.0):.6f}% |"
        )

    lines.extend(
        [
            "",
            "## By Symbol",
            "",
            "| Symbol | Calendar Sum | T3 Net PnL |",
            "|---|---:|---:|",
        ]
    )
    for symbol in symbols:
        lines.append(
            f"| `{symbol}` | {metrics.by_symbol.get(symbol, 0.0):.6f}% "
            f"| {metrics.t3_by_symbol.get(symbol, 0.0):.6f}% |"
        )

    lines.extend(
        [
            "",
            "## By Month",
            "",
            "| Month | Calendar Sum | T3 Net PnL |",
            "|---|---:|---:|",
        ]
    )
    for month in months:
        lines.append(
            f"| {month} | {metrics.by_month.get(month, 0.0):.6f}% "
            f"| {metrics.t3_by_month.get(month, 0.0):.6f}% |"
        )

    lines.extend(
        [
            "",
            "## Silo Detail",
            "",
            "| Symbol | Month | Return | Trades | T2 Trades | T3 Trades | T2 Net PnL | T3 Net PnL | T3 Win Rate | T3 Exit Reasons |",
            "|---|---|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for row in silos:
        lines.append(
            f"| `{row.symbol}` | {row.month} | {row.return_pct:.6f}% "
            f"| {row.total_trades} | {row.t2_trades} | {row.t3_trades} "
            f"| {row.t2_net_pnl_pct:.6f}% | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_win_rate_pct:.2f}% "
            f"| `{json.dumps(row.t3_exit_reasons, sort_keys=True)}` |"
        )

    (output_dir / "t3_lifecycle_stability_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="T3 lifecycle cross-calendar stability validation")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_lifecycle_stability_validation")
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbols", nargs="+", default=DEFAULT_SYMBOLS)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--pre-touch-max", type=float, default=900.0)
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

    months = [str(value) for value in args.months]
    symbols = [str(value) for value in args.symbols]
    silos, metrics = run_stability_validation(
        months=months,
        symbols=symbols,
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        pre_touch_max=float(args.pre_touch_max),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )
    output_dir = Path(args.output_dir)
    write_outputs(
        silos=silos,
        metrics=metrics,
        output_dir=output_dir,
        months=months,
        symbols=symbols,
        timeframe=str(args.timeframe),
        pre_touch_max=float(args.pre_touch_max),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )

    print(
        f"T3 lifecycle stability: calendar_sum={metrics.calendar_silo_sum_pct:.6f}%, "
        f"worst_silo={metrics.worst_calendar_silo_pct:.6f}%, "
        f"t3_pnl={metrics.t3_net_pnl_pct:.6f}%, "
        f"t3_silos=+{metrics.positive_t3_silos}/-{metrics.negative_t3_silos}/"
        f"0{metrics.flat_t3_silos}, output={output_dir}"
    )


if __name__ == "__main__":
    main()
