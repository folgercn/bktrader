#!/usr/bin/env python3
"""ETH Q1 2026 1s replay comparing baseline breakout vs t-3 breakout shape.

Research-only script. It does not touch live/execution code paths.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

from backTest import (
    _compute_backtest_stats,
    _get_reentry_window_real_order_size,
    _reentry_triggered,
    _resolve_reentry_price,
    _resolve_regime_ready,
    _resolve_stop_price,
    apply_breakout_levels,
)


COMMON_REPLAY_KWARGS = {
    "dir2_zero_initial": True,
    "zero_initial_mode": "reentry_window",
    "fixed_slippage": 0.0005,
    "stop_loss_atr": 0.05,
    "stop_mode": "atr",
    "max_trades_per_bar": 2,
    "reentry_size_schedule": [0.20, 0.10],
    "long_reentry_atr": 0.1,
    "short_reentry_atr": 0.0,
    "profit_protect_atr": 1.0,
    "trailing_stop_atr": 0.3,
    "delayed_trailing_activation": 0.5,
    "reentry_anchor_levels": "wick",
    "reentry_trigger_mode": "reclaim",
}

DEFAULT_TICK_FILES = [
    "dataset/archive/ETHUSDT-trades-2026-01/ETHUSDT-trades-2026-01.csv",
    "dataset/archive/ETHUSDT-trades-2026-02/ETHUSDT-trades-2026-02.zip",
    "dataset/archive/ETHUSDT-trades-2026-03/ETHUSDT-trades-2026-03.zip",
]

SCENARIOS = [
    {
        "scenario": "t3_sma5_baseline",
        "breakout_shape": "baseline_plus_t3",
        "replay_mode": "live_intrabar_sma5",
        "t3_reentry_size_schedule": [0.20, 0.10],
        "t3_cooldown_bars": 0,
        "timeframes": ["30min"],
        "t3_quality_filters": {},
    },
    {
        "scenario": "A_sep_0p25",
        "breakout_shape": "baseline_plus_t3",
        "replay_mode": "live_intrabar_sma5",
        "t3_reentry_size_schedule": [0.20, 0.10],
        "t3_cooldown_bars": 0,
        "timeframes": ["30min"],
        "t3_quality_filters": {"min_sma_atr_separation": 0.25},
        "quality_filter_shapes": ["t3_swing"],
    },
    {
        "scenario": "all_breakouts_sep_0p25",
        "breakout_shape": "baseline_plus_t3",
        "replay_mode": "live_intrabar_sma5",
        "t3_reentry_size_schedule": [0.20, 0.10],
        "t3_cooldown_bars": 0,
        "timeframes": ["30min"],
        "t3_quality_filters": {"min_sma_atr_separation": 0.25},
        "quality_filter_shapes": ["original_t2", "t3_swing"],
    },
]


def _as_utc_timestamp(value: str) -> pd.Timestamp:
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        return ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def _read_tick_chunks(path: str, chunksize: int):
    reader = pd.read_csv(
        path,
        header=0,
        usecols=["price", "qty", "time"],
        dtype={"price": "float32", "qty": "float32", "time": "int64"},
        chunksize=chunksize,
        compression="infer",
    )
    for chunk in reader:
        yield chunk.rename(columns={"time": "timestamp"})


def build_continuous_second_bars(paths, start: pd.Timestamp, end: pd.Timestamp, chunksize: int):
    start_ms = int(start.timestamp() * 1000)
    end_ms = int(end.timestamp() * 1000)
    pending = None
    summaries = []
    raw_tick_rows = 0
    kept_tick_rows = 0

    for path in paths:
        print(f"reading ticks: {path}", flush=True)
        for chunk_index, chunk in enumerate(_read_tick_chunks(path, chunksize), start=1):
            if chunk.empty:
                continue
            raw_tick_rows += len(chunk)
            if chunk["timestamp"].iloc[0] > end_ms:
                break
            if chunk["timestamp"].iloc[-1] < start_ms:
                continue

            chunk = chunk[(chunk["timestamp"] >= start_ms) & (chunk["timestamp"] <= end_ms)]
            if chunk.empty:
                continue
            kept_tick_rows += len(chunk)

            if pending is not None and not pending.empty:
                chunk = pd.concat([pending, chunk], ignore_index=True)
                pending = None

            chunk["second_ms"] = (chunk["timestamp"] // 1000) * 1000
            last_second = chunk["second_ms"].iloc[-1]
            pending = chunk[chunk["second_ms"] == last_second].copy()
            complete = chunk[chunk["second_ms"] != last_second]
            if complete.empty:
                continue

            second_df = complete.groupby("second_ms", sort=False).agg(
                open=("price", "first"),
                high=("price", "max"),
                low=("price", "min"),
                close=("price", "last"),
                volume=("qty", "sum"),
            )
            summaries.append(second_df.reset_index())
            if chunk_index % 50 == 0:
                print(f"  chunks={chunk_index} kept_rows={kept_tick_rows:,}", flush=True)

    if pending is not None and not pending.empty:
        second_df = pending.groupby("second_ms", sort=False).agg(
            open=("price", "first"),
            high=("price", "max"),
            low=("price", "min"),
            close=("price", "last"),
            volume=("qty", "sum"),
        )
        summaries.append(second_df.reset_index())

    if not summaries:
        raise RuntimeError("no tick data was aggregated into second bars")

    second_bars = pd.concat(summaries, ignore_index=True)
    second_bars.sort_values("second_ms", inplace=True)
    sparse_second_rows = int(len(second_bars))
    second_bars["timestamp"] = pd.to_datetime(second_bars["second_ms"], unit="ms", utc=True)
    second_bars.set_index("timestamp", inplace=True)
    second_bars = second_bars[["open", "high", "low", "close", "volume"]]

    full_index = pd.date_range(start=start, end=end, freq="1s", tz="UTC")
    second_bars = second_bars.reindex(full_index)
    first_close = float(second_bars["close"].dropna().iloc[0])
    second_bars["close"] = second_bars["close"].ffill().fillna(first_close)
    second_bars["open"] = second_bars["open"].fillna(second_bars["close"])
    second_bars["high"] = second_bars["high"].fillna(second_bars["close"])
    second_bars["low"] = second_bars["low"].fillna(second_bars["close"])
    second_bars["volume"] = second_bars["volume"].fillna(0.0)

    stats = {
        "raw_tick_rows": int(raw_tick_rows),
        "kept_tick_rows": int(kept_tick_rows),
        "sparse_second_rows": sparse_second_rows,
        "continuous_second_rows": int(len(second_bars)),
        "first_second": second_bars.index[0].isoformat(),
        "last_second": second_bars.index[-1].isoformat(),
    }
    return second_bars, stats


def build_signal_frame(second_bars: pd.DataFrame, timeframe: str):
    one_min = second_bars.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    signal = one_min.resample(timeframe).agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()

    true_range = pd.concat(
        [
            signal["high"] - signal["low"],
            (signal["high"] - signal["close"].shift()).abs(),
            (signal["low"] - signal["close"].shift()).abs(),
        ],
        axis=1,
    ).max(axis=1)
    signal["sma5"] = signal["close"].rolling(5).mean()
    signal["ma5"] = signal["sma5"]
    signal["prev_sma5_1"] = signal["sma5"].shift(1)
    signal["sma5_slope"] = signal["sma5"] - signal["prev_sma5_1"]
    signal["atr"] = true_range.rolling(14).mean()
    signal["atr_percentile"] = signal["atr"].rolling(240, min_periods=50).apply(_last_percentile, raw=True)
    for n in range(1, 13):
        signal[f"prev_close_{n}"] = signal["close"].shift(n)
    signal = apply_breakout_levels(signal, "wick")
    signal["prev_high_3"] = signal["high"].shift(3)
    signal["prev_low_3"] = signal["low"].shift(3)
    return one_min, signal


def _last_percentile(values: np.ndarray) -> float:
    clean = values[~np.isnan(values)]
    if len(clean) == 0:
        return np.nan
    return float((clean <= clean[-1]).mean() * 100.0)


def _positive(*values: float) -> bool:
    return all(pd.notna(v) and float(v) > 0 for v in values)


def _long_breakout(sig: pd.Series, current_high: float, breakout_shape: str):
    p1 = sig["prev_high_1"]
    p2 = sig["prev_high_2"]
    p3 = sig.get("prev_high_3", np.nan)
    if _positive(p1, p2) and p2 > p1 and current_high >= p2:
        return True, float(p2), "original_t2"
    if breakout_shape == "baseline_plus_t3":
        if _positive(p1, p2, p3) and p3 > p2 and p3 > p1 and p1 > p2 and current_high >= p3:
            return True, float(p3), "t3_swing"
    return False, np.nan, ""


def _short_breakout(sig: pd.Series, current_low: float, breakout_shape: str):
    p1 = sig["prev_low_1"]
    p2 = sig["prev_low_2"]
    p3 = sig.get("prev_low_3", np.nan)
    if _positive(p1, p2) and p2 < p1 and current_low <= p2:
        return True, float(p2), "original_t2"
    if breakout_shape == "baseline_plus_t3":
        if _positive(p1, p2, p3) and p3 < p2 and p3 < p1 and p1 < p2 and current_low <= p3:
            return True, float(p3), "t3_swing"
    return False, np.nan, ""


def _open_position(
    balance,
    sig,
    side,
    entry_price,
    notional_share,
    reason,
    stop_mode,
    stop_loss_atr,
    breakout_shape_name,
    replay_mode,
):
    notional_value = balance * notional_share
    position = {
        "side": side,
        "entry_p": entry_price,
        "sl": _resolve_stop_price(side, entry_price, sig, stop_mode, stop_loss_atr),
        "protected": reason == "PT-Reentry",
        "notional": notional_value,
        "breakout_shape_name": breakout_shape_name,
        "replay_mode": replay_mode,
    }
    if side == "long":
        position["hwm"] = entry_price
    else:
        position["lwm"] = entry_price
    balance -= notional_value * 0.001
    return balance, position


def _shape_schedule(shape_name: str, baseline_schedule: list[float], t3_schedule) -> list[float]:
    if shape_name == "t3_swing" and t3_schedule:
        return t3_schedule
    return baseline_schedule


def _t3_exit_override(shape_name: str, overrides: dict | None, key: str, default):
    if shape_name != "t3_swing" or not overrides or key not in overrides:
        return default
    return overrides[key]


def _ctx_side_return_atr(sig, side: str, lookback_bars: int) -> float:
    latest = sig.get("prev_close_1", np.nan)
    earlier = sig.get(f"prev_close_{int(lookback_bars)}", np.nan)
    atr = sig.get("atr", np.nan)
    if pd.isna(latest) or pd.isna(earlier) or not _positive(atr):
        return np.nan
    signed = float(latest) - float(earlier)
    if side == "short":
        signed *= -1.0
    return signed / float(atr)


def _side_close_position(sig, side: str) -> float:
    prev_high = sig.get("prev_high_1", np.nan)
    prev_low = sig.get("prev_low_1", np.nan)
    prev_close = sig.get("prev_close_1", np.nan)
    if pd.isna(prev_high) or pd.isna(prev_low) or pd.isna(prev_close):
        return np.nan
    span = float(prev_high) - float(prev_low)
    if span <= 0.0:
        return np.nan
    close_pos = (float(prev_close) - float(prev_low)) / span
    if side == "short":
        close_pos = 1.0 - close_pos
    return float(close_pos)


def _entry_context_metadata(
    sig,
    *,
    side: str,
    signal_start: pd.Timestamp,
    signal_bar_index: int,
    breakout_level: float,
    current_price: float,
    pre_touch_seconds: float | None,
    sizing_reject_reason: str,
) -> dict:
    atr = sig.get("atr", np.nan)
    breakout_extension_atr = np.nan
    level_to_signal_open_atr = np.nan
    prev1_range_atr = np.nan
    if _positive(atr):
        breakout_extension_atr = abs(float(current_price) - float(breakout_level)) / float(atr)
        signal_open = sig.get("open", np.nan)
        if pd.notna(signal_open):
            if side == "short":
                level_to_signal_open_atr = (float(signal_open) - float(breakout_level)) / float(atr)
            else:
                level_to_signal_open_atr = (float(breakout_level) - float(signal_open)) / float(atr)
        prev_high = sig.get("prev_high_1", np.nan)
        prev_low = sig.get("prev_low_1", np.nan)
        if pd.notna(prev_high) and pd.notna(prev_low):
            prev1_range_atr = (float(prev_high) - float(prev_low)) / float(atr)

    return {
        "side": side,
        "signal_start": signal_start,
        "signal_bar_index": int(signal_bar_index),
        "breakout_level": float(breakout_level),
        "breakout_pre_touch_seconds": float(pre_touch_seconds)
        if pre_touch_seconds is not None and np.isfinite(float(pre_touch_seconds))
        else np.nan,
        "breakout_extension_atr": breakout_extension_atr,
        "level_to_signal_open_atr": level_to_signal_open_atr,
        "atr": float(atr) if pd.notna(atr) else np.nan,
        "atr_percentile": float(sig.get("atr_percentile", np.nan))
        if pd.notna(sig.get("atr_percentile", np.nan))
        else np.nan,
        "ctx4h_side_return_atr": _ctx_side_return_atr(sig, side, 4),
        "ctx12h_side_return_atr": _ctx_side_return_atr(sig, side, 12),
        "prev1_range_atr": prev1_range_atr,
        "prev1_close_pos_side": _side_close_position(sig, side),
        "sizing_reject_reason": sizing_reject_reason,
    }


def _coerce_utc_timestamp(value) -> pd.Timestamp:
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        return ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def _prepare_external_breakout_events(events, default_shape_name: str) -> dict:
    if events is None:
        return {}
    if isinstance(events, pd.DataFrame):
        records = events.to_dict(orient="records")
    else:
        records = list(events)

    prepared: dict = {}
    for raw in records:
        row = dict(raw)
        side = str(row.get("side", "")).lower().strip()
        if side not in {"long", "short"}:
            continue
        if "signal_start" not in row or "touch_time" not in row:
            continue
        signal_start = _coerce_utc_timestamp(row["signal_start"])
        touch_time = _coerce_utc_timestamp(row["touch_time"])
        symbol = str(row.get("symbol", ""))
        event_key = str(row.get("event_key", "")).strip()
        if not event_key:
            event_key = f"{symbol}|{signal_start.isoformat()}|{touch_time.isoformat()}|{side}"
        event = {
            **row,
            "side": side,
            "signal_start": signal_start,
            "touch_time": touch_time,
            "event_key": event_key,
            "external_breakout_shape_name": str(
                row.get("external_breakout_shape_name")
                or row.get("context_combo_spec")
                or row.get("source_leg")
                or default_shape_name
            ),
        }
        prepared.setdefault(signal_start, {}).setdefault(side, []).append(event)

    for side_map in prepared.values():
        for side, side_events in side_map.items():
            side_map[side] = sorted(side_events, key=lambda item: item["touch_time"])
    return prepared


def _external_event_ready(
    events_by_signal: dict,
    *,
    signal_start: pd.Timestamp,
    side: str,
    bar_time: pd.Timestamp,
    consumed_event_keys: set[str],
) -> dict | None:
    side_events = events_by_signal.get(signal_start, {}).get(side, [])
    for event in side_events:
        if str(event["event_key"]) in consumed_event_keys:
            continue
        if event["touch_time"] <= bar_time:
            return event
        return None
    return None


def _has_external_side_interest(
    events_by_signal: dict,
    *,
    signal_start: pd.Timestamp,
    side: str,
    consumed_event_keys: set[str],
) -> bool:
    return any(
        str(event["event_key"]) not in consumed_event_keys
        for event in events_by_signal.get(signal_start, {}).get(side, [])
    )


def _external_event_context_metadata(event: dict) -> dict:
    metadata = {
        "external_event_key": str(event.get("event_key", "")),
        "external_touch_time": event.get("touch_time"),
        "external_shape_variant": str(event.get("shape_variant", "")),
        "external_context_combo_spec": str(event.get("context_combo_spec", "")),
        "external_context_model_status": str(event.get("context_model_status", "")),
    }
    for column in ("timing_prediction",):
        if column in event and pd.notna(event[column]):
            metadata[column] = str(event[column])
    for column in (
        "rf_probability",
        "sizing_multiplier",
        "cost_penalty",
        "roundtrip_cost_atr",
        "context_model_probability",
        "context_model_scale",
        "speed_300s_atr",
        "eff_300s",
        "touch_extension_atr",
        "live_touch_extension_atr",
        "pre_touch_seconds",
    ):
        if column in event and pd.notna(event[column]):
            metadata[column] = float(event[column])
    return metadata


def _external_breakout_lock_payload(
    *,
    event: dict,
    sig,
    side: str,
    signal_start: pd.Timestamp,
    signal_bar_index: int,
    current_price: float,
) -> tuple[str, dict]:
    shape_name = str(event.get("external_breakout_shape_name", "external_t2"))
    breakout_level = event.get("level", np.nan)
    if pd.isna(breakout_level):
        breakout_level = current_price
    touch_time = event.get("touch_time", signal_start)
    pre_touch_seconds = (_coerce_utc_timestamp(touch_time) - signal_start).total_seconds()
    metadata = _entry_context_metadata(
        sig,
        side=side,
        signal_start=signal_start,
        signal_bar_index=signal_bar_index,
        breakout_level=float(breakout_level),
        current_price=float(current_price),
        pre_touch_seconds=pre_touch_seconds,
        sizing_reject_reason="",
    )
    metadata.update(_external_event_context_metadata(event))
    return shape_name, metadata


def _normalize_reentry_fill_policy(policy: str | None) -> str:
    normalized = str(policy or "historical").lower().strip()
    if normalized in {"historical", "strict_next_second_cross"}:
        return normalized
    raise ValueError(f"unknown reentry_fill_policy: {policy}")


def _normalize_external_entry_mode(mode: str | None) -> str:
    normalized = str(mode or "reentry_window").lower().strip()
    if normalized in {
        "reentry_window",
        "next_second_open",
        "next_second_close",
        "next_second_adverse",
    }:
        return normalized
    raise ValueError(f"unknown external_entry_mode: {mode}")


def _external_direct_entry_price(
    *,
    side: str,
    mode: str,
    open_value: float,
    high_value: float,
    low_value: float,
    close_value: float,
) -> float:
    if mode == "next_second_open":
        return float(open_value)
    if mode == "next_second_close":
        return float(close_value)
    if mode == "next_second_adverse":
        return float(high_value) if side == "long" else float(low_value)
    raise ValueError(f"external direct entry price is not defined for mode: {mode}")


def _external_direct_entry_reason(mode: str) -> str:
    return {
        "next_second_open": "External-NextSecond-Open",
        "next_second_close": "External-NextSecond-Close",
        "next_second_adverse": "External-NextSecond-Adverse",
    }[mode]


def _normalize_sizing_filter_fail_action(action: str | None) -> str:
    normalized = str(action or "scale").lower().strip()
    if normalized in {"scale", "skip_lock"}:
        return normalized
    raise ValueError(f"unknown sizing_filter_fail_action: {action}")


def _strict_reentry_triggered(
    side: str,
    trigger_mode: str,
    high_value: float,
    low_value: float,
    close_value: float,
    prev_close_value,
    re_p: float,
):
    mode = str(trigger_mode).lower().strip()
    if prev_close_value is None or pd.isna(prev_close_value):
        return False, np.nan, "missing_prev_close"

    prev_close = float(prev_close_value)
    if mode == "pullback":
        if side == "long":
            touched = low_value <= re_p
            return touched, re_p if touched else np.nan, "" if touched else "no_pullback_touch"
        touched = high_value >= re_p
        return touched, re_p if touched else np.nan, "" if touched else "no_pullback_touch"

    if mode != "reclaim":
        raise ValueError(f"unknown reentry trigger mode: {trigger_mode}")

    if side == "long":
        crossed = prev_close < re_p <= high_value
        return crossed, re_p if crossed else np.nan, "" if crossed else "no_reclaim_cross"
    crossed = prev_close > re_p >= low_value
    return crossed, re_p if crossed else np.nan, "" if crossed else "no_reclaim_cross"


def _reentry_fill_triggered(
    *,
    side: str,
    trigger_mode: str,
    high_value: float,
    low_value: float,
    close_value: float,
    prev_close_value,
    re_p: float,
    policy: str,
    current_pos: int,
    trigger_pos: int,
    trigger_kind: str,
):
    normalized_policy = _normalize_reentry_fill_policy(policy)
    if normalized_policy == "historical":
        is_triggered, entry_p_raw = _reentry_triggered(
            side,
            trigger_mode,
            high_value,
            low_value,
            close_value,
            prev_close_value,
            re_p,
            False,
        )
        return is_triggered, entry_p_raw, ""

    if current_pos <= trigger_pos:
        return False, np.nan, f"same_second_{trigger_kind}"
    return _strict_reentry_triggered(
        side,
        trigger_mode,
        high_value,
        low_value,
        close_value,
        prev_close_value,
        re_p,
    )


def _allow_breakout_lock(shape_name: str, bar_index: int, last_t3_lock_bar_index: int, t3_cooldown_bars: int) -> bool:
    if shape_name != "t3_swing" or t3_cooldown_bars <= 0:
        return True
    return bar_index - last_t3_lock_bar_index > t3_cooldown_bars


def _t3_quality_reject_reason(
    sig,
    side: str,
    current_price: float,
    breakout_level: float,
    filters: dict,
    pre_touch_seconds: float | None = None,
) -> str:
    if not filters:
        return ""

    allowed_sides = filters.get("allowed_sides")
    if allowed_sides is not None:
        allowed = {str(value) for value in allowed_sides}
        if str(side) not in allowed:
            return "side"

    max_pre_touch_seconds = filters.get("max_pre_touch_seconds")
    if max_pre_touch_seconds is not None:
        if pre_touch_seconds is None or not np.isfinite(float(pre_touch_seconds)):
            return "pre_touch_missing"
        if float(pre_touch_seconds) > float(max_pre_touch_seconds):
            return "pre_touch_seconds"

    sma5 = sig.get("sma5", np.nan)
    sma5_slope = sig.get("sma5_slope", np.nan)
    atr = sig.get("atr", np.nan)

    if filters.get("trend"):
        if not _positive(sma5) or pd.isna(sma5_slope):
            return "trend_missing"
        if side == "long" and not (current_price > float(sma5) and float(sma5_slope) > 0):
            return "trend_long"
        if side == "short" and not (current_price < float(sma5) and float(sma5_slope) < 0):
            return "trend_short"

    min_sma_atr_separation = filters.get("min_sma_atr_separation")
    if min_sma_atr_separation is not None:
        if not _positive(sma5, atr):
            return "sma_atr_separation_missing"
        separation = abs(float(breakout_level) - float(sma5)) / float(atr)
        if separation < float(min_sma_atr_separation):
            return "sma_atr_separation"

    min_atr_percentile = filters.get("min_atr_percentile")
    if min_atr_percentile is not None:
        atr_percentile = sig.get("atr_percentile", np.nan)
        if pd.isna(atr_percentile) or float(atr_percentile) < float(min_atr_percentile):
            return "atr_percentile"

    max_atr_percentile = filters.get("max_atr_percentile")
    if max_atr_percentile is not None:
        atr_percentile = sig.get("atr_percentile", np.nan)
        if pd.isna(atr_percentile) or float(atr_percentile) > float(max_atr_percentile):
            return "atr_percentile_high"

    min_ctx_side_return_atr = filters.get("min_ctx_side_return_atr")
    if min_ctx_side_return_atr is not None:
        lookback_bars = int(filters.get("ctx_return_lookback_bars", 4))
        ctx_return = _ctx_side_return_atr(sig, side, lookback_bars)
        if pd.isna(ctx_return):
            return "ctx_side_return_missing"
        if float(ctx_return) < float(min_ctx_side_return_atr):
            return "ctx_side_return_atr"

    for lookback_bars in (4, 12):
        min_key = f"min_ctx{lookback_bars}h_side_return_atr"
        max_key = f"max_ctx{lookback_bars}h_side_return_atr"
        if min_key not in filters and max_key not in filters:
            continue
        ctx_return = _ctx_side_return_atr(sig, side, lookback_bars)
        if pd.isna(ctx_return):
            return f"ctx{lookback_bars}h_side_return_missing"
        if min_key in filters and float(ctx_return) < float(filters[min_key]):
            return f"ctx{lookback_bars}h_side_return_atr_low"
        if max_key in filters and float(ctx_return) > float(filters[max_key]):
            return f"ctx{lookback_bars}h_side_return_atr_high"

    max_breakout_extension_atr = filters.get("max_breakout_extension_atr")
    if max_breakout_extension_atr is not None:
        if not _positive(atr):
            return "breakout_extension_missing"
        extension = abs(float(current_price) - float(breakout_level)) / float(atr)
        if extension > float(max_breakout_extension_atr):
            return "breakout_extension"

    return ""


def _shape_sizing_multiplier(
    shape_name: str,
    sig,
    side: str,
    current_price: float,
    breakout_level: float,
    filters: dict,
    filter_shapes: set[str],
    fail_multiplier: float,
    pre_touch_seconds: float | None,
) -> tuple[float, str]:
    if not filters or shape_name not in filter_shapes:
        return 1.0, ""
    reject_reason = _t3_quality_reject_reason(
        sig,
        side,
        current_price,
        breakout_level,
        filters,
        pre_touch_seconds,
    )
    if reject_reason:
        return float(fail_multiplier), reject_reason
    return 1.0, ""


def _intrabar_signal(sig: dict, high_so_far: float, low_so_far: float, close_now: float) -> dict:
    sig["high"] = float(high_so_far)
    sig["low"] = float(low_so_far)
    sig["close"] = float(close_now)

    prev_closes = []
    for n in range(1, 5):
        value = sig.get(f"prev_close_{n}", np.nan)
        if pd.notna(value):
            prev_closes.append(float(value))

    if len(prev_closes) >= 4:
        sig["sma5"] = float(np.mean(prev_closes[:4] + [float(close_now)]))
        sig["ma5"] = sig["sma5"]
        prev_sma5 = sig.get("prev_sma5_1", np.nan)
        if pd.notna(prev_sma5):
            sig["sma5_slope"] = sig["sma5"] - float(prev_sma5)

    prev_close_1 = sig.get("prev_close_1", np.nan)
    if pd.notna(prev_close_1):
        prev_close_1 = float(prev_close_1)
    else:
        prev_close_1 = float(close_now)
    live_tr = max(
        float(high_so_far) - float(low_so_far),
        abs(float(high_so_far) - prev_close_1),
        abs(float(low_so_far) - prev_close_1),
    )

    closed_atr = sig.get("_closed_atr", sig.get("atr", np.nan))
    if pd.notna(closed_atr):
        sig["atr"] = ((float(closed_atr) * 13.0) + live_tr) / 14.0

    return sig


def run_second_bar_replay(
    df_seconds: pd.DataFrame,
    signal: pd.DataFrame,
    *,
    initial_balance: float,
    breakout_shape: str,
    replay_mode: str,
    t3_reentry_size_schedule=None,
    t3_cooldown_bars: int = 0,
    t3_quality_filters=None,
    quality_filter_shapes=None,
    t3_exit_overrides=None,
    shape_sizing_filters=None,
    sizing_filter_shapes=None,
    sizing_filter_fail_multiplier: float = 1.0,
    sizing_filter_fail_action: str = "scale",
    external_breakout_events=None,
    external_breakout_shape_name: str = "external_t2",
    external_entry_mode: str = "reentry_window",
    reentry_fill_policy: str = "historical",
):
    if replay_mode not in {"same_bar_parity", "live_intrabar_sma5"}:
        raise ValueError(f"unknown replay mode: {replay_mode}")
    external_entry_mode = _normalize_external_entry_mode(external_entry_mode)
    reentry_fill_policy = _normalize_reentry_fill_policy(reentry_fill_policy)
    sizing_filter_fail_action = _normalize_sizing_filter_fail_action(sizing_filter_fail_action)

    balance = initial_balance
    position = None
    trade_logs = []
    diagnostics = {
        "breakout_locks": {"long": {}, "short": {}},
        "t3_cooldown_skips": {"long": 0, "short": 0},
        "t3_quality_rejects": {"long": {}, "short": {}},
        "shape_sizing_filter_fails": {"long": {}, "short": {}},
        "external_breakout_locks": {"long": {}, "short": {}},
        "reentry_fill_rejects": {"long": {}, "short": {}},
    }
    second_index = df_seconds.index
    open_values = df_seconds["open"].to_numpy(dtype="float64", copy=False)
    high_values = df_seconds["high"].to_numpy(dtype="float64", copy=False)
    low_values = df_seconds["low"].to_numpy(dtype="float64", copy=False)
    close_values = df_seconds["close"].to_numpy(dtype="float64", copy=False)

    commission = 0.001
    max_trades_per_bar = int(COMMON_REPLAY_KWARGS["max_trades_per_bar"])
    slippage = float(COMMON_REPLAY_KWARGS["fixed_slippage"])
    stop_mode = str(COMMON_REPLAY_KWARGS["stop_mode"])
    stop_loss_atr = float(COMMON_REPLAY_KWARGS["stop_loss_atr"])
    trailing_stop_atr = float(COMMON_REPLAY_KWARGS["trailing_stop_atr"])
    delayed_trailing_activation = float(COMMON_REPLAY_KWARGS["delayed_trailing_activation"])
    profit_protect_atr = float(COMMON_REPLAY_KWARGS["profit_protect_atr"])
    long_reentry_atr = float(COMMON_REPLAY_KWARGS["long_reentry_atr"])
    short_reentry_atr = float(COMMON_REPLAY_KWARGS["short_reentry_atr"])
    reentry_trigger_mode = str(COMMON_REPLAY_KWARGS["reentry_trigger_mode"])
    reentry_anchor_levels = str(COMMON_REPLAY_KWARGS["reentry_anchor_levels"])
    reentry_size_schedule = [float(v) for v in COMMON_REPLAY_KWARGS["reentry_size_schedule"]]
    if t3_reentry_size_schedule is not None:
        t3_reentry_size_schedule = [float(v) for v in t3_reentry_size_schedule]
    t3_cooldown_bars = int(t3_cooldown_bars)
    t3_quality_filters = t3_quality_filters or {}
    quality_filter_shapes = set(quality_filter_shapes or ["t3_swing"])
    t3_exit_overrides = dict(t3_exit_overrides or {})
    shape_sizing_filters = dict(shape_sizing_filters or {})
    sizing_filter_shapes = set(sizing_filter_shapes or [])
    sizing_filter_fail_multiplier = float(sizing_filter_fail_multiplier)
    external_events_by_signal = _prepare_external_breakout_events(
        external_breakout_events,
        str(external_breakout_shape_name),
    )
    external_consumed_event_keys: set[str] = set()

    last_exit_bar_index = -999
    reentry_timeout = 1
    last_exit_reason = None
    last_exit_side = None
    last_exit_breakout_shape = ""
    last_exit_size_multiplier = 1.0
    last_exit_context_metadata = {}
    last_exit_second_pos = -999
    pending_zero_initial_side = None
    pending_zero_initial_breakout_shape = ""
    pending_zero_initial_size_multiplier = 1.0
    pending_zero_initial_context_metadata = {}
    pending_zero_initial_bar_index = -999
    pending_zero_initial_second_pos = -999
    last_t3_lock_bar_index = -999

    for i in range(len(signal) - 1):
        start_t, end_t = signal.index[i], signal.index[i + 1]
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="right"))
        if start_pos >= end_pos:
            continue

        base_sig = signal.iloc[i]
        if pd.isna(base_sig["atr"]):
            continue

        trades_in_bar = 0
        current_pos = start_pos
        breakout_locked_this_bar = False
        quality_reject_recorded_this_bar = {"long": set(), "short": set()}
        sizing_filter_recorded_this_bar = {"long": set(), "short": set()}
        bar_high_so_far = -np.inf
        bar_low_so_far = np.inf
        sig = base_sig
        live_sig = None
        if replay_mode == "live_intrabar_sma5":
            live_sig = base_sig.to_dict()
            live_sig["_closed_atr"] = float(base_sig["atr"])
        else:
            long_regime_ready, short_regime_ready = _resolve_regime_ready(sig, "1d")

        if i - last_exit_bar_index > reentry_timeout:
            last_exit_side = None
            last_exit_breakout_shape = ""
            last_exit_size_multiplier = 1.0
            last_exit_context_metadata = {}
            last_exit_second_pos = -999
        if i - pending_zero_initial_bar_index > reentry_timeout:
            pending_zero_initial_side = None
            pending_zero_initial_breakout_shape = ""
            pending_zero_initial_size_multiplier = 1.0
            pending_zero_initial_context_metadata = {}
            pending_zero_initial_second_pos = -999

        while current_pos < end_pos:
            bar_time = second_index[current_pos]
            open_value = open_values[current_pos]
            high_value = high_values[current_pos]
            low_value = low_values[current_pos]
            close_value = close_values[current_pos]
            prev_close = close_values[current_pos - 1] if current_pos > start_pos else None
            if replay_mode == "live_intrabar_sma5":
                bar_high_so_far = max(bar_high_so_far, high_value)
                bar_low_so_far = min(bar_low_so_far, low_value)
                sig = _intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)
                long_regime_ready, short_regime_ready = _resolve_regime_ready(sig, "1d")
            external_long_ready = _has_external_side_interest(
                external_events_by_signal,
                signal_start=start_t,
                side="long",
                consumed_event_keys=external_consumed_event_keys,
            )
            external_short_ready = _has_external_side_interest(
                external_events_by_signal,
                signal_start=start_t,
                side="short",
                consumed_event_keys=external_consumed_event_keys,
            )
            if external_long_ready or (
                pending_zero_initial_side == "long"
                and pending_zero_initial_context_metadata.get("external_event_key")
            ) or (
                last_exit_side == "long"
                and last_exit_context_metadata.get("external_event_key")
            ):
                long_regime_ready = True
            if external_short_ready or (
                pending_zero_initial_side == "short"
                and pending_zero_initial_context_metadata.get("external_event_key")
            ) or (
                last_exit_side == "short"
                and last_exit_context_metadata.get("external_event_key")
            ):
                short_regime_ready = True

            if position is None:
                if long_regime_ready:
                    triggered, breakout_level, shape_name = _long_breakout(sig, high_value, breakout_shape)
                    if trades_in_bar == 0 and triggered:
                        quality_reject_reason = ""
                        if shape_name in quality_filter_shapes:
                            quality_reject_reason = _t3_quality_reject_reason(
                                sig,
                                "long",
                                close_value,
                                breakout_level,
                                t3_quality_filters,
                                (bar_time - start_t).total_seconds(),
                            )
                        if quality_reject_reason:
                            if quality_reject_reason not in quality_reject_recorded_this_bar["long"]:
                                diagnostics["t3_quality_rejects"]["long"][quality_reject_reason] = (
                                    diagnostics["t3_quality_rejects"]["long"].get(quality_reject_reason, 0) + 1
                                )
                                quality_reject_recorded_this_bar["long"].add(quality_reject_reason)
                        elif _allow_breakout_lock(shape_name, i, last_t3_lock_bar_index, t3_cooldown_bars):
                            size_multiplier, sizing_reject_reason = _shape_sizing_multiplier(
                                shape_name,
                                sig,
                                "long",
                                close_value,
                                breakout_level,
                                shape_sizing_filters,
                                sizing_filter_shapes,
                                sizing_filter_fail_multiplier,
                                (bar_time - start_t).total_seconds(),
                            )
                            if sizing_reject_reason:
                                if sizing_reject_reason not in sizing_filter_recorded_this_bar["long"]:
                                    diagnostics["shape_sizing_filter_fails"]["long"][sizing_reject_reason] = (
                                        diagnostics["shape_sizing_filter_fails"]["long"].get(sizing_reject_reason, 0)
                                        + 1
                                    )
                                    sizing_filter_recorded_this_bar["long"].add(sizing_reject_reason)
                            skip_sizing_lock = bool(
                                sizing_reject_reason and sizing_filter_fail_action == "skip_lock"
                            )
                            if not skip_sizing_lock:
                                if shape_name == "t3_swing":
                                    last_t3_lock_bar_index = i
                                if not breakout_locked_this_bar:
                                    diagnostics["breakout_locks"]["long"][shape_name] = (
                                        diagnostics["breakout_locks"]["long"].get(shape_name, 0) + 1
                                    )
                                    breakout_locked_this_bar = True
                                pending_zero_initial_side = "long"
                                pending_zero_initial_breakout_shape = shape_name
                                pending_zero_initial_size_multiplier = size_multiplier
                                pending_zero_initial_context_metadata = _entry_context_metadata(
                                    sig,
                                    side="long",
                                    signal_start=start_t,
                                    signal_bar_index=i,
                                    breakout_level=breakout_level,
                                    current_price=close_value,
                                    pre_touch_seconds=(bar_time - start_t).total_seconds(),
                                    sizing_reject_reason=sizing_reject_reason,
                                )
                                pending_zero_initial_bar_index = i
                                pending_zero_initial_second_pos = current_pos
                        else:
                            diagnostics["t3_cooldown_skips"]["long"] += 1

                    external_event = _external_event_ready(
                        external_events_by_signal,
                        signal_start=start_t,
                        side="long",
                        bar_time=bar_time,
                        consumed_event_keys=external_consumed_event_keys,
                    )
                    if external_event is not None and trades_in_bar == 0 and not breakout_locked_this_bar:
                        shape_name, event_metadata = _external_breakout_lock_payload(
                            event=external_event,
                            sig=sig,
                            side="long",
                            signal_start=start_t,
                            signal_bar_index=i,
                            current_price=close_value,
                        )
                        event_touch_time = _coerce_utc_timestamp(external_event["touch_time"])
                        if external_entry_mode == "reentry_window" or bar_time > event_touch_time:
                            diagnostics["breakout_locks"]["long"][shape_name] = (
                                diagnostics["breakout_locks"]["long"].get(shape_name, 0) + 1
                            )
                            diagnostics["external_breakout_locks"]["long"][shape_name] = (
                                diagnostics["external_breakout_locks"]["long"].get(shape_name, 0) + 1
                            )
                            breakout_locked_this_bar = True
                            if external_entry_mode == "reentry_window":
                                pending_zero_initial_side = "long"
                                pending_zero_initial_breakout_shape = shape_name
                                pending_zero_initial_size_multiplier = 1.0
                                pending_zero_initial_context_metadata = event_metadata
                                pending_zero_initial_bar_index = i
                                pending_zero_initial_second_pos = current_pos
                            else:
                                active_schedule = _shape_schedule(
                                    shape_name,
                                    reentry_size_schedule,
                                    t3_reentry_size_schedule,
                                )
                                active_stop_loss_atr = float(
                                    _t3_exit_override(
                                        shape_name,
                                        t3_exit_overrides,
                                        "stop_loss_atr",
                                        stop_loss_atr,
                                    )
                                )
                                notional_share = _get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                                entry_p_raw = _external_direct_entry_price(
                                    side="long",
                                    mode=external_entry_mode,
                                    open_value=open_value,
                                    high_value=high_value,
                                    low_value=low_value,
                                    close_value=close_value,
                                )
                                entry_price = float(entry_p_raw) * (1 + slippage)
                                balance, position = _open_position(
                                    balance,
                                    sig,
                                    "long",
                                    entry_price,
                                    notional_share,
                                    _external_direct_entry_reason(external_entry_mode),
                                    stop_mode,
                                    active_stop_loss_atr,
                                    shape_name,
                                    replay_mode,
                                )
                                position["entry_time"] = bar_time
                                position["size_multiplier"] = 1.0
                                position["entry_context_metadata"] = dict(event_metadata)
                                trade_logs.append(
                                    {
                                        "time": bar_time,
                                        "type": "BUY",
                                        "price": entry_price,
                                        "reason": _external_direct_entry_reason(external_entry_mode),
                                        "notional": position["notional"],
                                        "bal": balance,
                                        "breakout_shape_name": position["breakout_shape_name"],
                                        "size_multiplier": 1.0,
                                        "replay_mode": replay_mode,
                                        **event_metadata,
                                    }
                                )
                                trades_in_bar += 1
                            external_consumed_event_keys.add(str(external_event["event_key"]))

                    has_exit_reentry_window = last_exit_side == "long" and (i - last_exit_bar_index <= reentry_timeout)
                    has_zero_initial_window = (
                        pending_zero_initial_side == "long" and (i - pending_zero_initial_bar_index <= reentry_timeout)
                    )
                    if has_exit_reentry_window or has_zero_initial_window:
                        re_p = _resolve_reentry_price(sig, "long", reentry_anchor_levels, long_reentry_atr)
                        trigger_kind = "exit" if has_exit_reentry_window else "zero_initial"
                        trigger_pos = last_exit_second_pos if has_exit_reentry_window else pending_zero_initial_second_pos
                        is_triggered, entry_p_raw, fill_reject_reason = _reentry_fill_triggered(
                            side="long",
                            trigger_mode=reentry_trigger_mode,
                            high_value=high_value,
                            low_value=low_value,
                            close_value=close_value,
                            prev_close_value=prev_close,
                            re_p=re_p,
                            policy=reentry_fill_policy,
                            current_pos=current_pos,
                            trigger_pos=trigger_pos,
                            trigger_kind=trigger_kind,
                        )
                        if is_triggered:
                            reason = "Zero-Initial-Reentry"
                            if has_exit_reentry_window:
                                reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
                            if trades_in_bar < max_trades_per_bar:
                                entry_breakout_shape = pending_zero_initial_breakout_shape
                                entry_size_multiplier = pending_zero_initial_size_multiplier
                                entry_context_metadata = dict(pending_zero_initial_context_metadata)
                                if has_exit_reentry_window:
                                    entry_breakout_shape = last_exit_breakout_shape
                                    entry_size_multiplier = last_exit_size_multiplier
                                    entry_context_metadata = dict(last_exit_context_metadata)
                                active_schedule = _shape_schedule(
                                    entry_breakout_shape,
                                    reentry_size_schedule,
                                    t3_reentry_size_schedule,
                                )
                                active_stop_loss_atr = float(
                                    _t3_exit_override(
                                        entry_breakout_shape,
                                        t3_exit_overrides,
                                        "stop_loss_atr",
                                        stop_loss_atr,
                                    )
                                )
                                notional_share = (
                                    _get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                                    * float(entry_size_multiplier)
                                )
                                entry_price = float(entry_p_raw) * (1 + slippage)
                                balance, position = _open_position(
                                    balance,
                                    sig,
                                    "long",
                                    entry_price,
                                    notional_share,
                                    reason,
                                    stop_mode,
                                    active_stop_loss_atr,
                                    entry_breakout_shape,
                                    replay_mode,
                                )
                                position["entry_time"] = bar_time
                                position["size_multiplier"] = float(entry_size_multiplier)
                                position["entry_context_metadata"] = dict(entry_context_metadata)
                                trade_logs.append(
                                    {
                                        "time": bar_time,
                                        "type": "BUY",
                                        "price": entry_price,
                                        "reason": reason,
                                        "notional": position["notional"],
                                        "bal": balance,
                                        "breakout_shape_name": position["breakout_shape_name"],
                                        "size_multiplier": float(entry_size_multiplier),
                                        "replay_mode": replay_mode,
                                        **entry_context_metadata,
                                    }
                                )
                                trades_in_bar += 1
                            if has_exit_reentry_window:
                                last_exit_side = None
                                last_exit_breakout_shape = ""
                                last_exit_size_multiplier = 1.0
                                last_exit_context_metadata = {}
                                last_exit_second_pos = -999
                            if has_zero_initial_window:
                                pending_zero_initial_side = None
                                pending_zero_initial_breakout_shape = ""
                                pending_zero_initial_size_multiplier = 1.0
                                pending_zero_initial_context_metadata = {}
                                pending_zero_initial_second_pos = -999
                        elif fill_reject_reason:
                            diagnostics["reentry_fill_rejects"]["long"][fill_reject_reason] = (
                                diagnostics["reentry_fill_rejects"]["long"].get(fill_reject_reason, 0) + 1
                            )

                elif short_regime_ready:
                    triggered, breakout_level, shape_name = _short_breakout(sig, low_value, breakout_shape)
                    if trades_in_bar == 0 and triggered:
                        quality_reject_reason = ""
                        if shape_name in quality_filter_shapes:
                            quality_reject_reason = _t3_quality_reject_reason(
                                sig,
                                "short",
                                close_value,
                                breakout_level,
                                t3_quality_filters,
                                (bar_time - start_t).total_seconds(),
                            )
                        if quality_reject_reason:
                            if quality_reject_reason not in quality_reject_recorded_this_bar["short"]:
                                diagnostics["t3_quality_rejects"]["short"][quality_reject_reason] = (
                                    diagnostics["t3_quality_rejects"]["short"].get(quality_reject_reason, 0) + 1
                                )
                                quality_reject_recorded_this_bar["short"].add(quality_reject_reason)
                        elif _allow_breakout_lock(shape_name, i, last_t3_lock_bar_index, t3_cooldown_bars):
                            size_multiplier, sizing_reject_reason = _shape_sizing_multiplier(
                                shape_name,
                                sig,
                                "short",
                                close_value,
                                breakout_level,
                                shape_sizing_filters,
                                sizing_filter_shapes,
                                sizing_filter_fail_multiplier,
                                (bar_time - start_t).total_seconds(),
                            )
                            if sizing_reject_reason:
                                if sizing_reject_reason not in sizing_filter_recorded_this_bar["short"]:
                                    diagnostics["shape_sizing_filter_fails"]["short"][sizing_reject_reason] = (
                                        diagnostics["shape_sizing_filter_fails"]["short"].get(sizing_reject_reason, 0)
                                        + 1
                                    )
                                    sizing_filter_recorded_this_bar["short"].add(sizing_reject_reason)
                            skip_sizing_lock = bool(
                                sizing_reject_reason and sizing_filter_fail_action == "skip_lock"
                            )
                            if not skip_sizing_lock:
                                if shape_name == "t3_swing":
                                    last_t3_lock_bar_index = i
                                if not breakout_locked_this_bar:
                                    diagnostics["breakout_locks"]["short"][shape_name] = (
                                        diagnostics["breakout_locks"]["short"].get(shape_name, 0) + 1
                                    )
                                    breakout_locked_this_bar = True
                                pending_zero_initial_side = "short"
                                pending_zero_initial_breakout_shape = shape_name
                                pending_zero_initial_size_multiplier = size_multiplier
                                pending_zero_initial_context_metadata = _entry_context_metadata(
                                    sig,
                                    side="short",
                                    signal_start=start_t,
                                    signal_bar_index=i,
                                    breakout_level=breakout_level,
                                    current_price=close_value,
                                    pre_touch_seconds=(bar_time - start_t).total_seconds(),
                                    sizing_reject_reason=sizing_reject_reason,
                                )
                                pending_zero_initial_bar_index = i
                                pending_zero_initial_second_pos = current_pos
                        else:
                            diagnostics["t3_cooldown_skips"]["short"] += 1

                    external_event = _external_event_ready(
                        external_events_by_signal,
                        signal_start=start_t,
                        side="short",
                        bar_time=bar_time,
                        consumed_event_keys=external_consumed_event_keys,
                    )
                    if external_event is not None and trades_in_bar == 0 and not breakout_locked_this_bar:
                        shape_name, event_metadata = _external_breakout_lock_payload(
                            event=external_event,
                            sig=sig,
                            side="short",
                            signal_start=start_t,
                            signal_bar_index=i,
                            current_price=close_value,
                        )
                        event_touch_time = _coerce_utc_timestamp(external_event["touch_time"])
                        if external_entry_mode == "reentry_window" or bar_time > event_touch_time:
                            diagnostics["breakout_locks"]["short"][shape_name] = (
                                diagnostics["breakout_locks"]["short"].get(shape_name, 0) + 1
                            )
                            diagnostics["external_breakout_locks"]["short"][shape_name] = (
                                diagnostics["external_breakout_locks"]["short"].get(shape_name, 0) + 1
                            )
                            breakout_locked_this_bar = True
                            if external_entry_mode == "reentry_window":
                                pending_zero_initial_side = "short"
                                pending_zero_initial_breakout_shape = shape_name
                                pending_zero_initial_size_multiplier = 1.0
                                pending_zero_initial_context_metadata = event_metadata
                                pending_zero_initial_bar_index = i
                                pending_zero_initial_second_pos = current_pos
                            else:
                                active_schedule = _shape_schedule(
                                    shape_name,
                                    reentry_size_schedule,
                                    t3_reentry_size_schedule,
                                )
                                active_stop_loss_atr = float(
                                    _t3_exit_override(
                                        shape_name,
                                        t3_exit_overrides,
                                        "stop_loss_atr",
                                        stop_loss_atr,
                                    )
                                )
                                notional_share = _get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                                entry_p_raw = _external_direct_entry_price(
                                    side="short",
                                    mode=external_entry_mode,
                                    open_value=open_value,
                                    high_value=high_value,
                                    low_value=low_value,
                                    close_value=close_value,
                                )
                                entry_price = float(entry_p_raw) * (1 - slippage)
                                balance, position = _open_position(
                                    balance,
                                    sig,
                                    "short",
                                    entry_price,
                                    notional_share,
                                    _external_direct_entry_reason(external_entry_mode),
                                    stop_mode,
                                    active_stop_loss_atr,
                                    shape_name,
                                    replay_mode,
                                )
                                position["entry_time"] = bar_time
                                position["size_multiplier"] = 1.0
                                position["entry_context_metadata"] = dict(event_metadata)
                                trade_logs.append(
                                    {
                                        "time": bar_time,
                                        "type": "SHORT",
                                        "price": entry_price,
                                        "reason": _external_direct_entry_reason(external_entry_mode),
                                        "notional": position["notional"],
                                        "bal": balance,
                                        "breakout_shape_name": position["breakout_shape_name"],
                                        "size_multiplier": 1.0,
                                        "replay_mode": replay_mode,
                                        **event_metadata,
                                    }
                                )
                                trades_in_bar += 1
                            external_consumed_event_keys.add(str(external_event["event_key"]))

                    has_exit_reentry_window = last_exit_side == "short" and (i - last_exit_bar_index <= reentry_timeout)
                    has_zero_initial_window = (
                        pending_zero_initial_side == "short" and (i - pending_zero_initial_bar_index <= reentry_timeout)
                    )
                    if has_exit_reentry_window or has_zero_initial_window:
                        re_p = _resolve_reentry_price(sig, "short", reentry_anchor_levels, short_reentry_atr)
                        trigger_kind = "exit" if has_exit_reentry_window else "zero_initial"
                        trigger_pos = last_exit_second_pos if has_exit_reentry_window else pending_zero_initial_second_pos
                        is_triggered, entry_p_raw, fill_reject_reason = _reentry_fill_triggered(
                            side="short",
                            trigger_mode=reentry_trigger_mode,
                            high_value=high_value,
                            low_value=low_value,
                            close_value=close_value,
                            prev_close_value=prev_close,
                            re_p=re_p,
                            policy=reentry_fill_policy,
                            current_pos=current_pos,
                            trigger_pos=trigger_pos,
                            trigger_kind=trigger_kind,
                        )
                        if is_triggered:
                            reason = "Zero-Initial-Reentry"
                            if has_exit_reentry_window:
                                reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
                            if trades_in_bar < max_trades_per_bar:
                                entry_breakout_shape = pending_zero_initial_breakout_shape
                                entry_size_multiplier = pending_zero_initial_size_multiplier
                                entry_context_metadata = dict(pending_zero_initial_context_metadata)
                                if has_exit_reentry_window:
                                    entry_breakout_shape = last_exit_breakout_shape
                                    entry_size_multiplier = last_exit_size_multiplier
                                    entry_context_metadata = dict(last_exit_context_metadata)
                                active_schedule = _shape_schedule(
                                    entry_breakout_shape,
                                    reentry_size_schedule,
                                    t3_reentry_size_schedule,
                                )
                                active_stop_loss_atr = float(
                                    _t3_exit_override(
                                        entry_breakout_shape,
                                        t3_exit_overrides,
                                        "stop_loss_atr",
                                        stop_loss_atr,
                                    )
                                )
                                notional_share = (
                                    _get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                                    * float(entry_size_multiplier)
                                )
                                entry_price = float(entry_p_raw) * (1 - slippage)
                                balance, position = _open_position(
                                    balance,
                                    sig,
                                    "short",
                                    entry_price,
                                    notional_share,
                                    reason,
                                    stop_mode,
                                    active_stop_loss_atr,
                                    entry_breakout_shape,
                                    replay_mode,
                                )
                                position["entry_time"] = bar_time
                                position["size_multiplier"] = float(entry_size_multiplier)
                                position["entry_context_metadata"] = dict(entry_context_metadata)
                                trade_logs.append(
                                    {
                                        "time": bar_time,
                                        "type": "SHORT",
                                        "price": entry_price,
                                        "reason": reason,
                                        "notional": position["notional"],
                                        "bal": balance,
                                        "breakout_shape_name": position["breakout_shape_name"],
                                        "size_multiplier": float(entry_size_multiplier),
                                        "replay_mode": replay_mode,
                                        **entry_context_metadata,
                                    }
                                )
                                trades_in_bar += 1
                            if has_exit_reentry_window:
                                last_exit_side = None
                                last_exit_breakout_shape = ""
                                last_exit_size_multiplier = 1.0
                                last_exit_context_metadata = {}
                                last_exit_second_pos = -999
                            if has_zero_initial_window:
                                pending_zero_initial_side = None
                                pending_zero_initial_breakout_shape = ""
                                pending_zero_initial_size_multiplier = 1.0
                                pending_zero_initial_context_metadata = {}
                                pending_zero_initial_second_pos = -999
                        elif fill_reject_reason:
                            diagnostics["reentry_fill_rejects"]["short"][fill_reject_reason] = (
                                diagnostics["reentry_fill_rejects"]["short"].get(fill_reject_reason, 0) + 1
                            )

            else:
                exit_triggered = False
                exit_p = 0.0
                reason = ""
                position_shape = position.get("breakout_shape_name", "")
                active_trailing_stop_atr = float(
                    _t3_exit_override(
                        position_shape,
                        t3_exit_overrides,
                        "trailing_stop_atr",
                        trailing_stop_atr,
                    )
                )
                active_delayed_trailing_activation = float(
                    _t3_exit_override(
                        position_shape,
                        t3_exit_overrides,
                        "delayed_trailing_activation",
                        delayed_trailing_activation,
                    )
                )
                active_profit_protect_atr = float(
                    _t3_exit_override(
                        position_shape,
                        t3_exit_overrides,
                        "profit_protect_atr",
                        profit_protect_atr,
                    )
                )
                active_max_hold_seconds = _t3_exit_override(
                    position_shape,
                    t3_exit_overrides,
                    "max_hold_seconds",
                    None,
                )
                active_min_hold_seconds_before_sl = _t3_exit_override(
                    position_shape,
                    t3_exit_overrides,
                    "min_hold_seconds_before_sl",
                    None,
                )
                hold_seconds = None
                if position.get("entry_time") is not None:
                    hold_seconds = (bar_time - position["entry_time"]).total_seconds()
                sl_exit_allowed = (
                    active_min_hold_seconds_before_sl is None
                    or hold_seconds is None
                    or hold_seconds >= float(active_min_hold_seconds_before_sl)
                )

                if position["side"] == "long":
                    prev_hwm = position.get("hwm", position["entry_p"])
                    protected_before_bar = position.get("protected", False)

                    profit_atr = (prev_hwm - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0
                    if profit_atr >= active_delayed_trailing_activation:
                        trailing_sl = prev_hwm - active_trailing_stop_atr * sig["atr"]
                        position["sl"] = max(position["sl"], trailing_sl)

                    if low_value <= position["sl"] and sl_exit_allowed:
                        exit_p, reason, exit_triggered = position["sl"], "SL", True
                    elif protected_before_bar and low_value <= sig["prev_low_1"]:
                        exit_p, reason, exit_triggered = sig["prev_low_1"], "PT", True
                    elif active_max_hold_seconds is not None and hold_seconds is not None:
                        if hold_seconds >= float(active_max_hold_seconds):
                            exit_p, reason, exit_triggered = close_value, "T3TimeCap", True

                    if not exit_triggered:
                        position["hwm"] = max(prev_hwm, high_value)
                        if not position["protected"] and high_value >= position["entry_p"] + active_profit_protect_atr * sig["atr"]:
                            position["protected"] = True
                        profit_atr = (position["hwm"] - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0
                        if profit_atr >= active_delayed_trailing_activation:
                            trailing_sl = position["hwm"] - active_trailing_stop_atr * sig["atr"]
                            position["sl"] = max(position["sl"], trailing_sl)

                else:
                    prev_lwm = position.get("lwm", position["entry_p"])
                    protected_before_bar = position.get("protected", False)

                    profit_atr = (position["entry_p"] - prev_lwm) / sig["atr"] if sig["atr"] > 0 else 0
                    if profit_atr >= active_delayed_trailing_activation:
                        trailing_sl = prev_lwm + active_trailing_stop_atr * sig["atr"]
                        position["sl"] = min(position["sl"], trailing_sl)

                    if high_value >= position["sl"] and sl_exit_allowed:
                        exit_p, reason, exit_triggered = position["sl"], "SL", True
                    elif protected_before_bar and high_value >= sig["prev_high_1"]:
                        exit_p, reason, exit_triggered = sig["prev_high_1"], "PT", True
                    elif active_max_hold_seconds is not None and hold_seconds is not None:
                        if hold_seconds >= float(active_max_hold_seconds):
                            exit_p, reason, exit_triggered = close_value, "T3TimeCap", True

                    if not exit_triggered:
                        position["lwm"] = min(prev_lwm, low_value)
                        if not position["protected"] and low_value <= position["entry_p"] - active_profit_protect_atr * sig["atr"]:
                            position["protected"] = True
                        profit_atr = (position["entry_p"] - position["lwm"]) / sig["atr"] if sig["atr"] > 0 else 0
                        if profit_atr >= active_delayed_trailing_activation:
                            trailing_sl = position["lwm"] + active_trailing_stop_atr * sig["atr"]
                            position["sl"] = min(position["sl"], trailing_sl)

                if exit_triggered:
                    exit_breakout_shape = position.get("breakout_shape_name", "")
                    side_mult = 1 if position["side"] == "long" else -1
                    exit_p = exit_p * (1 - slippage) if position["side"] == "long" else exit_p * (1 + slippage)
                    pnl = (
                        side_mult
                        * (exit_p - position["entry_p"])
                        / position["entry_p"]
                        * position["notional"]
                        if position["notional"] > 0
                        else 0.0
                    )
                    balance += pnl - (position["notional"] * commission)
                    trade_logs.append(
                        {
                            "time": bar_time,
                            "type": "EXIT",
                            "price": exit_p,
                            "reason": reason,
                            "notional": position["notional"],
                            "bal": balance,
                            "breakout_shape_name": exit_breakout_shape,
                            "replay_mode": replay_mode,
                        }
                    )
                    last_exit_reason = reason
                    last_exit_side = position["side"]
                    last_exit_breakout_shape = exit_breakout_shape
                    last_exit_size_multiplier = float(position.get("size_multiplier", 1.0))
                    last_exit_context_metadata = dict(position.get("entry_context_metadata", {}))
                    last_exit_bar_index = i
                    last_exit_second_pos = current_pos
                    position = None

            current_pos += 1

    if position is not None and not df_seconds.empty:
        last_bar_time = second_index[-1]
        last_close = float(close_values[-1])
        side_mult = 1 if position["side"] == "long" else -1
        final_exit_p = last_close * (1 - slippage) if position["side"] == "long" else last_close * (1 + slippage)
        pnl = (
            side_mult
            * (final_exit_p - position["entry_p"])
            / position["entry_p"]
            * position["notional"]
            if position["notional"] > 0
            else 0.0
        )
        balance += pnl - (position["notional"] * commission)
        trade_logs.append(
            {
                "time": last_bar_time,
                "type": "EXIT",
                "price": final_exit_p,
                "reason": "FinalMarkToMarket",
                "notional": position["notional"],
                "bal": balance,
                "breakout_shape_name": position.get("breakout_shape_name", ""),
                "replay_mode": replay_mode,
            }
        )

    return pd.DataFrame(trade_logs), diagnostics


def summarize_run(ledger: pd.DataFrame, initial_balance: float) -> dict:
    stats = _compute_backtest_stats(ledger, initial_balance)
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])] if not ledger.empty else ledger
    exits = ledger[ledger["type"] == "EXIT"] if not ledger.empty else ledger
    entry_counts = {str(k): int(v) for k, v in entries["reason"].value_counts().items()} if not entries.empty else {}
    exit_counts = {str(k): int(v) for k, v in exits["reason"].value_counts().items()} if not exits.empty else {}
    side_counts = {str(k): int(v) for k, v in entries["type"].value_counts().items()} if not entries.empty else {}
    zero_notional_entries = int((entries["notional"] <= 0).sum()) if not entries.empty else 0
    return {
        "final_balance": round(float(stats["final_bal"]), 2),
        "return_pct": round(float(stats["return"]) * 100, 2),
        "max_dd_pct": round(float(stats["max_dd"]) * 100, 2),
        "trades": int(stats["trades"]),
        "win_rate_pct": round(float(stats["win_rate"]) * 100, 2),
        "sharpe": round(float(stats["sharpe"]), 2),
        "first_entry": entries["time"].iloc[0].isoformat() if not entries.empty else "",
        "last_exit": exits["time"].iloc[-1].isoformat() if not exits.empty else "",
        "entry_reasons": entry_counts,
        "exit_reasons": exit_counts,
        "entry_types": side_counts,
        "integrity": {
            "rows": int(len(ledger)),
            "entries": int(len(entries)),
            "exits": int(len(exits)),
            "zero_notional_entries": zero_notional_entries,
        },
    }


def summarize_breakout_attribution(ledger: pd.DataFrame) -> dict:
    if ledger.empty or "breakout_shape_name" not in ledger.columns:
        return {}

    rows = []
    open_entry = None
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue
        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        pnl_pct = side_mult * (exit_price - entry_price) / entry_price if entry_price > 0 else 0.0
        pnl_value = pnl_pct * float(open_entry["notional"])
        rows.append(
            {
                "breakout_shape_name": str(open_entry.get("breakout_shape_name", "")),
                "entry_type": str(open_entry["type"]),
                "entry_reason": str(open_entry["reason"]),
                "exit_reason": str(row["reason"]),
                "pnl_pct": pnl_pct,
                "pnl_value": pnl_value,
            }
        )
        open_entry = None

    if not rows:
        return {}

    pairs = pd.DataFrame(rows)
    attribution = {}
    for shape_name, group in pairs.groupby("breakout_shape_name", dropna=False):
        pnl_values = group["pnl_value"].astype("float64")
        pnl_pct = group["pnl_pct"].astype("float64")
        cumulative_pnl = pnl_values.cumsum()
        shape_peak = cumulative_pnl.cummax()
        max_pnl_drawdown = float((cumulative_pnl - shape_peak).min()) if not cumulative_pnl.empty else 0.0
        gross_profit = float(pnl_values[pnl_values > 0].sum())
        gross_loss = abs(float(pnl_values[pnl_values < 0].sum()))
        attribution[str(shape_name) or "unknown"] = {
            "trades": int(len(group)),
            "win_rate_pct": round(float((pnl_values > 0).mean()) * 100, 2),
            "avg_pnl_pct": round(float(pnl_pct.mean()) * 100, 4),
            "median_pnl_pct": round(float(pnl_pct.median()) * 100, 4),
            "pnl_std_pct": round(float(pnl_pct.std(ddof=0)) * 100, 4),
            "worst_pnl_pct": round(float(pnl_pct.min()) * 100, 4),
            "net_pnl_value": round(float(pnl_values.sum()), 2),
            "max_pnl_drawdown": round(max_pnl_drawdown, 2),
            "profit_factor": round(gross_profit / gross_loss, 4) if gross_loss > 0 else None,
            "entry_types": {str(k): int(v) for k, v in group["entry_type"].value_counts().items()},
            "entry_reasons": {str(k): int(v) for k, v in group["entry_reason"].value_counts().items()},
            "exit_reasons": {str(k): int(v) for k, v in group["exit_reason"].value_counts().items()},
        }
    return attribution


def _scenario_delta(base_summary: dict, variant_summary: dict) -> dict:
    return {
        "final_balance_delta": round(variant_summary["final_balance"] - base_summary["final_balance"], 2),
        "return_pct_delta": round(variant_summary["return_pct"] - base_summary["return_pct"], 2),
        "max_dd_pct_delta": round(variant_summary["max_dd_pct"] - base_summary["max_dd_pct"], 2),
        "trades_delta": int(variant_summary["trades"] - base_summary["trades"]),
        "win_rate_pct_delta": round(variant_summary["win_rate_pct"] - base_summary["win_rate_pct"], 2),
        "sharpe_delta": round(variant_summary["sharpe"] - base_summary["sharpe"], 2),
    }


def write_markdown(summary: dict, output_path: Path):
    lines = [
        "# ETH Q1 2026 30min t3_sma5 Signal Quality Filtering",
        "",
        "Scope: research-only backtest work. No live or execution path was changed.",
        "",
        "## Setup",
        "",
        "- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`",
        "- Execution bars: continuous `1s` bars rebuilt from raw Binance trades",
        "- Main comparison baseline: `t3_sma5_baseline` with full-size schedule `[0.20, 0.10]`",
        "- Sizing baseline: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`",
        "- Shared risk params: `stop_mode=atr`, `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`",
        "",
        "## Replay Mode",
        "",
        "- `live_intrabar_sma5`: live-safe intrabar mode. Each replayed second updates the current signal bar close/high/low from data seen so far and computes `sma5/ma5` from four closed signal bars plus the current realtime close.",
        "",
        "## Breakout Shapes",
        "",
        "- Baseline long: `prev_t2.high > prev_t1.high` and current price crosses `prev_t2.high`.",
        "- Added long: `prev_t3.high > prev_t2.high`, `prev_t3.high > prev_t1.high`, `prev_t1.high > prev_t2.high`, and current price crosses `prev_t3.high`.",
        "- The short side uses the symmetric low-side condition.",
        "",
        "## Optimization Variants",
        "",
        "- `A_sep_0p25`: only the added t3 breakout requires `abs(breakout_level - sma5) >= 0.25 * atr`.",
        "- `all_breakouts_sep_0p25`: both original_t2 and added t3 breakouts require `abs(breakout_level - sma5) >= 0.25 * atr`.",
        "",
        "## Results",
        "",
        "| Timeframe | Scenario | Filters | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks | Quality Rejects |",
        "|---|---|---|---:|---:|---:|---:|---:|---:|---|---|---|",
    ]
    for result in summary["results"]:
        timeframe = result["timeframe"]
        for scenario in result["scenarios"]:
            s = scenario["summary"]
            entry_mix = ", ".join(f"{k}:{v}" for k, v in s["entry_reasons"].items())
            locks = scenario["diagnostics"].get("breakout_locks", {})
            lock_parts = []
            for side, counts in locks.items():
                if counts:
                    lock_parts.append(f"{side} " + "/".join(f"{k}:{v}" for k, v in counts.items()))
            lock_text = "; ".join(lock_parts)
            params = scenario["params"]
            filters = params.get("t3_quality_filters") or {}
            filter_text = json.dumps(filters, sort_keys=True) if filters else "none"
            rejects = scenario["diagnostics"].get("t3_quality_rejects", {})
            reject_parts = []
            for side, counts in rejects.items():
                if counts:
                    reject_parts.append(f"{side} " + "/".join(f"{k}:{v}" for k, v in counts.items()))
            reject_text = "; ".join(reject_parts)
            lines.append(
                f"| `{timeframe}` | `{scenario['scenario']}` | `{filter_text}` | {s['final_balance']:,.2f} | "
                f"{s['return_pct']:.2f}% | {s['max_dd_pct']:.2f}% | {s['trades']} | "
                f"{s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | `{entry_mix}` | `{lock_text}` | `{reject_text}` |"
            )
    lines.extend(["", "## Delta vs t3_sma5 Baseline", ""])
    lines.append("| Timeframe | Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |")
    lines.append("|---|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"]:
        for scenario_name, d in result["delta_vs_t3_sma5_baseline"].items():
            lines.append(
                f"| `{result['timeframe']}` | `{scenario_name}` | {d['final_balance_delta']:,.2f} | "
                f"{d['return_pct_delta']:.2f} pp | {d['max_dd_pct_delta']:.2f} pp | "
                f"{d['trades_delta']} | {d['win_rate_pct_delta']:.2f} pp | {d['sharpe_delta']:.2f} |"
            )
    lines.extend(["", "## Breakout Attribution", ""])
    lines.append("| Timeframe | Scenario | Shape | Trades | Win Rate | Avg PnL | Median PnL | PnL Std | Worst PnL | Net PnL | Shape PnL DD | Profit Factor |")
    lines.append("|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"]:
        for scenario in result["scenarios"]:
            for shape_name, a in scenario.get("attribution", {}).items():
                lines.append(
                    f"| `{result['timeframe']}` | `{scenario['scenario']}` | `{shape_name}` | {a['trades']} | "
                    f"{a['win_rate_pct']:.2f}% | {a['avg_pnl_pct']:.4f}% | {a['median_pnl_pct']:.4f}% | "
                    f"{a['pnl_std_pct']:.4f}% | {a['worst_pnl_pct']:.4f}% | "
                    f"{a['net_pnl_value']:,.2f} | {a['max_pnl_drawdown']:,.2f} | {a['profit_factor']} |"
                )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "`A_sep_0p25` is the prior t3-only separation filter. `all_breakouts_sep_0p25` applies the same `0.25 * atr` SMA5 separation gate to both the original_t2 breakout and the added t3_swing breakout.",
            "",
            "On this Q1 30min replay, extending the separation gate to original_t2 is defensive: it cuts 663 trades, improves MaxDD by 0.68 pp, win rate by 1.71 pp, and Sharpe by 1.20 versus `t3_sma5_baseline`, but gives up 132.36 pp of return.",
            "",
            "## Conclusion",
            "",
            "Keep `A_sep_0p25` as the return-oriented candidate. Treat `all_breakouts_sep_0p25` as a risk-constrained variant rather than a primary return upgrade, because filtering original_t2 removes too much profitable flow despite the cleaner Sharpe/MaxDD profile.",
            "",
        ]
    )
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH Q1 2026 30min t3_sma5 signal-quality filtering")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--timeframes", nargs="+", default=["30min"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument(
        "--summary-json",
        default="research/eth_2026_q1_30min_t2_t3_sep_0p25_summary.json",
    )
    parser.add_argument(
        "--markdown",
        default="research/20260427_eth_q1_30min_t2_t3_sep_0p25.md",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    start = _as_utc_timestamp(args.start)
    end = _as_utc_timestamp(args.end)
    second_bars, build_stats = build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    derived_one_min_rows = int(len(second_bars.resample("1min").agg({"close": "last"}).dropna()))

    all_results = []
    for timeframe in args.timeframes:
        one_min, signal = build_signal_frame(second_bars, timeframe)
        print(f"running timeframe={timeframe} signal_rows={len(signal)}", flush=True)
        result = {
            "timeframe": timeframe,
            "signal_stats": {
                "signal_rows": int(len(signal)),
                "signal_start": signal.index[0].isoformat() if not signal.empty else "",
                "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
                "valid_sma5_rows": int(signal["sma5"].notna().sum()),
                "valid_atr_rows": int(signal["atr"].notna().sum()),
            },
            "scenarios": [],
        }
        summaries_by_name = {}
        for scenario_config in SCENARIOS:
            allowed_timeframes = scenario_config.get("timeframes")
            if allowed_timeframes and timeframe not in allowed_timeframes:
                continue
            breakout_shape = scenario_config["breakout_shape"]
            replay_mode = scenario_config["replay_mode"]
            scenario_name = scenario_config["scenario"]
            started = time.time()
            ledger, diagnostics = run_second_bar_replay(
                second_bars,
                signal,
                initial_balance=args.initial_balance,
                breakout_shape=breakout_shape,
                replay_mode=replay_mode,
                t3_reentry_size_schedule=scenario_config["t3_reentry_size_schedule"],
                t3_cooldown_bars=scenario_config["t3_cooldown_bars"],
                t3_quality_filters=scenario_config.get("t3_quality_filters"),
                quality_filter_shapes=scenario_config.get("quality_filter_shapes"),
            )
            elapsed = round(time.time() - started, 2)
            ledger_path = Path(
                f"research/tmp_eth_2026_q1_{timeframe}_1s_{replay_mode}_{scenario_name}_ledger.csv"
            )
            ledger.to_csv(ledger_path, index=False)
            summary = summarize_run(ledger, args.initial_balance)
            params = {
                **COMMON_REPLAY_KWARGS,
                "t3_reentry_size_schedule": scenario_config["t3_reentry_size_schedule"],
                "t3_cooldown_bars": scenario_config["t3_cooldown_bars"],
                "t3_quality_filters": scenario_config.get("t3_quality_filters", {}),
                "quality_filter_shapes": scenario_config.get("quality_filter_shapes", ["t3_swing"]),
            }
            scenario = {
                "scenario": scenario_name,
                "breakout_shape": breakout_shape,
                "replay_mode": replay_mode,
                "params": params,
                "summary": summary,
                "attribution": summarize_breakout_attribution(ledger),
                "diagnostics": diagnostics,
                "ledger_path": str(ledger_path),
                "elapsed_seconds": elapsed,
            }
            result["scenarios"].append(scenario)
            print(
                f"  {replay_mode}/{scenario_name}: return={summary['return_pct']:.2f}% "
                f"trades={summary['trades']} final={summary['final_balance']:.2f} elapsed={elapsed}s",
                flush=True,
            )
            summaries_by_name[scenario_name] = summary
        result["delta_vs_t3_sma5_baseline"] = {}
        baseline_summary = summaries_by_name["t3_sma5_baseline"]
        for scenario_name, scenario_summary in summaries_by_name.items():
            if scenario_name == "t3_sma5_baseline":
                continue
            result["delta_vs_t3_sma5_baseline"][scenario_name] = _scenario_delta(
                baseline_summary,
                scenario_summary,
            )
        all_results.append(result)

    summary = {
        "window": {"start": start.isoformat(), "end": end.isoformat()},
        "build_stats": {**build_stats, "derived_one_min_rows": derived_one_min_rows},
        "results": all_results,
        "baseline_scenario": "t3_sma5_baseline",
        "note": "Research-only 30min optimization. Baseline is t3_sma5_baseline; variants only add signal-quality filters to the added t3 breakout lock and preserve sizing.",
    }
    summary_path = Path(args.summary_json)
    summary_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
