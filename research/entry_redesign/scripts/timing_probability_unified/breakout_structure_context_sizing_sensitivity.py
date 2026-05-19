"""Context-gated sizing sensitivity for the wf3 breakout-structure expansion.

Research-only. This keeps the aggressive `wf3_low_eff_low_atr` event family,
then tests whether 4h/12h context quality should control position size instead
of acting only as a binary event filter.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _load_all_1s_bars,
)
from timing_probability_unified.breakout_structure_cross_asset_gate_search import (  # noqa: E402
    _add_context_features,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    BASE_EVENTS_CSV,
    WALKFORWARD_CONTEXT_FIXED,
    WALKFORWARD_FIXED,
    _candidate_events,
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

BASE_CANDIDATE = "wf3_low_eff_low_atr"
CONTEXT_CANDIDATES = [
    "wf3_low_eff_low_atr_ctx4h_up",
    "wf3_low_eff_low_atr_ctx12h_up",
]
FAIL_WEIGHTS = [0.0, 0.25, 0.50, 0.75, 1.0]
ADVERSE_SCENARIO = "next_adverse_xslip10bps"


def _load_csv(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    df = pd.read_csv(path)
    for column in ("touch_time", "signal_start", "signal_end"):
        if column in df.columns:
            df[column] = pd.to_datetime(df[column], utc=True)
    return df


def _event_keys(events: pd.DataFrame) -> pd.Series:
    if "event_key" in events.columns:
        return events["event_key"].astype(str)
    return events.apply(
        lambda row: f"{pd.Timestamp(row['signal_start']).isoformat()}|{row['side']}",
        axis=1,
    )


def _candidate_by_name(name: str):
    for candidate in WALKFORWARD_FIXED + WALKFORWARD_CONTEXT_FIXED:
        if candidate.name == name:
            return candidate
    raise ValueError(f"unknown candidate: {name}")


def _scale_failed_context_events(
    *,
    base_events: pd.DataFrame,
    pass_keys: set[str],
    fail_weight: float,
) -> pd.DataFrame:
    events = base_events.copy().reset_index(drop=True)
    keys = _event_keys(events)
    pass_mask = keys.isin(pass_keys).to_numpy(dtype=bool)
    if fail_weight == 0.0:
        return events[pass_mask].copy().reset_index(drop=True)

    scale = np.where(pass_mask, 1.0, fail_weight)
    events["sizing_multiplier"] = pd.to_numeric(events["sizing_multiplier"], errors="coerce").fillna(0.0) * scale
    events["context_pass"] = pass_mask
    events["context_fail_weight"] = fail_weight
    return events.reset_index(drop=True)


def _write_report(summary: pd.DataFrame, diagnostics: dict[str, Any], output_path: Path) -> None:
    cols = [
        "context_candidate",
        "fail_weight",
        "extra_events",
        "pass_extra_events",
        "fail_extra_events",
        "combo_adverse10_calendar_sum",
        "combo_adverse10_delta_vs_lead",
        "combo_adverse10_worst_sm",
        "combo_adverse10_neg_sm",
        "combo_adverse10_trade_count",
    ]
    lines = [
        "# Breakout Structure Context Sizing Sensitivity",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. This tests context-quality position scaling for `wf3_low_eff_low_atr`; no live defaults are changed.",
        "",
        "## Summary",
        "",
        _markdown_table(summary, cols),
        "",
        "## Interpretation",
        "",
        "- `fail_weight=0` is the hard context overlay.",
        "- `fail_weight=1` is the bare `wf3_low_eff_low_atr` expansion.",
        "- Useful sizing control should keep most of the bare `wf3` return while improving worst month and negative-month count.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run() -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    bars_1s = _load_all_1s_bars("ETHUSDT")
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)
    base_events = _load_csv(BASE_EVENTS_CSV)
    base_events = base_events[base_events["timing_prediction"] != "skip"].copy().reset_index(drop=True)
    base_events = _add_context_features(base_events, bars_1s)

    canonical_keys = _canonical_overlap_keys()
    wf3_source = _candidate_events(_candidate_by_name(BASE_CANDIDATE), base_events)
    wf3_extra, overlap_removed = _filter_noncanonical(wf3_source, canonical_keys)
    wf3_extra_keys = set(_event_keys(wf3_extra))

    lead_all = _lead_trades_all()
    lead_same_all, lead_adverse_all, lead_diagnostics = _lead_replayed_trades(
        lead_all,
        bars_cache,
        ADVERSE_SCENARIO,
    )
    lead_trades = _traded_gate_on(lead_same_all)
    lead_adverse_trades = _traded_gate_on(lead_adverse_all)
    lead_same = _monthly_metrics(lead_trades)
    lead_adverse10 = _monthly_metrics(lead_adverse_trades)

    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_params)

    rows: list[dict[str, Any]] = []
    try:
        for context_name in CONTEXT_CANDIDATES:
            context_source = _candidate_events(_candidate_by_name(context_name), base_events)
            context_extra, context_overlap_removed = _filter_noncanonical(context_source, canonical_keys)
            pass_keys = set(_event_keys(context_extra)) & wf3_extra_keys
            for fail_weight in FAIL_WEIGHTS:
                scaled_events = _scale_failed_context_events(
                    base_events=wf3_extra,
                    pass_keys=pass_keys,
                    fail_weight=fail_weight,
                )
                _, extra_same_trades, extra_adverse_trades = _evaluate_events_with_adverse_trades(
                    scaled_events,
                    bars_cache,
                    ADVERSE_SCENARIO,
                )
                extra_same_trades = _traded_gate_on(extra_same_trades)
                extra_adverse_trades = _traded_gate_on(extra_adverse_trades)
                combo_same = _monthly_metrics(pd.concat([lead_trades, extra_same_trades], ignore_index=True))
                combo_adverse10 = _monthly_metrics(pd.concat([lead_adverse_trades, extra_adverse_trades], ignore_index=True))

                rows.append(
                    {
                        "context_candidate": context_name,
                        "fail_weight": fail_weight,
                        "source_events": int(len(wf3_source)),
                        "overlap_removed_events": int(overlap_removed),
                        "context_overlap_removed_events": int(context_overlap_removed),
                        "extra_events": int(len(scaled_events)),
                        "pass_extra_events": int(len(pass_keys)),
                        "fail_extra_events": int(len(wf3_extra_keys - pass_keys)),
                        "combo_same_close_calendar_sum": combo_same["calendar_sum"],
                        "combo_same_close_worst_sm": combo_same["worst_sm"],
                        "combo_same_close_neg_sm": combo_same["neg_sm"],
                        "combo_same_close_trade_count": combo_same["trade_count"],
                        "combo_adverse10_calendar_sum": combo_adverse10["calendar_sum"],
                        "combo_adverse10_delta_vs_lead": combo_adverse10["calendar_sum"] - lead_adverse10["calendar_sum"],
                        "combo_adverse10_worst_sm": combo_adverse10["worst_sm"],
                        "combo_adverse10_neg_sm": combo_adverse10["neg_sm"],
                        "combo_adverse10_trade_count": combo_adverse10["trade_count"],
                    }
                )
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    summary = pd.DataFrame(rows).sort_values(
        ["combo_adverse10_worst_sm", "combo_adverse10_calendar_sum"],
        ascending=[False, False],
    )
    summary_path = OUTPUT_DIR / "breakout_structure_context_sizing_sensitivity_summary.csv"
    report_path = OUTPUT_DIR / "breakout_structure_context_sizing_sensitivity_report.md"
    diagnostics_path = OUTPUT_DIR / "breakout_structure_context_sizing_sensitivity_diagnostics.json"

    diagnostics = {
        "base_candidate": BASE_CANDIDATE,
        "context_candidates": CONTEXT_CANDIDATES,
        "fail_weights": FAIL_WEIGHTS,
        "base_events_csv": str(BASE_EVENTS_CSV),
        "wf3_source_events": int(len(wf3_source)),
        "wf3_extra_events": int(len(wf3_extra)),
        "lead_same_close": lead_same,
        "lead_adverse10": lead_adverse10,
        "exec_params": {**saved_params, **replay_params},
        "runtime_seconds": time.time() - started,
        **lead_diagnostics,
    }
    summary.to_csv(summary_path, index=False)
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, diagnostics, report_path)
    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="Context sizing sensitivity for breakout-structure wf3")
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run()


if __name__ == "__main__":
    main()
