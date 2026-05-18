"""Combine canonical ETH pretouch lead with breakout structure-quality expansion.

Research-only. The canonical lead is kept as-is; expansion legs only add
current-shape live-like events that do not overlap the canonical event source
on (signal_start, side). This tests whether the structure-quality layer can
increase the existing research lead rather than replacing it.
"""

from __future__ import annotations

import argparse
import json
import logging
import re
import sys
import time
from copy import deepcopy
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.adverse_fill import (  # noqa: E402
    STANDARD_FILL_SCENARIOS,
    reprice_delay_results,
)
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _evaluate_fill_scenarios_fast,
    _load_all_1s_bars,
    _reprice_delay_results_fast,
    simulate_d0_delays,
)
from timing_probability_unified.breakout_structure_cross_asset_gate_search import (  # noqa: E402
    _add_context_features,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    CombinedPositionConfig,
    compute_combined_positions,
)
from timing_probability_unified.unified_runner import _simulate_delays_for_events  # noqa: E402

logger = logging.getLogger(__name__)

CANONICAL_EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "tick_flow_event_sources"
    / "20260514_pretouch_full_window"
    / "feature_filtered_seed_events"
    / "robust_quality"
    / "pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv"
)
LEAD_TRADES_CSV = OUTPUT_DIR / "unified_trades.csv"
LEAD_ADVERSE_CSV = OUTPUT_DIR / "adverse_fill_full_window.csv"
BASE_EVENTS_CSV = OUTPUT_DIR / "breakout_shape_expansion_events_restrictive_0p5bps.csv"
ETH_GATE_SEARCH_CANDIDATES_CSV = (
    OUTPUT_DIR / "breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_candidates.csv"
)
LEAD_REPLAY_EXEC_OVERRIDES = {
    "trail_start_r": 1.5,
    "max_hold_hours": 2.0,
}
CONDITION_REPLAY_EPS = 5e-7


@dataclass(frozen=True)
class ExpansionCandidate:
    name: str
    source: str
    path: Path
    gate: str | None = None
    description: str = ""


STATIC_CANDIDATES: list[ExpansionCandidate] = [
    ExpansionCandidate(
        name="static_low_eff_le_q20",
        source="static_events",
        path=OUTPUT_DIR / "breakout_structure_quality_events_low_eff_le_q20.csv",
        description="in-sample low eff_300s <= q20 structure-quality gate",
    ),
    ExpansionCandidate(
        name="static_low_eff_low_atr_pct",
        source="static_events",
        path=OUTPUT_DIR / "breakout_structure_quality_events_low_eff_low_atr_pct.csv",
        description="in-sample low eff_300s + low ATR percentile gate",
    ),
    ExpansionCandidate(
        name="static_low_rf_slope_up",
        source="static_events",
        path=OUTPUT_DIR / "breakout_structure_quality_events_low_rf_slope_up.csv",
        description="in-sample low RF + positive SMA5 slope gate",
    ),
    ExpansionCandidate(
        name="static_level_far_sma_gap_up",
        source="static_events",
        path=OUTPUT_DIR / "breakout_structure_quality_events_level_far_sma_gap_up.csv",
        description="in-sample level far from open + positive SMA5 gap gate",
    ),
]

WALKFORWARD_SELECTED: list[ExpansionCandidate] = [
    ExpansionCandidate(
        name="wf3_selected",
        source="selected_trades",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train3m_min20_selected_trades.csv",
        description="3m trailing train dynamic selector",
    ),
    ExpansionCandidate(
        name="wf4_selected",
        source="selected_trades",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train4m_min20_selected_trades.csv",
        description="4m trailing train dynamic selector",
    ),
    ExpansionCandidate(
        name="wf5_selected",
        source="selected_trades",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train5m_min20_selected_trades.csv",
        description="5m trailing train dynamic selector",
    ),
]

