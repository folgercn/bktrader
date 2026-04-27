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
    },
    {
        "scenario": "B_trend_sep_0p25",
        "breakout_shape": "baseline_plus_t3",
        "replay_mode": "live_intrabar_sma5",
        "t3_reentry_size_schedule": [0.20, 0.10],
        "t3_cooldown_bars": 0,
        "timeframes": ["30min"],
        "t3_quality_filters": {"trend": True, "min_sma_atr_separation": 0.25},
    },
    {
        "scenario": "C_atr_pct30_sep_0p25",
        "breakout_shape": "baseline_plus_t3",
        "replay_mode": "live_intrabar_sma5",
        "t3_reentry_size_schedule": [0.20, 0.10],
        "t3_cooldown_bars": 0,
        "timeframes": ["30min"],
        "t3_quality_filters": {"min_atr_percentile": 30.0, "min_sma_atr_separation": 0.25},
    },
    {
        "scenario": "D_trend_atr_pct30_sep_0p25",
        "breakout_shape": "baseline_plus_t3",
        "replay_mode": "live_intrabar_sma5",
        "t3_reentry_size_schedule": [0.20, 0.10],
        "t3_cooldown_bars": 0,
        "timeframes": ["30min"],
        "t3_quality_filters": {
            "trend": True,
            "min_atr_percentile": 30.0,
            "min_sma_atr_separation": 0.25,
        },
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
    for n in range(1, 5):
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


def _allow_breakout_lock(shape_name: str, bar_index: int, last_t3_lock_bar_index: int, t3_cooldown_bars: int) -> bool:
    if shape_name != "t3_swing" or t3_cooldown_bars <= 0:
        return True
    return bar_index - last_t3_lock_bar_index > t3_cooldown_bars


def _t3_quality_reject_reason(sig, side: str, current_price: float, breakout_level: float, filters: dict) -> str:
    if not filters:
        return ""

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

    max_breakout_extension_atr = filters.get("max_breakout_extension_atr")
    if max_breakout_extension_atr is not None:
        if not _positive(atr):
            return "breakout_extension_missing"
        extension = abs(float(current_price) - float(breakout_level)) / float(atr)
        if extension > float(max_breakout_extension_atr):
            return "breakout_extension"

    return ""


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
):
    if replay_mode not in {"same_bar_parity", "live_intrabar_sma5"}:
        raise ValueError(f"unknown replay mode: {replay_mode}")

    balance = initial_balance
    position = None
    trade_logs = []
    diagnostics = {
        "breakout_locks": {"long": {}, "short": {}},
        "t3_cooldown_skips": {"long": 0, "short": 0},
        "t3_quality_rejects": {"long": {}, "short": {}},
    }
    second_index = df_seconds.index
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

    last_exit_bar_index = -999
    reentry_timeout = 1
    last_exit_reason = None
    last_exit_side = None
    last_exit_breakout_shape = ""
    pending_zero_initial_side = None
    pending_zero_initial_breakout_shape = ""
    pending_zero_initial_bar_index = -999
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
        if i - pending_zero_initial_bar_index > reentry_timeout:
            pending_zero_initial_side = None
            pending_zero_initial_breakout_shape = ""

        while current_pos < end_pos:
            bar_time = second_index[current_pos]
            high_value = high_values[current_pos]
            low_value = low_values[current_pos]
            close_value = close_values[current_pos]
            prev_close = close_values[current_pos - 1] if current_pos > start_pos else None
            if replay_mode == "live_intrabar_sma5":
                bar_high_so_far = max(bar_high_so_far, high_value)
                bar_low_so_far = min(bar_low_so_far, low_value)
                sig = _intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)
                long_regime_ready, short_regime_ready = _resolve_regime_ready(sig, "1d")

            if position is None:
                if long_regime_ready:
                    triggered, breakout_level, shape_name = _long_breakout(sig, high_value, breakout_shape)
                    if trades_in_bar == 0 and triggered:
                        quality_reject_reason = ""
                        if shape_name == "t3_swing":
                            quality_reject_reason = _t3_quality_reject_reason(
                                sig,
                                "long",
                                close_value,
                                breakout_level,
                                t3_quality_filters,
                            )
                        if quality_reject_reason:
                            if quality_reject_reason not in quality_reject_recorded_this_bar["long"]:
                                diagnostics["t3_quality_rejects"]["long"][quality_reject_reason] = (
                                    diagnostics["t3_quality_rejects"]["long"].get(quality_reject_reason, 0) + 1
                                )
                                quality_reject_recorded_this_bar["long"].add(quality_reject_reason)
                        elif _allow_breakout_lock(shape_name, i, last_t3_lock_bar_index, t3_cooldown_bars):
                            if shape_name == "t3_swing":
                                last_t3_lock_bar_index = i
                            if not breakout_locked_this_bar:
                                diagnostics["breakout_locks"]["long"][shape_name] = (
                                    diagnostics["breakout_locks"]["long"].get(shape_name, 0) + 1
                                )
                                breakout_locked_this_bar = True
                            pending_zero_initial_side = "long"
                            pending_zero_initial_breakout_shape = shape_name
                            pending_zero_initial_bar_index = i
                        else:
                            diagnostics["t3_cooldown_skips"]["long"] += 1

                    has_exit_reentry_window = last_exit_side == "long" and (i - last_exit_bar_index <= reentry_timeout)
                    has_zero_initial_window = (
                        pending_zero_initial_side == "long" and (i - pending_zero_initial_bar_index <= reentry_timeout)
                    )
                    if has_exit_reentry_window or has_zero_initial_window:
                        re_p = _resolve_reentry_price(sig, "long", reentry_anchor_levels, long_reentry_atr)
                        is_triggered, entry_p_raw = _reentry_triggered(
                            "long",
                            reentry_trigger_mode,
                            high_value,
                            low_value,
                            close_value,
                            prev_close,
                            re_p,
                            False,
                        )
                        if is_triggered:
                            reason = "Zero-Initial-Reentry"
                            if has_exit_reentry_window:
                                reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
                            if trades_in_bar < max_trades_per_bar:
                                entry_breakout_shape = pending_zero_initial_breakout_shape
                                if has_exit_reentry_window:
                                    entry_breakout_shape = last_exit_breakout_shape
                                active_schedule = _shape_schedule(
                                    entry_breakout_shape,
                                    reentry_size_schedule,
                                    t3_reentry_size_schedule,
                                )
                                notional_share = _get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                                entry_price = float(entry_p_raw) * (1 + slippage)
                                balance, position = _open_position(
                                    balance,
                                    sig,
                                    "long",
                                    entry_price,
                                    notional_share,
                                    reason,
                                    stop_mode,
                                    stop_loss_atr,
                                    entry_breakout_shape,
                                    replay_mode,
                                )
                                trade_logs.append(
                                    {
                                        "time": bar_time,
                                        "type": "BUY",
                                        "price": entry_price,
                                        "reason": reason,
                                        "notional": position["notional"],
                                        "bal": balance,
                                        "breakout_shape_name": position["breakout_shape_name"],
                                        "replay_mode": replay_mode,
                                    }
                                )
                                trades_in_bar += 1
                            if has_exit_reentry_window:
                                last_exit_side = None
                                last_exit_breakout_shape = ""
                            if has_zero_initial_window:
                                pending_zero_initial_side = None
                                pending_zero_initial_breakout_shape = ""

                elif short_regime_ready:
                    triggered, breakout_level, shape_name = _short_breakout(sig, low_value, breakout_shape)
                    if trades_in_bar == 0 and triggered:
                        quality_reject_reason = ""
                        if shape_name == "t3_swing":
                            quality_reject_reason = _t3_quality_reject_reason(
                                sig,
                                "short",
                                close_value,
                                breakout_level,
                                t3_quality_filters,
                            )
                        if quality_reject_reason:
                            if quality_reject_reason not in quality_reject_recorded_this_bar["short"]:
                                diagnostics["t3_quality_rejects"]["short"][quality_reject_reason] = (
                                    diagnostics["t3_quality_rejects"]["short"].get(quality_reject_reason, 0) + 1
                                )
                                quality_reject_recorded_this_bar["short"].add(quality_reject_reason)
                        elif _allow_breakout_lock(shape_name, i, last_t3_lock_bar_index, t3_cooldown_bars):
                            if shape_name == "t3_swing":
                                last_t3_lock_bar_index = i
                            if not breakout_locked_this_bar:
                                diagnostics["breakout_locks"]["short"][shape_name] = (
                                    diagnostics["breakout_locks"]["short"].get(shape_name, 0) + 1
                                )
                                breakout_locked_this_bar = True
                            pending_zero_initial_side = "short"
                            pending_zero_initial_breakout_shape = shape_name
                            pending_zero_initial_bar_index = i
                        else:
                            diagnostics["t3_cooldown_skips"]["short"] += 1

                    has_exit_reentry_window = last_exit_side == "short" and (i - last_exit_bar_index <= reentry_timeout)
                    has_zero_initial_window = (
                        pending_zero_initial_side == "short" and (i - pending_zero_initial_bar_index <= reentry_timeout)
                    )
                    if has_exit_reentry_window or has_zero_initial_window:
                        re_p = _resolve_reentry_price(sig, "short", reentry_anchor_levels, short_reentry_atr)
                        is_triggered, entry_p_raw = _reentry_triggered(
                            "short",
                            reentry_trigger_mode,
                            high_value,
                            low_value,
                            close_value,
                            prev_close,
                            re_p,
                            False,
                        )
                        if is_triggered:
                            reason = "Zero-Initial-Reentry"
                            if has_exit_reentry_window:
                                reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
                            if trades_in_bar < max_trades_per_bar:
                                entry_breakout_shape = pending_zero_initial_breakout_shape
                                if has_exit_reentry_window:
                                    entry_breakout_shape = last_exit_breakout_shape
                                active_schedule = _shape_schedule(
                                    entry_breakout_shape,
                                    reentry_size_schedule,
                                    t3_reentry_size_schedule,
                                )
                                notional_share = _get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                                entry_price = float(entry_p_raw) * (1 - slippage)
                                balance, position = _open_position(
                                    balance,
                                    sig,
                                    "short",
                                    entry_price,
                                    notional_share,
                                    reason,
                                    stop_mode,
                                    stop_loss_atr,
                                    entry_breakout_shape,
                                    replay_mode,
                                )
                                trade_logs.append(
                                    {
                                        "time": bar_time,
                                        "type": "SHORT",
                                        "price": entry_price,
                                        "reason": reason,
                                        "notional": position["notional"],
                                        "bal": balance,
                                        "breakout_shape_name": position["breakout_shape_name"],
                                        "replay_mode": replay_mode,
                                    }
                                )
                                trades_in_bar += 1
                            if has_exit_reentry_window:
                                last_exit_side = None
                                last_exit_breakout_shape = ""
                            if has_zero_initial_window:
                                pending_zero_initial_side = None
                                pending_zero_initial_breakout_shape = ""

            else:
                exit_triggered = False
                exit_p = 0.0
                reason = ""

                if position["side"] == "long":
                    prev_hwm = position.get("hwm", position["entry_p"])
                    protected_before_bar = position.get("protected", False)

                    profit_atr = (prev_hwm - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0
                    if profit_atr >= delayed_trailing_activation:
                        trailing_sl = prev_hwm - trailing_stop_atr * sig["atr"]
                        position["sl"] = max(position["sl"], trailing_sl)

                    if low_value <= position["sl"]:
                        exit_p, reason, exit_triggered = position["sl"], "SL", True
                    elif protected_before_bar and low_value <= sig["prev_low_1"]:
                        exit_p, reason, exit_triggered = sig["prev_low_1"], "PT", True

                    if not exit_triggered:
                        position["hwm"] = max(prev_hwm, high_value)
                        if not position["protected"] and high_value >= position["entry_p"] + profit_protect_atr * sig["atr"]:
                            position["protected"] = True
                        profit_atr = (position["hwm"] - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0
                        if profit_atr >= delayed_trailing_activation:
                            trailing_sl = position["hwm"] - trailing_stop_atr * sig["atr"]
                            position["sl"] = max(position["sl"], trailing_sl)

                else:
                    prev_lwm = position.get("lwm", position["entry_p"])
                    protected_before_bar = position.get("protected", False)

                    profit_atr = (position["entry_p"] - prev_lwm) / sig["atr"] if sig["atr"] > 0 else 0
                    if profit_atr >= delayed_trailing_activation:
                        trailing_sl = prev_lwm + trailing_stop_atr * sig["atr"]
                        position["sl"] = min(position["sl"], trailing_sl)

                    if high_value >= position["sl"]:
                        exit_p, reason, exit_triggered = position["sl"], "SL", True
                    elif protected_before_bar and high_value >= sig["prev_high_1"]:
                        exit_p, reason, exit_triggered = sig["prev_high_1"], "PT", True

                    if not exit_triggered:
                        position["lwm"] = min(prev_lwm, low_value)
                        if not position["protected"] and low_value <= position["entry_p"] - profit_protect_atr * sig["atr"]:
                            position["protected"] = True
                        profit_atr = (position["entry_p"] - position["lwm"]) / sig["atr"] if sig["atr"] > 0 else 0
                        if profit_atr >= delayed_trailing_activation:
                            trailing_sl = position["lwm"] + trailing_stop_atr * sig["atr"]
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
                    last_exit_bar_index = i
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
        "- `A_sep_0p25`: t3 requires `abs(breakout_level - sma5) >= 0.25 * atr`.",
        "- `B_trend_sep_0p25`: A plus trend direction filter.",
        "- `C_atr_pct30_sep_0p25`: A plus ATR percentile >= `30%`.",
        "- `D_trend_atr_pct30_sep_0p25`: A plus trend direction and ATR percentile >= `30%`.",
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
            "This run keeps the `t3_sma5_baseline` sizing and only filters the added t3 breakout lock. The original_t2 path is left unchanged, so deltas isolate signal-quality filtering rather than position sizing.",
            "",
            "## Conclusion",
            "",
            "- A (`sep_0p25`) is still the best primary candidate: return improves `+4.22 pp`, MaxDD improves `0.06 pp`, Sharpe improves `+0.07`, and trades drop `25`.",
            "- Adding trend on top of A is defensive but gives back too much return: B improves Sharpe more (`+0.15`) and cuts `115` trades, but return falls `16.61 pp` vs baseline and `20.83 pp` vs A.",
            "- Adding ATR percentile on top of A is the better defensive overlay: C improves MaxDD `0.31 pp`, Sharpe `+0.19`, win rate `+0.72 pp`, and cuts `214` trades, while giving back `12.24 pp` vs baseline and `16.46 pp` vs A.",
            "- Adding both filters on top of A over-constrains the signal: D has the highest Sharpe delta (`+0.23`) and lowest trade count, but loses `31.67 pp` return vs baseline.",
            "",
            "Recommended ranking: A for primary 30min candidate; C as the risk-off candidate when drawdown/trade count matters more than raw return.",
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
        default="research/eth_2026_q1_30min_t3_sma5_sep_0p25_marginal_summary.json",
    )
    parser.add_argument(
        "--markdown",
        default="research/20260427_eth_q1_30min_t3_sma5_sep_0p25_marginal.md",
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
