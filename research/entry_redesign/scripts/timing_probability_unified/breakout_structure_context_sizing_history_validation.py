"""Historical validation for breakout-structure context sizing.

Research-only. This replays a prebuilt current-shape event source and its
walk-forward gate rows, then compares hard context filtering with partial
position sizing for context-failed `low_eff_low_atr` events.
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
    BARS_CACHE_DIR,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
)
from timing_probability_unified.breakout_structure_cross_asset_gate_search import (  # noqa: E402
    _add_context_features,
)
from timing_probability_unified.breakout_structure_cross_asset_validation import (  # noqa: E402
    _load_symbol_1s_bars,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    _apply_conditions,
    _evaluate_events_with_adverse_trades,
    _markdown_table,
    _monthly_metrics,
    _traded_gate_on,
)

logger = logging.getLogger(__name__)

BASE_GATE = "low_eff_low_atr_q20_q40"
CONTEXT_GATES = [
    "low_eff_low_atr_ctx4h_up",
    "low_eff_low_atr_ctx12h_up",
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


def _gate_events(events: pd.DataFrame, rows: pd.DataFrame, gate: str) -> pd.DataFrame:
    gate_rows = rows[rows["gate"] == gate].copy()
    parts: list[pd.DataFrame] = []
    for row in gate_rows.itertuples(index=False):
        month = str(row.forward_month)
        month_events = events[events["touch_time"].dt.strftime("%Y-%m") == month].copy()
        parts.append(_apply_conditions(month_events, str(row.conditions)))
    out = pd.concat(parts, ignore_index=True) if parts else pd.DataFrame(columns=events.columns)
    if "timing_prediction" in out.columns:
        out = out[out["timing_prediction"] != "skip"].copy()
    return out.reset_index(drop=True)


def _scale_failed_context_events(
    *,
    base_events: pd.DataFrame,
    pass_keys: set[str],
    fail_weight: float,
) -> pd.DataFrame:
    keys = _event_keys(base_events)
    pass_mask = keys.isin(pass_keys).to_numpy(dtype=bool)
    if fail_weight == 0.0:
        out = base_events[pass_mask].copy().reset_index(drop=True)
        out["context_pass"] = True
        out["context_fail_weight"] = fail_weight
        return out
    out = base_events.copy().reset_index(drop=True)
    scale = np.where(pass_mask, 1.0, fail_weight)
    out["sizing_multiplier"] = pd.to_numeric(out["sizing_multiplier"], errors="coerce").fillna(0.0) * scale
    out["context_pass"] = pass_mask
    out["context_fail_weight"] = fail_weight
    return out


def _evaluate_variant(
    *,
    name: str,
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    pass_events: int | None = None,
    fail_events: int | None = None,
) -> dict[str, Any]:
    _, same_trades, adverse_trades = _evaluate_events_with_adverse_trades(
        events,
        bars_cache,
        ADVERSE_SCENARIO,
    )
    same_trades = _traded_gate_on(same_trades)
    adverse_trades = _traded_gate_on(adverse_trades)
    same = _monthly_metrics(same_trades)
    adverse10 = _monthly_metrics(adverse_trades)
    return {
        "variant": name,
        "events": int(len(events)),
        "pass_events": pass_events if pass_events is not None else "",
        "fail_events": fail_events if fail_events is not None else "",
        "same_close_calendar_sum": same["calendar_sum"],
        "same_close_worst_sm": same["worst_sm"],
        "same_close_neg_sm": same["neg_sm"],
        "same_close_trade_count": same["trade_count"],
        "adverse10_calendar_sum": adverse10["calendar_sum"],
        "adverse10_worst_sm": adverse10["worst_sm"],
        "adverse10_neg_sm": adverse10["neg_sm"],
        "adverse10_trade_count": adverse10["trade_count"],
    }


def _write_report(summary: pd.DataFrame, diagnostics: dict[str, Any], output_path: Path) -> None:
    cols = [
        "variant",
        "events",
        "pass_events",
        "fail_events",
        "same_close_calendar_sum",
        "same_close_worst_sm",
        "same_close_neg_sm",
        "adverse10_calendar_sum",
        "adverse10_worst_sm",
        "adverse10_neg_sm",
        "adverse10_trade_count",
    ]
    lines = [
        "# Breakout Structure Context Sizing History Validation",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. This validates context sizing on a supplied historical current-shape event source.",
        "",
        "## Summary",
        "",
        _markdown_table(summary, cols),
        "",
        "## Interpretation",
        "",
        "- Baseline rows use the supplied walk-forward gate rows and historical event source.",
        "- Context-scaled rows keep all `low_eff_low_atr` events and scale events that fail the context gate.",
        "- A robust candidate should remain positive outside the 2025-11..2026-04 forward window.",
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
    symbol: str,
    events_csv: Path,
    candidate_rows_csv: Path,
    bars_cache_dir: Path,
    eval_start: pd.Timestamp,
    eval_end: pd.Timestamp,
    output_tag: str,
) -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    events = _load_csv(events_csv)
    events = events[(events["touch_time"] >= eval_start) & (events["touch_time"] < eval_end)].copy()
    bars_1s = _load_symbol_1s_bars(symbol, bars_cache_dir, eval_start, eval_end)
    events = _add_context_features(events.reset_index(drop=True), bars_1s)
    bars_cache = _bars_cache_for_symbol(symbol, bars_1s)
    rows = _load_csv(candidate_rows_csv)

    base_events = _gate_events(events, rows, BASE_GATE)
    base_keys = set(_event_keys(base_events))

    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_params)

    summary_rows: list[dict[str, Any]] = []
    try:
        summary_rows.append(
            _evaluate_variant(
                name="baseline_model_advance",
                events=_gate_events(events, rows, "baseline_model_advance"),
                bars_cache=bars_cache,
            )
        )
        summary_rows.append(
            _evaluate_variant(
                name=BASE_GATE,
                events=base_events,
                bars_cache=bars_cache,
            )
        )
        for context_gate in CONTEXT_GATES:
            context_events = _gate_events(events, rows, context_gate)
            pass_keys = set(_event_keys(context_events)) & base_keys
            fail_count = len(base_keys - pass_keys)
            for fail_weight in FAIL_WEIGHTS:
                scaled = _scale_failed_context_events(
                    base_events=base_events,
                    pass_keys=pass_keys,
                    fail_weight=fail_weight,
                )
                summary_rows.append(
                    _evaluate_variant(
                        name=f"{context_gate}_fail_weight_{fail_weight:.2f}",
                        events=scaled,
                        bars_cache=bars_cache,
                        pass_events=len(pass_keys),
                        fail_events=fail_count,
                    )
                )
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    summary = pd.DataFrame(summary_rows).sort_values(
        ["adverse10_calendar_sum", "adverse10_worst_sm"],
        ascending=[False, False],
    )
    summary_path = OUTPUT_DIR / f"breakout_structure_context_sizing_history_{output_tag}_summary.csv"
    report_path = OUTPUT_DIR / f"breakout_structure_context_sizing_history_{output_tag}_report.md"
    diagnostics_path = OUTPUT_DIR / f"breakout_structure_context_sizing_history_{output_tag}_diagnostics.json"
    diagnostics = {
        "symbol": symbol,
        "events_csv": str(events_csv),
        "candidate_rows_csv": str(candidate_rows_csv),
        "bars_cache_dir": str(bars_cache_dir),
        "eval_start": eval_start.isoformat(),
        "eval_end_exclusive": eval_end.isoformat(),
        "base_gate": BASE_GATE,
        "context_gates": CONTEXT_GATES,
        "fail_weights": FAIL_WEIGHTS,
        "events": int(len(events)),
        "base_gate_events": int(len(base_events)),
        "exec_params": {**saved_params, **replay_params},
        "runtime_seconds": time.time() - started,
    }
    summary.to_csv(summary_path, index=False)
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, diagnostics, report_path)
    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def _utc_timestamp(text: str) -> pd.Timestamp:
    ts = pd.Timestamp(text)
    return ts.tz_localize("UTC") if ts.tzinfo is None else ts.tz_convert("UTC")


def main() -> None:
    parser = argparse.ArgumentParser(description="Historical context sizing validation")
    parser.add_argument("--symbol", default="ETHUSDT", choices=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--events-csv", type=Path, required=True)
    parser.add_argument("--candidate-rows-csv", type=Path, required=True)
    parser.add_argument("--bars-cache-dir", type=Path, default=BARS_CACHE_DIR)
    parser.add_argument("--eval-start", required=True)
    parser.add_argument("--eval-end", required=True)
    parser.add_argument("--output-tag", required=True)
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(
        symbol=args.symbol,
        events_csv=args.events_csv,
        candidate_rows_csv=args.candidate_rows_csv,
        bars_cache_dir=args.bars_cache_dir,
        eval_start=_utc_timestamp(args.eval_start),
        eval_end=_utc_timestamp(args.eval_end),
        output_tag=args.output_tag,
    )


if __name__ == "__main__":
    main()