WALKFORWARD_FIXED: list[ExpansionCandidate] = [
    ExpansionCandidate(
        name="wf3_low_eff_q20",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train3m_min20_candidate_forward.csv",
        gate="low_eff_q20",
        description="3m train-calibrated low eff q20 candidate",
    ),
    ExpansionCandidate(
        name="wf3_low_eff_low_atr",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train3m_min20_candidate_forward.csv",
        gate="low_eff_low_atr_q20_q40",
        description="3m train-calibrated low eff + low ATR candidate",
    ),
    ExpansionCandidate(
        name="wf3_low_rf_slope_up",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train3m_min20_candidate_forward.csv",
        gate="low_rf_slope_up_q40_q60",
        description="3m train-calibrated low RF + slope candidate",
    ),
    ExpansionCandidate(
        name="wf4_low_eff_q20",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train4m_min20_candidate_forward.csv",
        gate="low_eff_q20",
        description="4m train-calibrated low eff q20 candidate",
    ),
    ExpansionCandidate(
        name="wf4_low_eff_low_atr",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train4m_min20_candidate_forward.csv",
        gate="low_eff_low_atr_q20_q40",
        description="4m train-calibrated low eff + low ATR candidate",
    ),
    ExpansionCandidate(
        name="wf5_low_eff_q20",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train5m_min20_candidate_forward.csv",
        gate="low_eff_q20",
        description="5m train-calibrated low eff q20 candidate",
    ),
    ExpansionCandidate(
        name="wf5_low_eff_low_atr",
        source="wf_candidate",
        path=OUTPUT_DIR / "breakout_structure_walkforward_train5m_min20_candidate_forward.csv",
        gate="low_eff_low_atr_q20_q40",
        description="5m train-calibrated low eff + low ATR candidate",
    ),
]

WALKFORWARD_CONTEXT_FIXED: list[ExpansionCandidate] = [
    ExpansionCandidate(
        name="wf3_low_eff_low_atr_ctx4h_up",
        source="gate_search_candidate",
        path=ETH_GATE_SEARCH_CANDIDATES_CSV,
        gate="low_eff_low_atr_ctx4h_up",
        description="3m train-calibrated low eff + low ATR with favorable prior 4h return",
    ),
    ExpansionCandidate(
        name="wf3_low_eff_low_atr_ctx12h_up",
        source="gate_search_candidate",
        path=ETH_GATE_SEARCH_CANDIDATES_CSV,
        gate="low_eff_low_atr_ctx12h_up",
        description="3m train-calibrated low eff + low ATR with favorable prior 12h return",
    ),
]


