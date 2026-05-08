#!/usr/bin/env python3
"""BTC/ETH Jan-Apr 2026 impulse bar-run follower research replay.

Research-only runner. Signals are closed 1h bars; execution uses continuous 1s
OHLC bars built from local Binance trade ticks. The strategy tests whether a
strong impulse bar after local compression can capture the next 5-6 bars of a
local trend run.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


def _as_utc(value: str) -> pd.Timestamp:
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        return ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def monthly_trade_files(symbol: str, start: pd.Timestamp, end: pd.Timestamp, archive_root: Path) -> list[str]:
    months = pd.period_range(
        start=start.tz_convert(None).to_period("M"),
        end=end.tz_convert(None).to_period("M"),
        freq="M",
    )
    paths = []
    missing = []
    for month in months:
        ym = str(month)
        base_dir = archive_root / f"{symbol}-trades-{ym}"
        stem = f"{symbol}-trades-{ym}"
        zip_path = base_dir / f"{stem}.zip"
        csv_path = base_dir / f"{stem}.csv"
        if zip_path.exists():
            paths.append(str(zip_path))
        elif csv_path.exists():
            paths.append(str(csv_path))
        else:
            missing.append(str(zip_path))
    if missing:
        raise FileNotFoundError("missing monthly trade archives:\n" + "\n".join(missing))
    return paths


def _last_percentile(values: np.ndarray) -> float:
    clean = values[~np.isnan(values)]
    if len(clean) == 0:
        return np.nan
    return float((clean <= clean[-1]).mean() * 100.0)


def _true_range(frame: pd.DataFrame) -> pd.Series:
    return pd.concat(
        [
            frame["high"] - frame["low"],
            (frame["high"] - frame["close"].shift()).abs(),
            (frame["low"] - frame["close"].shift()).abs(),
        ],
        axis=1,
    ).max(axis=1)


def _freq_to_timedelta(freq: str) -> pd.Timedelta:
    return pd.to_timedelta(pd.tseries.frequencies.to_offset(freq).nanos, unit="ns")


def build_frames(
    second_bars: pd.DataFrame,
    *,
    signal_timeframe: str,
    trend_timeframe: str,
) -> tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]:
    one_min = second_bars.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    signal = one_min.resample(signal_timeframe).agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    trend = one_min.resample(trend_timeframe).agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()

    tr = _true_range(signal)
    signal["atr"] = tr.rolling(14).mean()
    signal["atr_percentile"] = signal["atr"].rolling(240, min_periods=60).apply(_last_percentile, raw=True)
    signal["ema8"] = signal["close"].ewm(span=8, adjust=False, min_periods=8).mean()
    signal["ema20"] = signal["close"].ewm(span=20, adjust=False, min_periods=20).mean()
    signal["ema20_slope"] = signal["ema20"] - signal["ema20"].shift(1)
    signal["body"] = (signal["close"] - signal["open"]).abs()
    signal["range"] = signal["high"] - signal["low"]
    signal["body_ratio"] = signal["body"] / signal["range"].replace(0, np.nan)
    signal["close_pos"] = (signal["close"] - signal["low"]) / signal["range"].replace(0, np.nan)
    signal["pre_high_6"] = signal["high"].shift(1).rolling(6).max()
    signal["pre_low_6"] = signal["low"].shift(1).rolling(6).min()
    signal["pre_range_6"] = signal["pre_high_6"] - signal["pre_low_6"]
    for lookback in (8, 12):
        signal[f"prev_high_{lookback}"] = signal["high"].shift(1).rolling(lookback).max()
        signal[f"prev_low_{lookback}"] = signal["low"].shift(1).rolling(lookback).min()

    trend["ema20"] = trend["close"].ewm(span=20, adjust=False, min_periods=20).mean()
    trend["ema20_slope"] = trend["ema20"] - trend["ema20"].shift(1)
    trend_context = trend[["close", "ema20", "ema20_slope"]].copy()
    trend_context.columns = ["trend_close", "trend_ema20", "trend_ema20_slope"]
    trend_context.index = trend_context.index + _freq_to_timedelta(trend_timeframe)

    prepared = signal.reset_index(names="bar_start")
    prepared["bar_end"] = prepared["bar_start"] + _freq_to_timedelta(signal_timeframe)
    prepared = pd.merge_asof(
        prepared.sort_values("bar_end"),
        trend_context.sort_index(),
        left_on="bar_end",
        right_index=True,
        direction="backward",
    )
    prepared.set_index("bar_start", inplace=True)
    return one_min, prepared, trend


def parse_variant(raw: str) -> tuple[str, dict]:
    presets = {
        "break8_body55": {"break_lookback": 8, "body_min": 0.55},
        "break12_body55": {"break_lookback": 12, "body_min": 0.55},
        "break8_body65": {"break_lookback": 8, "body_min": 0.65},
    }
    params = {
        "break_lookback": 8,
        "body_min": 0.55,
        "close_top": 0.75,
        "range_min_atr": 0.80,
        "range_max_atr": 2.20,
        "pre_range_atr": 3.00,
        "max_atr_percentile": 95.0,
        "max_entry_extension_atr": 0.15,
        "confirm_atr": 0.0,
        "confirm_window_seconds": 0,
        "fail_retrace_atr": 0.0,
        "initial_stop_atr": 0.45,
        "stop_buffer_atr": 0.05,
        "stop_cap_atr": 0.80,
        "min_stop_bps": 12.0,
        "breakeven_at_r": 1.0,
        "cost_lock_bps": 10.0,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_bars": 6,
        "notional_share": 0.20,
        "slippage": 0.0002,
        "entry_fee": 0.0002,
        "exit_fee": 0.0004,
    }
    if raw in presets:
        params.update(presets[raw])
        return raw, params

    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    if name in presets:
        params.update(presets[name])
    key_map = {
        "break": "break_lookback",
        "body": "body_min",
        "close_top": "close_top",
        "range_min": "range_min_atr",
        "range_max": "range_max_atr",
        "pre_range": "pre_range_atr",
        "max_ext": "max_entry_extension_atr",
        "confirm": "confirm_atr",
        "window": "confirm_window_seconds",
        "fail": "fail_retrace_atr",
        "hold": "max_hold_bars",
        "trail_start": "trail_start_r",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = float(value)
    params["break_lookback"] = int(params["break_lookback"])
    params["max_hold_bars"] = int(params["max_hold_bars"])
    params["confirm_window_seconds"] = int(params["confirm_window_seconds"])
    return name, params


def _finite_positive(*values: float) -> bool:
    return all(pd.notna(v) and np.isfinite(float(v)) and float(v) > 0 for v in values)


def _trend_ready(sig: pd.Series, side: str) -> bool:
    if not _finite_positive(sig.get("ema20", np.nan), sig.get("trend_ema20", np.nan)):
        return False
    if side == "long":
        return (
            float(sig["close"]) > float(sig["ema20"])
            and float(sig["trend_close"]) > float(sig["trend_ema20"])
            and float(sig["ema20_slope"]) > 0
            and float(sig["trend_ema20_slope"]) > 0
        )
    return (
        float(sig["close"]) < float(sig["ema20"])
        and float(sig["trend_close"]) < float(sig["trend_ema20"])
        and float(sig["ema20_slope"]) < 0
        and float(sig["trend_ema20_slope"]) < 0
    )


def signal_side(sig: pd.Series, params: dict) -> str:
    atr = sig.get("atr", np.nan)
    bar_range = sig.get("range", np.nan)
    if not _finite_positive(atr, bar_range, sig.get("pre_range_6", np.nan)):
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
        _trend_ready(sig, "long")
        and float(sig["close"]) > float(sig[f"prev_high_{lookback}"])
        and float(sig["close_pos"]) >= float(params["close_top"])
    ):
        return "long"
    if (
        _trend_ready(sig, "short")
        and float(sig["close"]) < float(sig[f"prev_low_{lookback}"])
        and float(sig["close_pos"]) <= 1.0 - float(params["close_top"])
    ):
        return "short"
    return ""


def open_position(sig: pd.Series, side: str, entry_raw: float, entry_time, balance: float, params: dict) -> dict | None:
    atr = float(sig["atr"])
    slippage = float(params["slippage"])
    entry_p = float(entry_raw) * (1.0 + slippage if side == "long" else 1.0 - slippage)
    if side == "long":
        raw_stop = min(float(sig["low"]) - params["stop_buffer_atr"] * atr, entry_p - params["initial_stop_atr"] * atr)
        capped_stop = entry_p - params["stop_cap_atr"] * atr
        stop = max(raw_stop, capped_stop)
        risk = entry_p - stop
    else:
        raw_stop = max(float(sig["high"]) + params["stop_buffer_atr"] * atr, entry_p + params["initial_stop_atr"] * atr)
        capped_stop = entry_p + params["stop_cap_atr"] * atr
        stop = min(raw_stop, capped_stop)
        risk = stop - entry_p
    if risk <= 0 or risk < entry_p * float(params["min_stop_bps"]) / 10000.0:
        return None
    return {
        "side": side,
        "entry_time": entry_time,
        "entry_p": entry_p,
        "entry_raw": float(entry_raw),
        "sl": stop,
        "initial_sl": stop,
        "risk": risk,
        "atr_at_entry": atr,
        "notional": balance * float(params["notional_share"]),
        "notional_share": float(params["notional_share"]),
        "signal_bar_time": sig.name,
        "signal_close": float(sig["close"]),
        "signal_high": float(sig["high"]),
        "signal_low": float(sig["low"]),
        "protected": False,
        "trailing_active": False,
        "hwm": entry_p,
        "lwm": entry_p,
        "highest_hour_high": float(sig["high"]),
        "lowest_hour_low": float(sig["low"]),
        "no_new_extreme_bars": 0,
        "mfe_r": 0.0,
        "mae_r": 0.0,
    }


def append_entry(logs: list[dict], position: dict, balance: float) -> None:
    logs.append(
        {
            "time": position["entry_time"],
            "type": "BUY" if position["side"] == "long" else "SHORT",
            "price": position["entry_p"],
            "raw_price": position["entry_raw"],
            "reason": "Impulse-BarRun",
            "notional": position["notional"],
            "notional_share": position["notional_share"],
            "bal": balance,
            "signal_bar_time": position["signal_bar_time"],
            "signal_close": position["signal_close"],
            "risk": position["risk"],
            "mfe_r": np.nan,
            "mae_r": np.nan,
        }
    )


def append_exit(
    logs: list[dict],
    position: dict,
    *,
    raw_exit: float,
    time_value,
    reason: str,
    balance: float,
    params: dict,
) -> float:
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
            "risk": position["risk"],
            "mfe_r": float(position["mfe_r"]),
            "mae_r": float(position["mae_r"]),
            "stop_price": float(position["sl"]),
        }
    )
    return new_balance


def update_excursion(position: dict, high_value: float, low_value: float, params: dict) -> None:
    entry = float(position["entry_p"])
    risk = float(position["risk"])
    if position["side"] == "long":
        position["hwm"] = max(float(position["hwm"]), float(high_value))
        favorable = max(0.0, float(position["hwm"]) - entry)
        adverse = max(0.0, entry - float(low_value))
        if favorable / risk >= float(params["breakeven_at_r"]):
            be_sl = entry * (1.0 + float(params["cost_lock_bps"]) / 10000.0)
            if be_sl > float(position["sl"]):
                position["sl"] = be_sl
                position["protected"] = True
    else:
        position["lwm"] = min(float(position["lwm"]), float(low_value))
        favorable = max(0.0, entry - float(position["lwm"]))
        adverse = max(0.0, float(high_value) - entry)
        if favorable / risk >= float(params["breakeven_at_r"]):
            be_sl = entry * (1.0 - float(params["cost_lock_bps"]) / 10000.0)
            if be_sl < float(position["sl"]):
                position["sl"] = be_sl
                position["protected"] = True
    position["mfe_r"] = max(float(position["mfe_r"]), favorable / risk)
    position["mae_r"] = max(float(position["mae_r"]), adverse / risk)


def apply_hourly_event(position: dict, event: pd.Series, params: dict) -> tuple[bool, str]:
    if position["side"] == "long":
        if float(position["mfe_r"]) >= float(params["trail_start_r"]):
            trail = float(event["low"]) - float(params["trail_buffer_atr"]) * float(event["atr"])
            if trail > float(position["sl"]):
                position["sl"] = trail
                position["trailing_active"] = True
        if pd.notna(event.get("ema8", np.nan)) and float(event["close"]) < float(event["ema8"]):
            return True, "EMA8Exit"
        if float(event["high"]) > float(position["highest_hour_high"]):
            position["highest_hour_high"] = float(event["high"])
            position["no_new_extreme_bars"] = 0
        else:
            position["no_new_extreme_bars"] += 1
        if int(position["no_new_extreme_bars"]) >= 2:
            return True, "NoNewHighExit"
    else:
        if float(position["mfe_r"]) >= float(params["trail_start_r"]):
            trail = float(event["high"]) + float(params["trail_buffer_atr"]) * float(event["atr"])
            if trail < float(position["sl"]):
                position["sl"] = trail
                position["trailing_active"] = True
        if pd.notna(event.get("ema8", np.nan)) and float(event["close"]) > float(event["ema8"]):
            return True, "EMA8Exit"
        if float(event["low"]) < float(position["lowest_hour_low"]):
            position["lowest_hour_low"] = float(event["low"])
            position["no_new_extreme_bars"] = 0
        else:
            position["no_new_extreme_bars"] += 1
        if int(position["no_new_extreme_bars"]) >= 2:
            return True, "NoNewLowExit"
    return False, ""


def stop_trigger(position: dict, high_value: float, low_value: float) -> tuple[bool, float, str]:
    if position["side"] == "long":
        if low_value <= float(position["sl"]):
            if position["trailing_active"]:
                return True, float(position["sl"]), "TrailingSL"
            if position["protected"]:
                return True, float(position["sl"]), "BreakevenSL"
            return True, float(position["sl"]), "InitialSL"
    else:
        if high_value >= float(position["sl"]):
            if position["trailing_active"]:
                return True, float(position["sl"]), "TrailingSL"
            if position["protected"]:
                return True, float(position["sl"]), "BreakevenSL"
            return True, float(position["sl"]), "InitialSL"
    return False, 0.0, ""


def simulate_position(
    second_bars: pd.DataFrame,
    events_by_end: dict[pd.Timestamp, pd.Series],
    sig: pd.Series,
    side: str,
    params: dict,
    *,
    start_pos: int,
    balance: float,
):
    second_index = second_bars.index
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)
    atr = float(sig["atr"])
    confirm_atr = float(params.get("confirm_atr", 0.0))
    confirm_window_seconds = int(params.get("confirm_window_seconds", 0))
    fail_retrace_atr = float(params.get("fail_retrace_atr", 0.0))

    if confirm_window_seconds > 0 and confirm_atr > 0:
        window_end = second_index[start_pos] + pd.Timedelta(seconds=confirm_window_seconds)
        end_pos = min(int(second_index.searchsorted(window_end, side="right")), len(second_index))
        if side == "long":
            confirm_level = float(sig["close"]) + confirm_atr * atr
            fail_level = float(sig["close"]) - fail_retrace_atr * atr if fail_retrace_atr > 0 else -np.inf
            entry_pos = None
            for pos in range(start_pos, end_pos):
                if float(low_values[pos]) <= fail_level:
                    return None, balance, "early_reversal"
                if float(high_values[pos]) >= confirm_level:
                    entry_pos = pos
                    break
        else:
            confirm_level = float(sig["close"]) - confirm_atr * atr
            fail_level = float(sig["close"]) + fail_retrace_atr * atr if fail_retrace_atr > 0 else np.inf
            entry_pos = None
            for pos in range(start_pos, end_pos):
                if float(high_values[pos]) >= fail_level:
                    return None, balance, "early_reversal"
                if float(low_values[pos]) <= confirm_level:
                    entry_pos = pos
                    break
        if entry_pos is None:
            return None, balance, "confirm_timeout"
        start_pos = entry_pos

    entry_time = second_index[start_pos]
    entry_raw = float(close_values[start_pos])
    if side == "long":
        if entry_raw - float(sig["close"]) > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension"
    else:
        if float(sig["close"]) - entry_raw > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension"

    position = open_position(sig, side, entry_raw, entry_time, balance, params)
    if position is None:
        return None, balance, "min_stop"
    logs: list[dict] = []
    append_entry(logs, position, balance)

    max_hold_end = entry_time + pd.Timedelta(hours=int(params["max_hold_bars"]))
    end_pos = min(int(second_index.searchsorted(max_hold_end, side="left")), len(second_index) - 1)

    for pos in range(start_pos + 1, end_pos + 1):
        bar_time = second_index[pos]
        high_value = float(high_values[pos])
        low_value = float(low_values[pos])
        close_value = float(close_values[pos])
        update_excursion(position, high_value, low_value, params)

        triggered, raw_exit, reason = stop_trigger(position, high_value, low_value)
        if triggered:
            balance = append_exit(
                logs,
                position,
                raw_exit=raw_exit,
                time_value=bar_time,
                reason=reason,
                balance=balance,
                params=params,
            )
            return logs, balance, ""

        event = events_by_end.get(bar_time)
        if event is not None and bar_time > entry_time:
            exit_now, event_reason = apply_hourly_event(position, event, params)
            if exit_now:
                balance = append_exit(
                    logs,
                    position,
                    raw_exit=close_value,
                    time_value=bar_time,
                    reason=event_reason,
                    balance=balance,
                    params=params,
                )
                return logs, balance, ""

        if bar_time >= max_hold_end:
            balance = append_exit(
                logs,
                position,
                raw_exit=close_value,
                time_value=bar_time,
                reason="MaxHoldExit",
                balance=balance,
                params=params,
            )
            return logs, balance, ""

    balance = append_exit(
        logs,
        position,
        raw_exit=float(close_values[end_pos]),
        time_value=second_index[end_pos],
        reason="FinalMarkToMarket",
        balance=balance,
        params=params,
    )
    return logs, balance, ""


def run_strategy(second_bars: pd.DataFrame, signal: pd.DataFrame, params: dict, *, initial_balance: float) -> dict:
    second_index = second_bars.index
    events_by_end = {pd.Timestamp(row["bar_end"]): row for _, row in signal.iterrows()}
    balance = float(initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    diagnostics = {
        "candidate_signals": 0,
        "entries": 0,
        "busy_skipped": 0,
        "entry_extension_skipped": 0,
        "confirm_timeout_skipped": 0,
        "early_reversal_skipped": 0,
        "min_stop_skipped": 0,
        "long_signals": 0,
        "short_signals": 0,
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
        start_pos = int(second_index.searchsorted(entry_time, side="left"))
        if start_pos >= len(second_index):
            continue
        trade_logs, new_balance, skip_reason = simulate_position(
            second_bars,
            events_by_end,
            sig,
            side,
            params,
            start_pos=start_pos,
            balance=balance,
        )
        if skip_reason == "entry_extension":
            diagnostics["entry_extension_skipped"] += 1
            continue
        if skip_reason == "confirm_timeout":
            diagnostics["confirm_timeout_skipped"] += 1
            continue
        if skip_reason == "early_reversal":
            diagnostics["early_reversal_skipped"] += 1
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
                "hold_seconds": (pd.Timestamp(row["time"]) - pd.Timestamp(entry["time"])).total_seconds(),
                "mfe_r": float(row.get("mfe_r", 0.0)),
                "mae_r": float(row.get("mae_r", 0.0)),
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
        }

    raw_balance = float(initial_balance)
    slip_balance = float(initial_balance)
    realistic_balance = float(initial_balance)
    fee_rate = float(params["entry_fee"]) + float(params["exit_fee"])
    equity = [realistic_balance]
    for _, pair in pairs.iterrows():
        share = float(pair["notional_share"])
        raw_notional = raw_balance * share
        raw_balance += raw_notional * float(pair["raw_pnl_pct"])
        slip_notional = slip_balance * share
        slip_balance += slip_notional * float(pair["slip_pnl_pct"])
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
        "avg_pnl_pct": round(float(pairs["slip_pnl_pct"].mean()) * 100.0, 4),
        "median_pnl_pct": round(float(pairs["slip_pnl_pct"].median()) * 100.0, 4),
        "median_hold_seconds": round(float(pairs["hold_seconds"].median()), 2),
        "avg_hold_seconds": round(float(pairs["hold_seconds"].mean()), 2),
        "median_mfe_r": round(float(pairs["mfe_r"].median()), 4),
        "median_mae_r": round(float(pairs["mae_r"].median()), 4),
        "exit_reasons": {str(k): int(v) for k, v in pairs["exit_reason"].value_counts().items()},
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        f"# BTC/ETH Impulse Bar-Run 1s Replay ({summary['start']} to {summary['end']})",
        "",
        f"Scope: research-only. Signals use closed {summary['signal_timeframe']} bars with {summary['trend_timeframe']} EMA20 trend context; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.",
        "",
        "| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Confirm Miss | Early Rev | Busy Skip |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["diagnostics"]
        lines.append(
            f"| `{result['symbol']}` | `{result['variant']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | "
            f"{s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | {s.get('median_hold_seconds', 0.0):.2f}s | "
            f"{s.get('median_mfe_r', 0.0):.4f} | `{s['exit_reasons']}` | {d['candidate_signals']} | "
            f"{d['entries']} | {d['entry_extension_skipped']} | {d.get('confirm_timeout_skipped', 0)} | "
            f"{d.get('early_reversal_skipped', 0)} | {d['busy_skipped']} |"
        )
    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['symbol']} {result['variant']}` ledger: `{result['ledger_path']}`")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Impulse bar-run follower 1s replay")
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--trend-timeframe", default="4h")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument(
        "--variants",
        nargs="+",
        default=["break8_body55", "break12_body55", "break8_body65"],
    )
    parser.add_argument("--summary-json", default="research/btc_eth_2026_jan_apr_impulse_bar_run_summary.json")
    parser.add_argument("--markdown", default="research/20260507_btc_eth_2026_jan_apr_impulse_bar_run.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_eth_2026_jan_apr_impulse_bar_run")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = _as_utc(args.start)
    end = _as_utc(args.end)
    variants = [parse_variant(raw) for raw in args.variants]
    results = []

    for symbol in args.symbols:
        tick_files = monthly_trade_files(symbol, start, end, Path(args.archive_root))
        second_bars, build_stats = replay.build_continuous_second_bars(tick_files, start, end, args.chunksize)
        _, signal, trend = build_frames(
            second_bars,
            signal_timeframe=args.signal_timeframe,
            trend_timeframe=args.trend_timeframe,
        )
        print(
            f"{symbol}: second_rows={len(second_bars)} signal_rows={len(signal)} trend_rows={len(trend)} "
            f"signal_timeframe={args.signal_timeframe} trend_timeframe={args.trend_timeframe}",
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
                f"win={s['win_rate_pct']:.2f}% dd={s['max_dd_pct']:.4f}% diag={result['diagnostics']}",
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
        "signal": f"closed {args.signal_timeframe} impulse bars with {args.trend_timeframe} EMA20 trend context",
        "execution": "continuous 1s OHLC bars from local trade ticks",
        "accounting": "2bps/side slippage in ledger plus maker entry 2bps and market exit 4bps in realistic accounting",
        "variants": [{"name": name, "params": params} for name, params in variants],
        "results": results,
        "summary_path": str(summary_path),
        "markdown_path": str(markdown_path),
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, markdown_path)
    print(
        json.dumps(
            {
                "summary_path": str(summary_path),
                "markdown_path": str(markdown_path),
                "elapsed_seconds": summary["elapsed_seconds"],
            },
            indent=2,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
