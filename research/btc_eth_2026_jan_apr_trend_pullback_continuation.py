#!/usr/bin/env python3
"""BTC/ETH Jan-Apr 2026 trend-pullback-continuation research replay.

Research-only runner. It keeps execution on continuous 1s bars and tests a
non-chasing breakout idea:

1. A 4h structural breakout only arms a setup.
2. Entry waits for a post-breakout pullback near the breakout level or 4h EMA20.
3. Real exposure opens only when price reclaims the pullback high/low by an ATR
   buffer, filled at the triggering 1s close.
4. Exits use pullback-anchored initial SL, 1R cost lock, and trailing.
"""

from __future__ import annotations

import argparse
import json
import math
import time
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay


DEFAULT_FILES = {
    "BTCUSDT": [
        "dataset/archive/BTCUSDT-trades-2026-01/BTCUSDT-trades-2026-01.csv",
        "dataset/archive/BTCUSDT-trades-2026-02/BTCUSDT-trades-2026-02.csv",
        "dataset/archive/BTCUSDT-trades-2026-03/BTCUSDT-trades-2026-03.csv",
        "dataset/archive/BTCUSDT-trades-2026-04/BTCUSDT-trades-2026-04.zip",
    ],
    "ETHUSDT": [
        "dataset/archive/ETHUSDT-trades-2026-01/ETHUSDT-trades-2026-01.csv",
        "dataset/archive/ETHUSDT-trades-2026-02/ETHUSDT-trades-2026-02.zip",
        "dataset/archive/ETHUSDT-trades-2026-03/ETHUSDT-trades-2026-03.zip",
        "dataset/archive/ETHUSDT-trades-2026-04/ETHUSDT-trades-2026-04.zip",
    ],
}


def _as_utc(value: str) -> pd.Timestamp:
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        return ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def _add_right_boundary(signal: pd.DataFrame, timeframe: str) -> pd.DataFrame:
    if signal.empty:
        return signal
    offset = pd.tseries.frequencies.to_offset(timeframe)
    boundary = signal.iloc[[-1]].copy()
    boundary.index = pd.DatetimeIndex([signal.index[-1] + offset])
    return pd.concat([signal, boundary])


def _last_percentile(values: np.ndarray) -> float:
    clean = values[~np.isnan(values)]
    if len(clean) == 0:
        return np.nan
    return float((clean <= clean[-1]).mean() * 100.0)


