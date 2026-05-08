#!/usr/bin/env python3
"""Micro-strength closed-bar breakout proxy and structure-trailing research.

Research-only. This runner is a closed-bar proxy: the aggregated signal bar
close is used to identify a breakout candidate, then execution continues on
1s OHLC built from local Binance trade ticks. It is not the live-style
three-bar intrabar breakout where the third bar is still open and high/low
touches the structural level.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_2026_jan_apr_impulse_bar_run as base
import eth_q1_breakout_t3_shape_compare as replay


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")
DEFAULT_CACHE_ROOT = Path("research/cache/second_bars")


def _maybe_number(value: str):
    lowered = value.strip().lower()
    if lowered in {"true", "false"}:
        return 1.0 if lowered == "true" else 0.0
    try:
        return float(value)
    except ValueError:
        return value.strip()


def parse_variant(raw: str) -> tuple[str, dict]:
    presets = {
        "dual_old": {"trend_mode": "dual", "exit_mode": "old"},
        "signal_structure": {"trend_mode": "signal", "exit_mode": "structure"},
        "none_structure": {"trend_mode": "none", "exit_mode": "structure"},
        "none_hybrid": {"trend_mode": "none", "exit_mode": "hybrid"},
    }
    params = {
        "trend_mode": "signal",
        "exit_mode": "structure",
        "break_lookback": 8,
        "body_min": 0.65,
        "close_top": 0.75,
        "range_min_atr": 1.00,
        "range_max_atr": 2.20,
        "pre_range_atr": 3.00,
        "max_atr_percentile": 95.0,
        "max_entry_extension_atr": 0.30,
        "initial_stop_atr": 0.45,
        "stop_buffer_atr": 0.05,
        "stop_cap_atr": 0.80,
        "min_stop_bps": 12.0,
        "breakeven_at_r": 1.0,
        "cost_lock_bps": 10.0,
        "micro_window_seconds": 300,
        "micro_fast_seconds": 60,
        "base_speed_atr": 0.02,
        "base_fast_atr": 0.00,
        "base_efficiency": 0.12,
        "strong_speed_atr": 0.08,
        "strong_fast_atr": 0.02,
        "strong_efficiency": 0.30,
        "strong_close_pos": 0.60,
        "weak_share": 0.10,
        "base_share": 0.20,
        "strong_share": 0.30,
        "skip_weak": 0.0,
        "old_trail_start_atr": 0.50,
        "old_trail_retrace_atr": 0.30,
        "structure_start_atr": 1.00,
        "structure_bars": 2,
        "structure_buffer_atr": 0.05,
        "disable_event_exits_after_structure": 1.0,
        "ema8_exit": 1.0,
        "no_new_extreme_bars": 2,
        "max_hold_hours": 10,
        "slippage": 0.0002,
        "entry_fee": 0.0002,
        "exit_fee": 0.0004,
    }
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    if name in presets:
        params.update(presets[name])
    key_map = {
        "trend": "trend_mode",
        "exit": "exit_mode",
        "break": "break_lookback",
        "body": "body_min",
        "close_top": "close_top",
        "range_min": "range_min_atr",
        "range_max": "range_max_atr",
        "pre_range": "pre_range_atr",
        "max_ext": "max_entry_extension_atr",
        "sl": "initial_stop_atr",
        "stop_cap": "stop_cap_atr",
        "micro": "micro_window_seconds",
        "fast": "micro_fast_seconds",
        "base_speed": "base_speed_atr",
        "base_fast": "base_fast_atr",
        "base_eff": "base_efficiency",
        "strong_speed": "strong_speed_atr",
        "strong_fast": "strong_fast_atr",
        "strong_eff": "strong_efficiency",
        "strong_pos": "strong_close_pos",
        "weak_share": "weak_share",
        "base_share": "base_share",
        "strong_share": "strong_share",
        "skip_weak": "skip_weak",
        "old_start": "old_trail_start_atr",
        "old_retrace": "old_trail_retrace_atr",
        "struct_start": "structure_start_atr",
        "struct_bars": "structure_bars",
        "struct_buffer": "structure_buffer_atr",
        "event_after_struct": "disable_event_exits_after_structure",
        "ema_exit": "ema8_exit",
        "nonew": "no_new_extreme_bars",
        "hold": "max_hold_hours",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _maybe_number(value)

    for key in ("break_lookback", "micro_window_seconds", "micro_fast_seconds", "structure_bars", "max_hold_hours"):
        params[key] = int(params[key])
    return name, params


def cache_path(symbol: str, start: pd.Timestamp, end: pd.Timestamp, cache_root: Path) -> Path:
    start_key = start.strftime("%Y%m%dT%H%M%S")
    end_key = end.strftime("%Y%m%dT%H%M%S")
    return cache_root / f"{symbol}_1s_{start_key}_{end_key}.parquet"


def read_second_bar_cache(path: Path) -> pd.DataFrame:
    import pyarrow.parquet as pq

    second_bars = pq.read_table(path).to_pandas()
    if second_bars.index.tz is None:
        second_bars.index = second_bars.index.tz_localize("UTC")
    else:
        second_bars.index = second_bars.index.tz_convert("UTC")
    return second_bars


def write_second_bar_cache(second_bars: pd.DataFrame, path: Path) -> None:
    import pyarrow as pa
    import pyarrow.parquet as pq

    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = path.with_suffix(".tmp.parquet")
    table = pa.Table.from_pandas(second_bars, preserve_index=True)
    pq.write_table(table, tmp_path, compression="zstd")
    tmp_path.replace(path)


def load_or_build_second_bars(
    symbol: str,
    start: pd.Timestamp,
    end: pd.Timestamp,
    archive_root: Path,
    chunksize: int,
    cache_root: Path,
    use_cache: bool,
) -> tuple[pd.DataFrame, dict]:
    path = cache_path(symbol, start, end, cache_root)
    if use_cache and path.exists():
        second_bars = read_second_bar_cache(path)
        return second_bars, {"cache_hit": True, "cache_path": str(path), "rows": int(len(second_bars))}

    tick_files = base.monthly_trade_files(symbol, start, end, archive_root)
    second_bars, build_stats = replay.build_continuous_second_bars(tick_files, start, end, chunksize)
    build_stats = dict(build_stats)
    build_stats["cache_hit"] = False
    build_stats["cache_path"] = str(path)
    if use_cache:
        write_second_bar_cache(second_bars, path)
        build_stats["cache_written"] = True
    return second_bars, build_stats


def build_frames(second_bars: pd.DataFrame, signal_timeframe: str, trend_timeframe: str):
    one_min, signal, trend = base.build_frames(
        second_bars,
        signal_timeframe=signal_timeframe,
        trend_timeframe=trend_timeframe,
    )
    for lookback in (1, 2, 3, 4):
        signal[f"struct_low_{lookback}"] = signal["low"].rolling(lookback, min_periods=1).min()
        signal[f"struct_high_{lookback}"] = signal["high"].rolling(lookback, min_periods=1).max()
    return one_min, signal, trend


def _signal_trend_ready(sig: pd.Series, side: str) -> bool:
    if not base._finite_positive(sig.get("ema20", np.nan)):
        return False
    if side == "long":
        return float(sig["close"]) > float(sig["ema20"]) and float(sig["ema20_slope"]) > 0
    return float(sig["close"]) < float(sig["ema20"]) and float(sig["ema20_slope"]) < 0


def _trend_ready(sig: pd.Series, side: str, params: dict) -> bool:
    mode = str(params["trend_mode"]).lower()
    if mode == "none":
        return True
    if mode == "signal":
        return _signal_trend_ready(sig, side)
    if mode == "dual":
        return base._trend_ready(sig, side)
    raise ValueError(f"unknown trend_mode {params['trend_mode']}")


def signal_side(sig: pd.Series, params: dict) -> str:
    atr = sig.get("atr", np.nan)
    bar_range = sig.get("range", np.nan)
    if not base._finite_positive(atr, bar_range, sig.get("pre_range_6", np.nan)):
        return ""
    atr_pct = sig.get("atr_percentile", np.nan)
    if pd.notna(atr_pct) and float(atr_pct) > float(params["max_atr_percentile"]):
        return ""
    if float(sig["pre_range_6"]) > float(params["pre_range_atr"]) * float(atr):
        return ""
    range_atr = float(bar_range) / float(atr)
    if range_atr < float(params["range_min_atr"]) or range_atr > float(params["range_max_atr"]):
        return ""
    if float(sig.get("body_ratio", 0.0)) < float(params["body_min"]):
        return ""

    lookback = int(params["break_lookback"])
    if (
        _trend_ready(sig, "long", params)
        and float(sig["close"]) > float(sig[f"prev_high_{lookback}"])
        and float(sig["close_pos"]) >= float(params["close_top"])
    ):
        return "long"
    if (
        _trend_ready(sig, "short", params)
        and float(sig["close"]) < float(sig[f"prev_low_{lookback}"])
        and float(sig["close_pos"]) <= 1.0 - float(params["close_top"])
    ):
        return "short"
    return ""


def micro_context(second_bars: pd.DataFrame, start_pos: int, sig: pd.Series, side: str, params: dict) -> dict:
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)
    atr = float(sig["atr"])
    side_mult = 1.0 if side == "long" else -1.0
    entry_time = idx[start_pos]
    lookback_time = entry_time - pd.Timedelta(seconds=int(params["micro_window_seconds"]))
    fast_time = entry_time - pd.Timedelta(seconds=int(params["micro_fast_seconds"]))
    lookback_pos = max(0, int(idx.searchsorted(lookback_time, side="left")))
    fast_pos = max(0, int(idx.searchsorted(fast_time, side="left")))

    entry_close = float(closes[start_pos])
    slow_move_atr = side_mult * (entry_close - float(closes[lookback_pos])) / atr
    fast_move_atr = side_mult * (entry_close - float(closes[fast_pos])) / atr
    window_high = float(np.max(highs[lookback_pos : start_pos + 1]))
    window_low = float(np.min(lows[lookback_pos : start_pos + 1]))
    window_range = max(window_high - window_low, 1e-12)
    efficiency = side_mult * (entry_close - float(closes[lookback_pos])) / window_range
    if side == "long":
        close_pos = (entry_close - window_low) / window_range
    else:
        close_pos = (window_high - entry_close) / window_range

    if (
        slow_move_atr >= float(params["strong_speed_atr"])
        and fast_move_atr >= float(params["strong_fast_atr"])
        and efficiency >= float(params["strong_efficiency"])
        and close_pos >= float(params["strong_close_pos"])
    ):
        tier = "strong"
        share = float(params["strong_share"])
    elif (
        slow_move_atr >= float(params["base_speed_atr"])
        and fast_move_atr >= float(params["base_fast_atr"])
        and efficiency >= float(params["base_efficiency"])
    ):
        tier = "base"
        share = float(params["base_share"])
    else:
        tier = "weak"
        share = float(params["weak_share"])

    return {
        "quality_tier": tier,
        "notional_share": share,
        "micro_speed_atr": float(slow_move_atr),
        "micro_fast_atr": float(fast_move_atr),
        "micro_efficiency": float(efficiency),
        "micro_close_pos": float(close_pos),
    }


def open_position(sig: pd.Series, side: str, entry_raw: float, entry_time, balance: float, params: dict, micro: dict):
    atr = float(sig["atr"])
    slippage = float(params["slippage"])
    entry_p = float(entry_raw) * (1.0 + slippage if side == "long" else 1.0 - slippage)
    if side == "long":
        raw_stop = min(float(sig["low"]) - float(params["stop_buffer_atr"]) * atr, entry_p - float(params["initial_stop_atr"]) * atr)
        capped_stop = entry_p - float(params["stop_cap_atr"]) * atr
        stop = max(raw_stop, capped_stop)
        risk = entry_p - stop
    else:
        raw_stop = max(float(sig["high"]) + float(params["stop_buffer_atr"]) * atr, entry_p + float(params["initial_stop_atr"]) * atr)
        capped_stop = entry_p + float(params["stop_cap_atr"]) * atr
        stop = min(raw_stop, capped_stop)
        risk = stop - entry_p
    if risk <= 0 or risk < entry_p * float(params["min_stop_bps"]) / 10000.0:
        return None
    share = float(micro["notional_share"])
    return {
        "side": side,
        "entry_time": entry_time,
        "entry_p": entry_p,
        "entry_raw": float(entry_raw),
        "sl": stop,
        "stop_reason": "InitialSL",
        "risk": risk,
        "atr_at_entry": atr,
        "notional": balance * share,
        "notional_share": share,
        "signal_bar_time": sig.name,
        "signal_close": float(sig["close"]),
        "signal_high": float(sig["high"]),
        "signal_low": float(sig["low"]),
        "quality_tier": micro["quality_tier"],
        "micro_speed_atr": micro["micro_speed_atr"],
        "micro_fast_atr": micro["micro_fast_atr"],
        "micro_efficiency": micro["micro_efficiency"],
        "micro_close_pos": micro["micro_close_pos"],
        "protected": False,
        "structure_active": False,
        "hwm": entry_p,
        "lwm": entry_p,
        "highest_hour_high": float(sig["high"]),
        "lowest_hour_low": float(sig["low"]),
        "no_new_extreme_bars": 0,
        "mfe_r": 0.0,
        "mae_r": 0.0,
        "mfe_atr": 0.0,
        "mae_atr": 0.0,
    }


def append_entry(logs: list[dict], position: dict, balance: float) -> None:
    logs.append(
        {
            "time": position["entry_time"],
            "type": "BUY" if position["side"] == "long" else "SHORT",
            "price": position["entry_p"],
            "raw_price": position["entry_raw"],
            "reason": "Micro-Breakout",
            "notional": position["notional"],
            "notional_share": position["notional_share"],
            "bal": balance,
            "signal_bar_time": position["signal_bar_time"],
            "signal_close": position["signal_close"],
            "quality_tier": position["quality_tier"],
            "micro_speed_atr": position["micro_speed_atr"],
            "micro_fast_atr": position["micro_fast_atr"],
            "micro_efficiency": position["micro_efficiency"],
            "risk": position["risk"],
            "mfe_r": np.nan,
            "mae_r": np.nan,
            "mfe_atr": np.nan,
        }
    )


def append_exit(logs: list[dict], position: dict, *, raw_exit: float, time_value, reason: str, balance: float, params: dict):
    slippage = float(params["slippage"])
    side_mult = 1.0 if position["side"] == "long" else -1.0
    exit_p = float(raw_exit) * (1.0 - slippage if position["side"] == "long" else 1.0 + slippage)
    pnl_pct = side_mult * (exit_p - float(position["entry_p"])) / float(position["entry_p"])
    new_balance = balance + pnl_pct * float(position["notional"])
    logs.append(
        {
            "time": time_value,
            "type": "EXIT",
            "price": exit_p,
            "raw_price": float(raw_exit),
            "reason": reason,
            "notional": position["notional"],
            "notional_share": position["notional_share"],
            "bal": new_balance,
            "signal_bar_time": position["signal_bar_time"],
            "signal_close": position["signal_close"],
            "quality_tier": position["quality_tier"],
            "micro_speed_atr": position["micro_speed_atr"],
            "micro_fast_atr": position["micro_fast_atr"],
            "micro_efficiency": position["micro_efficiency"],
            "risk": position["risk"],
            "mfe_r": float(position["mfe_r"]),
            "mae_r": float(position["mae_r"]),
            "mfe_atr": float(position["mfe_atr"]),
            "mae_atr": float(position["mae_atr"]),
            "stop_price": float(position["sl"]),
        }
    )
    return new_balance


def update_excursion(position: dict, high_value: float, low_value: float, params: dict) -> None:
    entry = float(position["entry_p"])
    risk = float(position["risk"])
    atr = float(position["atr_at_entry"])
    if position["side"] == "long":
        position["hwm"] = max(float(position["hwm"]), float(high_value))
        favorable = max(0.0, float(position["hwm"]) - entry)
        adverse = max(0.0, entry - float(low_value))
        if favorable / risk >= float(params["breakeven_at_r"]):
            be_sl = entry * (1.0 + float(params["cost_lock_bps"]) / 10000.0)
            if be_sl > float(position["sl"]):
                position["sl"] = be_sl
                position["stop_reason"] = "BreakevenSL"
                position["protected"] = True
        if str(params["exit_mode"]) in {"old", "hybrid"} and favorable / atr < float(params["structure_start_atr"]):
            if favorable / atr >= float(params["old_trail_start_atr"]):
                trail = float(position["hwm"]) - float(params["old_trail_retrace_atr"]) * atr
                if trail > float(position["sl"]):
                    position["sl"] = trail
                    position["stop_reason"] = "TrailingSL"
    else:
        position["lwm"] = min(float(position["lwm"]), float(low_value))
        favorable = max(0.0, entry - float(position["lwm"]))
        adverse = max(0.0, float(high_value) - entry)
        if favorable / risk >= float(params["breakeven_at_r"]):
            be_sl = entry * (1.0 - float(params["cost_lock_bps"]) / 10000.0)
            if be_sl < float(position["sl"]):
                position["sl"] = be_sl
                position["stop_reason"] = "BreakevenSL"
                position["protected"] = True
        if str(params["exit_mode"]) in {"old", "hybrid"} and favorable / atr < float(params["structure_start_atr"]):
            if favorable / atr >= float(params["old_trail_start_atr"]):
                trail = float(position["lwm"]) + float(params["old_trail_retrace_atr"]) * atr
                if trail < float(position["sl"]):
                    position["sl"] = trail
                    position["stop_reason"] = "TrailingSL"
    position["mfe_r"] = max(float(position["mfe_r"]), favorable / risk)
    position["mae_r"] = max(float(position["mae_r"]), adverse / risk)
    position["mfe_atr"] = max(float(position["mfe_atr"]), favorable / atr)
    position["mae_atr"] = max(float(position["mae_atr"]), adverse / atr)


def stop_trigger(position: dict, high_value: float, low_value: float) -> tuple[bool, float, str]:
    if position["side"] == "long" and low_value <= float(position["sl"]):
        return True, float(position["sl"]), str(position.get("stop_reason", "InitialSL"))
    if position["side"] == "short" and high_value >= float(position["sl"]):
        return True, float(position["sl"]), str(position.get("stop_reason", "InitialSL"))
    return False, 0.0, ""


def apply_signal_event(position: dict, event: pd.Series, params: dict) -> tuple[bool, str]:
    atr = float(position["atr_at_entry"])
    exit_mode = str(params["exit_mode"])
    if exit_mode in {"structure", "hybrid"} and float(position["mfe_atr"]) >= float(params["structure_start_atr"]):
        position["structure_active"] = True
        lookback = int(params["structure_bars"])
        if position["side"] == "long":
            trail = float(event[f"struct_low_{lookback}"]) - float(params["structure_buffer_atr"]) * atr
            if trail > float(position["sl"]):
                position["sl"] = trail
                position["stop_reason"] = "StructureSL"
        else:
            trail = float(event[f"struct_high_{lookback}"]) + float(params["structure_buffer_atr"]) * atr
            if trail < float(position["sl"]):
                position["sl"] = trail
                position["stop_reason"] = "StructureSL"

    if position["structure_active"] and float(params["disable_event_exits_after_structure"]) > 0:
        return False, ""

    if position["side"] == "long":
        if float(params["ema8_exit"]) > 0 and pd.notna(event.get("ema8", np.nan)) and float(event["close"]) < float(event["ema8"]):
            return True, "EMA8Exit"
        if float(event["high"]) > float(position["highest_hour_high"]):
            position["highest_hour_high"] = float(event["high"])
            position["no_new_extreme_bars"] = 0
        else:
            position["no_new_extreme_bars"] += 1
        if int(params["no_new_extreme_bars"]) > 0 and int(position["no_new_extreme_bars"]) >= int(params["no_new_extreme_bars"]):
            return True, "NoNewHighExit"
    else:
        if float(params["ema8_exit"]) > 0 and pd.notna(event.get("ema8", np.nan)) and float(event["close"]) > float(event["ema8"]):
            return True, "EMA8Exit"
        if float(event["low"]) < float(position["lowest_hour_low"]):
            position["lowest_hour_low"] = float(event["low"])
            position["no_new_extreme_bars"] = 0
        else:
            position["no_new_extreme_bars"] += 1
        if int(params["no_new_extreme_bars"]) > 0 and int(position["no_new_extreme_bars"]) >= int(params["no_new_extreme_bars"]):
            return True, "NoNewLowExit"
    return False, ""


def simulate_position(second_bars: pd.DataFrame, events_by_end: dict[pd.Timestamp, pd.Series], sig: pd.Series, side: str, params: dict, *, start_pos: int, balance: float):
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)
    entry_time = idx[start_pos]
    entry_raw = float(closes[start_pos])
    atr = float(sig["atr"])
    if side == "long":
        if entry_raw - float(sig["close"]) > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension", None
    else:
        if float(sig["close"]) - entry_raw > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension", None

    micro = micro_context(second_bars, start_pos, sig, side, params)
    if micro["quality_tier"] == "weak" and float(params["skip_weak"]) > 0:
        return None, balance, "weak_skipped", micro
    if float(micro["notional_share"]) <= 0:
        return None, balance, "weak_skipped", micro

    position = open_position(sig, side, entry_raw, entry_time, balance, params, micro)
    if position is None:
        return None, balance, "min_stop", micro
    logs: list[dict] = []
    append_entry(logs, position, balance)
    max_hold_hours = int(params["max_hold_hours"])
    if max_hold_hours > 0:
        max_hold_end = entry_time + pd.Timedelta(hours=max_hold_hours)
        end_pos = min(int(idx.searchsorted(max_hold_end, side="left")), len(idx) - 1)
    else:
        max_hold_end = None
        end_pos = len(idx) - 1

    for pos in range(start_pos + 1, end_pos + 1):
        bar_time = idx[pos]
        high_value = float(highs[pos])
        low_value = float(lows[pos])
        close_value = float(closes[pos])
        update_excursion(position, high_value, low_value, params)
        triggered, raw_exit, reason = stop_trigger(position, high_value, low_value)
        if triggered:
            balance = append_exit(logs, position, raw_exit=raw_exit, time_value=bar_time, reason=reason, balance=balance, params=params)
            return logs, balance, "", micro

        event = events_by_end.get(bar_time)
        if event is not None and bar_time > entry_time:
            exit_now, event_reason = apply_signal_event(position, event, params)
            if exit_now:
                balance = append_exit(logs, position, raw_exit=close_value, time_value=bar_time, reason=event_reason, balance=balance, params=params)
                return logs, balance, "", micro

        if max_hold_end is not None and bar_time >= max_hold_end:
            balance = append_exit(logs, position, raw_exit=close_value, time_value=bar_time, reason="MaxHoldExit", balance=balance, params=params)
            return logs, balance, "", micro

    balance = append_exit(logs, position, raw_exit=float(closes[end_pos]), time_value=idx[end_pos], reason="FinalMarkToMarket", balance=balance, params=params)
    return logs, balance, "", micro


def run_strategy(second_bars: pd.DataFrame, signal: pd.DataFrame, params: dict, *, initial_balance: float) -> dict:
    idx = second_bars.index
    events_by_end = {pd.Timestamp(row["bar_end"]): row for _, row in signal.iterrows()}
    balance = float(initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    diagnostics = {
        "candidate_signals": 0,
        "entries": 0,
        "busy_skipped": 0,
        "entry_extension_skipped": 0,
        "weak_skipped": 0,
        "min_stop_skipped": 0,
        "long_signals": 0,
        "short_signals": 0,
        "quality_counts": {"weak": 0, "base": 0, "strong": 0},
    }
    for _, sig in signal.iterrows():
        side = signal_side(sig, params)
        if not side:
            continue
        diagnostics["candidate_signals"] += 1
        diagnostics[f"{side}_signals"] += 1
        entry_time = pd.Timestamp(sig["bar_end"])
        if entry_time <= last_exit_time:
            diagnostics["busy_skipped"] += 1
            continue
        start_pos = int(idx.searchsorted(entry_time, side="left"))
        if start_pos >= len(idx):
            continue
        trade_logs, new_balance, skip_reason, micro = simulate_position(
            second_bars,
            events_by_end,
            sig,
            side,
            params,
            start_pos=start_pos,
            balance=balance,
        )
        if micro is not None:
            diagnostics["quality_counts"][micro["quality_tier"]] += 1
        if skip_reason == "entry_extension":
            diagnostics["entry_extension_skipped"] += 1
            continue
        if skip_reason == "weak_skipped":
            diagnostics["weak_skipped"] += 1
            continue
        if skip_reason == "min_stop":
            diagnostics["min_stop_skipped"] += 1
            continue
        if not trade_logs:
            continue
        logs.extend(trade_logs)
        balance = new_balance
        diagnostics["entries"] += 1
        last_exit_time = pd.Timestamp(trade_logs[-1]["time"])

    ledger = pd.DataFrame(logs)
    return {
        "summary": summarize_ledger(ledger, initial_balance, params),
        "diagnostics": diagnostics,
        "ledger": ledger,
    }


def paired_trades(ledger: pd.DataFrame) -> pd.DataFrame:
    rows = []
    entry = None
    if ledger.empty:
        return pd.DataFrame(rows)
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            entry = row
            continue
        if row["type"] != "EXIT" or entry is None:
            continue
        side_mult = 1.0 if entry["type"] == "BUY" else -1.0
        entry_price = float(entry["price"])
        exit_price = float(row["price"])
        raw_entry = float(entry["raw_price"])
        raw_exit = float(row["raw_price"])
        rows.append(
            {
                "entry_time": entry["time"],
                "exit_time": row["time"],
                "side": entry["type"],
                "exit_reason": row["reason"],
                "slip_pnl_pct": side_mult * (exit_price - entry_price) / entry_price,
                "raw_pnl_pct": side_mult * (raw_exit - raw_entry) / raw_entry,
                "notional_share": float(entry["notional_share"]),
                "quality_tier": str(entry.get("quality_tier", "")),
                "hold_seconds": (pd.Timestamp(row["time"]) - pd.Timestamp(entry["time"])).total_seconds(),
                "mfe_r": float(row.get("mfe_r", 0.0)),
                "mae_r": float(row.get("mae_r", 0.0)),
                "mfe_atr": float(row.get("mfe_atr", 0.0)),
                "mae_atr": float(row.get("mae_atr", 0.0)),
            }
        )
        entry = None
    return pd.DataFrame(rows)


def summarize_ledger(ledger: pd.DataFrame, initial_balance: float, params: dict) -> dict:
    pairs = paired_trades(ledger)
    if pairs.empty:
        return {
            "trades": 0,
            "raw_no_fee_no_slip_return_pct": 0.0,
            "price_pnl_with_2bps_slip_no_fee_return_pct": 0.0,
            "realistic_return_pct": 0.0,
            "win_rate_pct": 0.0,
            "max_dd_pct": 0.0,
            "exit_reasons": {},
            "quality_trades": {},
        }

    raw_balance = float(initial_balance)
    slip_balance = float(initial_balance)
    realistic_balance = float(initial_balance)
    fee_rate = float(params["entry_fee"]) + float(params["exit_fee"])
    equity = [realistic_balance]
    for _, pair in pairs.iterrows():
        share = float(pair["notional_share"])
        raw_balance += raw_balance * share * float(pair["raw_pnl_pct"])
        slip_balance += slip_balance * share * float(pair["slip_pnl_pct"])
        realistic_notional = realistic_balance * share
        realistic_balance += realistic_notional * float(pair["slip_pnl_pct"]) - realistic_notional * fee_rate
        equity.append(realistic_balance)

    equity_arr = np.array(equity, dtype="float64")
    peak = np.maximum.accumulate(equity_arr)
    dd = equity_arr / peak - 1.0
    return {
        "trades": int(len(pairs)),
        "raw_no_fee_no_slip_return_pct": round((raw_balance / initial_balance - 1.0) * 100.0, 4),
        "price_pnl_with_2bps_slip_no_fee_return_pct": round((slip_balance / initial_balance - 1.0) * 100.0, 4),
        "realistic_return_pct": round((realistic_balance / initial_balance - 1.0) * 100.0, 4),
        "win_rate_pct": round(float((pairs["slip_pnl_pct"] > 0).mean()) * 100.0, 2),
        "max_dd_pct": round(float(dd.min()) * 100.0, 4),
        "avg_notional_share": round(float(pairs["notional_share"].mean()), 4),
        "median_hold_seconds": round(float(pairs["hold_seconds"].median()), 2),
        "avg_hold_seconds": round(float(pairs["hold_seconds"].mean()), 2),
        "median_mfe_r": round(float(pairs["mfe_r"].median()), 4),
        "median_mfe_atr": round(float(pairs["mfe_atr"].median()), 4),
        "exit_reasons": {str(k): int(v) for k, v in pairs["exit_reason"].value_counts().items()},
        "quality_trades": {str(k): int(v) for k, v in pairs["quality_tier"].value_counts().items()},
    }


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# Micro Breakout Structure 1s 回测（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。这是 closed-bar breakout proxy：先用聚合后的 signal-bar close 识别突破候选，再进入连续 `1s OHLC` 执行段。它不是 live 风格的三根 bar intrabar breakout；真实结构突破里第三根 bar 仍未闭合，由当前 bar 内 `1s high/low` 触碰结构 level 触发。高周期趋势过滤由 variant 控制；进场仓位根据近期 `1s` speed/efficiency 调整；结构退出在达到配置的 ATR 盈利阈值后，沿已完成 signal-bar 结构移动止损。",
        "",
        "| Symbol | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | 胜率 | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["diagnostics"]
        lines.append(
            f"| `{result['symbol']}` | `{result['variant']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | "
            f"{s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | {s.get('avg_notional_share', 0.0):.4f} | "
            f"{s.get('median_hold_seconds', 0.0):.2f}s | {s.get('median_mfe_atr', 0.0):.4f} | "
            f"`{s['exit_reasons']}` | `{s.get('quality_trades', {})}` | {d['candidate_signals']} | "
            f"{d['entries']} | {d['weak_skipped']} | {d['busy_skipped']} |"
        )
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['symbol']} {result['variant']}` ledger：`{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Micro-strength breakout structure replay")
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--trend-timeframe", default="4h")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument(
        "--variants",
        nargs="+",
        default=[
            "dual_old",
            "signal_structure",
            "none_structure",
            "none_hybrid",
        ],
    )
    parser.add_argument("--summary-json", default="research/btc_eth_micro_breakout_structure_summary.json")
    parser.add_argument("--markdown", default="research/20260507_btc_eth_micro_breakout_structure.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_eth_micro_breakout_structure")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    variants = [parse_variant(raw) for raw in args.variants]
    results = []
    for symbol in args.symbols:
        second_bars, build_stats = load_or_build_second_bars(
            symbol,
            start,
            end,
            Path(args.archive_root),
            args.chunksize,
            Path(args.cache_root),
            not args.no_cache,
        )
        _, signal, trend = build_frames(second_bars, args.signal_timeframe, args.trend_timeframe)
        print(
            f"{symbol}: second_rows={len(second_bars)} signal_rows={len(signal)} trend_rows={len(trend)}",
            flush=True,
        )
        for variant_name, params in variants:
            print(f"running {symbol} {variant_name}", flush=True)
            result = run_strategy(second_bars, signal, params, initial_balance=args.initial_balance)
            ledger_path = Path(f"{args.ledger_prefix}_{symbol}_{variant_name}_ledger.csv")
            result["ledger"].to_csv(ledger_path, index=False)
            del result["ledger"]
            result.update(
                {
                    "symbol": symbol,
                    "variant": variant_name,
                    "params": params,
                    "ledger_path": str(ledger_path),
                    "build_stats": build_stats,
                }
            )
            s = result["summary"]
            print(
                f"{symbol} {variant_name}: realistic={s['realistic_return_pct']:.4f}% "
                f"raw={s['raw_no_fee_no_slip_return_pct']:.4f}% trades={s['trades']} "
                f"win={s['win_rate_pct']:.2f}% dd={s['max_dd_pct']:.4f}% "
                f"avg_share={s.get('avg_notional_share', 0.0):.4f} diag={result['diagnostics']}",
                flush=True,
            )
            results.append(result)

    summary_path = Path(args.summary_json)
    markdown_path = Path(args.markdown)
    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "signal_timeframe": args.signal_timeframe,
        "trend_timeframe": args.trend_timeframe,
        "execution": "continuous 1s OHLC bars from local trade ticks",
        "accounting": "2bps/side slippage plus maker entry 2bps and market exit 4bps realistic accounting",
        "variants": [{"name": name, "params": params} for name, params in variants],
        "results": results,
        "summary_path": str(summary_path),
        "markdown_path": str(markdown_path),
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, markdown_path)
    print(json.dumps({"summary_path": str(summary_path), "markdown_path": str(markdown_path), "elapsed_seconds": summary["elapsed_seconds"]}, indent=2), flush=True)


if __name__ == "__main__":
    main()