def _load_csv(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    df = pd.read_csv(path)
    for column in ("touch_time", "signal_start", "signal_end"):
        if column in df.columns:
            df[column] = pd.to_datetime(df[column], utc=True)
    return df


def _canonical_overlap_keys() -> set[tuple[str, str]]:
    events = _load_csv(CANONICAL_EVENTS_CSV)
    events = events[events["symbol"] == "ETHUSDT"].copy()
    events["signal_start"] = pd.to_datetime(events["signal_start"], utc=True)
    return {
        (pd.Timestamp(row.signal_start).isoformat(), str(row.side))
        for row in events.itertuples(index=False)
    }


def _lead_trades_gate_on() -> pd.DataFrame:
    trades = _lead_trades_all()
    trades = trades[
        (trades["speed_gate_pass"] == True)  # noqa: E712
        & (trades["selected_delay"] != "none")
    ].copy()
    trades["source_leg"] = "canonical_lead"
    return trades.reset_index(drop=True)


def _lead_trades_all() -> pd.DataFrame:
    trades = _load_csv(LEAD_TRADES_CSV)
    trades = trades[
        trades["symbol"] == "ETHUSDT"
    ].copy()
    trades["source_leg"] = "canonical_lead"
    return trades.reset_index(drop=True)


def _lead_adverse_matrix() -> pd.DataFrame:
    return _load_csv(LEAD_ADVERSE_CSV)


def _event_overlap_key(events: pd.DataFrame) -> pd.Series:
    return events.apply(
        lambda row: (pd.Timestamp(row["signal_start"]).isoformat(), str(row["side"])),
        axis=1,
    )


def _filter_noncanonical(events: pd.DataFrame, canonical_keys: set[tuple[str, str]]) -> tuple[pd.DataFrame, int]:
    if events.empty:
        return events.copy(), 0
    keys = _event_overlap_key(events)
    mask = ~keys.isin(canonical_keys)
    removed = int((~mask).sum())
    return events[mask].copy().reset_index(drop=True), removed


def _parse_condition(text: str) -> tuple[str, str, float]:
    match = re.fullmatch(
        r"\s*([A-Za-z0-9_]+)\s*(<=|>=)\s*(-?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)\s*",
        text,
    )
    if not match:
        raise ValueError(f"cannot parse condition: {text!r}")
    column, op, value = match.groups()
    return column, op, float(value)


def _apply_conditions(events: pd.DataFrame, conditions: str) -> pd.DataFrame:
    if conditions == "none" or not conditions.strip():
        return events.copy().reset_index(drop=True)
    mask = np.ones(len(events), dtype=bool)
    for part in conditions.split("&"):
        column, op, value = _parse_condition(part)
        series = pd.to_numeric(events[column], errors="coerce")
        if op == "<=":
            mask &= (series <= value + CONDITION_REPLAY_EPS).fillna(False).to_numpy()
        elif op == ">=":
            mask &= (series >= value - CONDITION_REPLAY_EPS).fillna(False).to_numpy()
        else:
            raise ValueError(f"unsupported op: {op}")
    return events[mask].copy().reset_index(drop=True)


def _candidate_events(candidate: ExpansionCandidate, base_events: pd.DataFrame) -> pd.DataFrame:
    if candidate.source == "static_events":
        events = _load_csv(candidate.path)
    elif candidate.source == "selected_trades":
        selected = _load_csv(candidate.path)
        keys = set(selected["event_key"].astype(str))
        events = base_events[base_events["event_key"].isin(keys)].copy()
    elif candidate.source in {"wf_candidate", "gate_search_candidate"}:
        rows = _load_csv(candidate.path)
        if candidate.gate is None:
            raise ValueError(f"wf candidate missing gate: {candidate.name}")
        rows = rows[rows["gate"] == candidate.gate].copy()
        parts: list[pd.DataFrame] = []
        for row in rows.itertuples(index=False):
            month = str(row.forward_month)
            conditions = str(row.conditions)
            month_events = base_events[base_events["touch_time"].dt.strftime("%Y-%m") == month].copy()
            parts.append(_apply_conditions(month_events, conditions))
        events = pd.concat(parts, ignore_index=True) if parts else pd.DataFrame(columns=base_events.columns)
    else:
        raise ValueError(f"unknown candidate source: {candidate.source}")

    if "timing_prediction" in events.columns:
        events = events[events["timing_prediction"] != "skip"].copy()
    return events.reset_index(drop=True)


def _monthly_metrics(trades: pd.DataFrame, pnl_col: str = "weighted_pnl") -> dict[str, Any]:
    if trades.empty:
        return {"calendar_sum": 0.0, "worst_sm": 0.0, "neg_sm": 0, "trade_count": 0}
    df = trades.copy()
    df["touch_time"] = pd.to_datetime(df["touch_time"], utc=True)
    df["year_month"] = df["touch_time"].dt.strftime("%Y-%m")
    monthly = df.groupby("year_month")[pnl_col].sum()
    return {
        "calendar_sum": float(monthly.sum()),
        "worst_sm": float(monthly.min()) if len(monthly) else 0.0,
        "neg_sm": int((monthly < 0).sum()),
        "trade_count": int(len(df)),
    }


def _matrix_row(matrix: pd.DataFrame, scenario: str) -> dict[str, Any]:
    if matrix.empty:
        return {"calendar_sum": 0.0, "worst_sm": 0.0, "neg_sm": 0, "trade_count": 0}
    row = matrix[matrix["scenario"] == scenario]
    if row.empty:
        return {"calendar_sum": 0.0, "worst_sm": 0.0, "neg_sm": 0, "trade_count": 0}
    item = row.iloc[0]
    return {
        "calendar_sum": float(item["calendar_sum_gate_on"]),
        "worst_sm": float(item["worst_sm_gate_on"]),
        "neg_sm": int(item["neg_sm_count"]),
        "trade_count": int(item["trade_count_gate_on"]),
    }


def _fill_scenario(name: str):
    for scenario in STANDARD_FILL_SCENARIOS:
        if scenario.name == name:
            return scenario
    raise ValueError(f"unknown fill scenario: {name}")


def _traded_gate_on(trades: pd.DataFrame) -> pd.DataFrame:
    if trades.empty:
        return trades.copy()
    return trades[
        (trades["speed_gate_pass"] == True)  # noqa: E712
        & (trades["selected_delay"] != "none")
    ].copy().reset_index(drop=True)


def _lead_events_for_trades(lead_trades: pd.DataFrame) -> pd.DataFrame:
    canonical = _load_csv(CANONICAL_EVENTS_CSV)
    canonical = canonical[canonical["symbol"] == "ETHUSDT"].copy()
    canonical["event_id"] = canonical["event_id"].astype(str)
    indexed = canonical.drop_duplicates("event_id").set_index("event_id", drop=False)

    event_ids = lead_trades["event_id"].astype(str).tolist()
    missing = [event_id for event_id in event_ids if event_id not in indexed.index]
    if missing:
        raise ValueError(f"lead trades missing canonical events: {missing[:5]}")
    return indexed.loc[event_ids].reset_index(drop=True)


def _lead_replayed_trades(
    lead_all: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    scenario_name: str,
) -> tuple[pd.DataFrame, pd.DataFrame, dict[str, Any]]:
    """Rebuild the canonical lead per-trade same-close and adverse ledgers.

    `unified_trades.csv` and `adverse_fill_full_window.csv` are useful
    provenance artifacts, but the current production template uses
    `trail_start_r=1.5` and `max_hold_hours=2.0`. This function replays the
    lead under that production-aligned exit contract, using the model
    predictions, sizing multipliers, and speed gate flags from
    `unified_trades.csv`.
    """
    events = _lead_events_for_trades(lead_all)
    scenario = _fill_scenario(scenario_name)

    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    lead_params = deepcopy(DEFAULT_EXEC_PARAMS)
    lead_params.update(LEAD_REPLAY_EXEC_OVERRIDES)
    try:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(lead_params)
        delay_results, errors = _simulate_delays_for_events(events, bars_cache)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    same_trades = compute_combined_positions(
        events=events,
        timing_predictions=lead_all["timing_prediction"].to_numpy(dtype=object),
        sizing_multipliers=lead_all["sizing_multiplier"].to_numpy(dtype="float64"),
        delay_results=delay_results,
        speed_gate_pass=lead_all["speed_gate_pass"].to_numpy(dtype=bool),
        config=CombinedPositionConfig(base_notional_share=BASE_SHARE),
    )
    same_trades["source_leg"] = "canonical_lead"

    repriced = reprice_delay_results(
        delay_results=delay_results,
        events=events,
        bars_cache=bars_cache,
        scenario=scenario,
    )
    trades = compute_combined_positions(
        events=events,
        timing_predictions=lead_all["timing_prediction"].to_numpy(dtype=object),
        sizing_multipliers=lead_all["sizing_multiplier"].to_numpy(dtype="float64"),
        delay_results=repriced,
        speed_gate_pass=lead_all["speed_gate_pass"].to_numpy(dtype=bool),
        config=CombinedPositionConfig(base_notional_share=BASE_SHARE),
    )
    trades["source_leg"] = "canonical_lead"
    return same_trades.reset_index(drop=True), trades.reset_index(drop=True), {
        "lead_replayed_events": int(len(events)),
        "lead_delay_errors": int(len(errors)),
        "lead_exec_params": lead_params,
    }


def _evaluate_events_with_adverse_trades(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    scenario_name: str,
) -> tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]:
    if events.empty:
        return pd.DataFrame(), pd.DataFrame(), pd.DataFrame()

    delay_results = simulate_d0_delays(events, bars_cache)
    predictions = events["timing_prediction"].to_numpy(dtype=object)
    multipliers = events["sizing_multiplier"].to_numpy(dtype="float64")
    speed_gate_pass = np.ones(len(events), dtype=bool)
    config = CombinedPositionConfig(base_notional_share=BASE_SHARE)

    base_trades = compute_combined_positions(
        events=events,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )
    if "shape_variant" in events.columns:
        base_trades["shape_variant"] = events["shape_variant"].values
    if "event_key" in events.columns:
        base_trades["event_key"] = events["event_key"].values

    matrix = _evaluate_fill_scenarios_fast(
        delay_results=delay_results,
        events=events,
        bars_cache=bars_cache,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        speed_gate_pass=speed_gate_pass,
    )

    scenario = _fill_scenario(scenario_name)
    adverse_results = _reprice_delay_results_fast(
        delay_results=delay_results,
        events=events,
        bars_cache=bars_cache,
        scenario=scenario,
    )
    adverse_trades = compute_combined_positions(
        events=events,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        delay_results=adverse_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )
    if "shape_variant" in events.columns:
        adverse_trades["shape_variant"] = events["shape_variant"].values
    if "event_key" in events.columns:
        adverse_trades["event_key"] = events["event_key"].values
    return matrix, base_trades, adverse_trades


