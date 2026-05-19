"""Breakout shape expansion sweep for the 2026-05-15 ETH pretouch lead.

Research-only. This script does not touch live/runtime code.

It rebuilds ETHUSDT 1h pretouch events from the 1s bars cache, scans the
current production shape (`restrictive_0p5bps`), original strict T2 shape, and
near-equal slack variants, then evaluates each variant with the frozen
20260515_v1 timing/RF model plus same-close and next-second adverse fill
stress.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from dataclasses import replace
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS, execute_trade  # noqa: E402
from pre_breakout_timing.delay_simulator import DelayResult  # noqa: E402
from timing_probability_unified.adverse_fill import (  # noqa: E402
    STANDARD_FILL_SCENARIOS,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    CombinedPositionConfig,
    compute_calendar_sum,
    compute_combined_positions,
    compute_worst_sm,
)

logger = logging.getLogger(__name__)

BARS_CACHE_DIR = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest"
    / "bars_cache"
)
MODEL_PATH = PROJECT_ROOT / "data" / "pretouch_model.json"
OUTPUT_DIR = SCRIPTS_DIR / "output" / "timing_probability_unified"
CANONICAL_EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "tick_flow_event_sources"
    / "20260514_pretouch_full_window"
    / "feature_filtered_seed_events"
    / "robust_quality"
    / "pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv"
)

FORWARD_START = pd.Timestamp("2025-11-01", tz="UTC")
EVAL_START = pd.Timestamp("2025-06-01", tz="UTC")
EVAL_END = pd.Timestamp("2026-05-01", tz="UTC")

BASE_SHARE = 0.80
SPEED_THRESHOLD = 0.228106
MAX_PRE_TOUCH_SECONDS = 1800.0
MAX_EFF_300S = 1.0
COST_Q50_THRESHOLD = 0.116865
COST_Q50_PENALTY = 0.50
DEFAULT_ROUNDTRIP_COST_ATR = 0.10


@dataclass(frozen=True)
class ShapeVariant:
    name: str
    mode: str  # "restrictive", "strict", "slack"
    bps: float
    description: str


VARIANTS: list[ShapeVariant] = [
    ShapeVariant(
        name="restrictive_0p5bps",
        mode="restrictive",
        bps=0.5,
        description="current production: prev2 must be separated by +0.5bps",
    ),
    ShapeVariant(
        name="strict_0bps",
        mode="strict",
        bps=0.0,
        description="original_t2 strict compare: prev2 > prev1 / prev2 < prev1",
    ),
    ShapeVariant(
        name="near_equal_slack_0p25bps",
        mode="slack",
        bps=0.25,
        description="allow near-equal prev2 within 0.25bps of prev1",
    ),
    ShapeVariant(
        name="near_equal_slack_0p5bps",
        mode="slack",
        bps=0.5,
        description="allow near-equal prev2 within 0.5bps of prev1",
    ),
    ShapeVariant(
        name="near_equal_slack_1p0bps",
        mode="slack",
        bps=1.0,
        description="allow near-equal prev2 within 1.0bps of prev1",
    ),
]


def _load_all_1s_bars(symbol: str) -> pd.DataFrame:
    files = sorted(BARS_CACHE_DIR.glob(f"{symbol}_*_flow_1s.pkl"))
    if not files:
        raise FileNotFoundError(f"no 1s bars cache for {symbol} under {BARS_CACHE_DIR}")

    dfs = []
    for path in files:
        df = pd.read_pickle(path)
        if df.index.tz is None:
            df.index = df.index.tz_localize("UTC")
        else:
            df.index = df.index.tz_convert("UTC")
        dfs.append(df[["open", "high", "low", "close", "volume"]])
        logger.info("loaded %s rows=%d", path.name, len(df))

    bars = pd.concat(dfs).sort_index()
    bars = bars[~bars.index.duplicated(keep="first")]
    bars = bars[(bars.index >= EVAL_START - pd.Timedelta(hours=24)) & (bars.index < EVAL_END)]
    logger.info(
        "combined %s rows=%d range=%s..%s",
        symbol,
        len(bars),
        bars.index.min(),
        bars.index.max(),
    )
    return bars


def _resample_1h(bars_1s: pd.DataFrame) -> pd.DataFrame:
    bars = (
        bars_1s.resample("1h")
        .agg(
            {
                "open": "first",
                "high": "max",
                "low": "min",
                "close": "last",
                "volume": "sum",
            }
        )
        .dropna(subset=["open"])
    )
    ranges = bars["high"] - bars["low"]
    # Live detector uses recent closed bar ranges only, excluding current bar.
    bars["atr_closed_14"] = ranges.shift(1).rolling(14, min_periods=5).mean()
    bars["atr_percentile_24"] = bars["atr_closed_14"].rolling(24, min_periods=5).apply(
        lambda values: float(np.mean(values <= values[-1])) if len(values) else np.nan,
        raw=True,
    )
    for n in range(1, 7):
        bars[f"prev_close_{n}"] = bars["close"].shift(n)
    return bars


def _structure_ready(variant: ShapeVariant, side: str, prev2: float, prev1: float) -> bool:
    if prev2 <= 0 or prev1 <= 0 or pd.isna(prev2) or pd.isna(prev1):
        return False
    tol = variant.bps / 10000.0
    if side == "long":
        if variant.mode == "restrictive":
            return prev2 > prev1 * (1.0 + tol)
        if variant.mode == "strict":
            return prev2 > prev1
        if variant.mode == "slack":
            return prev2 >= prev1 * (1.0 - tol)
    if side == "short":
        if variant.mode == "restrictive":
            return prev2 < prev1 * (1.0 - tol)
        if variant.mode == "strict":
            return prev2 < prev1
        if variant.mode == "slack":
            return prev2 <= prev1 * (1.0 + tol)
    raise ValueError(f"unknown variant/side: {variant} {side}")


def _window_stats_from_arrays(
    *,
    times_ns: np.ndarray,
    high_values: np.ndarray,
    low_values: np.ndarray,
    close_values: np.ndarray,
    end_ns: int,
    end_pos: int,
    seconds: int,
) -> tuple[float, float, float, float] | None:
    start_ns = end_ns - pd.Timedelta(seconds=seconds).value
    start_pos = int(np.searchsorted(times_ns, start_ns, side="left"))
    stop_pos = end_pos + 1
    if stop_pos - start_pos < 10:
        return None
    first = float(close_values[start_pos])
    last = float(close_values[stop_pos - 1])
    high = float(np.max(high_values[start_pos:stop_pos]))
    low = float(np.min(low_values[start_pos:stop_pos]))
    return first, last, high, low


def _side_normalized_move(side: str, start_price: float, end_price: float, atr: float) -> float:
    if atr <= 0:
        return 0.0
    if side == "short":
        return (start_price - end_price) / atr
    return (end_price - start_price) / atr


def _close_pos_side(prev_bar: pd.Series, side: str) -> float:
    rng = float(prev_bar["high"] - prev_bar["low"])
    if rng <= 0:
        return 0.5
    pos = float((prev_bar["close"] - prev_bar["low"]) / rng)
    return 1.0 - pos if side == "short" else pos


def _sma_gap_and_slope(bars_1h: pd.DataFrame, bar_idx: int, atr: float) -> tuple[float, float]:
    if atr <= 0 or bar_idx < 6:
        return 0.0, 0.0
    closed = bars_1h.iloc[:bar_idx]
    last5 = closed["close"].iloc[-5:]
    prev5 = closed["close"].iloc[-6:-1]
    if len(last5) < 5 or len(prev5) < 5:
        return 0.0, 0.0
    sma_current = float(last5.mean())
    sma_prev = float(prev5.mean())
    prev_close = float(closed["close"].iloc[-1])
    return (prev_close - sma_current) / atr, (sma_current - sma_prev) / atr


def _build_event_from_arrays(
    *,
    symbol: str,
    variant: ShapeVariant,
    bars_1s_index: pd.DatetimeIndex,
    times_ns: np.ndarray,
    open_values: np.ndarray,
    high_values: np.ndarray,
    low_values: np.ndarray,
    close_values: np.ndarray,
    bars_1h: pd.DataFrame,
    bar_idx: int,
    side: str,
    signal_start_pos: int,
    touch_abs_pos: int,
    touch_price: float,
    level: float,
    ambiguous_same_second: bool,
) -> dict[str, Any] | None:
    signal_start = bars_1h.index[bar_idx]
    signal_end = signal_start + pd.Timedelta(hours=1)
    touch_time = bars_1s_index[touch_abs_pos]
    prev1 = bars_1h.iloc[bar_idx - 1]
    prev2 = bars_1h.iloc[bar_idx - 2]
    current = bars_1h.iloc[bar_idx]
    atr = float(current["atr_closed_14"])
    if not np.isfinite(atr) or atr <= 0:
        return None

    pre_touch_seconds = float((touch_time - signal_start).total_seconds())
    touch_close = float(close_values[touch_abs_pos])
    so_far_high = float(np.max(high_values[signal_start_pos : touch_abs_pos + 1]))
    so_far_low = float(np.min(low_values[signal_start_pos : touch_abs_pos + 1]))
    if not np.isfinite(so_far_high) or not np.isfinite(so_far_low):
        return None

    touch_ns = int(times_ns[touch_abs_pos])
    stats_300 = _window_stats_from_arrays(
        times_ns=times_ns,
        high_values=high_values,
        low_values=low_values,
        close_values=close_values,
        end_ns=touch_ns,
        end_pos=touch_abs_pos,
        seconds=300,
    )
    if stats_300 is None:
        return None
    first300, last300, high300, low300 = stats_300
    speed_300s_atr = _side_normalized_move(side, first300, last300, atr)
    raw_speed_300s_atr = (last300 - first300) / atr
    total_range_300 = high300 - low300
    eff_300s = abs(last300 - first300) / total_range_300 if total_range_300 > 0 else 0.0

    stats_60 = _window_stats_from_arrays(
        times_ns=times_ns,
        high_values=high_values,
        low_values=low_values,
        close_values=close_values,
        end_ns=touch_ns,
        end_pos=touch_abs_pos,
        seconds=60,
    )
    speed_60s_atr = np.nan
    if stats_60 is not None:
        first60, last60, _, _ = stats_60
        speed_60s_atr = _side_normalized_move(side, first60, last60, atr)

    prev1_range = float(prev1["high"] - prev1["low"])
    prev1_body_atr = abs(float(prev1["close"] - prev1["open"])) / atr
    prev1_range_atr = prev1_range / atr if atr > 0 else 0.0
    prev_sma5_gap_atr, prev_sma5_slope_atr = _sma_gap_and_slope(bars_1h, bar_idx, atr)

    signal_open = float(open_values[signal_start_pos])
    if side == "short":
        touch_extension_atr = (level - touch_close) / atr
        live_touch_extension_atr = (level - touch_price) / atr
        level_to_prev_close_atr = (float(prev1["close"]) - level) / atr
        level_to_signal_open_atr = (signal_open - level) / atr
    else:
        touch_extension_atr = (touch_close - level) / atr
        live_touch_extension_atr = (touch_price - level) / atr
        level_to_prev_close_atr = (level - float(prev1["close"])) / atr
        level_to_signal_open_atr = (level - signal_open) / atr

    event_key = f"{symbol}|{signal_start.isoformat()}|{touch_time.isoformat()}|{side}"
    return {
        "event_id": f"{event_key}|{variant.name}",
        "event_key": event_key,
        "shape_variant": variant.name,
        "shape_mode": variant.mode,
        "shape_tolerance_bps": variant.bps,
        "symbol": symbol,
        "signal_start": signal_start,
        "signal_end": signal_end,
        "touch_time": touch_time,
        "side": side,
        "shape": "original_t2_pretouch",
        "level": level,
        "touch_price": touch_price,
        "touch_close": touch_close,
        "touch_extension_atr": touch_extension_atr,
        "live_touch_extension_atr": live_touch_extension_atr,
        "atr": atr,
        "atr_source": "closed_14_range_mean",
        "signal_open": signal_open,
        "signal_high": float(current["high"]),
        "signal_low": float(current["low"]),
        "signal_close": float(current["close"]),
        "touch_high_so_far": so_far_high,
        "touch_low_so_far": so_far_low,
        "signal_atr_percentile": float(current.get("atr_percentile_24", 0.5))
        if pd.notna(current.get("atr_percentile_24", np.nan))
        else 0.5,
        "prev1_body_atr": prev1_body_atr,
        "prev1_range_atr": prev1_range_atr,
        "prev1_close_pos_side": _close_pos_side(prev1, side),
        "prev_sma5_gap_atr": prev_sma5_gap_atr,
        "prev_sma5_slope_atr": prev_sma5_slope_atr,
        "level_to_prev_close_atr": level_to_prev_close_atr,
        "level_to_signal_open_atr": level_to_signal_open_atr,
        "roundtrip_cost_atr": DEFAULT_ROUNDTRIP_COST_ATR,
        "speed_60s_atr": speed_60s_atr,
        "speed_300s_atr": speed_300s_atr,
        "raw_speed_300s_atr": raw_speed_300s_atr,
        "eff_300s": eff_300s,
        "pre_touch_seconds": pre_touch_seconds,
        "prev_high_1": float(prev1["high"]),
        "prev_high_2": float(prev2["high"]),
        "prev_low_1": float(prev1["low"]),
        "prev_low_2": float(prev2["low"]),
        "ambiguous_same_second": bool(ambiguous_same_second),
    }


def detect_variant_events(
    symbol: str,
    bars_1s: pd.DataFrame,
    bars_1h: pd.DataFrame,
    variant: ShapeVariant,
) -> tuple[pd.DataFrame, dict[str, int]]:
    events: list[dict[str, Any]] = []
    diagnostics = {"dual_touch_same_second_skipped": 0, "bars_scanned": 0}
    bars_index = bars_1s.index
    times_ns = bars_index.astype(np.int64).to_numpy()
    open_values = bars_1s["open"].to_numpy(dtype="float64", copy=False)
    high_values = bars_1s["high"].to_numpy(dtype="float64", copy=False)
    low_values = bars_1s["low"].to_numpy(dtype="float64", copy=False)
    close_values = bars_1s["close"].to_numpy(dtype="float64", copy=False)

    for i in range(2, len(bars_1h)):
        signal_start = bars_1h.index[i]
        if signal_start < EVAL_START or signal_start >= EVAL_END:
            continue
        signal_end = signal_start + pd.Timedelta(hours=1)
        current = bars_1h.iloc[i]
        atr = float(current.get("atr_closed_14", np.nan))
        if not np.isfinite(atr) or atr <= 0:
            continue

        prev1 = bars_1h.iloc[i - 1]
        prev2 = bars_1h.iloc[i - 2]
        long_ready = _structure_ready(variant, "long", float(prev2["high"]), float(prev1["high"]))
        short_ready = _structure_ready(variant, "short", float(prev2["low"]), float(prev1["low"]))
        if not long_ready and not short_ready:
            continue

        start_pos = int(np.searchsorted(times_ns, signal_start.value, side="left"))
        end_pos = int(np.searchsorted(times_ns, signal_end.value, side="left"))
        if start_pos >= end_pos:
            continue

        candidates: list[tuple[int, str, float]] = []
        if long_ready:
            long_level = float(prev2["high"])
            idx = np.where(high_values[start_pos:end_pos] >= long_level)[0]
            if len(idx) > 0:
                local_pos = int(idx[0])
                abs_pos = start_pos + local_pos
                candidates.append((abs_pos, "long", float(high_values[abs_pos])))
        if short_ready:
            short_level = float(prev2["low"])
            idx = np.where(low_values[start_pos:end_pos] <= short_level)[0]
            if len(idx) > 0:
                local_pos = int(idx[0])
                abs_pos = start_pos + local_pos
                candidates.append((abs_pos, "short", float(low_values[abs_pos])))

        if not candidates:
            continue

        candidates.sort(key=lambda item: item[0])
        ambiguous_same_second = len(candidates) > 1 and candidates[0][0] == candidates[1][0]
        if ambiguous_same_second:
            # 1s OHLC cannot reveal tick ordering. Skip to avoid making the
            # expansion look better because of an arbitrary side choice.
            diagnostics["dual_touch_same_second_skipped"] += 1
            continue

        touch_abs_pos, side, touch_price = candidates[0]
        level = float(prev2["high"] if side == "long" else prev2["low"])
        event = _build_event_from_arrays(
            symbol=symbol,
            variant=variant,
            bars_1s_index=bars_index,
            times_ns=times_ns,
            open_values=open_values,
            high_values=high_values,
            low_values=low_values,
            close_values=close_values,
            bars_1h=bars_1h,
            bar_idx=i,
            side=side,
            signal_start_pos=start_pos,
            touch_abs_pos=touch_abs_pos,
            touch_price=touch_price,
            level=level,
            ambiguous_same_second=ambiguous_same_second,
        )
        if event is not None:
            events.append(event)
        diagnostics["bars_scanned"] += 1

    df = pd.DataFrame(events)
    if not df.empty:
        df["touch_time"] = pd.to_datetime(df["touch_time"], utc=True)
        df["signal_start"] = pd.to_datetime(df["signal_start"], utc=True)
        df["signal_end"] = pd.to_datetime(df["signal_end"], utc=True)
    return df, diagnostics


def _tree_leaf_value(node: dict[str, Any], features: list[float]) -> str:
    current = node
    while current.get("l") is not None or current.get("r") is not None:
        feature_idx = int(current.get("f", 0))
        threshold = float(current.get("t", 0.0))
        if feature_idx < 0 or feature_idx >= len(features):
            break
        current = current["l"] if features[feature_idx] <= threshold else current["r"]
    return str(current.get("v", ""))


def _tree_proba(node: dict[str, Any], features: list[float]) -> float:
    current = node
    while current.get("l") is not None or current.get("r") is not None:
        feature_idx = int(current.get("f", 0))
        threshold = float(current.get("t", 0.0))
        if feature_idx < 0 or feature_idx >= len(features):
            return 0.5
        current = current["l"] if features[feature_idx] <= threshold else current["r"]
    return float(current.get("p", 0.0))


def _rf_proba(rf: dict[str, Any], features: list[float]) -> float:
    trees = rf.get("trees") or []
    if not trees:
        return 0.5
    return float(np.mean([_tree_proba(tree, features) for tree in trees]))


def apply_frozen_model(events: pd.DataFrame, model: dict[str, Any]) -> pd.DataFrame:
    if events.empty:
        return events.copy()
    feature_names = list(model["feature_names"])
    medians = list(model["medians"])

    timing: list[str] = []
    rf_probs: list[float] = []
    feature_nan_counts: list[int] = []

    for _, row in events.iterrows():
        values: list[float] = []
        missing = 0
        for idx, name in enumerate(feature_names):
            value = row.get(name, np.nan)
            if pd.isna(value):
                value = medians[idx]
                missing += 1
            values.append(float(value))
        pred = _tree_leaf_value(model["timing_tree"], values)
        if pred not in {"fast", "slow"}:
            pred = "skip"
        prob = _rf_proba(model["rf_model"], values)
        timing.append(pred)
        rf_probs.append(prob)
        feature_nan_counts.append(missing)

    out = events.copy()
    out["timing_prediction"] = timing
    out["rf_probability"] = rf_probs
    out["sizing_multiplier"] = np.clip(out["rf_probability"].to_numpy(dtype="float64") * 2.0, 0, 2)
    out["cost_penalty"] = np.where(
        out["roundtrip_cost_atr"].to_numpy(dtype="float64") >= COST_Q50_THRESHOLD,
        COST_Q50_PENALTY,
        1.0,
    )
    out["sizing_multiplier"] = out["sizing_multiplier"] * out["cost_penalty"]
    out["model_feature_imputations"] = feature_nan_counts
    return out


def live_quality_filter(events: pd.DataFrame) -> pd.DataFrame:
    if events.empty:
        return events.copy()
    mask = (
        (events["pre_touch_seconds"] <= MAX_PRE_TOUCH_SECONDS)
        & (events["speed_300s_atr"].abs() >= SPEED_THRESHOLD)
        & (events["eff_300s"] <= MAX_EFF_300S)
    )
    return events[mask].reset_index(drop=True)


def _bars_cache_for_symbol(symbol: str, bars_1s: pd.DataFrame) -> dict[str, pd.DataFrame]:
    cache: dict[str, pd.DataFrame] = {}
    for month, df in bars_1s.groupby(pd.Grouper(freq="MS")):
        if df.empty:
            continue
        cache[f"{symbol}_{month.strftime('%Y%m')}"] = df
    return cache


def simulate_d0_delays(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> list[list[DelayResult]]:
    results: list[list[DelayResult]] = []
    max_hold = pd.Timedelta(hours=float(DEFAULT_EXEC_PARAMS.get("max_hold_hours", 2.0)))
    for _, event in events.iterrows():
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")
        else:
            touch_time = touch_time.tz_convert("UTC")
        month_key = f"{event['symbol']}_{touch_time.strftime('%Y%m')}"
        bars = bars_cache.get(month_key)
        if bars is None or bars.empty:
            results.append([
                DelayResult(str(event["event_id"]), "D0", 0, None, None, None, "NoData", None, None, None, None, False)
            ])
            continue
        entry_pos = int(bars.index.searchsorted(touch_time, side="left"))
        if entry_pos >= len(bars):
            results.append([
                DelayResult(str(event["event_id"]), "D0", 0, None, None, None, "NoData", None, None, None, None, False)
            ])
            continue
        entry_time = bars.index[entry_pos]
        entry_price = float(bars.loc[entry_time, "close"])
        end_pos = int(bars.index.searchsorted(entry_time + max_hold + pd.Timedelta(seconds=1), side="right"))
        bars_window = bars.iloc[entry_pos:end_pos]
        trade = execute_trade(bars_window, event, entry_time, entry_price, params=DEFAULT_EXEC_PARAMS)
        if trade is None:
            results.append([
                DelayResult(
                    str(event["event_id"]),
                    "D0",
                    0,
                    entry_time,
                    entry_price,
                    None,
                    "MinStopFilter",
                    None,
                    None,
                    None,
                    None,
                    False,
                )
            ])
        else:
            results.append([
                DelayResult(
                    str(event["event_id"]),
                    "D0",
                    0,
                    entry_time,
                    entry_price,
                    float(trade["realistic_pnl_pct"]),
                    str(trade["exit_reason"]),
                    trade["exit_time"],
                    float(trade["hold_seconds"]),
                    float(trade["mfe_r"]),
                    float(trade["mae_r"]),
                    True,
                )
            ])
    return results


def _adjust_entry_price_fast(
    bars: pd.DataFrame,
    original_entry_time: pd.Timestamp,
    original_entry_price: float,
    side: str,
    scenario,
) -> tuple[pd.Timestamp, float] | None:
    if scenario.use_next_bar:
        target_pos = int(bars.index.searchsorted(original_entry_time, side="right"))
    else:
        target_pos = int(bars.index.searchsorted(original_entry_time, side="left"))
        if target_pos >= len(bars) or bars.index[target_pos] != original_entry_time:
            return None
    if target_pos >= len(bars):
        return None
    target_time = bars.index[target_pos]
    target_bar = bars.iloc[target_pos]

    if scenario.use_adverse:
        base_price = float(target_bar["high"] if side == "long" else target_bar["low"])
    else:
        base_price = float(target_bar["close"])

    if scenario.extra_slippage_bps > 0:
        slip_pct = scenario.extra_slippage_bps / 10000.0
        base_price = base_price * (1.0 + slip_pct) if side == "long" else base_price * (1.0 - slip_pct)
    return target_time, base_price


def _reprice_delay_results_fast(
    delay_results: list[list[DelayResult]],
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    scenario,
) -> list[list[DelayResult]]:
    repriced: list[list[DelayResult]] = []
    for event_idx, event_delays in enumerate(delay_results):
        event = events.iloc[event_idx]
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")
        else:
            touch_time = touch_time.tz_convert("UTC")
        bars = bars_cache.get(f"{event['symbol']}_{touch_time.strftime('%Y%m')}")
        if bars is None or bars.empty:
            repriced.append(event_delays)
            continue

        side = str(event["side"])
        new_delays: list[DelayResult] = []
        for dr in event_delays:
            if not dr.traded or dr.entry_price is None or dr.entry_time is None or dr.pnl_pct is None:
                new_delays.append(dr)
                continue
            adj = _adjust_entry_price_fast(
                bars=bars,
                original_entry_time=pd.Timestamp(dr.entry_time),
                original_entry_price=float(dr.entry_price),
                side=side,
                scenario=scenario,
            )
            if adj is None:
                new_delays.append(dr)
                continue
            new_entry_time, new_entry_price = adj
            old_entry = float(dr.entry_price)
            if side == "long":
                pnl_delta = -(new_entry_price - old_entry) / old_entry
            else:
                pnl_delta = -(old_entry - new_entry_price) / old_entry
            new_delays.append(replace(dr, entry_time=new_entry_time, entry_price=new_entry_price, pnl_pct=float(dr.pnl_pct) + pnl_delta))
        repriced.append(new_delays)
    return repriced


def _evaluate_fill_scenarios_fast(
    delay_results: list[list[DelayResult]],
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    timing_predictions: np.ndarray,
    sizing_multipliers: np.ndarray,
    speed_gate_pass: np.ndarray,
) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    config = CombinedPositionConfig(base_notional_share=BASE_SHARE)
    for scenario in STANDARD_FILL_SCENARIOS:
        repriced = _reprice_delay_results_fast(delay_results, events, bars_cache, scenario)
        trades = compute_combined_positions(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=repriced,
            speed_gate_pass=speed_gate_pass,
            config=config,
        )
        gate_on = trades[trades["speed_gate_pass"] == True]  # noqa: E712
        rows.append(
            {
                "scenario": scenario.name,
                "calendar_sum_gate_on": compute_calendar_sum(trades, gate_filter=True),
                "calendar_sum_gate_off": compute_calendar_sum(trades, gate_filter=False),
                "worst_sm_gate_on": compute_worst_sm(trades, gate_filter=True),
                "neg_sm_count": _neg_sm_count(gate_on),
                "btc_cs_gate_on": 0.0,
                "eth_cs_gate_on": float(gate_on[gate_on["symbol"] == "ETHUSDT"]["weighted_pnl"].sum()),
                "trade_count_gate_on": int((gate_on["selected_delay"] != "none").sum()),
            }
        )
    return pd.DataFrame(rows)


def _neg_sm_count(trades: pd.DataFrame) -> int:
    if trades.empty:
        return 0
    df = trades.copy()
    df["year_month"] = pd.to_datetime(df["touch_time"], utc=True).dt.strftime("%Y-%m")
    monthly = df.groupby(["symbol", "year_month"])["weighted_pnl"].sum()
    return int((monthly < 0).sum())


def _evaluate_events(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> tuple[pd.DataFrame, pd.DataFrame]:
    if events.empty:
        empty_matrix = pd.DataFrame()
        empty_trades = pd.DataFrame()
        return empty_matrix, empty_trades

    delay_results = simulate_d0_delays(events, bars_cache)
    predictions = events["timing_prediction"].to_numpy(dtype=object)
    multipliers = events["sizing_multiplier"].to_numpy(dtype="float64")
    speed_gate_pass = np.ones(len(events), dtype=bool)

    base_trades = compute_combined_positions(
        events=events,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=CombinedPositionConfig(base_notional_share=BASE_SHARE),
    )
    base_trades["shape_variant"] = events["shape_variant"].values
    base_trades["event_key"] = events["event_key"].values

    matrix = _evaluate_fill_scenarios_fast(
        delay_results=delay_results,
        events=events,
        bars_cache=bars_cache,
        timing_predictions=predictions,
        sizing_multipliers=multipliers,
        speed_gate_pass=speed_gate_pass,
    )
    return matrix, base_trades


def _canonical_lead_coverage(bars_1h: pd.DataFrame) -> pd.DataFrame:
    if not CANONICAL_EVENTS_CSV.exists():
        logger.warning("canonical events CSV missing: %s", CANONICAL_EVENTS_CSV)
        return pd.DataFrame()
    events = pd.read_csv(CANONICAL_EVENTS_CSV)
    events = events[events["symbol"] == "ETHUSDT"].copy()
    events["signal_start"] = pd.to_datetime(events["signal_start"], utc=True)

    rows: list[dict[str, Any]] = []
    event_ready: dict[str, dict[str, bool]] = {variant.name: {} for variant in VARIANTS}
    missing_bar = 0
    for _, event in events.iterrows():
        signal_start = pd.Timestamp(event["signal_start"])
        if signal_start not in bars_1h.index:
            missing_bar += 1
            continue
        bar_idx = int(bars_1h.index.get_loc(signal_start))
        if bar_idx < 2:
            missing_bar += 1
            continue
        prev1 = bars_1h.iloc[bar_idx - 1]
        prev2 = bars_1h.iloc[bar_idx - 2]
        side = str(event["side"])
        for variant in VARIANTS:
            if side == "long":
                ready = _structure_ready(variant, side, float(prev2["high"]), float(prev1["high"]))
            else:
                ready = _structure_ready(variant, side, float(prev2["low"]), float(prev1["low"]))
            event_ready[variant.name][str(event["event_id"])] = bool(ready)

    trades_path = OUTPUT_DIR / "unified_trades.csv"
    trades = pd.read_csv(trades_path) if trades_path.exists() else pd.DataFrame()

    for variant in VARIANTS:
        ready_map = event_ready[variant.name]
        ready_ids = {event_id for event_id, ready in ready_map.items() if ready}
        row: dict[str, Any] = {
            "variant": variant.name,
            "canonical_eth_events": len(events),
            "missing_signal_bars": missing_bar,
            "ready_events": len(ready_ids),
            "ready_rate": len(ready_ids) / len(events) if len(events) else 0.0,
        }
        if not trades.empty:
            pass_trades = trades[trades["event_id"].isin(ready_ids)].copy()
            fail_trades = trades[~trades["event_id"].isin(ready_ids)].copy()
            row.update(
                {
                    "unified_trades_ready": len(pass_trades),
                    "unified_trades_not_ready": len(fail_trades),
                    "ready_weighted_pnl": float(pass_trades["weighted_pnl"].sum()),
                    "not_ready_weighted_pnl": float(fail_trades["weighted_pnl"].sum()),
                    "total_weighted_pnl": float(trades["weighted_pnl"].sum()),
                }
            )
        rows.append(row)
    return pd.DataFrame(rows)


def _summarize_variant(
    variant: ShapeVariant,
    raw_events: pd.DataFrame,
    quality_events: pd.DataFrame,
    eval_events: pd.DataFrame,
    matrix: pd.DataFrame,
    trades: pd.DataFrame,
    diagnostics: dict[str, int],
) -> dict[str, Any]:
    row: dict[str, Any] = {
        "variant": variant.name,
        "mode": variant.mode,
        "bps": variant.bps,
        "description": variant.description,
        "raw_events": len(raw_events),
        "quality_events": len(quality_events),
        "model_advance_events": int((eval_events["timing_prediction"] != "skip").sum()) if not eval_events.empty else 0,
        "d0_traded_events": int((trades["selected_delay"] != "none").sum()) if not trades.empty else 0,
        "avg_rf_probability": float(eval_events["rf_probability"].mean()) if not eval_events.empty else 0.0,
        "median_rf_probability": float(eval_events["rf_probability"].median()) if not eval_events.empty else 0.0,
        "avg_position_size": float(trades["position_size"].mean()) if not trades.empty else 0.0,
        "dual_touch_same_second_skipped": diagnostics.get("dual_touch_same_second_skipped", 0),
    }
    if not matrix.empty:
        for scenario in ("same_close_xslip0bps", "next_adverse_xslip10bps"):
            m = matrix[matrix["scenario"] == scenario]
            if not m.empty:
                prefix = "same_close" if scenario.startswith("same_close") else "adverse10"
                row[f"{prefix}_calendar_sum"] = float(m.iloc[0]["calendar_sum_gate_on"])
                row[f"{prefix}_worst_sm"] = float(m.iloc[0]["worst_sm_gate_on"])
                row[f"{prefix}_neg_sm"] = int(m.iloc[0]["neg_sm_count"])
    if not trades.empty:
        row["same_close_calendar_sum_direct"] = compute_calendar_sum(trades, gate_filter=True)
        row["same_close_worst_sm_direct"] = compute_worst_sm(trades, gate_filter=True)
        row["same_close_neg_sm_direct"] = _neg_sm_count(trades)
    return row


def _write_report(
    summary: pd.DataFrame,
    adverse: pd.DataFrame,
    incremental: pd.DataFrame,
    canonical_coverage: pd.DataFrame,
    diagnostics: dict[str, Any],
    output_path: Path,
) -> None:
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
            "| " + " | ".join(row[idx].ljust(widths[idx]) for idx in range(len(widths))) + " |"
            for row in rows
        ]
        return "\n".join([header, sep] + body)

    lines: list[str] = []
    lines.append("# Breakout Shape Expansion — ETH Pretouch Timing Lead")
    lines.append("")
    lines.append(f"Generated: {pd.Timestamp.utcnow().isoformat()}")
    lines.append("")
    lines.append("Scope: research-only, ETHUSDT 1h, 2025-06-01..2026-04-30, frozen `data/pretouch_model.json` `20260515_v1`.")
    lines.append("Execution uses D0 same-close baseline plus next-second adverse fill stress. No live defaults are changed.")
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    cols = [
        "variant",
        "raw_events",
        "quality_events",
        "model_advance_events",
        "d0_traded_events",
        "same_close_calendar_sum",
        "same_close_worst_sm",
        "same_close_neg_sm",
        "adverse10_calendar_sum",
        "adverse10_worst_sm",
        "adverse10_neg_sm",
        "avg_rf_probability",
    ]
    lines.append(_markdown_table(summary, cols))
    lines.append("")
    lines.append("## Incremental Events vs Current Production Shape")
    lines.append("")
    if incremental.empty:
        lines.append("No incremental events were found.")
    else:
        lines.append(_markdown_table(incremental, list(incremental.columns)))
    lines.append("")
    lines.append("## Canonical Lead Coverage")
    lines.append("")
    if canonical_coverage.empty:
        lines.append("Canonical coverage was not available.")
    else:
        lines.append(_markdown_table(canonical_coverage, list(canonical_coverage.columns)))
    lines.append("")
    lines.append("## Fill Stress Matrix")
    lines.append("")
    stress_cols = [
        "variant",
        "scenario",
        "calendar_sum_gate_on",
        "worst_sm_gate_on",
        "neg_sm_count",
        "trade_count_gate_on",
    ]
    lines.append(_markdown_table(adverse, stress_cols))
    lines.append("")
    lines.append("## Notes")
    lines.append("")
    lines.append("- `restrictive_0p5bps` is the current production structure gate.")
    lines.append("- `strict_0bps` restores original strict T2 comparison without the extra 0.5bps separation.")
    lines.append("- `near_equal_slack_*` allows near-equal `prev2`/`prev1` structures and is deliberately not a live default.")
    lines.append("- Rebuilt OHLC replay does not have real order-book spread; `roundtrip_cost_atr` is set to the live fallback `0.10`, while slippage stress is handled by the adverse-fill matrix.")
    lines.append("- Feature signs are side-normalized to stay aligned with the 2026-05-15 research model artifact.")
    lines.append("- The production-like D0 rebuilt replay is intentionally a different lens from the canonical `unified_trades.csv`, which uses the frozen canonical event source and timing-selected delays.")
    lines.append("")
    lines.append("## Diagnostics")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str))
    lines.append("```")
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(symbol: str = "ETHUSDT") -> None:
    start = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    with MODEL_PATH.open("r", encoding="utf-8") as fh:
        model = json.load(fh)
    logger.info("loaded model version=%s features=%d", model.get("version"), len(model.get("feature_names", [])))

    bars_1s = _load_all_1s_bars(symbol)
    bars_1h = _resample_1h(bars_1s)
    bars_cache = _bars_cache_for_symbol(symbol, bars_1s)

    original_exec_params = deepcopy(DEFAULT_EXEC_PARAMS)
    DEFAULT_EXEC_PARAMS.update(
        {
            "initial_stop_atr": 0.45,
            "breakeven_at_r": 0.8,
            "trail_start_r": 1.5,
            "trail_buffer_atr": 0.05,
            "max_hold_hours": 2.0,
        }
    )

    summary_rows: list[dict[str, Any]] = []
    adverse_rows: list[pd.DataFrame] = []
    all_eval_events: dict[str, pd.DataFrame] = {}
    all_trades: dict[str, pd.DataFrame] = {}
    diagnostics: dict[str, Any] = {
        "symbol": symbol,
        "model_version": model.get("version"),
        "eval_start": EVAL_START.isoformat(),
        "eval_end_exclusive": EVAL_END.isoformat(),
        "speed_threshold": SPEED_THRESHOLD,
        "max_pre_touch_seconds": MAX_PRE_TOUCH_SECONDS,
        "max_eff_300s": MAX_EFF_300S,
        "base_share": BASE_SHARE,
        "exec_params": {k: DEFAULT_EXEC_PARAMS[k] for k in sorted(DEFAULT_EXEC_PARAMS)},
        "variants": {},
    }

    try:
        for variant in VARIANTS:
            logger.info("=" * 72)
            logger.info("variant %s", variant.name)
            raw_events, variant_diag = detect_variant_events(symbol, bars_1s, bars_1h, variant)
            logger.info("%s detected raw events=%d", variant.name, len(raw_events))
            quality_events = live_quality_filter(raw_events)
            logger.info("%s live quality events=%d", variant.name, len(quality_events))
            eval_events = apply_frozen_model(quality_events, model)
            logger.info(
                "%s model advance events=%d",
                variant.name,
                int((eval_events["timing_prediction"] != "skip").sum()) if not eval_events.empty else 0,
            )
            matrix, trades = _evaluate_events(eval_events, bars_cache)
            if not matrix.empty:
                matrix.insert(0, "variant", variant.name)
                adverse_rows.append(matrix)
            eval_events.to_csv(OUTPUT_DIR / f"breakout_shape_expansion_events_{variant.name}.csv", index=False)
            if not trades.empty:
                trades.to_csv(OUTPUT_DIR / f"breakout_shape_expansion_trades_{variant.name}.csv", index=False)
            all_eval_events[variant.name] = eval_events
            all_trades[variant.name] = trades
            summary_rows.append(
                _summarize_variant(
                    variant,
                    raw_events,
                    quality_events,
                    eval_events,
                    matrix,
                    trades,
                    variant_diag,
                )
            )
            diagnostics["variants"][variant.name] = variant_diag | {
                "raw_events": len(raw_events),
                "quality_events": len(quality_events),
                "eval_events": len(eval_events),
            }
            logger.info(
                "%s raw=%d quality=%d model_advance=%d",
                variant.name,
                len(raw_events),
                len(quality_events),
                int((eval_events["timing_prediction"] != "skip").sum()) if not eval_events.empty else 0,
            )
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(original_exec_params)

    summary = pd.DataFrame(summary_rows)
    adverse = pd.concat(adverse_rows, ignore_index=True) if adverse_rows else pd.DataFrame()
    canonical_coverage = _canonical_lead_coverage(bars_1h)

    base_keys = set(all_eval_events.get("restrictive_0p5bps", pd.DataFrame()).get("event_key", []))
    incremental_rows: list[dict[str, Any]] = []
    for variant in VARIANTS:
        if variant.name == "restrictive_0p5bps":
            continue
        events = all_eval_events.get(variant.name, pd.DataFrame())
        trades = all_trades.get(variant.name, pd.DataFrame())
        if events.empty or trades.empty:
            incremental_rows.append({"variant": variant.name, "extra_events": 0})
            continue
        extra_mask = ~events["event_key"].isin(base_keys)
        extra_events = events[extra_mask].reset_index(drop=True)
        extra_trades = trades[trades["event_key"].isin(set(extra_events["event_key"]))].reset_index(drop=True)
        incremental_rows.append(
            {
                "variant": variant.name,
                "extra_quality_events": len(extra_events),
                "extra_model_advance_events": int((extra_events["timing_prediction"] != "skip").sum()),
                "extra_d0_traded_events": int((extra_trades["selected_delay"] != "none").sum()) if not extra_trades.empty else 0,
                "extra_same_close_calendar_sum": compute_calendar_sum(extra_trades, gate_filter=True)
                if not extra_trades.empty
                else 0.0,
                "extra_same_close_worst_sm": compute_worst_sm(extra_trades, gate_filter=True)
                if not extra_trades.empty
                else 0.0,
                "extra_avg_rf_probability": float(extra_events["rf_probability"].mean()) if not extra_events.empty else 0.0,
            }
        )
    incremental = pd.DataFrame(incremental_rows)

    summary_path = OUTPUT_DIR / "breakout_shape_expansion_summary.csv"
    adverse_path = OUTPUT_DIR / "breakout_shape_expansion_adverse_matrix.csv"
    incremental_path = OUTPUT_DIR / "breakout_shape_expansion_incremental.csv"
    canonical_coverage_path = OUTPUT_DIR / "breakout_shape_expansion_canonical_coverage.csv"
    diagnostics_path = OUTPUT_DIR / "breakout_shape_expansion_diagnostics.json"
    report_path = OUTPUT_DIR / "breakout_shape_expansion_report.md"

    summary.to_csv(summary_path, index=False)
    adverse.to_csv(adverse_path, index=False)
    incremental.to_csv(incremental_path, index=False)
    canonical_coverage.to_csv(canonical_coverage_path, index=False)
    diagnostics["runtime_seconds"] = time.time() - start
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, adverse, incremental, canonical_coverage, diagnostics, report_path)

    logger.info("written %s", summary_path)
    logger.info("written %s", adverse_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="ETH pretouch breakout shape expansion sweep")
    parser.add_argument("--symbol", default="ETHUSDT", choices=["ETHUSDT"])
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(symbol=args.symbol)


if __name__ == "__main__":
    main()
