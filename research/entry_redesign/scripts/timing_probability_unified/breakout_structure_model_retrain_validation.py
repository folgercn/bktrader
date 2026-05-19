"""Retrain timing/probability models on breakout structure expansion pools.

Research-only. This is a promotion check for the structure-quality expansion
line: instead of reusing the old model predictions attached to expansion
events, rebuild the event pool, simulate delays under the production template
exit contract, retrain timing/RF models, and evaluate forward performance.
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
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.adverse_fill import (  # noqa: E402
    STANDARD_FILL_SCENARIOS,
    evaluate_fill_scenarios,
)
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _load_all_1s_bars,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    BASE_EVENTS_CSV,
    CANONICAL_EVENTS_CSV,
    WALKFORWARD_CONTEXT_FIXED,
    WALKFORWARD_FIXED,
    _candidate_events,
    _canonical_overlap_keys,
    _filter_noncanonical,
)
from timing_probability_unified.breakout_structure_cross_asset_gate_search import (  # noqa: E402
    _add_context_features,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    CombinedPositionConfig,
    compute_calendar_sum,
    compute_combined_positions,
    compute_worst_sm,
)
from timing_probability_unified.probability_model import (  # noqa: E402
    compute_sizing_multiplier,
    generate_rf_binary_labels,
    train_rf_probability,
)
from timing_probability_unified.timing_classifier import (  # noqa: E402
    generate_3regime_labels,
    get_selected_delay_pnl,
    train_and_select,
)
from timing_probability_unified.unified_runner import _simulate_delays_for_events  # noqa: E402

logger = logging.getLogger(__name__)

FORWARD_START = pd.Timestamp("2025-11-01", tz="UTC")

PRODUCTION_FEATURES = [
    "roundtrip_cost_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "level_to_signal_open_atr",
    "touch_extension_atr",
    "speed_300s_atr",
    "eff_300s",
    "pre_touch_seconds",
]

EXPANDED_STRUCTURE_FEATURES = PRODUCTION_FEATURES + [
    "signal_atr_percentile",
    "prev1_body_atr",
    "prev_sma5_gap_atr",
    "prev_sma5_slope_atr",
    "level_to_prev_close_atr",
]


@dataclass(frozen=True)
class PoolSpec:
    name: str
    description: str
    expansion_candidate: str | None = None
    context_candidate: str | None = None
    fail_weight: float | None = None


POOL_SPECS = [
    PoolSpec(
        name="canonical_only",
        description="canonical ETH pretouch lead event source only",
    ),
    PoolSpec(
        name="combo_wf3_low_eff_low_atr",
        description="canonical plus 3m train-calibrated low eff + low ATR expansion",
        expansion_candidate="wf3_low_eff_low_atr",
    ),
    PoolSpec(
        name="combo_wf3_low_eff_low_atr_ctx4h_up",
        description="canonical plus 3m low eff + low ATR expansion with favorable prior 4h return",
        expansion_candidate="wf3_low_eff_low_atr_ctx4h_up",
    ),
    PoolSpec(
        name="combo_wf3_low_eff_low_atr_ctx12h_up",
        description="canonical plus 3m low eff + low ATR expansion with favorable prior 12h return",
        expansion_candidate="wf3_low_eff_low_atr_ctx12h_up",
    ),
    PoolSpec(
        name="combo_wf3_low_eff_low_atr_ctx4h_scaled025",
        description="canonical plus wf3 low eff + low ATR expansion; context-failed prior 4h events at 25% size",
        expansion_candidate="wf3_low_eff_low_atr",
        context_candidate="wf3_low_eff_low_atr_ctx4h_up",
        fail_weight=0.25,
    ),
    PoolSpec(
        name="combo_wf5_low_eff_q20",
        description="canonical plus 5m train-calibrated low eff q20 expansion",
        expansion_candidate="wf5_low_eff_q20",
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


def _load_canonical_events() -> pd.DataFrame:
    events = _load_csv(CANONICAL_EVENTS_CSV)
    events = events[events["symbol"] == "ETHUSDT"].copy()
    events["source_leg"] = "canonical"
    return events.reset_index(drop=True)


def _load_base_events() -> pd.DataFrame:
    events = _load_csv(BASE_EVENTS_CSV)
    events = events[events["timing_prediction"] != "skip"].copy()
    return events.reset_index(drop=True)


def _event_keys(events: pd.DataFrame) -> pd.Series:
    if "event_key" in events.columns:
        return events["event_key"].astype(str)
    return events.apply(
        lambda row: f"{pd.Timestamp(row['signal_start']).isoformat()}|{row['side']}",
        axis=1,
    )


def _event_weights(events: pd.DataFrame) -> np.ndarray:
    if events.empty:
        return np.array([], dtype="float64")
    if "pool_size_weight" not in events.columns:
        return np.ones(len(events), dtype="float64")
    return (
        pd.to_numeric(events["pool_size_weight"], errors="coerce")
        .fillna(1.0)
        .to_numpy(dtype="float64")
    )


def _pool_events(spec: PoolSpec, canonical: pd.DataFrame, base_events: pd.DataFrame) -> tuple[pd.DataFrame, int]:
    canonical = canonical.copy().reset_index(drop=True)
    canonical["pool_size_weight"] = 1.0
    if spec.expansion_candidate is None:
        return canonical, 0

    candidate_by_name = {candidate.name: candidate for candidate in WALKFORWARD_FIXED + WALKFORWARD_CONTEXT_FIXED}
    if spec.expansion_candidate not in candidate_by_name:
        raise ValueError(f"unknown expansion candidate: {spec.expansion_candidate}")
    expansion = _candidate_events(candidate_by_name[spec.expansion_candidate], base_events)
    extra, overlap_removed = _filter_noncanonical(expansion, _canonical_overlap_keys())
    extra = extra.copy()
    extra["source_leg"] = spec.expansion_candidate
    extra["pool_size_weight"] = 1.0
    if spec.context_candidate is not None:
        if spec.context_candidate not in candidate_by_name:
            raise ValueError(f"unknown context candidate: {spec.context_candidate}")
        if spec.fail_weight is None:
            raise ValueError(f"context candidate needs fail_weight: {spec.name}")
        context = _candidate_events(candidate_by_name[spec.context_candidate], base_events)
        context_extra, _ = _filter_noncanonical(context, _canonical_overlap_keys())
        pass_keys = set(_event_keys(context_extra))
        pass_mask = _event_keys(extra).isin(pass_keys).to_numpy(dtype=bool)
        extra["context_candidate"] = spec.context_candidate
        extra["context_pass"] = pass_mask
        extra["pool_size_weight"] = np.where(pass_mask, 1.0, float(spec.fail_weight))
    pooled = pd.concat([canonical, extra], ignore_index=True, sort=False)
    pooled["touch_time"] = pd.to_datetime(pooled["touch_time"], utc=True)
    pooled = pooled.sort_values("touch_time").reset_index(drop=True)
    return pooled, overlap_removed


def _split_train_test_forward(events: pd.DataFrame, train_ratio: float = 0.6) -> tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]:
    events = events.copy()
    events["touch_time"] = pd.to_datetime(events["touch_time"], utc=True)
    events = events.sort_values("touch_time").reset_index(drop=True)
    full_window = events[events["touch_time"] < FORWARD_START].copy().reset_index(drop=True)
    forward = events[events["touch_time"] >= FORWARD_START].copy().reset_index(drop=True)
    split_idx = int(len(full_window) * train_ratio)
    train = full_window.iloc[:split_idx].copy().reset_index(drop=True)
    test = full_window.iloc[split_idx:].copy().reset_index(drop=True)
    return train, test, forward


def _features(events: pd.DataFrame, columns: list[str], medians: pd.Series | None = None) -> tuple[pd.DataFrame, pd.Series]:
    out = pd.DataFrame(index=events.index)
    for column in columns:
        if column in events.columns:
            out[column] = pd.to_numeric(events[column], errors="coerce")
        else:
            out[column] = np.nan
    if medians is None:
        medians = out.median(numeric_only=True).fillna(0.0)
    out = out.fillna(medians).fillna(0.0)
    return out, medians


def _predict_rf_probabilities(model: Any, features: pd.DataFrame) -> np.ndarray:
    if len(features) == 0:
        return np.array([], dtype="float64")
    if not hasattr(model, "classes_") or len(model.classes_) < 2:
        return np.full(len(features), 0.5, dtype="float64")
    classes = list(model.classes_)
    if 1 not in classes:
        return np.full(len(features), 0.5, dtype="float64")
    return model.predict_proba(features)[:, classes.index(1)]


def _speed_gate(events: pd.DataFrame, threshold: float) -> np.ndarray:
    if len(events) == 0:
        return np.array([], dtype=bool)
    return (pd.to_numeric(events["speed_300s_atr"], errors="coerce").fillna(-np.inf) >= threshold).to_numpy(dtype=bool)


def _scenario_row(matrix: pd.DataFrame, scenario: str) -> dict[str, Any]:
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


def _evaluate_period(
    *,
    period: str,
    events: pd.DataFrame,
    delay_results: list,
    predictions: np.ndarray,
    multipliers: np.ndarray,
    speed_gate_pass: np.ndarray,
    bars_cache: dict[str, pd.DataFrame],
) -> tuple[dict[str, Any], pd.DataFrame]:
    if events.empty:
        return {
            "period": period,
            "events": 0,
            "speed_gate_pass": 0,
            "same_close_calendar_sum": 0.0,
            "same_close_worst_sm": 0.0,
            "same_close_neg_sm": 0,
            "adverse10_calendar_sum": 0.0,
            "adverse10_worst_sm": 0.0,
            "adverse10_neg_sm": 0,
            "trade_count": 0,
        }, pd.DataFrame()

    scenarios = [
        scenario
        for scenario in STANDARD_FILL_SCENARIOS
        if scenario.name in {"same_close_xslip0bps", "next_adverse_xslip10bps"}
    ]
    matrix = evaluate_fill_scenarios(
        delay_results=delay_results,
        events=events,
        bars_cache=bars_cache,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        speed_gate_pass=speed_gate_pass,
        base_share=BASE_SHARE,
        scenarios=scenarios,
    )
    same = _scenario_row(matrix, "same_close_xslip0bps")
    adverse10 = _scenario_row(matrix, "next_adverse_xslip10bps")
    trades = compute_combined_positions(
        events=events,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=CombinedPositionConfig(base_notional_share=BASE_SHARE),
    )
    trades["period"] = period
    return {
        "period": period,
        "events": int(len(events)),
        "speed_gate_pass": int(speed_gate_pass.sum()),
        "same_close_calendar_sum": same["calendar_sum"],
        "same_close_worst_sm": same["worst_sm"],
        "same_close_neg_sm": same["neg_sm"],
        "adverse10_calendar_sum": adverse10["calendar_sum"],
        "adverse10_worst_sm": adverse10["worst_sm"],
        "adverse10_neg_sm": adverse10["neg_sm"],
        "trade_count": same["trade_count"],
        "gate_off_same_close_calendar_sum": compute_calendar_sum(trades, gate_filter=False),
        "gate_off_worst_sm": compute_worst_sm(trades, gate_filter=False),
    }, trades


def _rf_labels(predictions: np.ndarray, delay_results: list) -> pd.Series:
    pnls = pd.Series(
        [get_selected_delay_pnl(predictions[i], delay_results[i])[1] for i in range(len(predictions))],
        dtype=float,
    )
    return generate_rf_binary_labels(pd.Series(predictions), pnls)


def _evaluate_pool(
    spec: PoolSpec,
    events: pd.DataFrame,
    overlap_removed: int,
    bars_cache: dict[str, pd.DataFrame],
    feature_columns: list[str],
    feature_set: str,
    prepared: dict[str, Any],
) -> tuple[dict[str, Any], pd.DataFrame]:
    train_events = prepared["train_events"]
    test_events = prepared["test_events"]
    forward_events = prepared["forward_events"]
    full_window_events = pd.concat([train_events, test_events], ignore_index=True)
    train_delays = prepared["train_delays"]
    test_delays = prepared["test_delays"]
    forward_delays = prepared["forward_delays"]
    train_errors = prepared["train_errors"]
    test_errors = prepared["test_errors"]
    forward_errors = prepared["forward_errors"]

    train_x_raw, medians = _features(train_events, feature_columns)
    test_x, _ = _features(test_events, feature_columns, medians)
    forward_x, _ = _features(forward_events, feature_columns, medians)
    train_x = train_x_raw

    train_timing_labels = generate_3regime_labels(train_delays)
    test_timing_labels = generate_3regime_labels(test_delays)
    timing_result = train_and_select(
        train_features=train_x,
        train_labels=train_timing_labels,
        delay_results_train=train_delays,
        test_features=test_x,
        test_labels=test_timing_labels,
        delay_results_test=test_delays,
        train_events=train_events,
        test_events=test_events,
    )

    train_rf_labels = _rf_labels(timing_result.train_predictions, train_delays)
    test_rf_labels = _rf_labels(timing_result.test_predictions, test_delays)
    rf_result = train_rf_probability(
        train_features=train_x,
        train_labels=train_rf_labels,
        test_features=test_x,
        test_labels=test_rf_labels,
        n_estimators=200,
        random_state=42,
    )

    train_probs = rf_result.train_probabilities
    test_probs = rf_result.test_probabilities
    forward_predictions = timing_result.classifier.predict(forward_x) if len(forward_x) else np.array([], dtype=object)
    forward_probs = _predict_rf_probabilities(rf_result.model, forward_x)

    train_multipliers = compute_sizing_multiplier(train_probs)
    test_multipliers = compute_sizing_multiplier(test_probs)
    forward_multipliers = compute_sizing_multiplier(forward_probs)
    train_multipliers = train_multipliers * _event_weights(train_events)
    test_multipliers = test_multipliers * _event_weights(test_events)
    forward_multipliers = forward_multipliers * _event_weights(forward_events)

    speed_threshold = float(pd.to_numeric(train_events["speed_300s_atr"], errors="coerce").quantile(0.10))
    train_speed = _speed_gate(train_events, speed_threshold)
    test_speed = _speed_gate(test_events, speed_threshold)
    forward_speed = _speed_gate(forward_events, speed_threshold)
    full_speed = np.concatenate([train_speed, test_speed])

    full_predictions = np.concatenate([timing_result.train_predictions, timing_result.test_predictions])
    full_multipliers = np.concatenate([train_multipliers, test_multipliers])
    full_delays = train_delays + test_delays

    full_metrics, full_trades = _evaluate_period(
        period="full_window",
        events=full_window_events,
        delay_results=full_delays,
        predictions=full_predictions,
        multipliers=full_multipliers,
        speed_gate_pass=full_speed,
        bars_cache=bars_cache,
    )
    forward_metrics, forward_trades = _evaluate_period(
        period="forward",
        events=forward_events,
        delay_results=forward_delays,
        predictions=forward_predictions,
        multipliers=forward_multipliers,
        speed_gate_pass=forward_speed,
        bars_cache=bars_cache,
    )

    trades = pd.concat([full_trades, forward_trades], ignore_index=True, sort=False)
    if not trades.empty:
        trades["pool"] = spec.name
        trades["feature_set"] = feature_set

    train_source_counts = train_events.get("source_leg", pd.Series(dtype=object)).value_counts().to_dict()
    forward_source_counts = forward_events.get("source_leg", pd.Series(dtype=object)).value_counts().to_dict()
    forward_weights = _event_weights(forward_events)

    summary = {
        "pool": spec.name,
        "description": spec.description,
        "feature_set": feature_set,
        "total_events": int(len(events)),
        "overlap_removed_events": int(overlap_removed),
        "train_events": int(len(train_events)),
        "test_events": int(len(test_events)),
        "forward_events": int(len(forward_events)),
        "train_source_counts": json.dumps(train_source_counts, ensure_ascii=False, sort_keys=True),
        "forward_source_counts": json.dumps(forward_source_counts, ensure_ascii=False, sort_keys=True),
        "forward_avg_pool_size_weight": float(np.mean(forward_weights)) if len(forward_weights) else 0.0,
        "forward_min_pool_size_weight": float(np.min(forward_weights)) if len(forward_weights) else 0.0,
        "train_delay_errors": int(len(train_errors)),
        "test_delay_errors": int(len(test_errors)),
        "forward_delay_errors": int(len(forward_errors)),
        "timing_depth": int(timing_result.selected_depth),
        "timing_dt3_loocv": float(timing_result.dt3_loocv_calendar_sum),
        "timing_dt4_loocv": float(timing_result.dt4_loocv_calendar_sum),
        "timing_test_calendar_sum": float(timing_result.test_calendar_sum),
        "rf_train_auc": float(rf_result.train_auc),
        "rf_test_auc": float(rf_result.test_auc),
        "rf_no_signal_warning": bool(rf_result.rf_no_signal_warning),
        "speed_threshold": speed_threshold,
        "full_same_close_calendar_sum": full_metrics["same_close_calendar_sum"],
        "full_adverse10_calendar_sum": full_metrics["adverse10_calendar_sum"],
        "full_adverse10_worst_sm": full_metrics["adverse10_worst_sm"],
        "full_adverse10_neg_sm": full_metrics["adverse10_neg_sm"],
        "full_trade_count": full_metrics["trade_count"],
        "forward_same_close_calendar_sum": forward_metrics["same_close_calendar_sum"],
        "forward_adverse10_calendar_sum": forward_metrics["adverse10_calendar_sum"],
        "forward_adverse10_worst_sm": forward_metrics["adverse10_worst_sm"],
        "forward_adverse10_neg_sm": forward_metrics["adverse10_neg_sm"],
        "forward_trade_count": forward_metrics["trade_count"],
    }
    return summary, trades


def _prepare_pool(events: pd.DataFrame, bars_cache: dict[str, pd.DataFrame]) -> dict[str, Any]:
    train_events, test_events, forward_events = _split_train_test_forward(events)
    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_params)
    try:
        train_delays, train_errors = _simulate_delays_for_events(train_events, bars_cache)
        test_delays, test_errors = _simulate_delays_for_events(test_events, bars_cache)
        forward_delays, forward_errors = _simulate_delays_for_events(forward_events, bars_cache)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)
    return {
        "train_events": train_events,
        "test_events": test_events,
        "forward_events": forward_events,
        "train_delays": train_delays,
        "test_delays": test_delays,
        "forward_delays": forward_delays,
        "train_errors": train_errors,
        "test_errors": test_errors,
        "forward_errors": forward_errors,
    }


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
    lines = [
        "# Breakout Structure Model Retrain Validation",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. Event pools are retrained under production template exit params "
        "(`trail_start_r=1.5`, `max_hold_hours=2.0`) and evaluated on full-window plus forward splits.",
        "",
        "## Summary",
        "",
    ]
    cols = [
        "pool",
        "feature_set",
        "total_events",
        "train_events",
        "test_events",
        "forward_events",
        "forward_avg_pool_size_weight",
        "timing_depth",
        "rf_test_auc",
        "full_adverse10_calendar_sum",
        "full_adverse10_worst_sm",
        "full_adverse10_neg_sm",
        "forward_adverse10_calendar_sum",
        "forward_adverse10_worst_sm",
        "forward_adverse10_neg_sm",
        "forward_trade_count",
    ]
    lines.append(_markdown_table(summary, cols))
    lines.extend([
        "",
        "## Interpretation",
        "",
        "- `production8` is the Go live trainer feature contract.",
        "- `structure13` adds ATR percentile, prior body, SMA gap/slope, and level-to-prev-close structure features.",
        "- A pool only deserves promotion if forward adverse10 improves without relying on in-sample `static_*` gates.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ])
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run() -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    bars_1s = _load_all_1s_bars("ETHUSDT")
    canonical = _load_canonical_events()
    base_events = _add_context_features(_load_base_events(), bars_1s)
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)

    feature_sets = {
        "production8": PRODUCTION_FEATURES,
        "structure13": EXPANDED_STRUCTURE_FEATURES,
    }

    summaries: list[dict[str, Any]] = []
    trades_parts: list[pd.DataFrame] = []
    pool_sizes: dict[str, Any] = {}
    for spec in POOL_SPECS:
        events, overlap_removed = _pool_events(spec, canonical, base_events)
        prepared = _prepare_pool(events, bars_cache)
        pool_sizes[spec.name] = {
            "events": int(len(events)),
            "overlap_removed_events": int(overlap_removed),
            "source_counts": events["source_leg"].value_counts().to_dict() if "source_leg" in events.columns else {},
        }
        for feature_set, feature_columns in feature_sets.items():
            logger.info("pool=%s feature_set=%s events=%d", spec.name, feature_set, len(events))
            summary, trades = _evaluate_pool(
                spec=spec,
                events=events,
                overlap_removed=overlap_removed,
                bars_cache=bars_cache,
                feature_columns=feature_columns,
                feature_set=feature_set,
                prepared=prepared,
            )
            summaries.append(summary)
            if not trades.empty:
                trades_parts.append(trades)

    summary = pd.DataFrame(summaries).sort_values(
        ["forward_adverse10_calendar_sum", "full_adverse10_calendar_sum"],
        ascending=[False, False],
    )
    trades = pd.concat(trades_parts, ignore_index=True, sort=False) if trades_parts else pd.DataFrame()

    summary_path = OUTPUT_DIR / "breakout_structure_model_retrain_summary.csv"
    trades_path = OUTPUT_DIR / "breakout_structure_model_retrain_trades.csv"
    diagnostics_path = OUTPUT_DIR / "breakout_structure_model_retrain_diagnostics.json"
    report_path = OUTPUT_DIR / "breakout_structure_model_retrain_report.md"

    diagnostics = {
        "canonical_events_csv": str(CANONICAL_EVENTS_CSV),
        "base_events_csv": str(BASE_EVENTS_CSV),
        "pool_sizes": pool_sizes,
        "feature_sets": feature_sets,
        "base_share": BASE_SHARE,
        "forward_start": FORWARD_START.isoformat(),
        "runtime_seconds": time.time() - started,
    }
    summary.to_csv(summary_path, index=False)
    if not trades.empty:
        trades.to_csv(trades_path, index=False)
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, diagnostics, report_path)

    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="Breakout structure model retrain validation")
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run()


if __name__ == "__main__":
    main()