def build_frames(second_bars: pd.DataFrame, signal_timeframe: str) -> tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]:
    one_min = second_bars.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    signal = one_min.resample(signal_timeframe).agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    daily = one_min.resample("1d").agg(
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
    signal["atr"] = true_range.rolling(14).mean()
    signal["atr_percentile"] = signal["atr"].rolling(90, min_periods=30).apply(_last_percentile, raw=True)
    signal["ema20"] = signal["close"].ewm(span=20, adjust=False, min_periods=20).mean()
    signal["ema50"] = signal["close"].ewm(span=50, adjust=False, min_periods=50).mean()
    signal["ema20_slope"] = signal["ema20"] - signal["ema20"].shift(1)
    signal["prev_high_12"] = signal["high"].rolling(12).max().shift(1)
    signal["prev_low_12"] = signal["low"].rolling(12).min().shift(1)

    daily["ema20"] = daily["close"].ewm(span=20, adjust=False, min_periods=20).mean()
    daily["ema50"] = daily["close"].ewm(span=50, adjust=False, min_periods=50).mean()
    daily["ema20_slope"] = daily["ema20"] - daily["ema20"].shift(1)
    daily_context = daily[["close", "ema20", "ema50", "ema20_slope"]].copy()
    daily_context.columns = ["d_close", "d_ema20", "d_ema50", "d_ema20_slope"]
    signal = pd.merge_asof(
        signal.sort_index(),
        daily_context.shift(1).sort_index(),
        left_index=True,
        right_index=True,
        direction="backward",
    )
    return one_min, signal, daily


def _finite_positive(*values: float) -> bool:
    return all(pd.notna(v) and np.isfinite(float(v)) and float(v) > 0 for v in values)


def _trend_ready(sig: pd.Series, side: str, min_atr_percentile: float) -> bool:
    if not _finite_positive(sig.get("atr", np.nan), sig.get("d_ema20", np.nan), sig.get("d_ema50", np.nan)):
        return False
    atr_pct = sig.get("atr_percentile", np.nan)
    if pd.notna(atr_pct) and float(atr_pct) < min_atr_percentile:
        return False
    d_close = float(sig["d_close"])
    d_ema20 = float(sig["d_ema20"])
    d_ema50 = float(sig["d_ema50"])
    d_slope = float(sig.get("d_ema20_slope", 0.0))
    if side == "long":
        return d_close > d_ema20 > d_ema50 and d_slope > 0
    return d_close < d_ema20 < d_ema50 and d_slope < 0


def _setup_from_breakout(sig: pd.Series, high_value: float, low_value: float, params: dict):
    atr = float(sig["atr"])
    if not _finite_positive(atr, sig.get("prev_high_12", np.nan), sig.get("prev_low_12", np.nan)):
        return None
    if _trend_ready(sig, "long", params["min_atr_percentile"]) and high_value >= float(sig["prev_high_12"]):
        return {
            "side": "long",
            "breakout_level": float(sig["prev_high_12"]),
            "setup_atr": atr,
            "setup_ema20": float(sig.get("ema20", np.nan)),
            "bars_left": int(params["setup_expiry_bars"]),
            "touched": False,
            "pullback_extreme": np.inf,
            "pullback_reclaim_level": np.nan,
            "signal_bar_time": sig.name,
        }
    if _trend_ready(sig, "short", params["min_atr_percentile"]) and low_value <= float(sig["prev_low_12"]):
        return {
            "side": "short",
            "breakout_level": float(sig["prev_low_12"]),
            "setup_atr": atr,
            "setup_ema20": float(sig.get("ema20", np.nan)),
            "bars_left": int(params["setup_expiry_bars"]),
            "touched": False,
            "pullback_extreme": -np.inf,
            "pullback_reclaim_level": np.nan,
            "signal_bar_time": sig.name,
        }
    return None


def _touch_level(setup: dict, params: dict) -> float:
    atr = float(setup["setup_atr"])
    ema20 = float(setup.get("setup_ema20", np.nan))
    breakout_level = float(setup["breakout_level"])
    if setup["side"] == "long":
        candidates = [breakout_level + params["pullback_atr"] * atr]
        if np.isfinite(ema20):
            candidates.append(ema20 + params["pullback_atr"] * atr)
        return max(candidates)
    candidates = [breakout_level - params["pullback_atr"] * atr]
    if np.isfinite(ema20):
        candidates.append(ema20 - params["pullback_atr"] * atr)
    return min(candidates)


def _invalid_level(setup: dict, params: dict) -> float:
    atr = float(setup["setup_atr"])
    if setup["side"] == "long":
        return float(setup["breakout_level"]) - params["invalid_atr"] * atr
    return float(setup["breakout_level"]) + params["invalid_atr"] * atr


def _entry_trigger_level(setup: dict, params: dict) -> float:
    atr = float(setup["setup_atr"])
    if setup["side"] == "long":
        return max(float(setup["pullback_reclaim_level"]), float(setup["breakout_level"])) + params["reclaim_atr"] * atr
    return min(float(setup["pullback_reclaim_level"]), float(setup["breakout_level"])) - params["reclaim_atr"] * atr


def _open_position(setup: dict, close_value: float, balance: float, params: dict, *, time_value) -> dict | None:
    side = setup["side"]
    atr = float(setup["setup_atr"])
    slippage = float(params["slippage"])
    entry_price = float(close_value) * (1.0 + slippage if side == "long" else 1.0 - slippage)
    if side == "long":
        raw_stop = float(setup["pullback_extreme"]) - params["stop_buffer_atr"] * atr
        capped_stop = entry_price - params["stop_cap_atr"] * atr
        stop_price = max(raw_stop, capped_stop)
        risk = entry_price - stop_price
    else:
        raw_stop = float(setup["pullback_extreme"]) + params["stop_buffer_atr"] * atr
        capped_stop = entry_price + params["stop_cap_atr"] * atr
        stop_price = min(raw_stop, capped_stop)
        risk = stop_price - entry_price
    min_risk = entry_price * params["min_stop_bps"] / 10000.0
    if risk <= 0 or risk < min_risk:
        return None
    notional = balance * params["notional_share"]
    return {
        "side": side,
        "entry_time": time_value,
        "entry_p": entry_price,
        "entry_raw": float(close_value),
        "sl": stop_price,
        "initial_sl": stop_price,
        "risk": risk,
        "atr_at_entry": atr,
        "notional": notional,
        "notional_share": params["notional_share"],
        "breakout_level": float(setup["breakout_level"]),
        "signal_bar_time": setup["signal_bar_time"],
        "pullback_extreme": float(setup["pullback_extreme"]),
        "protected": False,
        "trailing_active": False,
        "hwm": entry_price,
        "lwm": entry_price,
        "mfe_r": 0.0,
        "mae_r": 0.0,
    }


def _update_position(position: dict, high_value: float, low_value: float, params: dict):
    side = position["side"]
    entry = float(position["entry_p"])
    risk = float(position["risk"])
    if side == "long":
        position["hwm"] = max(float(position["hwm"]), float(high_value))
        favorable = max(0.0, float(position["hwm"]) - entry)
        adverse = max(0.0, entry - float(low_value))
        position["mfe_r"] = max(float(position["mfe_r"]), favorable / risk)
        position["mae_r"] = max(float(position["mae_r"]), adverse / risk)
        if position["mfe_r"] >= params["breakeven_at_r"]:
            be_sl = entry * (1.0 + params["cost_lock_bps"] / 10000.0)
            if be_sl > float(position["sl"]):
                position["sl"] = be_sl
                position["protected"] = True
        if position["mfe_r"] >= params["trail_start_r"]:
            trail_sl = float(position["hwm"]) - params["trail_atr"] * float(position["atr_at_entry"])
            if trail_sl > float(position["sl"]):
                position["sl"] = trail_sl
                position["trailing_active"] = True
        if low_value <= float(position["sl"]):
            if position["trailing_active"]:
                reason = "TrailingSL"
            elif position["protected"]:
                reason = "BreakevenSL"
            else:
                reason = "InitialSL"
            return True, float(position["sl"]), reason
    else:
        position["lwm"] = min(float(position["lwm"]), float(low_value))
        favorable = max(0.0, entry - float(position["lwm"]))
        adverse = max(0.0, float(high_value) - entry)
        position["mfe_r"] = max(float(position["mfe_r"]), favorable / risk)
        position["mae_r"] = max(float(position["mae_r"]), adverse / risk)
        if position["mfe_r"] >= params["breakeven_at_r"]:
            be_sl = entry * (1.0 - params["cost_lock_bps"] / 10000.0)
            if be_sl < float(position["sl"]):
                position["sl"] = be_sl
                position["protected"] = True
        if position["mfe_r"] >= params["trail_start_r"]:
            trail_sl = float(position["lwm"]) + params["trail_atr"] * float(position["atr_at_entry"])
            if trail_sl < float(position["sl"]):
                position["sl"] = trail_sl
                position["trailing_active"] = True
        if high_value >= float(position["sl"]):
            if position["trailing_active"]:
                reason = "TrailingSL"
            elif position["protected"]:
                reason = "BreakevenSL"
            else:
                reason = "InitialSL"
            return True, float(position["sl"]), reason
    return False, 0.0, ""


def _append_entry(logs: list[dict], position: dict) -> None:
    logs.append(
        {
            "time": position["entry_time"],
            "type": "BUY" if position["side"] == "long" else "SHORT",
            "price": position["entry_p"],
            "reason": "Pullback-Continuation",
            "notional": position["notional"],
            "notional_share": position["notional_share"],
            "bal": np.nan,
            "breakout_level": position["breakout_level"],
            "signal_bar_time": position["signal_bar_time"],
            "pullback_extreme": position["pullback_extreme"],
            "risk": position["risk"],
            "mfe_r": np.nan,
            "mae_r": np.nan,
        }
    )


def _append_exit(logs: list[dict], position: dict, *, time_value, raw_exit: float, reason: str, balance: float, params: dict):
    slippage = float(params["slippage"])
    side_mult = 1.0 if position["side"] == "long" else -1.0
    exit_price = float(raw_exit) * (1.0 - slippage if position["side"] == "long" else 1.0 + slippage)
    pnl_pct = side_mult * (exit_price - float(position["entry_p"])) / float(position["entry_p"])
    pnl = pnl_pct * float(position["notional"])
    new_balance = balance + pnl
    logs.append(
        {
            "time": time_value,
            "type": "EXIT",
            "price": exit_price,
            "reason": reason,
            "notional": position["notional"],
            "notional_share": position["notional_share"],
            "bal": new_balance,
            "breakout_level": position["breakout_level"],
            "signal_bar_time": position["signal_bar_time"],
            "pullback_extreme": position["pullback_extreme"],
            "risk": position["risk"],
            "raw_exit_price": float(raw_exit),
            "mfe_r": float(position["mfe_r"]),
            "mae_r": float(position["mae_r"]),
        }
    )
    return new_balance


def run_strategy(second_bars: pd.DataFrame, signal: pd.DataFrame, params: dict, *, initial_balance: float):
    second_index = second_bars.index
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)

    balance = float(initial_balance)
    logs: list[dict] = []
    setup = None
    position = None
    diagnostics = {
        "setups": 0,
        "pullback_touches": 0,
        "entries": 0,
        "invalidated": 0,
        "expired": 0,
        "skipped_min_stop": 0,
    }

    for i in range(len(signal) - 1):
        start_t, end_t = signal.index[i], signal.index[i + 1]
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="left"))
        if start_pos >= end_pos:
            continue
        sig = signal.iloc[i]
        sig.name = start_t
        if pd.isna(sig.get("atr", np.nan)):
            continue

        if setup is not None:
            setup["bars_left"] -= 1
            if setup["bars_left"] <= 0:
                diagnostics["expired"] += 1
                setup = None

        for current_pos in range(start_pos, end_pos):
            bar_time = second_index[current_pos]
            high_value = float(high_values[current_pos])
            low_value = float(low_values[current_pos])
            close_value = float(close_values[current_pos])

            if position is not None:
                exit_triggered, raw_exit, reason = _update_position(position, high_value, low_value, params)
                if exit_triggered:
                    balance = _append_exit(
                        logs,
                        position,
                        time_value=bar_time,
                        raw_exit=raw_exit,
                        reason=reason,
                        balance=balance,
                        params=params,
                    )
                    position = None
                continue

            if setup is not None:
                invalid = _invalid_level(setup, params)
                if setup["side"] == "long" and low_value <= invalid:
                    diagnostics["invalidated"] += 1
                    setup = None
                    continue
                if setup["side"] == "short" and high_value >= invalid:
                    diagnostics["invalidated"] += 1
                    setup = None
                    continue

                touch = _touch_level(setup, params)
                if setup["side"] == "long":
                    if low_value <= touch:
                        if not setup["touched"]:
                            diagnostics["pullback_touches"] += 1
                        setup["touched"] = True
                        setup["pullback_extreme"] = min(float(setup["pullback_extreme"]), low_value)
                        setup["pullback_reclaim_level"] = max(
                            float(setup.get("pullback_reclaim_level", -np.inf))
                            if np.isfinite(setup.get("pullback_reclaim_level", np.nan))
                            else -np.inf,
                            high_value,
                        )
                    if setup["touched"] and high_value >= _entry_trigger_level(setup, params):
                        candidate = _open_position(setup, close_value, balance, params, time_value=bar_time)
                        if candidate is None:
                            diagnostics["skipped_min_stop"] += 1
                            setup = None
                            continue
                        position = candidate
                        _append_entry(logs, position)
                        diagnostics["entries"] += 1
                        setup = None
                else:
                    if high_value >= touch:
                        if not setup["touched"]:
                            diagnostics["pullback_touches"] += 1
                        setup["touched"] = True
                        setup["pullback_extreme"] = max(float(setup["pullback_extreme"]), high_value)
                        setup["pullback_reclaim_level"] = min(
                            float(setup.get("pullback_reclaim_level", np.inf))
                            if np.isfinite(setup.get("pullback_reclaim_level", np.nan))
                            else np.inf,
                            low_value,
                        )
                    if setup["touched"] and low_value <= _entry_trigger_level(setup, params):
                        candidate = _open_position(setup, close_value, balance, params, time_value=bar_time)
                        if candidate is None:
                            diagnostics["skipped_min_stop"] += 1
                            setup = None
                            continue
                        position = candidate
                        _append_entry(logs, position)
                        diagnostics["entries"] += 1
                        setup = None
                continue

            new_setup = _setup_from_breakout(sig, high_value, low_value, params)
            if new_setup is not None:
                setup = new_setup
                diagnostics["setups"] += 1

    if position is not None:
        balance = _append_exit(
            logs,
            position,
            time_value=second_index[-1],
            raw_exit=float(close_values[-1]),
            reason="FinalMarkToMarket",
            balance=balance,
            params=params,
        )

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
        pnl_pct = side_mult * (exit_price - entry_price) / entry_price
        rows.append(
            {
                "entry_time": entry["time"],
                "exit_time": row["time"],
                "side": entry["type"],
                "exit_reason": row["reason"],
                "pnl_pct": pnl_pct,
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
    equity = [realistic_balance]
    fee_rate = params["entry_fee"] + params["exit_fee"]
    for _, pair in pairs.iterrows():
        share = float(pair["notional_share"])
        slip_pnl_pct = float(pair["pnl_pct"])
        # Ledger prices already include both-side slippage. Add it back for raw.
        raw_pnl_pct = slip_pnl_pct + 2.0 * float(params["slippage"])
        raw_notional = raw_balance * share
        raw_balance += raw_notional * raw_pnl_pct
        slip_notional = slip_balance * share
        slip_balance += slip_notional * slip_pnl_pct
        realistic_notional = realistic_balance * share
        realistic_balance += realistic_notional * slip_pnl_pct - realistic_notional * fee_rate
        equity.append(realistic_balance)

    equity_arr = np.array(equity, dtype="float64")
    peak = np.maximum.accumulate(equity_arr)
    drawdown = equity_arr / peak - 1.0
    pnl = pairs["pnl_pct"].astype("float64")
    return {
        "trades": int(len(pairs)),
        "raw_no_fee_no_slip_return_pct": round((raw_balance / initial_balance - 1.0) * 100.0, 4),
        "price_pnl_with_2bps_slip_no_fee_return_pct": round((slip_balance / initial_balance - 1.0) * 100.0, 4),
        "realistic_return_pct": round((realistic_balance / initial_balance - 1.0) * 100.0, 4),
        "win_rate_pct": round(float((pnl > 0).mean()) * 100.0, 2),
        "max_dd_pct": round(float(drawdown.min()) * 100.0, 4),
        "avg_pnl_pct": round(float(pnl.mean()) * 100.0, 4),
        "median_pnl_pct": round(float(pnl.median()) * 100.0, 4),
        "median_hold_seconds": round(float(pairs["hold_seconds"].median()), 2),
        "avg_hold_seconds": round(float(pairs["hold_seconds"].mean()), 2),
        "median_mfe_r": round(float(pairs["mfe_r"].median()), 4),
        "median_mae_r": round(float(pairs["mae_r"].median()), 4),
        "exit_reasons": {str(k): int(v) for k, v in pairs["exit_reason"].value_counts().items()},
    }


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# Trend Pullback Continuation 1s Replay",
        "",
        "Scope: research-only. Execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at triggering 1s close with 2 bps/side slippage. Accounting also includes maker entry 2 bps and market exit 4 bps.",
        "",
        "| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Setups | Touches | Invalid | Expired |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["diagnostics"]
        lines.append(
            f"| `{result['symbol']}` | `{result['variant']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | "
            f"{s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | {s.get('median_hold_seconds', 0.0):.2f}s | "
            f"{s.get('median_mfe_r', 0.0):.4f} | `{s['exit_reasons']}` | {d['setups']} | {d['pullback_touches']} | "
            f"{d['invalidated']} | {d['expired']} |"
        )
    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['symbol']} {result['variant']}` ledger: `{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_variant(raw: str) -> tuple[str, dict]:
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    params = {
        "setup_expiry_bars": 6,
        "min_atr_percentile": 30.0,
        "pullback_atr": 0.25,
        "invalid_atr": 0.45,
        "reclaim_atr": 0.10,
        "stop_buffer_atr": 0.10,
        "stop_cap_atr": 0.60,
        "min_stop_bps": 12.0,
        "breakeven_at_r": 1.0,
        "cost_lock_bps": 10.0,
        "trail_start_r": 1.5,
        "trail_atr": 0.50,
        "notional_share": 0.20,
        "slippage": 0.0002,
        "entry_fee": 0.0002,
        "exit_fee": 0.0004,
    }
    key_map = {
        "pullback": "pullback_atr",
        "invalid": "invalid_atr",
        "reclaim": "reclaim_atr",
        "stop_cap": "stop_cap_atr",
        "trail": "trail_atr",
        "trail_start": "trail_start_r",
        "expiry": "setup_expiry_bars",
        "atr_pct": "min_atr_percentile",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = float(value)
    if isinstance(params["setup_expiry_bars"], float):
        params["setup_expiry_bars"] = int(params["setup_expiry_bars"])
    return name, params


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Trend pullback continuation 1s replay")
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--timeframe", default="4h")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument(
        "--variants",
        nargs="+",
        default=[
            "base",
            "loose_pullback=pullback:0.35,reclaim:0.08",
            "tight_reclaim=pullback:0.25,reclaim:0.05",
        ],
    )
    parser.add_argument("--summary-json", default="research/btc_eth_2026_jan_apr_trend_pullback_continuation_summary.json")
    parser.add_argument("--markdown", default="research/20260507_btc_eth_2026_jan_apr_trend_pullback_continuation.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = _as_utc(args.start)
    end = _as_utc(args.end)
    variants = [parse_variant(raw) for raw in args.variants]
    results = []

    for symbol in args.symbols:
        tick_files = DEFAULT_FILES[symbol]
        second_bars, build_stats = replay.build_continuous_second_bars(tick_files, start, end, args.chunksize)
        _, signal, daily = build_frames(second_bars, args.timeframe)
        run_signal = _add_right_boundary(signal, args.timeframe)
        print(
            f"{symbol}: second_rows={len(second_bars)} signal_rows={len(signal)} daily_rows={len(daily)}",
            flush=True,
        )
        for variant_name, params in variants:
            print(f"running {symbol} {variant_name}", flush=True)
            result = run_strategy(second_bars, run_signal, params, initial_balance=args.initial_balance)
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
                f"{symbol} {variant_name}: realistic={s['realistic_return_pct']:.4f}% raw={s['raw_no_fee_no_slip_return_pct']:.4f}% "
                f"trades={s['trades']} win={s['win_rate_pct']:.2f}% dd={s['max_dd_pct']:.4f}% diag={result['diagnostics']}",
                flush=True,
            )
            results.append(result)

    summary_path = Path(args.summary_json)
    markdown_path = Path(args.markdown)
    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "timeframe": args.timeframe,
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
    print(json.dumps({"summary_path": str(summary_path), "markdown_path": str(markdown_path), "elapsed_seconds": summary["elapsed_seconds"]}, indent=2), flush=True)


if __name__ == "__main__":
    main()
