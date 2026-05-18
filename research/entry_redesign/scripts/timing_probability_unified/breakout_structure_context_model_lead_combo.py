"""Combine canonical ETH pretouch lead with context-model-selected expansion.

Research-only. This is a follow-up to the context-model sizing runner: instead
of reading a model-selected current-shape pool standalone, it removes canonical
overlap and asks whether the selected events add value to the production-aligned
ETH pretouch research lead under next-second adverse fill stress.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BARS_CACHE_DIR,
    EVAL_END,
    EVAL_START,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _load_all_1s_bars,
)
from timing_probability_unified.breakout_structure_context_model_sizing import (  # noqa: E402
    _apply_gate_row,
    _event_keys,
    _fit_predict_context_model,
    _gate_row,
    _load_csv,
    _scaled_events,
)
from timing_probability_unified.breakout_structure_cross_asset_gate_search import (  # noqa: E402
    _add_context_features,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    LEAD_REPLAY_EXEC_OVERRIDES,
    _canonical_overlap_keys,
    _evaluate_events_with_adverse_trades,
    _filter_noncanonical,
    _lead_replayed_trades,
    _lead_trades_all,
    _markdown_table,
    _monthly_metrics,
    _traded_gate_on,
)

logger = logging.getLogger(__name__)

ADVERSE_SCENARIO = "next_adverse_xslip10bps"
EVENTS_CSV = OUTPUT_DIR / "breakout_structure_cross_asset_ethusdt_train3m_events.csv"
CANDIDATE_ROWS_CSV = OUTPUT_DIR / "breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_candidates.csv"
REPLAY_EXEC_PARAMS = {
    "initial_stop_atr": 0.45,
    "breakeven_at_r": 0.8,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 2.0,
}


@dataclass(frozen=True)
class ComboSpec:
    name: str
    base_gate: str
    model_variant: str
    min_train_events: int
    description: str


COMBO_SPECS: tuple[ComboSpec, ...] = (
    ComboSpec(
        name="wide_rf_binary_000",
        base_gate="baseline_model_advance",
        model_variant="rf_binary_000",
        min_train_events=30,
        description="wide current-shape pool, hard keep probability >= 0.5",
    ),
    ComboSpec(
        name="wide_rf_rank_q70_000",
        base_gate="baseline_model_advance",
        model_variant="rf_rank_q70_000",
        min_train_events=30,
        description="wide current-shape pool, hard keep events above train q70 probability",
    ),
    ComboSpec(
        name="wide_rf_binary_025",
        base_gate="baseline_model_advance",
        model_variant="rf_binary_025",
        min_train_events=30,
        description="wide current-shape pool, keep rejected events at 25% size",
    ),
    ComboSpec(
        name="low_eff_rf_rank_median_000",
        base_gate="low_eff_low_atr_q20_q40",
        model_variant="rf_rank_median_000",
        min_train_events=8,
        description="low-eff/low-ATR pool, hard keep events above train median probability",
    ),
)


def _variant_threshold(model_diag: dict[str, Any], variant: str) -> float:
    if variant == "rf_rank_q70_000":
        return float(model_diag.get("train_prob_q70", 0.5))
    if variant == "rf_rank_q60_000":
        return float(model_diag.get("train_prob_q60", 0.5))
    return float(model_diag.get("train_prob_median", 0.5))


def _context_model_events(
    *,
    spec: ComboSpec,
    events: pd.DataFrame,
    candidate_rows: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    eval_start: pd.Timestamp,
    eval_end: pd.Timestamp,
    train_months: int,
) -> tuple[pd.DataFrame, list[dict[str, Any]]]:
    months = sorted(candidate_rows[candidate_rows["gate"] == spec.base_gate]["forward_month"].astype(str).unique())
    parts: list[pd.DataFrame] = []
    diagnostics: list[dict[str, Any]] = []

    for forward_month in months:
        forward_start = pd.Timestamp(f"{forward_month}-01", tz="UTC")
        if forward_start < eval_start or forward_start >= eval_end:
            continue
        forward_end = forward_start + pd.DateOffset(months=1)
        train_start = forward_start - pd.DateOffset(months=train_months)
        base_row = _gate_row(candidate_rows, forward_month, spec.base_gate)
        if base_row is None:
            continue

        train_all = events[(events["touch_time"] >= train_start) & (events["touch_time"] < forward_start)].copy()
        forward_all = events[(events["touch_time"] >= forward_start) & (events["touch_time"] < forward_end)].copy()
        train_events = _apply_gate_row(train_all, base_row)
        forward_events = _apply_gate_row(forward_all, base_row)
        if forward_events.empty:
            continue

        probabilities, model_diag = _fit_predict_context_model(
            train_events=train_events,
            forward_events=forward_events,
            bars_cache=bars_cache,
            min_train_events=spec.min_train_events,
        )
        threshold = _variant_threshold(model_diag, spec.model_variant)
        scaled = _scaled_events(
            forward_events,
            variant=spec.model_variant,
            probabilities=probabilities,
            train_prob_median=threshold,
        )
        scaled["forward_month"] = forward_month
        scaled["context_combo_spec"] = spec.name
        scaled["context_model_status"] = str(model_diag.get("model_status", ""))
        scaled["event_key"] = _event_keys(scaled)
        active = scaled[pd.to_numeric(scaled["sizing_multiplier"], errors="coerce").fillna(0.0) > 0.0].copy()
        parts.append(active)
        diagnostics.append(
            {
                "forward_month": forward_month,
                "base_gate": spec.base_gate,
                "model_variant": spec.model_variant,
                "train_events": int(len(train_events)),
                "forward_events": int(len(forward_events)),
                "active_events": int(len(active)),
                **model_diag,
            }
        )

    selected = pd.concat(parts, ignore_index=True, sort=False) if parts else pd.DataFrame(columns=events.columns)
    return selected.reset_index(drop=True), diagnostics


def _write_report(summary: pd.DataFrame, diagnostics: dict[str, Any], output_path: Path) -> None:
    cols = [
        "variant",
        "base_gate",
        "model_variant",
        "active_source_events",
        "overlap_removed_events",
        "extra_events",
        "extra_adverse10_calendar_sum",
        "combo_adverse10_calendar_sum",
        "combo_adverse10_delta_vs_lead",
        "combo_adverse10_worst_sm",
        "combo_adverse10_neg_sm",
        "combo_adverse10_trade_count",
        "combo_same_close_calendar_sum",
        "combo_same_close_delta_vs_lead",
    ]
    lines = [
        "# Canonical Lead + Context Model Expansion Combo",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. Canonical lead trades are preserved; model-selected current-shape events are overlap-removed by `(signal_start, side)` before combination.",
        "",
        "## Summary",
        "",
        _markdown_table(summary, cols),
        "",
        "## Interpretation",
        "",
        "- This tests additive value to the current research lead, not standalone pool performance.",
        "- `*_000` variants are hard-select checks; rejected events are not traded.",
        "- Promotion still requires early ETH and BTC falsification after any late-ETH additive result.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(
    *,
    train_months: int,
    eval_start: pd.Timestamp,
    eval_end: pd.Timestamp,
) -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    bars_1s = _load_all_1s_bars("ETHUSDT")
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)
    events = _load_csv(EVENTS_CSV)
    events = events[(events["touch_time"] >= eval_start) & (events["touch_time"] < eval_end)].copy().reset_index(drop=True)
    events = _add_context_features(events, bars_1s)
    candidate_rows = _load_csv(CANDIDATE_ROWS_CSV)
    canonical_keys = _canonical_overlap_keys()

    lead_all_trades = _lead_trades_all()
    lead_same_all, lead_adverse_all, lead_diag = _lead_replayed_trades(
        lead_all_trades,
        bars_cache,
        ADVERSE_SCENARIO,
    )
    lead_same_trades = _traded_gate_on(lead_same_all)
    lead_adverse_trades = _traded_gate_on(lead_adverse_all)
    lead_same = _monthly_metrics(lead_same_trades)
    lead_adverse = _monthly_metrics(lead_adverse_trades)

    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    DEFAULT_EXEC_PARAMS.update(REPLAY_EXEC_PARAMS)
    rows: list[dict[str, Any]] = []
    split_diagnostics: dict[str, Any] = {}
    try:
        for spec in COMBO_SPECS:
            logger.info("combo spec %s", spec.name)
            selected_events, spec_diagnostics = _context_model_events(
                spec=spec,
                events=events,
                candidate_rows=candidate_rows,
                bars_cache=bars_cache,
                eval_start=eval_start,
                eval_end=eval_end,
                train_months=train_months,
            )
            split_diagnostics[spec.name] = spec_diagnostics
            selected_events.to_csv(
                OUTPUT_DIR / f"breakout_structure_context_model_lead_combo_active_events_{spec.name}.csv",
                index=False,
            )

            extra_events, overlap_removed = _filter_noncanonical(selected_events, canonical_keys)
            _, extra_same_trades, extra_adverse_trades = _evaluate_events_with_adverse_trades(
                extra_events,
                bars_cache,
                ADVERSE_SCENARIO,
            )
            extra_same_trades = _traded_gate_on(extra_same_trades)
            extra_adverse_trades = _traded_gate_on(extra_adverse_trades)
            extra_same_trades["source_leg"] = spec.name
            extra_adverse_trades["source_leg"] = spec.name
            extra_same_trades.to_csv(
                OUTPUT_DIR / f"breakout_structure_context_model_lead_combo_extra_same_trades_{spec.name}.csv",
                index=False,
            )
            extra_adverse_trades.to_csv(
                OUTPUT_DIR / f"breakout_structure_context_model_lead_combo_extra_adverse10_trades_{spec.name}.csv",
                index=False,
            )

            extra_same = _monthly_metrics(extra_same_trades)
            extra_adverse = _monthly_metrics(extra_adverse_trades)
            combo_same = _monthly_metrics(pd.concat([lead_same_trades, extra_same_trades], ignore_index=True))
            combo_adverse = _monthly_metrics(pd.concat([lead_adverse_trades, extra_adverse_trades], ignore_index=True))

            rows.append(
                {
                    "variant": spec.name,
                    "description": spec.description,
                    "base_gate": spec.base_gate,
                    "model_variant": spec.model_variant,
                    "active_source_events": int(len(selected_events)),
                    "overlap_removed_events": int(overlap_removed),
                    "extra_events": int(len(extra_events)),
                    "extra_same_close_calendar_sum": extra_same["calendar_sum"],
                    "extra_adverse10_calendar_sum": extra_adverse["calendar_sum"],
                    "lead_same_close_calendar_sum": lead_same["calendar_sum"],
                    "lead_adverse10_calendar_sum": lead_adverse["calendar_sum"],
                    "combo_same_close_calendar_sum": combo_same["calendar_sum"],
                    "combo_same_close_delta_vs_lead": combo_same["calendar_sum"] - lead_same["calendar_sum"],
                    "combo_same_close_worst_sm": combo_same["worst_sm"],
                    "combo_same_close_neg_sm": combo_same["neg_sm"],
                    "combo_adverse10_calendar_sum": combo_adverse["calendar_sum"],
                    "combo_adverse10_delta_vs_lead": combo_adverse["calendar_sum"] - lead_adverse["calendar_sum"],
                    "combo_adverse10_worst_sm": combo_adverse["worst_sm"],
                    "combo_adverse10_neg_sm": combo_adverse["neg_sm"],
                    "combo_adverse10_trade_count": combo_adverse["trade_count"],
                }
            )
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    summary = pd.DataFrame(rows).sort_values(
        ["combo_adverse10_calendar_sum", "combo_adverse10_worst_sm"],
        ascending=[False, False],
    )
    summary_path = OUTPUT_DIR / "breakout_structure_context_model_lead_combo_summary.csv"
    report_path = OUTPUT_DIR / "breakout_structure_context_model_lead_combo_report.md"
    diagnostics_path = OUTPUT_DIR / "breakout_structure_context_model_lead_combo_diagnostics.json"
    summary.to_csv(summary_path, index=False)
    diagnostics = {
        "events_csv": str(EVENTS_CSV),
        "candidate_rows_csv": str(CANDIDATE_ROWS_CSV),
        "bars_cache_dir": str(BARS_CACHE_DIR),
        "eval_start": eval_start.isoformat(),
        "eval_end_exclusive": eval_end.isoformat(),
        "train_months": train_months,
        "lead_same_close": lead_same,
        "lead_adverse10": lead_adverse,
        "exec_params": {**saved_params, **REPLAY_EXEC_PARAMS, **LEAD_REPLAY_EXEC_OVERRIDES},
        "specs": [spec.__dict__ for spec in COMBO_SPECS],
        "splits": split_diagnostics,
        "lead_replay": lead_diag,
        "runtime_seconds": time.time() - started,
    }
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, diagnostics, report_path)
    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def _utc_timestamp(text: str) -> pd.Timestamp:
    ts = pd.Timestamp(text)
    return ts.tz_localize("UTC") if ts.tzinfo is None else ts.tz_convert("UTC")


def main() -> None:
    parser = argparse.ArgumentParser(description="Canonical lead + context model expansion combo")
    parser.add_argument("--train-months", type=int, default=3)
    parser.add_argument("--eval-start", default=EVAL_START.isoformat())
    parser.add_argument("--eval-end", default=EVAL_END.isoformat())
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(
        train_months=args.train_months,
        eval_start=_utc_timestamp(args.eval_start),
        eval_end=_utc_timestamp(args.eval_end),
    )


if __name__ == "__main__":
    main()
