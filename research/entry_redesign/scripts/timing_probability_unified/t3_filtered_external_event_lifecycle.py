"""Strict lifecycle replay for filtered T3 external event sources.

Research-only. This runner takes scored T3 events, applies simple explanatory
filters such as ``short + speed_abs>=0.35``, then injects the remaining events
as explicit ``t3_swing`` breakout locks. Native original_t2 and native t3_swing
locks are disabled so the result measures only the filtered event source under
the same strict lifecycle and T3 60m exit contract.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from dataclasses import asdict, dataclass
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
from timing_probability_unified.t2_lifecycle_context_sizing import EXTENDED_MONTHS  # noqa: E402
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

OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)
DEFAULT_SCORED_EVENTS = OUTPUT_DIR / "t3_probability_overlay_extended" / "t3_probability_overlay_scored_events.csv"
T3_60M_EXIT_OVERRIDES = {"min_hold_seconds_before_sl": 3600.0}


@dataclass(frozen=True)
class FilteredT3Spec:
    """One filtered T3 event-source candidate."""

    label: str
    side: str | None = None
    speed_abs_min: float | None = None
    speed_abs_max: float | None = None
    extension_abs_min: float | None = None
    extension_abs_max: float | None = None
    rf_min: float | None = None
    rf_max: float | None = None
    timing: str | None = None
    read: str = ""


@dataclass
class FilteredT3Silo:
    """One fixed symbol-month lifecycle result."""

    candidate: str
    symbol: str
    month: str
    return_pct: float
    final_balance: float
    total_trades: int
    t3_trades: int
    t3_net_pnl_pct: float
    events_available: int
    external_locks: int
    external_lock_reasons: dict
    reentry_fill_rejects: dict
    elapsed_seconds: float


@dataclass
class FilteredT3Metrics:
    """Aggregate fixed-calendar metrics for one candidate."""

    candidate: str
    filters: dict[str, Any]
    calendar_silo_sum_pct: float
    calendar_avg_symbol_month_pct: float
    worst_calendar_silo_pct: float
    negative_calendar_silos: int
    traded_symbol_months: int
    flat_symbol_months: int
    total_trades: int
    t3_trades: int
    t3_net_pnl_pct: float
    events_available: int
    external_locks: int
    reentry_fill_rejects: dict
    by_month: dict[str, float]
    by_symbol: dict[str, float]


def build_specs(candidate_set: str) -> list[FilteredT3Spec]:
    specs = [
        FilteredT3Spec(
            "short_all",
            side="short",
            read="all short-side T3 events; sanity check against native short-only lifecycle",
        ),
        FilteredT3Spec(
            "short_speed_abs_ge_0p35",
            side="short",
            speed_abs_min=0.35,
            read="short T3 events with absolute 300s speed >= 0.35 ATR",
        ),
        FilteredT3Spec(
            "long_speed_abs_ge_0p35",
            side="long",
            speed_abs_min=0.35,
            read="long T3 events with absolute 300s speed >= 0.35 ATR; direct-entry diagnostic",
        ),
        FilteredT3Spec(
            "all_speed_abs_ge_0p35",
            speed_abs_min=0.35,
            read="both sides with absolute 300s speed >= 0.35 ATR; direct-entry diagnostic",
        ),
        FilteredT3Spec(
            "short_speed_abs_0p35_0p50",
            side="short",
            speed_abs_min=0.35,
            speed_abs_max=0.50,
            read="short T3 events in the strongest overlay speed bucket",
        ),
        FilteredT3Spec(
            "short_ext_abs_0p02_0p05",
            side="short",
            extension_abs_min=0.02,
            extension_abs_max=0.05,
            read="short T3 events with moderate touch extension 0.02-0.05 ATR",
        ),
        FilteredT3Spec(
            "short_ext_abs_0p10_0p20",
            side="short",
            extension_abs_min=0.10,
            extension_abs_max=0.20,
            read="short T3 events with larger touch extension 0.10-0.20 ATR; tiny-sample diagnostic",
        ),
        FilteredT3Spec(
            "short_rf_0p50_0p55",
            side="short",
            rf_min=0.50,
            rf_max=0.55,
            read="non-monotonic RF bucket that looked good in the overlay audit; tiny-sample diagnostic",
        ),
    ]
    if candidate_set == "focused":
        return specs
    if candidate_set == "smoke":
        labels = {"short_all", "short_speed_abs_ge_0p35"}
        return [spec for spec in specs if spec.label in labels]
    raise ValueError(f"unknown candidate set: {candidate_set}")


def filter_specs(specs: list[FilteredT3Spec], labels: list[str] | None) -> list[FilteredT3Spec]:
    if not labels:
        return specs
    lookup = {spec.label: spec for spec in specs}
    missing = [label for label in labels if label not in lookup]
    if missing:
        raise ValueError(f"unknown candidate labels: {', '.join(missing)}")
    return [lookup[label] for label in labels]


def load_scored_events(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    events = pd.read_csv(path)
    for column in ("touch_time", "signal_start", "signal_end"):
        if column in events.columns:
            events[column] = pd.to_datetime(events[column], utc=True)
    return events


def apply_spec(events: pd.DataFrame, spec: FilteredT3Spec) -> pd.DataFrame:
    if events.empty:
        return events.copy()
    mask = pd.Series(True, index=events.index)
    if spec.side is not None:
        mask &= events["side"].astype(str) == spec.side
    if spec.speed_abs_min is not None or spec.speed_abs_max is not None:
        speed_abs = pd.to_numeric(events["speed_300s_atr"], errors="coerce").abs()
        if spec.speed_abs_min is not None:
            mask &= speed_abs >= float(spec.speed_abs_min)
        if spec.speed_abs_max is not None:
            mask &= speed_abs < float(spec.speed_abs_max)
    if spec.extension_abs_min is not None or spec.extension_abs_max is not None:
        extension_abs = pd.to_numeric(events["touch_extension_atr"], errors="coerce").abs()
        if spec.extension_abs_min is not None:
            mask &= extension_abs >= float(spec.extension_abs_min)
        if spec.extension_abs_max is not None:
            mask &= extension_abs < float(spec.extension_abs_max)
    if spec.rf_min is not None or spec.rf_max is not None:
        rf = pd.to_numeric(events["rf_probability"], errors="coerce")
        if spec.rf_min is not None:
            mask &= rf >= float(spec.rf_min)
        if spec.rf_max is not None:
            mask &= rf < float(spec.rf_max)
    if spec.timing is not None:
        mask &= events["timing_prediction"].astype(str) == spec.timing
    return events[mask].copy()


def _window_events(events: pd.DataFrame, symbol: str, start: pd.Timestamp, end: pd.Timestamp) -> pd.DataFrame:
    if events.empty:
        return events.copy()
    mask = (
        (events["symbol"].astype(str) == symbol)
        & (events["touch_time"] >= start)
        & (events["touch_time"] <= end)
    )
    return events[mask].copy()


def _flat_counts(nested: dict[str, Any]) -> dict[str, int]:
    out: dict[str, int] = {}
    for side_counts in nested.values():
        for reason, count in dict(side_counts).items():
            out[str(reason)] = int(out.get(str(reason), 0)) + int(count)
    return out


def run_silo(
    *,
    spec: FilteredT3Spec,
    filtered_events: pd.DataFrame,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
    external_entry_mode: str,
    t3_size_scale: float,
) -> FilteredT3Silo:
    start, end = _month_bounds(month)
    external_events = _window_events(filtered_events, symbol, start, end)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)

    started = time.time()
    t3_schedule = [float(size) * float(t3_size_scale) for size in T3_REENTRY_SIZE_SCHEDULE]
    with _patched_replay_kwargs(symbol):
        ledger, diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=t3_schedule,
            t3_cooldown_bars=0,
            t3_quality_filters={"allowed_sides": []},
            quality_filter_shapes=["t3_swing"],
            shape_sizing_filters={"allowed_sides": []},
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=0.0,
            sizing_filter_fail_action="skip_lock",
            t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
            external_breakout_events=external_events,
            external_breakout_shape_name="t3_swing",
            external_entry_mode=external_entry_mode,
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t3_attr = _shape_attr(attribution, "t3_swing")
    external_locks = _flat_counts(diagnostics.get("external_breakout_locks", {}))
    reentry_rejects = _flat_counts(diagnostics.get("reentry_fill_rejects", {}))
    return FilteredT3Silo(
        candidate=spec.label,
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        total_trades=int(summary["trades"]),
        t3_trades=int(t3_attr.get("trades", 0)),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        events_available=int(len(external_events)),
        external_locks=int(sum(external_locks.values())),
        external_lock_reasons=external_locks,
        reentry_fill_rejects=reentry_rejects,
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_metrics(
    spec: FilteredT3Spec,
    silos: list[FilteredT3Silo],
    months: list[str],
    symbols: list[str],
) -> FilteredT3Metrics:
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    reentry_rejects: dict[str, int] = {}
    for month in months:
        for symbol in symbols:
            row = lookup.get((symbol, month))
            if row is None:
                grid_rows.append((symbol, month, 0.0, 0, 0, 0.0, 0, 0))
                continue
            grid_rows.append(
                (
                    symbol,
                    month,
                    row.return_pct,
                    row.total_trades,
                    row.t3_trades,
                    row.t3_net_pnl_pct,
                    row.events_available,
                    row.external_locks,
                )
            )
            for reason, count in row.reentry_fill_rejects.items():
                reentry_rejects[reason] = int(reentry_rejects.get(reason, 0)) + int(count)

    total_slots = len(months) * len(symbols)
    returns = [float(row[2]) for row in grid_rows]
    total_return = round(float(sum(returns)), 6)
    filters = asdict(spec)
    filters.pop("read", None)
    filters.pop("label", None)
    return FilteredT3Metrics(
        candidate=spec.label,
        filters={key: value for key, value in filters.items() if value is not None},
        calendar_silo_sum_pct=total_return,
        calendar_avg_symbol_month_pct=round(total_return / total_slots, 6) if total_slots else 0.0,
        worst_calendar_silo_pct=round(float(min(returns, default=0.0)), 6),
        negative_calendar_silos=int(sum(1 for value in returns if value < 0.0)),
        traded_symbol_months=int(sum(1 for row in grid_rows if row[3] > 0)),
        flat_symbol_months=int(sum(1 for row in grid_rows if row[3] == 0)),
        total_trades=int(sum(row[3] for row in grid_rows)),
        t3_trades=int(sum(row[4] for row in grid_rows)),
        t3_net_pnl_pct=round(float(sum(row[5] for row in grid_rows)), 6),
        events_available=int(sum(row[6] for row in grid_rows)),
        external_locks=int(sum(row[7] for row in grid_rows)),
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


def run_validation(
    *,
    specs: list[FilteredT3Spec],
    scored_events: pd.DataFrame,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
    external_entry_mode: str,
    t3_size_scale: float,
) -> tuple[list[FilteredT3Silo], list[FilteredT3Metrics]]:
    all_silos: list[FilteredT3Silo] = []
    metrics: list[FilteredT3Metrics] = []
    for spec in specs:
        filtered_events = apply_spec(scored_events, spec)
        candidate_silos = []
        for symbol in symbols:
            for month in months:
                logger.info("Running filtered T3 lifecycle %s %s %s", spec.label, symbol, month)
                row = run_silo(
                    spec=spec,
                    filtered_events=filtered_events,
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    reentry_fill_policy=reentry_fill_policy,
                    external_entry_mode=external_entry_mode,
                    t3_size_scale=t3_size_scale,
                )
                candidate_silos.append(row)
                all_silos.append(row)
        metrics.append(compute_metrics(spec, candidate_silos, months, symbols))
    return all_silos, metrics


def write_outputs(
    *,
    output_dir: Path,
    specs: list[FilteredT3Spec],
    silos: list[FilteredT3Silo],
    metrics: list[FilteredT3Metrics],
    months: list[str],
    symbols: list[str],
    scored_events_path: Path,
    timeframe: str,
    reentry_fill_policy: str,
    external_entry_mode: str,
    t3_size_scale: float,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "note": (
            "Research-only strict lifecycle replay for filtered scored T3 event sources. Native original_t2 "
            "and native t3_swing locks are disabled; external events are injected as t3_swing so T3 60m exit "
            "overrides still apply."
        ),
        "scored_events_path": str(scored_events_path),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "external_entry_mode": external_entry_mode,
        "t3_size_scale": float(t3_size_scale),
        "t3_reentry_size_schedule": [float(size) * float(t3_size_scale) for size in T3_REENTRY_SIZE_SCHEDULE],
        "t3_exit_overrides": T3_60M_EXIT_OVERRIDES,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "candidate_specs": [asdict(spec) for spec in specs],
        "candidates": [asdict(row) for row in metrics],
        "silos": [asdict(row) for row in silos],
    }
    (output_dir / "t3_filtered_external_event_lifecycle_summary.json").write_text(
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
    pd.DataFrame(rows).to_csv(output_dir / "t3_filtered_external_event_lifecycle_candidates.csv", index=False)

    reads = {spec.label: spec.read for spec in specs}
    lines = [
        "# T3 Filtered External Event Lifecycle",
        "",
        "Research-only strict lifecycle replay for filtered scored T3 event sources.",
        "",
        f"- Scored events: `{scored_events_path}`",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- External entry mode: `{external_entry_mode}`",
        f"- T3 size scale: `{t3_size_scale}`",
        f"- T3 reentry size schedule: `{json.dumps([float(size) * float(t3_size_scale) for size in T3_REENTRY_SIZE_SCHEDULE])}`",
        f"- T3 exit overrides: `{json.dumps(T3_60M_EXIT_OVERRIDES, sort_keys=True)}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Net PnL | Events | Locks | Filters | Read |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for row in metrics:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.calendar_avg_symbol_month_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} "
            f"| {row.total_trades} "
            f"| {row.t3_net_pnl_pct:.6f}% "
            f"| {row.events_available} "
            f"| {row.external_locks} "
            f"| `{json.dumps(row.filters, sort_keys=True)}` "
            f"| {reads.get(row.candidate, '')} |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- This isolates filtered T3 event sources; it is not mixed with native original_t2 or native t3_swing.",
            "- `reentry_window` is promotion-comparable to the strict lifecycle floor; `next_second_*` modes are entry-redesign diagnostics.",
            "- Tiny-sample buckets are useful only as direction finders before broader validation.",
        ]
    )
    (output_dir / "t3_filtered_external_event_lifecycle_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate-set", choices=["focused", "smoke"], default="focused")
    parser.add_argument("--labels", nargs="+", default=None)
    parser.add_argument("--scored-events", type=Path, default=DEFAULT_SCORED_EVENTS)
    parser.add_argument(
        "--external-entry-mode",
        choices=["reentry_window", "next_second_open", "next_second_close", "next_second_adverse"],
        default="reentry_window",
    )
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbols", nargs="+", default=DEFAULT_SYMBOLS)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--reentry-fill-policy", default="strict_next_second_cross")
    parser.add_argument(
        "--t3-size-scale",
        type=float,
        default=1.0,
        help="Research-only multiplier for the T3 [0.20, 0.10] size schedule; default preserves baseline.",
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=OUTPUT_DIR / "t3_filtered_external_events_extended",
    )
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    specs = filter_specs(build_specs(str(args.candidate_set)), args.labels)
    scored_events = load_scored_events(Path(args.scored_events))
    silos, metrics = run_validation(
        specs=specs,
        scored_events=scored_events,
        months=list(args.months),
        symbols=list(args.symbols),
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        reentry_fill_policy=str(args.reentry_fill_policy),
        external_entry_mode=str(args.external_entry_mode),
        t3_size_scale=float(args.t3_size_scale),
    )
    write_outputs(
        output_dir=Path(args.output_dir),
        specs=specs,
        silos=silos,
        metrics=metrics,
        months=list(args.months),
        symbols=list(args.symbols),
        scored_events_path=Path(args.scored_events),
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
        external_entry_mode=str(args.external_entry_mode),
        t3_size_scale=float(args.t3_size_scale),
    )
    best = max(metrics, key=lambda row: row.calendar_silo_sum_pct)
    logger.info(
        "Best filtered T3 source %s calendar_sum=%.6f%% trades=%d",
        best.candidate,
        best.calendar_silo_sum_pct,
        best.total_trades,
    )
    logger.info("Wrote %s", args.output_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
