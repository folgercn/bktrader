"""Lifecycle replay comparison for T3 pre-touch caps.

This research-only tool tests whether the T3 ``pre_touch_seconds`` cap that
helped the timing/probability ledger still behaves under the full
``reentry_window`` lifecycle replay. It uses the shared breakout replay from
``research/eth_q1_breakout_t3_shape_compare.py`` and keeps each symbol-month as
an independent calendar silo.

The gate is applied only to ``t3_swing`` breakout locks. Original T2 locks keep
the same lifecycle path, so this is a lifecycle analogue of the T3 quality
filter, not a live-trading promotion.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from contextlib import contextmanager
from dataclasses import asdict, dataclass
from pathlib import Path

import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, _SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import btc_q1_30min_low_vol_entry_filters as btc_replay_profile  # noqa: E402
import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.multi_timeframe_builder import load_all_1s_bars  # noqa: E402

logger = logging.getLogger(__name__)

INITIAL_BALANCE = 100000.0
T3_REENTRY_SIZE_SCHEDULE = [0.20, 0.10]
DEFAULT_MONTHS = ["2026-02", "2026-03", "2026-04"]
DEFAULT_SYMBOLS = ["ETHUSDT", "BTCUSDT"]
DEFAULT_PRE_TOUCH_MAXES = [900.0, 600.0]


@dataclass
class LifecycleSiloResult:
    """One fixed calendar symbol-month replay result."""

    candidate: str
    t3_pre_touch_max: float
    symbol: str
    month: str
    return_pct: float
    final_balance: float
    trades: int
    win_rate_pct: float
    max_dd_pct: float
    t2_trades: int
    t3_trades: int
    t3_net_pnl_pct: float
    t2_net_pnl_pct: float
    t3_rejects: int
    breakout_locks: dict
    entry_reasons: dict
    exit_reasons: dict
    elapsed_seconds: float


@dataclass
class LifecycleCandidateMetrics:
    """Fixed-calendar aggregate for one candidate."""

    candidate: str
    t3_pre_touch_max: float
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
class LifecycleCandidateDelta:
    """Delta from the first lifecycle candidate."""

    candidate: str
    baseline_candidate: str
    calendar_silo_sum_delta_pct: float
    worst_calendar_silo_delta_pct: float
    trade_count_delta: int
    t3_trade_delta: int
    t3_net_pnl_delta_pct: float
    t3_reject_delta: int
    negative_calendar_silos_delta: int


_BARS_CACHE: dict[str, pd.DataFrame] = {}


def _candidate_label(pre_touch_max: float) -> str:
    return f"pre<={int(pre_touch_max)}"


def _month_bounds(month: str) -> tuple[pd.Timestamp, pd.Timestamp]:
    start = pd.Timestamp(f"{month}-01", tz="UTC")
    end = start + pd.offsets.MonthEnd(0) + pd.Timedelta(hours=23, minutes=59, seconds=59)
    return start, end


def _symbol_replay_kwargs(symbol: str) -> dict:
    if symbol == "BTCUSDT":
        return dict(btc_replay_profile.BTC_LIVE_LIKE_REPLAY_KWARGS)
    return dict(lifecycle.COMMON_REPLAY_KWARGS)


@contextmanager
def _patched_replay_kwargs(symbol: str):
    original = dict(lifecycle.COMMON_REPLAY_KWARGS)
    target = _symbol_replay_kwargs(symbol)
    lifecycle.COMMON_REPLAY_KWARGS.clear()
    lifecycle.COMMON_REPLAY_KWARGS.update(target)
    try:
        yield
    finally:
        lifecycle.COMMON_REPLAY_KWARGS.clear()
        lifecycle.COMMON_REPLAY_KWARGS.update(original)


def _load_window_bars(symbol: str, start: pd.Timestamp, end: pd.Timestamp) -> pd.DataFrame:
    if symbol not in _BARS_CACHE:
        logger.info("Loading all 1s bars for %s", symbol)
        bars = load_all_1s_bars(symbol)
        bars.index = pd.to_datetime(bars.index, utc=True)
        _BARS_CACHE[symbol] = bars
    bars = _BARS_CACHE[symbol]
    window = bars[(bars.index >= start) & (bars.index <= end)].copy()
    if window.empty:
        raise ValueError(f"no 1s bars for {symbol} in {start}..{end}")
    return window


def _nested_sum_counts(payload: dict) -> int:
    total = 0
    for value in payload.values():
        if isinstance(value, dict):
            total += sum(int(v) for v in value.values())
        else:
            total += int(value)
    return total


def _shape_attr(attribution: dict, shape: str) -> dict:
    return attribution.get(shape, {})


def _net_pnl_pct(attribution: dict, shape: str, initial_balance: float) -> float:
    shape_row = _shape_attr(attribution, shape)
    if not shape_row:
        return 0.0
    return float(shape_row.get("net_pnl_value", 0.0)) / float(initial_balance) * 100.0


def run_lifecycle_silo(
    *,
    symbol: str,
    month: str,
    pre_touch_max: float,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> LifecycleSiloResult:
    """Run one symbol-month lifecycle silo."""
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)

    started = time.time()
    candidate = _candidate_label(pre_touch_max)
    with _patched_replay_kwargs(symbol):
        ledger, diagnostics = lifecycle.run_second_bar_replay(
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
    rejects = diagnostics.get("t3_quality_rejects", {})
    locks = diagnostics.get("breakout_locks", {})

    return LifecycleSiloResult(
        candidate=candidate,
        t3_pre_touch_max=float(pre_touch_max),
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        trades=int(summary["trades"]),
        win_rate_pct=round(float(summary["win_rate_pct"]), 6),
        max_dd_pct=round(float(summary["max_dd_pct"]), 6),
        t2_trades=int(t2_attr.get("trades", 0)),
        t3_trades=int(t3_attr.get("trades", 0)),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        t2_net_pnl_pct=round(_net_pnl_pct(attribution, "original_t2", initial_balance), 6),
        t3_rejects=_nested_sum_counts(rejects),
        breakout_locks=locks,
        entry_reasons=summary.get("entry_reasons", {}),
        exit_reasons=summary.get("exit_reasons", {}),
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_candidate_metrics(
    candidate: str,
    pre_touch_max: float,
    silos: list[LifecycleSiloResult],
    months: list[str],
    symbols: list[str],
) -> LifecycleCandidateMetrics:
    """Aggregate one candidate on a fixed calendar grid."""
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    for month in months:
        for symbol in symbols:
            row = lookup.get((symbol, month))
            if row is None:
                grid_rows.append((symbol, month, 0.0, 0, 0, 0, 0.0, 0.0, 0))
            else:
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
    by_month = {
        month: round(float(sum(row[2] for row in grid_rows if row[1] == month)), 6)
        for month in months
    }
    by_symbol = {
        symbol: round(float(sum(row[2] for row in grid_rows if row[0] == symbol)), 6)
        for symbol in symbols
    }
    total_return = round(float(sum(returns)), 6)

    return LifecycleCandidateMetrics(
        candidate=candidate,
        t3_pre_touch_max=float(pre_touch_max),
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


def compute_deltas(metrics: list[LifecycleCandidateMetrics]) -> list[LifecycleCandidateDelta]:
    if not metrics:
        return []
    baseline = metrics[0]
    deltas = []
    for row in metrics[1:]:
        deltas.append(
            LifecycleCandidateDelta(
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
        )
    return deltas


def run_lifecycle_candidates(
    *,
    pre_touch_maxes: list[float],
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> tuple[list[LifecycleSiloResult], list[LifecycleCandidateMetrics]]:
    all_silos: list[LifecycleSiloResult] = []
    metrics: list[LifecycleCandidateMetrics] = []

    for pre_touch_max in pre_touch_maxes:
        candidate = _candidate_label(pre_touch_max)
        candidate_silos = []
        for symbol in symbols:
            for month in months:
                logger.info("Running lifecycle %s %s %s", candidate, symbol, month)
                row = run_lifecycle_silo(
                    symbol=symbol,
                    month=month,
                    pre_touch_max=pre_touch_max,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    reentry_fill_policy=reentry_fill_policy,
                )
                candidate_silos.append(row)
                all_silos.append(row)
        metrics.append(
            compute_candidate_metrics(
                candidate,
                pre_touch_max,
                candidate_silos,
                months,
                symbols,
            )
        )

    return all_silos, metrics


def write_outputs(
    *,
    silos: list[LifecycleSiloResult],
    metrics: list[LifecycleCandidateMetrics],
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    deltas = compute_deltas(metrics)
    payload = {
        "note": (
            "Research-only lifecycle replay of T3 pre-touch caps. T3 gate is "
            "applied to t3_swing breakout locks inside baseline_plus_t3; this "
            "does not include timing/RF model selection and is not a live promotion."
        ),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "candidates": [asdict(row) for row in metrics],
        "deltas_vs_first_candidate": [asdict(row) for row in deltas],
        "silos": [asdict(row) for row in silos],
    }
    (output_dir / "t3_pre_touch_lifecycle_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    lines = [
        "# T3 Pre-Touch Lifecycle Comparison",
        "",
        "Research-only full reentry-window lifecycle comparison. Missing symbol-months are counted as 0.0.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Traded/Flat | Trades | T2 Trades | T3 Trades | T3 Net PnL | T3 Rejects |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in metrics:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.calendar_avg_symbol_month_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} "
            f"| {row.traded_symbol_months}/{row.flat_symbol_months} "
            f"| {row.total_trades} | {row.t2_trades} | {row.t3_trades} "
            f"| {row.t3_net_pnl_pct:.6f}% | {row.t3_rejects} |"
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
            "| Candidate | Symbol | Month | Return | Trades | T2 Trades | T3 Trades | T3 PnL | T3 Rejects | Entry Reasons |",
            "|---|---|---|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for row in silos:
        lines.append(
            f"| `{row.candidate}` | `{row.symbol}` | {row.month} "
            f"| {row.return_pct:.6f}% | {row.trades} | {row.t2_trades} "
            f"| {row.t3_trades} | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_rejects} | `{json.dumps(row.entry_reasons, sort_keys=True)}` |"
        )

    (output_dir / "t3_pre_touch_lifecycle_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Lifecycle replay for T3 pre-touch caps")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_pre_touch_lifecycle")
    parser.add_argument("--pre-touch-maxes", nargs="+", type=float, default=DEFAULT_PRE_TOUCH_MAXES)
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

    silos, metrics = run_lifecycle_candidates(
        pre_touch_maxes=[float(value) for value in args.pre_touch_maxes],
        months=[str(value) for value in args.months],
        symbols=[str(value) for value in args.symbols],
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )
    output_dir = Path(args.output_dir)
    write_outputs(
        silos=silos,
        metrics=metrics,
        output_dir=output_dir,
        months=[str(value) for value in args.months],
        symbols=[str(value) for value in args.symbols],
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )

    best = max(metrics, key=lambda row: row.calendar_silo_sum_pct)
    print(
        f"Best lifecycle candidate: {best.candidate} "
        f"calendar_sum={best.calendar_silo_sum_pct:.6f}%, "
        f"worst_silo={best.worst_calendar_silo_pct:.6f}%, "
        f"trades={best.total_trades}, t3_trades={best.t3_trades}"
    )
    deltas = compute_deltas(metrics)
    if deltas:
        first = deltas[0]
        print(
            f"Delta: {first.candidate} vs {first.baseline_candidate} "
            f"calendar={first.calendar_silo_sum_delta_pct:+.6f}%, "
            f"t3_pnl={first.t3_net_pnl_delta_pct:+.6f}%"
        )
    print(f"Reports: {output_dir}/t3_pre_touch_lifecycle_{{summary.json,report.md}}")


if __name__ == "__main__":
    main()
