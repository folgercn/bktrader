"""Replay deterministic T3 stop-gate action on the relaxed T3 event pool.

This script keeps the relaxed ``prev3_dominates`` event source and the
walk-forward 0.20-0.40 ETH quantity-band event scores fixed. It changes only
the lifecycle of events selected by the deterministic stop gate:

- selected events: hard stop 3.0 ATR from entry, trailing updates delayed 4740s
- non-selected events: PR447 60m lifecycle baseline
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from dataclasses import asdict
from pathlib import Path

import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t2_lifecycle_context_sizing import EXTENDED_MONTHS  # noqa: E402
from timing_probability_unified.t3_filtered_external_event_lifecycle import (  # noqa: E402
    T3_60M_EXIT_OVERRIDES,
    FilteredT3Spec,
    apply_spec,
    load_scored_events,
)
from timing_probability_unified.t3_lifecycle_outcome_diagnostics import pair_lifecycle_trades  # noqa: E402
from timing_probability_unified.t3_overlay_lead_exposure_audit import (  # noqa: E402
    _apply_round_trip_fee_adjustment,
    _window_events,
)
from timing_probability_unified.t3_overlay_rf_cost_sizing import (  # noqa: E402
    BASELINE_LEAD_ADVERSE10_PCT,
    OUTPUT_DIR,
    SizingVariant,
    _equity_max_drawdown_pct,
    _month_grid,
    _numeric,
    _path_label,
    apply_event_scores_to_trades,
    summarize_variant,
)
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _patched_replay_kwargs,
)

logger = logging.getLogger(__name__)

DEFAULT_RELAXED_SCORED_EVENTS = (
    OUTPUT_DIR
    / "t3_probability_overlay_relaxed_prev3_dominates_20260529"
    / "t3_probability_overlay_scored_events.csv"
)
DEFAULT_RF_COST_DIR = OUTPUT_DIR / "t3_overlay_rf_cost_sizing_relaxed_prev3_dominates_20260529"
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_deterministic_stop_gate_relaxed_prev3_dominates_20260529"
QUANTITY_VARIANT = "wf_t3_rf_cost_quantity_0p20_0p40_shadow"
RELAXED_LEAD_ADVERSE10_PCT = 61.07091667649647

DETERMINISTIC_EXIT_OVERRIDES = {
    "stop_loss_atr": 3.0,
    "min_hold_seconds_before_trailing_sl": 4740.0,
}


def _deterministic_stop_gate(events: pd.DataFrame) -> pd.Series:
    return (
        _numeric(events, "speed_300s_atr").abs().ge(0.65)
        & _numeric(events, "eff_300s").ge(0.85)
        & _numeric(events, "pre_touch_seconds").between(250.0, 900.0, inclusive="both")
        & _numeric(events, "touch_extension_atr").abs().le(0.40)
    )


def _attach_external_event_key(events: pd.DataFrame) -> pd.DataFrame:
    out = events.copy()
    signal_start = pd.to_datetime(out["signal_start"], utc=True)
    touch_time = pd.to_datetime(out["touch_time"], utc=True)
    out["external_event_key"] = (
        out["symbol"].astype(str)
        + "|"
        + signal_start.map(lambda value: value.isoformat())
        + "|"
        + touch_time.map(lambda value: value.isoformat())
        + "|"
        + out["side"].astype(str).str.lower()
    )
    return out


def _collect_selected_action_trades(
    *,
    selected_events: pd.DataFrame,
    months: list[str],
    symbol: str,
    timeframe: str,
    initial_balance: float,
    external_entry_mode: str,
    t3_size_scale: float,
    reentry_fill_policy: str,
    early_horizon_seconds: int,
) -> pd.DataFrame:
    parts: list[pd.DataFrame] = []
    t3_schedule = [float(size) * float(t3_size_scale) for size in T3_REENTRY_SIZE_SCHEDULE]
    overrides = dict(T3_60M_EXIT_OVERRIDES)
    overrides.update(DETERMINISTIC_EXIT_OVERRIDES)
    for month in months:
        start, end = _month_bounds(month)
        external_events = _window_events(selected_events, symbol, start, end)
        if external_events.empty:
            continue
        logger.info("Replaying deterministic selected T3 events %s %s (%d)", symbol, month, len(external_events))
        second_bars = _load_window_bars(symbol, start, end)
        _, signal = lifecycle.build_signal_frame(second_bars, timeframe)
        with _patched_replay_kwargs(symbol):
            ledger, _diagnostics = lifecycle.run_second_bar_replay(
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
                t3_exit_overrides=overrides,
                external_breakout_events=external_events,
                external_breakout_shape_name="t3_swing",
                external_entry_mode=external_entry_mode,
                reentry_fill_policy=reentry_fill_policy,
            )
        trades = pair_lifecycle_trades(
            ledger,
            second_bars,
            symbol=symbol,
            month=month,
            initial_balance=initial_balance,
            early_horizon_seconds=early_horizon_seconds,
        )
        if not trades.empty:
            trades = trades[trades["breakout_shape_name"] == "t3_swing"].copy()
            trades = _apply_round_trip_fee_adjustment(trades, initial_balance=initial_balance)
            parts.append(trades)
    return pd.concat(parts, ignore_index=True) if parts else pd.DataFrame()


def _write_report(summary: dict, path: Path) -> None:
    metrics = summary["metrics"]
    lines = [
        "# Relaxed T3 deterministic stop-gate replay",
        "",
        f"- Relaxed scored events: `{summary['scored_events']}`",
        f"- RF/cost event scores: `{summary['event_scores']}`",
        f"- Base trades: `{summary['base_trades']}`",
        f"- Event source: `{summary['event_source']}`",
        f"- Quantity variant: `{summary['quantity_variant']}`",
        f"- Deterministic selected events: `{summary['selected_events']}` / `{summary['active_events']}` active qband events",
        f"- Selected action: `{json.dumps(summary['selected_exit_overrides'], sort_keys=True)}`",
        "",
        "| Metric | Value |",
        "|---|---:|",
        f"| Overlay PnL | `{metrics['calendar_sum_pct']:.6f}%` |",
        f"| Delta vs relaxed q020-q040 baseline | `{summary['overlay_delta_vs_relaxed_qband_pp']:.6f}pp` |",
        f"| Lead q020-q040 + overlay | `{metrics['lead_adverse10_plus_overlay_pct']:.6f}%` |",
        f"| Worst month | `{metrics['worst_month_pct']:.6f}%` |",
        f"| Negative months | `{metrics['negative_months']}` |",
        f"| Max drawdown | `{metrics['max_drawdown_pct']:.6f}%` |",
        f"| Filled trades | `{metrics['filled_trades']}` |",
        f"| Active events | `{metrics['active_events']}` |",
        "",
        "## Monthly PnL",
        "",
        "| Month | PnL |",
        "|---|---:|",
    ]
    for month, value in metrics["by_month"].items():
        lines.append(f"| `{month}` | `{value:.6f}%` |")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--scored-events", type=Path, default=DEFAULT_RELAXED_SCORED_EVENTS)
    parser.add_argument("--rf-cost-dir", type=Path, default=DEFAULT_RF_COST_DIR)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--external-entry-mode", default="next_second_adverse")
    parser.add_argument("--t3-size-scale", type=float, default=2.0)
    parser.add_argument("--reentry-fill-policy", default="historical")
    parser.add_argument("--early-horizon-seconds", type=int, default=300)
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    logging.basicConfig(level=getattr(logging, args.log_level.upper(), logging.INFO), format="%(levelname)s %(message)s")
    args.output_dir.mkdir(parents=True, exist_ok=True)

    spec = FilteredT3Spec("all_speed_abs_ge_0p35", speed_abs_min=0.35)
    relaxed_events = apply_spec(load_scored_events(args.scored_events), spec)
    relaxed_events = relaxed_events[relaxed_events["symbol"].astype(str) == args.symbol].copy()
    relaxed_events = _attach_external_event_key(relaxed_events)
    relaxed_events["event_month"] = pd.to_datetime(relaxed_events["signal_start"], utc=True).dt.strftime("%Y-%m")
    selected_events = relaxed_events[_deterministic_stop_gate(relaxed_events)].copy()
    selected_keys = set(selected_events["external_event_key"].astype(str))

    base_trades = pd.read_csv(args.rf_cost_dir / "t3_overlay_rf_cost_base_trades.csv")
    event_scores_all = pd.read_csv(args.rf_cost_dir / "t3_overlay_rf_cost_event_scores.csv")
    event_scores = event_scores_all[event_scores_all["sizing_variant"] == QUANTITY_VARIANT].copy()
    active_event_keys = set(event_scores.loc[_numeric(event_scores, "event_multiplier") > 0.0, "external_event_key"].astype(str))
    selected_keys &= active_event_keys
    selected_events = selected_events[selected_events["external_event_key"].astype(str).isin(selected_keys)].copy()

    selected_action_trades = _collect_selected_action_trades(
        selected_events=selected_events,
        months=list(EXTENDED_MONTHS),
        symbol=args.symbol,
        timeframe=args.timeframe,
        initial_balance=args.initial_balance,
        external_entry_mode=args.external_entry_mode,
        t3_size_scale=args.t3_size_scale,
        reentry_fill_policy=args.reentry_fill_policy,
        early_horizon_seconds=args.early_horizon_seconds,
    )
    if not selected_action_trades.empty:
        selected_action_trades["deterministic_stop_gate_action"] = True
    base_keep = base_trades[~base_trades["external_event_key"].astype(str).isin(selected_keys)].copy()
    base_keep["deterministic_stop_gate_action"] = False
    combined = pd.concat([base_keep, selected_action_trades], ignore_index=True)

    weighted = apply_event_scores_to_trades(combined, event_scores, initial_balance=args.initial_balance)
    weighted["sizing_variant"] = "wf_t3_rf_cost_quantity_0p20_0p40_deterministic_stop_gate"
    variant = SizingVariant(
        label="wf_t3_rf_cost_quantity_0p20_0p40_deterministic_stop_gate",
        method="wf_t3_rf_quantity",
        min_quantity=0.20,
        max_quantity=0.40,
        live_compatible=False,
        read="Relaxed prev3_dominates q020-q040 T3 overlay with deterministic selected hard3/delay79m lifecycle.",
    )
    metrics = summarize_variant(
        variant=variant,
        trades=weighted,
        event_scores=event_scores,
        months=list(EXTENDED_MONTHS),
        fixed_overlay_pct=0.0,
        baseline_lead_adverse10_pct=RELAXED_LEAD_ADVERSE10_PCT,
    )

    relaxed_summary_path = args.rf_cost_dir / "t3_overlay_rf_cost_sizing_summary.json"
    relaxed_qband_pct = 0.0
    if relaxed_summary_path.exists():
        relaxed_summary = json.loads(relaxed_summary_path.read_text(encoding="utf-8"))
        for row in relaxed_summary.get("metrics", []):
            if row.get("variant") == QUANTITY_VARIANT:
                relaxed_qband_pct = float(row.get("calendar_sum_pct", 0.0))
                break

    base_trades.to_csv(args.output_dir / "t3_overlay_deterministic_stop_gate_base_trades.csv", index=False)
    selected_action_trades.to_csv(args.output_dir / "t3_overlay_deterministic_stop_gate_selected_action_trades.csv", index=False)
    combined.to_csv(args.output_dir / "t3_overlay_deterministic_stop_gate_combined_trades.csv", index=False)
    weighted.to_csv(args.output_dir / "t3_overlay_deterministic_stop_gate_weighted_trades.csv", index=False)
    selected_events.to_csv(args.output_dir / "t3_overlay_deterministic_stop_gate_selected_events.csv", index=False)

    summary = {
        "note": "Relaxed prev3_dominates T3 event pool with q020-q040 quantity-band event scores; deterministic selected events use hard3 immediate hard stop and 4740s delayed trailing updates.",
        "scored_events": _path_label(args.scored_events),
        "event_scores": _path_label(args.rf_cost_dir / "t3_overlay_rf_cost_event_scores.csv"),
        "base_trades": _path_label(args.rf_cost_dir / "t3_overlay_rf_cost_base_trades.csv"),
        "event_source": spec.label,
        "quantity_variant": QUANTITY_VARIANT,
        "months": list(EXTENDED_MONTHS),
        "selected_exit_overrides": dict(DETERMINISTIC_EXIT_OVERRIDES),
        "selected_events": int(len(selected_events)),
        "active_events": int(len(active_event_keys)),
        "relaxed_qband_overlay_pct": round(relaxed_qband_pct, 6),
        "overlay_delta_vs_relaxed_qband_pp": round(metrics.calendar_sum_pct - relaxed_qband_pct, 6),
        "metrics": asdict(metrics),
    }
    (args.output_dir / "t3_overlay_deterministic_stop_gate_summary.json").write_text(
        json.dumps(summary, indent=2, sort_keys=True),
        encoding="utf-8",
    )
    _write_report(summary, args.output_dir / "t3_overlay_deterministic_stop_gate_report.md")
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
