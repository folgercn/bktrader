"""Strict lifecycle replay for probability-selected external T2 events.

Research-only. The pass-bucket audit showed current original_t2 is not a
tradable leg under strict lifecycle. This runner tests the next question: can a
probability/RF-selected event set from the breakout-structure ledger be injected
as explicit breakout locks and survive the same lifecycle contract?
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
DEFAULT_EXTERNAL_EVENTS = (
    OUTPUT_DIR / "breakout_structure_context_model_lead_combo_active_events_low_eff_rf_rank_median_000.csv"
)
T3_60M_EXIT_OVERRIDES = {"min_hold_seconds_before_sl": 3600.0}
EXTERNAL_SHAPE_NAME = "low_eff_rf_rank_median_000"


@dataclass(frozen=True)
class ExternalEventSpec:
    """One strict lifecycle external-event candidate."""

    label: str
    external_shape_name: str
    external_events_path: str
    confirmation_events_path: str
    external_entry_mode: str
    read: str


@dataclass
class ExternalEventSilo:
    """One fixed symbol-month lifecycle result."""

    candidate: str
    symbol: str
    month: str
    return_pct: float
    final_balance: float
    total_trades: int
    t3_trades: int
    external_trades: int
    t3_net_pnl_pct: float
    external_net_pnl_pct: float
    external_events_available: int
    external_locks: int
    external_lock_reasons: dict
    reentry_fill_rejects: dict
    elapsed_seconds: float


@dataclass
class ExternalEventMetrics:
    """Aggregate fixed-calendar metrics for one candidate."""

    candidate: str
    calendar_silo_sum_pct: float
    calendar_avg_symbol_month_pct: float
    worst_calendar_silo_pct: float
    negative_calendar_silos: int
    traded_symbol_months: int
    flat_symbol_months: int
    total_trades: int
    t3_trades: int
    external_trades: int
    t3_net_pnl_pct: float
    external_net_pnl_pct: float
    external_events_available: int
    external_locks: int
    reentry_fill_rejects: dict
    by_month: dict[str, float]
    by_symbol: dict[str, float]


def _event_source_label(confirmation_events_path: Path | None) -> str:
    if confirmation_events_path is None:
        return "raw_touch"
    name = confirmation_events_path.stem
    return name.removeprefix("breakout_structure_confirmation_events_")


def build_specs(
    candidate_set: str,
    external_events_path: Path,
    confirmation_events_path: Path | None,
    external_entry_mode: str,
) -> list[ExternalEventSpec]:
    if candidate_set != "low_eff_rf":
        raise ValueError(f"unknown candidate set: {candidate_set}")
    event_source = _event_source_label(confirmation_events_path)
    return [
        ExternalEventSpec(
            label=f"low_eff_rf_rank_median_external_{event_source}_{external_entry_mode}_t3_60m",
            external_shape_name=EXTERNAL_SHAPE_NAME,
            external_events_path=str(external_events_path),
            confirmation_events_path=str(confirmation_events_path) if confirmation_events_path else "",
            external_entry_mode=external_entry_mode,
            read=(
                "Inject low_eff_rf_rank_median_000 active events as explicit breakout locks; native original_t2 "
                f"remains disabled; event source={event_source}; external entry mode={external_entry_mode}."
            ),
        )
    ]


def _parse_event_times(df: pd.DataFrame) -> pd.DataFrame:
    for column in ("signal_start", "signal_end", "touch_time", "confirmation_time", "original_touch_time"):
        if column in df.columns:
            df[column] = pd.to_datetime(df[column], utc=True)
    return df


def _load_external_events(path: Path, confirmation_events_path: Path | None = None) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    active = _parse_event_times(pd.read_csv(path))
    if confirmation_events_path is None:
        return active
    if not confirmation_events_path.exists():
        raise FileNotFoundError(confirmation_events_path)
    confirmed = _parse_event_times(pd.read_csv(confirmation_events_path))
    if active.empty or confirmed.empty:
        return confirmed.iloc[0:0].copy()
    active["event_key"] = active["event_key"].astype(str)
    confirmed["event_key"] = confirmed["event_key"].astype(str)
    active_context = active.drop_duplicates("event_key").set_index("event_key")
    confirmed = confirmed[confirmed["event_key"].isin(active_context.index)].copy()

    active_preferred_columns = {
        "context_combo_spec",
        "context_model_status",
        "context_model_variant",
        "context_model_probability",
        "context_model_scale",
        "forward_month",
        "rf_probability",
        "sizing_multiplier",
        "cost_penalty",
        "model_feature_imputations",
        "ctx4h_side_return_atr",
        "ctx12h_side_return_atr",
        "ctx4h_range_atr",
        "ctx12h_range_atr",
    }
    for column in active_preferred_columns:
        if column not in active_context.columns:
            continue
        mapped = confirmed["event_key"].map(active_context[column])
        if column in confirmed.columns:
            confirmed[column] = mapped.combine_first(confirmed[column])
        else:
            confirmed[column] = mapped
    return _parse_event_times(confirmed)


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
    spec: ExternalEventSpec,
    all_external_events: pd.DataFrame,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
    external_entry_mode: str,
) -> ExternalEventSilo:
    start, end = _month_bounds(month)
    external_events = _window_events(all_external_events, symbol, start, end)
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
            shape_sizing_filters={"allowed_sides": []},
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=0.0,
            sizing_filter_fail_action="skip_lock",
            t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
            external_breakout_events=external_events,
            external_breakout_shape_name=spec.external_shape_name,
            external_entry_mode=external_entry_mode,
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t3_attr = _shape_attr(attribution, "t3_swing")
    external_attr = _shape_attr(attribution, spec.external_shape_name)
    external_locks = _flat_counts(diagnostics.get("external_breakout_locks", {}))
    reentry_rejects = _flat_counts(diagnostics.get("reentry_fill_rejects", {}))
    return ExternalEventSilo(
        candidate=spec.label,
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        total_trades=int(summary["trades"]),
        t3_trades=int(t3_attr.get("trades", 0)),
        external_trades=int(external_attr.get("trades", 0)),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        external_net_pnl_pct=round(_net_pnl_pct(attribution, spec.external_shape_name, initial_balance), 6),
        external_events_available=int(len(external_events)),
        external_locks=int(sum(external_locks.values())),
        external_lock_reasons=external_locks,
        reentry_fill_rejects=reentry_rejects,
        elapsed_seconds=round(time.time() - started, 2),
    )


def compute_metrics(
    spec: ExternalEventSpec,
    silos: list[ExternalEventSilo],
    months: list[str],
    symbols: list[str],
) -> ExternalEventMetrics:
    lookup = {(row.symbol, row.month): row for row in silos}
    grid_rows = []
    reentry_rejects: dict[str, int] = {}
    for month in months:
        for symbol in symbols:
            row = lookup.get((symbol, month))
            if row is None:
                grid_rows.append((symbol, month, 0.0, 0, 0, 0, 0.0, 0.0, 0, 0))
                continue
            grid_rows.append(
                (
                    symbol,
                    month,
                    row.return_pct,
                    row.total_trades,
                    row.t3_trades,
                    row.external_trades,
                    row.t3_net_pnl_pct,
                    row.external_net_pnl_pct,
                    row.external_events_available,
                    row.external_locks,
                )
            )
            for reason, count in row.reentry_fill_rejects.items():
                reentry_rejects[reason] = int(reentry_rejects.get(reason, 0)) + int(count)

    total_slots = len(months) * len(symbols)
    returns = [float(row[2]) for row in grid_rows]
    total_return = round(float(sum(returns)), 6)
    return ExternalEventMetrics(
        candidate=spec.label,
        calendar_silo_sum_pct=total_return,
        calendar_avg_symbol_month_pct=round(total_return / total_slots, 6) if total_slots else 0.0,
        worst_calendar_silo_pct=round(float(min(returns, default=0.0)), 6),
        negative_calendar_silos=int(sum(1 for value in returns if value < 0.0)),
        traded_symbol_months=int(sum(1 for row in grid_rows if row[3] > 0)),
        flat_symbol_months=int(sum(1 for row in grid_rows if row[3] == 0)),
        total_trades=int(sum(row[3] for row in grid_rows)),
        t3_trades=int(sum(row[4] for row in grid_rows)),
        external_trades=int(sum(row[5] for row in grid_rows)),
        t3_net_pnl_pct=round(float(sum(row[6] for row in grid_rows)), 6),
        external_net_pnl_pct=round(float(sum(row[7] for row in grid_rows)), 6),
        external_events_available=int(sum(row[8] for row in grid_rows)),
        external_locks=int(sum(row[9] for row in grid_rows)),
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
    specs: list[ExternalEventSpec],
    external_events: pd.DataFrame,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
    external_entry_mode: str,
) -> tuple[list[ExternalEventSilo], list[ExternalEventMetrics]]:
    all_silos: list[ExternalEventSilo] = []
    metrics: list[ExternalEventMetrics] = []
    for spec in specs:
        candidate_silos = []
        for symbol in symbols:
            for month in months:
                logger.info("Running external-event lifecycle %s %s %s", spec.label, symbol, month)
                row = run_silo(
                    spec=spec,
                    all_external_events=external_events,
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    reentry_fill_policy=reentry_fill_policy,
                    external_entry_mode=external_entry_mode,
                )
                candidate_silos.append(row)
                all_silos.append(row)
        metrics.append(compute_metrics(spec, candidate_silos, months, symbols))
    return all_silos, metrics


def write_outputs(
    *,
    output_dir: Path,
    specs: list[ExternalEventSpec],
    silos: list[ExternalEventSilo],
    metrics: list[ExternalEventMetrics],
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
    external_entry_mode: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "note": (
            "Research-only strict lifecycle replay for probability-selected external T2 events. "
            "Native original_t2 is disabled; external RF events are injected as explicit breakout locks."
        ),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "external_entry_mode": external_entry_mode,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "candidate_specs": [asdict(spec) for spec in specs],
        "candidates": [asdict(row) for row in metrics],
        "silos": [asdict(row) for row in silos],
    }
    (output_dir / "t2_lifecycle_external_event_gate_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    lines = [
        "# T2 Lifecycle External Event Gate",
        "",
        "Research-only strict lifecycle replay for probability/RF-selected external T2 events.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- External entry mode: `{external_entry_mode}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Trades | External Trades | T3 Net PnL | External Net PnL | External Events | External Locks | Read |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    reads = {spec.label: spec.read for spec in specs}
    for row in metrics:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.calendar_avg_symbol_month_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} "
            f"| {row.total_trades} "
            f"| {row.t3_trades} "
            f"| {row.external_trades} "
            f"| {row.t3_net_pnl_pct:.6f}% "
            f"| {row.external_net_pnl_pct:.6f}% "
            f"| {row.external_events_available} "
            f"| {row.external_locks} "
            f"| {reads.get(row.candidate, '')} |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- This is the exact lifecycle bridge for the existing RF-selected active event file, not a new model fit.",
            "- Native original_t2 is disabled, so any delta versus the T2-disabled floor comes from the external probability events.",
            "- `reentry_window` preserves the original strict next-second cross lifecycle; `next_second_*` modes test post-touch direct entry without same-second fills.",
            "- A good result must beat `t2_disabled_t3_60m` before it is worth promoting into broader risk audit.",
        ]
    )
    (output_dir / "t2_lifecycle_external_event_gate_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate-set", choices=["low_eff_rf"], default="low_eff_rf")
    parser.add_argument("--external-events", type=Path, default=DEFAULT_EXTERNAL_EVENTS)
    parser.add_argument(
        "--confirmation-events",
        type=Path,
        default=None,
        help="Optional confirmed-event CSV; intersected by event_key with --external-events.",
    )
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
        "--output-dir",
        type=Path,
        default=OUTPUT_DIR / "t2_external_low_eff_rf_median_extended",
    )
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    specs = build_specs(
        args.candidate_set,
        Path(args.external_events),
        Path(args.confirmation_events) if args.confirmation_events else None,
        str(args.external_entry_mode),
    )
    external_events = _load_external_events(
        Path(args.external_events),
        Path(args.confirmation_events) if args.confirmation_events else None,
    )
    silos, metrics = run_validation(
        specs=specs,
        external_events=external_events,
        months=list(args.months),
        symbols=list(args.symbols),
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        reentry_fill_policy=str(args.reentry_fill_policy),
        external_entry_mode=str(args.external_entry_mode),
    )
    write_outputs(
        output_dir=Path(args.output_dir),
        specs=specs,
        silos=silos,
        metrics=metrics,
        months=list(args.months),
        symbols=list(args.symbols),
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
        external_entry_mode=str(args.external_entry_mode),
    )
    logger.info("Wrote %s", args.output_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
