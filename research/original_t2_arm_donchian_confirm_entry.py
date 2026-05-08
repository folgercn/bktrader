#!/usr/bin/env python3
"""original_t2 arm + Donchian confirm entry research replay.

Research-only. This runner keeps true original_t2 as an intrabar arm condition
and opens real exposure only after the same signal bar touches the 8-bar
Donchian boundary. Execution uses continuous 1s OHLC built from local Binance
trade ticks.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_2026_jan_apr_direct_breakout as direct
import btc_eth_micro_breakout_structure as micro
import eth_original_t2_pretouch_entry_replay as pretouch
import eth_q1_breakout_t3_shape_compare as replay


def _freq_delta(freq: str) -> pd.Timedelta:
    return pd.to_timedelta(pd.tseries.frequencies.to_offset(freq).nanos, unit="ns")


def _last_percentile(values: np.ndarray) -> float:
    clean = values[~np.isnan(values)]
    if len(clean) == 0:
        return np.nan
    return float((clean <= clean[-1]).mean() * 100.0)


def _finite_positive(*values: float) -> bool:
    return all(pd.notna(v) and np.isfinite(float(v)) and float(v) > 0 for v in values)


def build_live_signal_frame(second_bars: pd.DataFrame, timeframe: str) -> pd.DataFrame:
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
    closed_atr = true_range.shift(1).rolling(14).mean()
    signal["closed_atr"] = closed_atr
    signal["atr"] = closed_atr
    signal["atr_percentile"] = closed_atr.rolling(240, min_periods=60).apply(_last_percentile, raw=True)

    for lookback in range(1, 9):
        signal[f"prev_high_{lookback}"] = signal["high"].shift(lookback)
        signal[f"prev_low_{lookback}"] = signal["low"].shift(lookback)
    signal["prev_high_8"] = signal["high"].shift(1).rolling(8).max()
    signal["prev_low_8"] = signal["low"].shift(1).rolling(8).min()
    for lookback in range(1, 5):
        signal[f"prev_close_{lookback}"] = signal["close"].shift(lookback)

    signal["prev_ema20_1"] = signal["close"].ewm(span=20, adjust=False, min_periods=20).mean().shift(1)
    signal["prev_ema20_2"] = signal["close"].ewm(span=20, adjust=False, min_periods=20).mean().shift(2)
    signal["pre_high_6"] = signal["high"].shift(1).rolling(6).max()
    signal["pre_low_6"] = signal["low"].shift(1).rolling(6).min()
    signal["pre_range_6"] = signal["pre_high_6"] - signal["pre_low_6"]
    return signal


def _intrabar_signal(base: pd.Series, high_so_far: float, low_so_far: float, close_now: float) -> dict:
    sig = base.to_dict()
    sig["high"] = float(high_so_far)
    sig["low"] = float(low_so_far)
    sig["close"] = float(close_now)
    bar_range = float(high_so_far) - float(low_so_far)
    sig["range"] = bar_range
    sig["body"] = abs(float(close_now) - float(base["open"]))
    sig["body_ratio"] = sig["body"] / bar_range if bar_range > 0 else np.nan
    sig["close_pos"] = (float(close_now) - float(low_so_far)) / bar_range if bar_range > 0 else np.nan

    prev_close = float(sig.get("prev_close_1", close_now)) if pd.notna(sig.get("prev_close_1", np.nan)) else close_now
    live_tr = max(
        float(high_so_far) - float(low_so_far),
        abs(float(high_so_far) - prev_close),
        abs(float(low_so_far) - prev_close),
    )
    closed_atr = sig.get("closed_atr", np.nan)
    if pd.notna(closed_atr) and float(closed_atr) > 0:
        sig["atr"] = ((float(closed_atr) * 13.0) + live_tr) / 14.0

    prev_ema20 = sig.get("prev_ema20_1", np.nan)
    prev_ema20_2 = sig.get("prev_ema20_2", np.nan)
    if pd.notna(prev_ema20):
        alpha = 2.0 / 21.0
        sig["ema20"] = alpha * float(close_now) + (1.0 - alpha) * float(prev_ema20)
        sig["ema20_slope"] = sig["ema20"] - float(prev_ema20)
    elif pd.notna(prev_ema20_2):
        sig["ema20"] = prev_ema20_2
        sig["ema20_slope"] = np.nan
    else:
        sig["ema20"] = np.nan
        sig["ema20_slope"] = np.nan
    return sig


def _maybe_number(value: str):
    lowered = value.strip().lower()
    if lowered in {"true", "false"}:
        return 1.0 if lowered == "true" else 0.0
    try:
        return float(value)
    except ValueError:
        return value.strip()


def parse_variant(raw: str) -> tuple[str, dict]:
    params = {
        "trend_mode": "signal",
        "body_min": 0.65,
        "close_top": 0.75,
        "range_min_atr": 1.00,
        "range_max_atr": 2.20,
        "pre_range_atr": 3.00,
        "max_atr_percentile": 95.0,
        "max_entry_extension_atr": 0.30,
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
        "skip_weak": 1.0,
        "min_donchian_gap_atr": 0.0,
        "max_donchian_gap_atr": 99.0,
    }
    presets = {
        "s10b4": {},
        "s10b4_notrend": {"trend_mode": "none"},
        "b55_loose": {"body_min": 0.55, "close_top": 0.65, "range_min_atr": 0.60, "trend_mode": "signal"},
        "b55_loose_notrend": {
            "body_min": 0.55,
            "close_top": 0.65,
            "range_min_atr": 0.60,
            "trend_mode": "none",
        },
        "s10b4_near": {"max_donchian_gap_atr": 0.10},
    }
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    if name in presets:
        params.update(presets[name])
    key_map = {
        "trend": "trend_mode",
        "body": "body_min",
        "close_top": "close_top",
        "range_min": "range_min_atr",
        "range_max": "range_max_atr",
        "pre_range": "pre_range_atr",
        "max_ext": "max_entry_extension_atr",
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
        "min_gap": "min_donchian_gap_atr",
        "max_gap": "max_donchian_gap_atr",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _maybe_number(value)
    for key in ("micro_window_seconds", "micro_fast_seconds"):
        params[key] = int(params[key])
    return name, params


def _original_t2_levels(sig: dict) -> tuple[float, float]:
    long_level = np.nan
    short_level = np.nan
    p1 = sig.get("prev_high_1", np.nan)
    p2 = sig.get("prev_high_2", np.nan)
    if _finite_positive(p1, p2) and float(p2) > float(p1):
        long_level = float(p2)
    p1 = sig.get("prev_low_1", np.nan)
    p2 = sig.get("prev_low_2", np.nan)
    if _finite_positive(p1, p2) and float(p2) < float(p1):
        short_level = float(p2)
    return long_level, short_level


def _trend_ready(sig: dict, side: str, params: dict) -> bool:
    if str(params["trend_mode"]).lower() == "none":
        return True
    if not _finite_positive(sig.get("ema20", np.nan)) or pd.isna(sig.get("ema20_slope", np.nan)):
        return False
    if side == "long":
        return float(sig["close"]) > float(sig["ema20"]) and float(sig["ema20_slope"]) > 0
    return float(sig["close"]) < float(sig["ema20"]) and float(sig["ema20_slope"]) < 0


def _bar_gate(sig: dict, side: str, params: dict) -> bool:
    atr = sig.get("atr", np.nan)
    bar_range = sig.get("range", np.nan)
    if not _finite_positive(atr, bar_range, sig.get("pre_range_6", np.nan)):
        return False
    atr_pct = sig.get("atr_percentile", np.nan)
    if pd.notna(atr_pct) and float(atr_pct) > float(params["max_atr_percentile"]):
        return False
    if float(sig["pre_range_6"]) > float(params["pre_range_atr"]) * float(atr):
        return False
    range_atr = float(bar_range) / float(atr)
    if range_atr < float(params["range_min_atr"]) or range_atr > float(params["range_max_atr"]):
        return False
    if float(sig.get("body_ratio", 0.0)) < float(params["body_min"]):
        return False
    close_pos = float(sig.get("close_pos", np.nan))
    if side == "long":
        if float(sig["close"]) <= float(sig["open"]) or close_pos < float(params["close_top"]):
            return False
    else:
        if float(sig["close"]) >= float(sig["open"]) or close_pos > 1.0 - float(params["close_top"]):
            return False
    return _trend_ready(sig, side, params)


def _micro_context(
    second_index: pd.DatetimeIndex,
    highs: np.ndarray,
    lows: np.ndarray,
    closes: np.ndarray,
    pos: int,
    side: str,
    atr: float,
    params: dict,
) -> dict:
    side_mult = 1.0 if side == "long" else -1.0
    lookback_time = second_index[pos] - pd.Timedelta(seconds=int(params["micro_window_seconds"]))
    fast_time = second_index[pos] - pd.Timedelta(seconds=int(params["micro_fast_seconds"]))
    lookback_pos = max(0, int(second_index.searchsorted(lookback_time, side="left")))
    fast_pos = max(0, int(second_index.searchsorted(fast_time, side="left")))
    entry_close = float(closes[pos])
    slow_move_atr = side_mult * (entry_close - float(closes[lookback_pos])) / atr
    fast_move_atr = side_mult * (entry_close - float(closes[fast_pos])) / atr
    window_high = float(np.max(highs[lookback_pos : pos + 1]))
    window_low = float(np.min(lows[lookback_pos : pos + 1]))
    window_range = max(window_high - window_low, 1e-12)
    efficiency = side_mult * (entry_close - float(closes[lookback_pos])) / window_range
    close_pos = (entry_close - window_low) / window_range if side == "long" else (window_high - entry_close) / window_range

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
        share = 0.0 if float(params["skip_weak"]) > 0 else float(params["weak_share"])

    return {
        "quality_tier": tier,
        "notional_share": share,
        "micro_speed_atr": float(slow_move_atr),
        "micro_fast_atr": float(fast_move_atr),
        "micro_efficiency": float(efficiency),
        "micro_close_pos": float(close_pos),
    }


def _signal_for_pos(
    signal_context: dict[pd.Timestamp, pd.Series],
    second_index: pd.DatetimeIndex,
    bar_high_so_far: np.ndarray,
    bar_low_so_far: np.ndarray,
    closes: np.ndarray,
    timeframe: str,
    pos: int,
) -> dict | None:
    bar_time = second_index[pos].floor(timeframe)
    base = signal_context.get(bar_time)
    if base is None or pd.isna(base.get("closed_atr", np.nan)):
        return None
    return _intrabar_signal(base, float(bar_high_so_far[pos]), float(bar_low_so_far[pos]), float(closes[pos]))


def _append_entry(
    state: dict,
    position: dict,
    *,
    bar_time: pd.Timestamp,
    raw_entry: float,
    side: str,
    signal_bar_time: pd.Timestamp,
    original_t2_level: float,
    donchian_level: float,
    donchian_gap_atr: float,
    micro_ctx: dict,
    variant_name: str,
    exit_policy_name: str,
) -> dict:
    entry_vs_confirm_bps = (
        (float(raw_entry) - donchian_level) / donchian_level
        if side == "long"
        else (donchian_level - float(raw_entry)) / donchian_level
    ) * 10000.0
    log = {
        "time": bar_time,
        "type": "BUY" if side == "long" else "SHORT",
        "price": position["entry_p"],
        "reason": "OriginalT2-Arm-DonchianConfirm",
        "notional": position["notional"],
        "notional_share": position["notional_share"],
        "bal": state["balance"],
        "breakout_shape_name": "original_t2_arm_donchian8_confirm",
        "breakout_level": float(donchian_level),
        "original_t2_level": float(original_t2_level),
        "donchian_level": float(donchian_level),
        "donchian_gap_atr": float(donchian_gap_atr),
        "observed_fill_raw": float(raw_entry),
        "signal_bar_time": signal_bar_time,
        "trade_slot": 0,
        "raw_exit_price": np.nan,
        "real_stop_price": position["sl"],
        "entry_vs_breakout_bps": entry_vs_confirm_bps,
        "quality_tier": micro_ctx["quality_tier"],
        "micro_speed_atr": micro_ctx["micro_speed_atr"],
        "micro_fast_atr": micro_ctx["micro_fast_atr"],
        "micro_efficiency": micro_ctx["micro_efficiency"],
        "micro_close_pos": micro_ctx["micro_close_pos"],
        "variant": variant_name,
        "exit_policy_name": exit_policy_name,
    }
    state["trade_logs"].append(log)
    return log


def _copy_entry_context_to_exit(exit_log: dict, entry_log: dict) -> None:
    for key in (
        "variant",
        "original_t2_level",
        "donchian_level",
        "donchian_gap_atr",
        "quality_tier",
        "micro_speed_atr",
        "micro_fast_atr",
        "micro_efficiency",
        "micro_close_pos",
        "exit_policy_name",
    ):
        exit_log[key] = entry_log.get(key, np.nan)


def run_variant(
    second_bars: pd.DataFrame,
    signal: pd.DataFrame,
    *,
    timeframe: str,
    variant_name: str,
    params: dict,
    exit_policy: dict,
    initial_balance: float,
) -> dict:
    started = time.time()
    second_index = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)
    grouped = second_bars.groupby(second_bars.index.floor(timeframe), sort=False)
    bar_high_so_far = grouped["high"].cummax().to_numpy(dtype="float64", copy=False)
    bar_low_so_far = grouped["low"].cummin().to_numpy(dtype="float64", copy=False)
    signal_context = {pd.Timestamp(idx): row for idx, row in signal.iterrows()}
    structure_events = pretouch._structure_events(signal, timeframe)
    offset = _freq_delta(timeframe)

    state = {
        "balance": float(initial_balance),
        "position": None,
        "trade_logs": [],
    }
    diagnostics = {
        "signal_bars": 0,
        "original_t2_armed_long": 0,
        "original_t2_armed_short": 0,
        "donchian_touches": 0,
        "bar_gate_skipped": 0,
        "micro_weak_skipped": 0,
        "entry_extension_skipped": 0,
        "min_gap_skipped": 0,
        "max_gap_skipped": 0,
        "bad_signal_skipped": 0,
        "entries": 0,
        "busy_skipped": 0,
        "long_entries": 0,
        "short_entries": 0,
        "quality_counts": {"weak": 0, "base": 0, "strong": 0},
    }
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")

    for signal_bar_time, base in signal.iloc[:-1].iterrows():
        signal_bar_time = pd.Timestamp(signal_bar_time)
        start_t = signal_bar_time
        end_t = signal_bar_time + offset
        if start_t <= last_exit_time:
            diagnostics["busy_skipped"] += 1
            continue
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="left"))
        if start_pos >= end_pos:
            continue
        if pd.isna(base.get("closed_atr", np.nan)):
            continue
        base_dict = base.to_dict()
        long_t2, short_t2 = _original_t2_levels(base_dict)
        long_confirm = float(base.get("prev_high_8", np.nan))
        short_confirm = float(base.get("prev_low_8", np.nan))
        has_long = np.isfinite(long_t2) and np.isfinite(long_confirm)
        has_short = np.isfinite(short_t2) and np.isfinite(short_confirm)
        if not has_long and not has_short:
            continue
        diagnostics["signal_bars"] += 1
        armed = {"long": False, "short": False}
        armed_level = {"long": np.nan, "short": np.nan}
        entry_log = None

        for pos in range(start_pos, end_pos):
            high_value = float(highs[pos])
            low_value = float(lows[pos])

            if has_long and not armed["long"] and high_value >= long_t2:
                armed["long"] = True
                armed_level["long"] = long_t2
                diagnostics["original_t2_armed_long"] += 1
            if has_short and not armed["short"] and low_value <= short_t2:
                armed["short"] = True
                armed_level["short"] = short_t2
                diagnostics["original_t2_armed_short"] += 1

            entry_side = ""
            confirm_level = np.nan
            original_t2_level = np.nan
            if armed["long"] and high_value >= long_confirm:
                entry_side = "long"
                confirm_level = long_confirm
                original_t2_level = float(armed_level["long"])
            if armed["short"] and low_value <= short_confirm:
                if entry_side:
                    continue
                entry_side = "short"
                confirm_level = short_confirm
                original_t2_level = float(armed_level["short"])
            if not entry_side:
                continue
            diagnostics["donchian_touches"] += 1
            sig = _signal_for_pos(signal_context, second_index, bar_high_so_far, bar_low_so_far, closes, timeframe, pos)
            if sig is None:
                diagnostics["bad_signal_skipped"] += 1
                continue

            atr = float(sig.get("atr", np.nan))
            if not np.isfinite(atr) or atr <= 0:
                continue
            donchian_gap_atr = (
                (confirm_level - original_t2_level) / atr
                if entry_side == "long"
                else (original_t2_level - confirm_level) / atr
            )
            if donchian_gap_atr < float(params["min_donchian_gap_atr"]):
                diagnostics["min_gap_skipped"] += 1
                continue
            if donchian_gap_atr > float(params["max_donchian_gap_atr"]):
                diagnostics["max_gap_skipped"] += 1
                continue
            if not _bar_gate(sig, entry_side, params):
                diagnostics["bar_gate_skipped"] += 1
                continue

            entry_pos = pos + 1
            if entry_pos >= end_pos or entry_pos >= len(second_index):
                continue
            entry_sig = _signal_for_pos(
                signal_context, second_index, bar_high_so_far, bar_low_so_far, closes, timeframe, entry_pos
            )
            if entry_sig is None:
                diagnostics["bad_signal_skipped"] += 1
                continue
            raw_entry = float(closes[entry_pos])
            extension = (
                (raw_entry - confirm_level) / atr if entry_side == "long" else (confirm_level - raw_entry) / atr
            )
            if extension > float(params["max_entry_extension_atr"]):
                diagnostics["entry_extension_skipped"] += 1
                continue
            micro_ctx = _micro_context(second_index, highs, lows, closes, entry_pos, entry_side, atr, params)
            diagnostics["quality_counts"][micro_ctx["quality_tier"]] += 1
            if micro_ctx["notional_share"] <= 0:
                diagnostics["micro_weak_skipped"] += 1
                continue

            state["balance"], state["position"] = direct._open_direct_position(
                state["balance"],
                entry_sig,
                side=entry_side,
                fill_raw=raw_entry,
                notional_share=float(micro_ctx["notional_share"]),
                shape_name="original_t2_arm_donchian8_confirm",
                breakout_level=confirm_level,
                signal_bar_time=signal_bar_time,
                trade_slot=0,
                exit_policy=exit_policy,
            )
            position = state["position"]
            entry_log = _append_entry(
                state,
                position,
                bar_time=second_index[entry_pos],
                raw_entry=raw_entry,
                side=entry_side,
                signal_bar_time=signal_bar_time,
                original_t2_level=original_t2_level,
                donchian_level=confirm_level,
                donchian_gap_atr=donchian_gap_atr,
                micro_ctx=micro_ctx,
                variant_name=variant_name,
                exit_policy_name=str(exit_policy.get("name", "")),
            )
            diagnostics["entries"] += 1
            diagnostics[f"{entry_side}_entries"] += 1

            exit_pos = None
            for exit_scan_pos in range(entry_pos + 1, len(second_index)):
                exit_sig = _signal_for_pos(
                    signal_context,
                    second_index,
                    bar_high_so_far,
                    bar_low_so_far,
                    closes,
                    timeframe,
                    exit_scan_pos,
                )
                if exit_sig is None:
                    continue
                exit_triggered, raw_exit_price, reason = direct._advance_position(
                    state["position"],
                    exit_sig,
                    float(highs[exit_scan_pos]),
                    float(lows[exit_scan_pos]),
                )
                if exit_triggered:
                    if state["position"].get("structure_stop_active", False) and reason == "InitialSL":
                        reason = "StructureSL"
                    before = len(state["trade_logs"])
                    direct._append_exit(
                        state,
                        state["position"],
                        bar_time=second_index[exit_scan_pos],
                        raw_exit_price=raw_exit_price,
                        reason=reason,
                    )
                    if len(state["trade_logs"]) > before and entry_log is not None:
                        _copy_entry_context_to_exit(state["trade_logs"][-1], entry_log)
                    state["position"] = None
                    exit_pos = exit_scan_pos
                    last_exit_time = pd.Timestamp(second_index[exit_scan_pos])
                    break
                event = structure_events.get(second_index[exit_scan_pos])
                if event is not None:
                    pretouch._apply_structure_exit_policy(state["position"], event)
            if exit_pos is None:
                break
            break

    if state["position"] is not None and len(closes) > 0:
        before = len(state["trade_logs"])
        direct._append_exit(
            state,
            state["position"],
            bar_time=second_index[-1],
            raw_exit_price=float(closes[-1]),
            reason="FinalMarkToMarket",
        )
        if len(state["trade_logs"]) > before:
            state["trade_logs"][-1]["variant"] = variant_name
        state["position"] = None

    ledger = pd.DataFrame(state["trade_logs"])
    pairs = direct._paired_trades(ledger)
    return {
        "timeframe": timeframe,
        "variant": variant_name,
        "params": params,
        "exit_policy": exit_policy,
        "summary": replay.summarize_run(ledger, initial_balance),
        "pair_diagnostics": direct._pair_diagnostics(pairs),
        "entry_distance_diagnostics": direct._entry_distance_diagnostics(ledger),
        "exit_hold_diagnostics": direct._exit_hold_diagnostics(pairs),
        "mfe_mae_diagnostics": direct._mfe_mae_diagnostics(ledger),
        "accounting_2bps_maker_entry_market_exit": direct._accounting_from_ledger(
            ledger,
            initial_balance=initial_balance,
            slippage=float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"]),
            entry_fee=0.0002,
            exit_fee=0.0004,
        ),
        "diagnostics": diagnostics,
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# original_t2 arm + Donchian confirm 回测（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。`original_t2` 只负责在当前 signal bar 内 armed；真实开仓等同一根 bar 触碰 `prev_high_8/prev_low_8` 后，以下一根 `1s close` 市价成交。`prev_high_8/prev_low_8` 是 Donchian-style confirm level，不是 baseline 结构语义。",
        "",
        "成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。",
        "",
        "| Symbol | Variant | Exit | Trades | Realistic | Raw | 2bps Slip No Fee | Fees | Win | Max DD | Avg Hold | Exits | Entries | Touches | BarGateSkip | WeakSkip | Quality |",
        "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---|",
    ]
    for result in summary["results"]:
        acct = result["accounting_2bps_maker_entry_market_exit"]
        s = result["summary"]
        d = result["diagnostics"]
        p = result["pair_diagnostics"]
        slot0 = result.get("trade_slot_diagnostics", {}).get("0", {})
        win = float(slot0.get("win_rate_pct", s.get("win_rate_pct", 0.0)))
        lines.append(
            f"| `{result['symbol']}` | `{result['variant']}` | `{result['exit_policy'].get('name', '')}` | "
            f"{acct['round_trips']} | {acct['realistic_return_pct']:.4f}% | "
            f"{acct['raw_no_fee_no_slip_return_pct']:.4f}% | "
            f"{acct['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{acct['realistic_fees_pct']:.4f}% | {win:.2f}% | {s['max_dd_pct']:.2f}% | "
            f"{p.get('avg_hold_seconds', 0.0):.2f}s | `{s['exit_reasons']}` | "
            f"{d['entries']} | {d['donchian_touches']} | {d['bar_gate_skipped']} | "
            f"{d['micro_weak_skipped']} | `{d['quality_counts']}` |"
        )
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['symbol']} {result['variant']} {result['exit_policy'].get('name', '')}` ledger：`{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="original_t2 arm + Donchian confirm entry replay")
    parser.add_argument("--symbols", nargs="+", default=["ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--stop-loss-atr", type=float, default=0.30)
    parser.add_argument("--variants", nargs="+", default=["s10b4", "s10b4_notrend", "b55_loose"])
    parser.add_argument("--exit-variants", nargs="+", default=["baseline", "trail0p5_act1p0", "structure1p0_b4"])
    parser.add_argument("--summary-json", default="research/original_t2_arm_donchian_confirm_entry_summary.json")
    parser.add_argument("--markdown", default="research/20260508_original_t2_arm_donchian_confirm_entry.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_original_t2_arm_donchian_confirm_entry")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = micro.base._as_utc(args.start)
    end = micro.base._as_utc(args.end)
    replay.COMMON_REPLAY_KWARGS.update(direct.BTC_LIVE_LIKE_REPLAY_KWARGS)
    replay.COMMON_REPLAY_KWARGS.update(
        {
            "fixed_slippage": 0.0002,
            "commission": 0.0,
            "stop_loss_atr": float(args.stop_loss_atr),
            "stop_mode": "atr",
            "max_trades_per_bar": 1,
            "reentry_size_schedule": [0.20],
        }
    )
    variants = [parse_variant(raw) for raw in args.variants]
    exit_policies = pretouch._parse_exit_variants(args.exit_variants)
    results = []

    for symbol in args.symbols:
        second_bars, build_stats = micro.load_or_build_second_bars(
            symbol,
            start,
            end,
            Path(args.archive_root),
            args.chunksize,
            Path(args.cache_root),
            not args.no_cache,
        )
        signal = build_live_signal_frame(second_bars, args.timeframe)
        print(f"{symbol}: second_rows={len(second_bars)} signal_rows={len(signal)}", flush=True)
        for variant_name, params in variants:
            for exit_policy in exit_policies:
                policy_name = str(exit_policy.get("name", "baseline"))
                result_name = variant_name if policy_name == "baseline" else f"{variant_name}_{policy_name}"
                print(f"running {symbol} {result_name}", flush=True)
                result = run_variant(
                    second_bars,
                    signal,
                    timeframe=args.timeframe,
                    variant_name=result_name,
                    params=params,
                    exit_policy=exit_policy,
                    initial_balance=float(args.initial_balance),
                )
                ledger_path = Path(f"{args.ledger_prefix}_{symbol}_{args.timeframe}_{result_name}_ledger.csv")
                result["ledger"].to_csv(ledger_path, index=False)
                del result["ledger"]
                result.update(
                    {
                        "symbol": symbol,
                        "ledger_path": str(ledger_path),
                        "build_stats": build_stats,
                    }
                )
                acct = result["accounting_2bps_maker_entry_market_exit"]
                print(
                    f"{symbol} {result_name}: realistic={acct['realistic_return_pct']:.4f}% "
                    f"raw={acct['raw_no_fee_no_slip_return_pct']:.4f}% trades={acct['round_trips']} "
                    f"diag={result['diagnostics']}",
                    flush=True,
                )
                results.append(result)

    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "timeframe": args.timeframe,
        "mode": {
            "arm": "true original_t2 intrabar touch of prev_high_2/prev_low_2",
            "confirm": "same signal bar intrabar touch of prev_high_8/prev_low_8",
            "entry": "next 1s close after confirm touch",
            "scope": "research-only",
        },
        "cost_model": {
            "slippage_bps_per_side": 2.0,
            "entry_fee_bps": 2.0,
            "exit_fee_bps": 4.0,
        },
        "stop_loss_atr": float(args.stop_loss_atr),
        "results": results,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {"summary_path": args.summary_json, "markdown_path": args.markdown, "elapsed_seconds": summary["elapsed_seconds"]},
            indent=2,
            ensure_ascii=False,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
