"""Fixed-calendar comparison for T3 pre-touch quality-gate candidates.

This research-only diagnostic reruns the rolling T2/T3 union pipeline for a
set of T3 pre-touch caps, then evaluates each result on the same zero-filled
calendar grid. It is stricter than active-row summaries because months/symbols
with no trades are explicit 0 return silos.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from dataclasses import asdict, dataclass
from pathlib import Path

import pandas as pd

_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from timing_probability_unified.union_strategy_runner import (
    RollingWindowResult,
    UnionStrategyConfig,
    generate_union_report_json,
    generate_union_report_md,
    run_rolling_windows,
)

logger = logging.getLogger(__name__)


@dataclass
class CalendarCandidateMetrics:
    """Zero-filled fixed-calendar metrics for one pre-touch candidate."""

    label: str
    t3_pre_touch_max: float
    calendar_silo_sum: float
    calendar_avg_symbol_month: float
    worst_calendar_silo: float
    negative_calendar_silos: int
    traded_symbol_months: int
    flat_symbol_months: int
    union_trade_count: int
    t2_calendar_sum: float
    t3_calendar_sum: float
    month_totals: dict[str, float]
    pool_month_totals: dict[str, dict[str, float]]
    silos: list[dict]


@dataclass
class CandidateDelta:
    """Delta from the first candidate in the comparison set."""

    label: str
    baseline_label: str
    calendar_silo_sum_delta: float
    t3_calendar_sum_delta: float
    worst_calendar_silo_delta: float
    trade_count_delta: int
    negative_calendar_silos_delta: int


def _candidate_label(pre_touch_max: float) -> str:
    return f"pre<={int(pre_touch_max)}"


def _calendar_months_from_starts(rolling_months: list[str]) -> list[str]:
    return [pd.Timestamp(month).strftime("%Y-%m") for month in rolling_months]


def _active_trades(trades: pd.DataFrame) -> pd.DataFrame:
    """Keep rows that represent actual entries under timing and speed gates."""
    if trades.empty:
        return trades.copy()
    return trades[
        (trades["timing_prediction"] != "skip")
        & (trades["speed_gate_pass"] == True)  # noqa: E712
    ].copy()


def _fixed_calendar_silos(
    trades: pd.DataFrame,
    months: list[str],
    symbols: list[str],
) -> list[dict]:
    """Build a zero-filled symbol-month grid from an active trade ledger."""
    active = _active_trades(trades)
    lookup: dict[tuple[str, str], dict[str, float | int]] = {}

    if not active.empty:
        active = active.copy()
        active["year_month"] = pd.to_datetime(active["touch_time"], utc=True).dt.strftime("%Y-%m")
        grouped = active.groupby(["symbol", "year_month"])
        for (symbol, month), group in grouped:
            lookup[(str(symbol), str(month))] = {
                "calendar_pnl": float(group["weighted_pnl"].sum()),
                "trades": int(len(group)),
            }

    silos = []
    for month in months:
        for symbol in symbols:
            values = lookup.get((symbol, month), {"calendar_pnl": 0.0, "trades": 0})
            pnl = float(values["calendar_pnl"])
            trades_count = int(values["trades"])
            silos.append(
                {
                    "month": month,
                    "symbol": symbol,
                    "calendar_pnl": round(pnl, 6),
                    "trades": trades_count,
                    "flat": trades_count == 0,
                }
            )
    return silos


def _pool_month_totals(
    trades: pd.DataFrame,
    months: list[str],
) -> dict[str, dict[str, float]]:
    """Return fixed month totals split by T2/T3 pool."""
    totals = {month: {"T2": 0.0, "T3": 0.0, "union": 0.0} for month in months}
    active = _active_trades(trades)
    if active.empty or "pool" not in active.columns:
        return totals

    active = active.copy()
    active["year_month"] = pd.to_datetime(active["touch_time"], utc=True).dt.strftime("%Y-%m")
    grouped = active.groupby(["year_month", "pool"])["weighted_pnl"].sum()
    for (month, pool), value in grouped.items():
        if month not in totals or pool not in ("T2", "T3"):
            continue
        totals[month][str(pool)] = round(float(value), 6)

    for month, values in totals.items():
        values["union"] = round(values["T2"] + values["T3"], 6)
    return totals


def compute_calendar_candidate_metrics(
    label: str,
    t3_pre_touch_max: float,
    rolling: RollingWindowResult,
    months: list[str],
    symbols: list[str],
) -> CalendarCandidateMetrics:
    """Compute fixed-calendar metrics for a rolling union result."""
    trades = rolling.combined_forward_trades
    silos = _fixed_calendar_silos(trades, months, symbols)
    pool_months = _pool_month_totals(trades, months)

    calendar_silo_sum = round(float(sum(row["calendar_pnl"] for row in silos)), 6)
    total_slots = len(months) * len(symbols)
    calendar_avg = round(calendar_silo_sum / total_slots, 6) if total_slots else 0.0
    worst_silo = round(float(min((row["calendar_pnl"] for row in silos), default=0.0)), 6)
    negative_silos = int(sum(1 for row in silos if row["calendar_pnl"] < 0.0))
    traded_silos = int(sum(1 for row in silos if not row["flat"]))

    t2_total = round(float(sum(values["T2"] for values in pool_months.values())), 6)
    t3_total = round(float(sum(values["T3"] for values in pool_months.values())), 6)
    month_totals = {
        month: round(float(values["union"]), 6)
        for month, values in pool_months.items()
    }

    return CalendarCandidateMetrics(
        label=label,
        t3_pre_touch_max=t3_pre_touch_max,
        calendar_silo_sum=calendar_silo_sum,
        calendar_avg_symbol_month=calendar_avg,
        worst_calendar_silo=worst_silo,
        negative_calendar_silos=negative_silos,
        traded_symbol_months=traded_silos,
        flat_symbol_months=total_slots - traded_silos,
        union_trade_count=int(rolling.total_trade_count),
        t2_calendar_sum=t2_total,
        t3_calendar_sum=t3_total,
        month_totals=month_totals,
        pool_month_totals=pool_months,
        silos=silos,
    )


def compute_deltas(metrics: list[CalendarCandidateMetrics]) -> list[CandidateDelta]:
    """Compare every candidate to the first candidate."""
    if not metrics:
        return []
    baseline = metrics[0]
    deltas = []
    for row in metrics[1:]:
        deltas.append(
            CandidateDelta(
                label=row.label,
                baseline_label=baseline.label,
                calendar_silo_sum_delta=round(
                    row.calendar_silo_sum - baseline.calendar_silo_sum,
                    6,
                ),
                t3_calendar_sum_delta=round(
                    row.t3_calendar_sum - baseline.t3_calendar_sum,
                    6,
                ),
                worst_calendar_silo_delta=round(
                    row.worst_calendar_silo - baseline.worst_calendar_silo,
                    6,
                ),
                trade_count_delta=row.union_trade_count - baseline.union_trade_count,
                negative_calendar_silos_delta=(
                    row.negative_calendar_silos - baseline.negative_calendar_silos
                ),
            )
        )
    return deltas


def run_candidates(
    pre_touch_maxes: list[float],
    rolling_months: list[str],
    output_dir: Path,
    write_union_reports: bool,
) -> list[CalendarCandidateMetrics]:
    """Run each candidate and return fixed-calendar metrics."""
    months = _calendar_months_from_starts(rolling_months)
    symbols = ["ETHUSDT"]
    metrics: list[CalendarCandidateMetrics] = []

    for pre_touch_max in pre_touch_maxes:
        label = _candidate_label(pre_touch_max)
        logger.info("Running %s", label)
        config = UnionStrategyConfig(
            rolling_months=rolling_months,
            t3_pre_touch_max=pre_touch_max,
        )
        rolling = run_rolling_windows(config)

        if write_union_reports:
            candidate_dir = output_dir / label.replace("<=", "le")
            generate_union_report_json(rolling, candidate_dir)
            generate_union_report_md(rolling, candidate_dir)

        metrics.append(
            compute_calendar_candidate_metrics(
                label=label,
                t3_pre_touch_max=pre_touch_max,
                rolling=rolling,
                months=months,
                symbols=symbols,
            )
        )

    return metrics


def write_outputs(
    metrics: list[CalendarCandidateMetrics],
    output_dir: Path,
    months: list[str],
    symbols: list[str],
) -> None:
    """Write JSON and Markdown comparison reports."""
    output_dir.mkdir(parents=True, exist_ok=True)
    deltas = compute_deltas(metrics)

    payload = {
        "note": (
            "Fixed-calendar zero-filled comparison for T3 pre-touch quality "
            "gate candidates. This reruns the timing/probability union runner; "
            "it is not a live promotion or a production lifecycle claim."
        ),
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "candidates": [asdict(row) for row in metrics],
        "deltas_vs_first_candidate": [asdict(row) for row in deltas],
    }
    (output_dir / "t3_pre_touch_holdout_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    lines = [
        "# T3 Pre-Touch Fixed Calendar Comparison",
        "",
        "Research-only fixed-calendar comparison. Missing symbol-months are counted as `0.0`; this is stricter than active-row-only summaries.",
        "",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Union CS | T2 CS | T3 CS | Worst Silo | Neg Silos | Traded/Flat Silos | Trades |",
        "|---|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in metrics:
        lines.append(
            f"| `{row.label}` | {row.calendar_silo_sum:.6f} "
            f"| {row.t2_calendar_sum:.6f} | {row.t3_calendar_sum:.6f} "
            f"| {row.worst_calendar_silo:.6f} | {row.negative_calendar_silos} "
            f"| {row.traded_symbol_months}/{row.flat_symbol_months} "
            f"| {row.union_trade_count} |"
        )

    if deltas:
        lines.extend(
            [
                "",
                "## Delta Vs Baseline",
                "",
                "| Candidate | Union Delta | T3 Delta | Worst Silo Delta | Trade Delta | Neg Silo Delta |",
                "|---|---:|---:|---:|---:|---:|",
            ]
        )
        for row in deltas:
            lines.append(
                f"| `{row.label}` vs `{row.baseline_label}` "
                f"| {row.calendar_silo_sum_delta:+.6f} "
                f"| {row.t3_calendar_sum_delta:+.6f} "
                f"| {row.worst_calendar_silo_delta:+.6f} "
                f"| {row.trade_count_delta:+d} "
                f"| {row.negative_calendar_silos_delta:+d} |"
            )

    lines.extend(
        [
            "",
            "## Monthly Pool Attribution",
            "",
            "| Candidate | Month | T2 | T3 | Union |",
            "|---|---|---:|---:|---:|",
        ]
    )
    for row in metrics:
        for month in months:
            values = row.pool_month_totals[month]
            lines.append(
                f"| `{row.label}` | {month} | {values['T2']:.6f} "
                f"| {values['T3']:.6f} | {values['union']:.6f} |"
            )

    (output_dir / "t3_pre_touch_holdout_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Fixed-calendar comparison for T3 pre-touch candidates"
    )
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_pre_touch_holdout")
    parser.add_argument("--pre-touch-maxes", nargs="+", type=float, default=[900.0, 600.0])
    parser.add_argument(
        "--rolling-months",
        nargs="+",
        default=["2026-02-01", "2026-03-01", "2026-04-01"],
    )
    parser.add_argument(
        "--write-union-reports",
        action="store_true",
        help="Also write the underlying union_strategy_report files per candidate.",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    output_dir = Path(args.output_dir)
    metrics = run_candidates(
        pre_touch_maxes=[float(value) for value in args.pre_touch_maxes],
        rolling_months=[str(value) for value in args.rolling_months],
        output_dir=output_dir,
        write_union_reports=bool(args.write_union_reports),
    )
    months = _calendar_months_from_starts([str(value) for value in args.rolling_months])
    symbols = ["ETHUSDT"]
    write_outputs(metrics, output_dir, months, symbols)

    deltas = compute_deltas(metrics)
    best = max(metrics, key=lambda row: row.calendar_silo_sum)
    print(
        f"Best candidate: {best.label} "
        f"union_cs={best.calendar_silo_sum:.6f}, "
        f"t3_cs={best.t3_calendar_sum:.6f}, "
        f"worst_silo={best.worst_calendar_silo:.6f}"
    )
    if deltas:
        first_delta = deltas[0]
        print(
            f"Delta: {first_delta.label} vs {first_delta.baseline_label} "
            f"union={first_delta.calendar_silo_sum_delta:+.6f}, "
            f"t3={first_delta.t3_calendar_sum_delta:+.6f}"
        )
    print(f"Reports: {output_dir}/t3_pre_touch_holdout_{{summary.json,report.md}}")


if __name__ == "__main__":
    main()