def _markdown_table(df: pd.DataFrame, cols: list[str]) -> str:
    if df.empty:
        return "_empty_"
    view = df[cols].copy()

    def fmt(value: Any) -> str:
        if isinstance(value, (float, np.floating)):
            if not np.isfinite(value):
                return ""
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
        "| " + " | ".join(row[idx].ljust(widths[idx]) for idx in range(len(widths)))
        + " |"
        for row in rows
    ]
    return "\n".join([header, sep] + body)


def _write_report(summary: pd.DataFrame, diagnostics: dict[str, Any], output_path: Path) -> None:
    lines: list[str] = []
    lines.append("# Canonical Lead + Breakout Structure Expansion Combo")
    lines.append("")
    lines.append(f"Generated: {pd.Timestamp.utcnow().isoformat()}")
    lines.append("")
    lines.append(
        "Scope: research-only. Canonical lead trades are preserved; expansion legs only add "
        "non-overlapping current-shape live-like events."
    )
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    cols = [
        "variant",
        "candidate_source_events",
        "overlap_removed_events",
        "extra_events",
        "extra_same_close_calendar_sum",
        "combo_same_close_calendar_sum",
        "combo_same_close_delta_vs_lead",
        "extra_adverse10_calendar_sum",
        "combo_adverse10_calendar_sum_exact",
        "combo_adverse10_delta_vs_lead_exact",
        "combo_adverse10_worst_sm_exact",
        "combo_adverse10_neg_sm_exact",
        "combo_same_close_worst_sm",
        "combo_same_close_neg_sm",
    ]
    lines.append(_markdown_table(summary, cols))
    lines.append("")
    lines.append("## Notes")
    lines.append("")
    lines.append("- Overlap is removed by canonical `(signal_start, side)`, using the full canonical event source, not only traded rows.")
    lines.append("- Same-close combo metrics are exact trade-ledger combinations.")
    lines.append("- Adverse combo metrics are now rebuilt from one combined per-trade ledger: canonical lead adverse trades plus expansion adverse trades.")
    lines.append("- Canonical lead metrics are replayed with the production template exit contract (`trail_start_r=1.5`, `max_hold_hours=2.0`); deltas versus current CSV artifacts are recorded in diagnostics.")
    lines.append("- `static_*` candidates are in-sample gates; `wf*` candidates are train-calibrated walk-forward gates.")
    lines.append("")
    lines.append("## Diagnostics")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str))
    lines.append("```")
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run() -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    canonical_keys = _canonical_overlap_keys()
    lead_all_trades = _lead_trades_all()
    lead_csv_gate_on = _lead_trades_gate_on()
    lead_csv_same = _monthly_metrics(lead_csv_gate_on)
    lead_adverse = _lead_adverse_matrix()
    lead_adverse10 = _matrix_row(lead_adverse, "next_adverse_xslip10bps")

    bars_1s = _load_all_1s_bars("ETHUSDT")
    base_events = _load_csv(BASE_EVENTS_CSV)
    base_events = base_events[base_events["timing_prediction"] != "skip"].copy().reset_index(drop=True)
    base_events = _add_context_features(base_events, bars_1s)
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)
    adverse_scenario = "next_adverse_xslip10bps"
    lead_same_trades_all, lead_adverse_trades_all, lead_replay_diagnostics = _lead_replayed_trades(
        lead_all_trades,
        bars_cache,
        adverse_scenario,
    )
    lead_trades = _traded_gate_on(lead_same_trades_all)
    lead_adverse_trades = _traded_gate_on(lead_adverse_trades_all)
    lead_same = _monthly_metrics(lead_trades)
    lead_adverse10_exact = _monthly_metrics(lead_adverse_trades)
    lead_trades.to_csv(
        OUTPUT_DIR / "breakout_structure_lead_combo_lead_same_close_trades.csv",
        index=False,
    )
    lead_adverse_trades.to_csv(
        OUTPUT_DIR / "breakout_structure_lead_combo_lead_adverse10_trades.csv",
        index=False,
    )

    original_exec_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_exec_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_exec_params)

    rows: list[dict[str, Any]] = []
    candidates = STATIC_CANDIDATES + WALKFORWARD_SELECTED + WALKFORWARD_FIXED + WALKFORWARD_CONTEXT_FIXED
    try:
        for candidate in candidates:
            logger.info("candidate %s", candidate.name)
            events = _candidate_events(candidate, base_events)
            extra_events, overlap_removed = _filter_noncanonical(events, canonical_keys)
            matrix, extra_trades, extra_adverse_trades = _evaluate_events_with_adverse_trades(
                extra_events,
                bars_cache,
                adverse_scenario,
            )
            if not extra_trades.empty:
                extra_trades = extra_trades[extra_trades["selected_delay"] != "none"].copy()
                extra_trades["source_leg"] = candidate.name
                extra_trades.to_csv(
                    OUTPUT_DIR / f"breakout_structure_lead_combo_extra_trades_{candidate.name}.csv",
                    index=False,
                )
            if not extra_adverse_trades.empty:
                extra_adverse_trades = _traded_gate_on(extra_adverse_trades)
                extra_adverse_trades["source_leg"] = candidate.name
                extra_adverse_trades.to_csv(
                    OUTPUT_DIR / f"breakout_structure_lead_combo_extra_adverse10_trades_{candidate.name}.csv",
                    index=False,
                )
            matrix.to_csv(
                OUTPUT_DIR / f"breakout_structure_lead_combo_extra_adverse_{candidate.name}.csv",
                index=False,
            )

            extra_same = _monthly_metrics(extra_trades)
            extra_adverse10 = _matrix_row(matrix, "next_adverse_xslip10bps")
            combo_same_trades = pd.concat([lead_trades, extra_trades], ignore_index=True)
            combo_same = _monthly_metrics(combo_same_trades)
            extra_adverse10_exact = _monthly_metrics(extra_adverse_trades)
            combo_adverse_trades = pd.concat(
                [lead_adverse_trades, extra_adverse_trades],
                ignore_index=True,
            )
            combo_adverse10_exact = _monthly_metrics(combo_adverse_trades)

            row = {
                "variant": candidate.name,
                "description": candidate.description,
                "candidate_source": candidate.source,
                "candidate_source_events": int(len(events)),
                "overlap_removed_events": overlap_removed,
                "extra_events": int(len(extra_events)),
                "extra_same_close_calendar_sum": extra_same["calendar_sum"],
                "extra_same_close_worst_sm": extra_same["worst_sm"],
                "extra_same_close_neg_sm": extra_same["neg_sm"],
                "extra_adverse10_calendar_sum": extra_adverse10["calendar_sum"],
                "extra_adverse10_worst_sm": extra_adverse10["worst_sm"],
                "extra_adverse10_neg_sm": extra_adverse10["neg_sm"],
                "extra_adverse10_calendar_sum_exact": extra_adverse10_exact["calendar_sum"],
                "extra_adverse10_worst_sm_exact": extra_adverse10_exact["worst_sm"],
                "extra_adverse10_neg_sm_exact": extra_adverse10_exact["neg_sm"],
                "lead_same_close_calendar_sum_csv_artifact": lead_csv_same["calendar_sum"],
                "lead_same_close_calendar_sum": lead_same["calendar_sum"],
                "lead_adverse10_calendar_sum_csv_artifact": lead_adverse10["calendar_sum"],
                "lead_adverse10_calendar_sum": lead_adverse10_exact["calendar_sum"],
                "lead_adverse10_calendar_sum_exact": lead_adverse10_exact["calendar_sum"],
                "combo_same_close_calendar_sum": combo_same["calendar_sum"],
                "combo_same_close_delta_vs_lead": combo_same["calendar_sum"] - lead_same["calendar_sum"],
                "combo_same_close_worst_sm": combo_same["worst_sm"],
                "combo_same_close_neg_sm": combo_same["neg_sm"],
                "combo_same_close_trade_count": combo_same["trade_count"],
                "combo_adverse10_calendar_sum_additive": lead_adverse10_exact["calendar_sum"]
                + extra_adverse10["calendar_sum"],
                "combo_adverse10_delta_vs_lead": extra_adverse10["calendar_sum"],
                "combo_adverse10_calendar_sum_exact": combo_adverse10_exact["calendar_sum"],
                "combo_adverse10_delta_vs_lead_exact": combo_adverse10_exact["calendar_sum"]
                - lead_adverse10_exact["calendar_sum"],
                "combo_adverse10_worst_sm_exact": combo_adverse10_exact["worst_sm"],
                "combo_adverse10_neg_sm_exact": combo_adverse10_exact["neg_sm"],
                "combo_adverse10_trade_count_exact": combo_adverse10_exact["trade_count"],
            }
            rows.append(row)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(original_exec_params)

    summary = pd.DataFrame(rows).sort_values(
        ["combo_adverse10_calendar_sum_exact", "combo_same_close_calendar_sum"],
        ascending=[False, False],
    )
    summary_path = OUTPUT_DIR / "breakout_structure_lead_expansion_combo_summary.csv"
    report_path = OUTPUT_DIR / "breakout_structure_lead_expansion_combo_report.md"
    diagnostics_path = OUTPUT_DIR / "breakout_structure_lead_expansion_combo_diagnostics.json"
    summary.to_csv(summary_path, index=False)

    diagnostics = {
        "canonical_events_csv": str(CANONICAL_EVENTS_CSV),
        "canonical_overlap_keys": len(canonical_keys),
        "lead_trades_csv": str(LEAD_TRADES_CSV),
        "lead_gate_on_trades": int(len(lead_trades)),
        "lead_same_close_csv_artifact": lead_csv_same,
        "lead_same_close": lead_same,
        "lead_adverse10_csv_artifact": lead_adverse10,
        "lead_adverse10_exact": lead_adverse10_exact,
        "lead_same_close_exact_delta_vs_csv_artifact": lead_same["calendar_sum"]
        - lead_csv_same["calendar_sum"],
        "lead_adverse10_exact_delta_vs_csv_artifact": lead_adverse10_exact["calendar_sum"]
        - lead_adverse10["calendar_sum"],
        **lead_replay_diagnostics,
        "base_events_csv": str(BASE_EVENTS_CSV),
        "base_model_advance_events": int(len(base_events)),
        "exec_params": {**original_exec_params, **replay_exec_params},
        "runtime_seconds": time.time() - started,
    }
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, diagnostics, report_path)
    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="Canonical lead + breakout structure expansion combo")
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run()


if __name__ == "__main__":
    main()
