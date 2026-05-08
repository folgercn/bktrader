#!/usr/bin/env python3
"""BTC Q1 2026 30min virtual-SL decoupled real-stop sweep.

Research-only script. This removes `re_p` and removes the fixed 0.3 ATR
initial-stop assumption for entry research. A structural breakout level
(`prev_t2.high/low` or the t3 breakout level) creates a virtual zero-notional
Initial state. If that virtual state is stopped out, the engine waits for price
to cross back through a level above/below that virtual SL and then opens at the
next observed 1s open. The real entry's initial stop is the same virtual SL
level. Real SL exits then arm the same downstream SL-Reentry pattern from the
observed real stop level, still without using `re_p`.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay
from btc_q1_30min_low_vol_entry_filters import (
    BTC_LIVE_LIKE_REPLAY_KWARGS,
    DEFAULT_TICK_FILES,
    _delta,
    _pair_diagnostics,
    _paired_trades,
)


GRID = [
    # turn_mode, virtual_sl_atr, turn_offset_atr, real_stop_mode, real_stop_atr, real_stop_buffer_atr
    # `vsl` keeps the prior tight real stop at the virtual SL level for comparison.
    ("fixed", 0.40, 0.10, "vsl", None, None),
    ("fixed", 0.40, 0.20, "vsl", None, None),
    ("fixed", 0.60, 0.20, "vsl", None, None),
    ("fixed", 0.80, 0.20, "vsl", None, None),
    # Decoupled: real stop is a fixed ATR distance from the observed entry.
    ("fixed", 0.40, 0.10, "entry_atr", 0.20, None),
    ("fixed", 0.40, 0.10, "entry_atr", 0.30, None),
    ("fixed", 0.40, 0.20, "entry_atr", 0.20, None),
    ("fixed", 0.40, 0.20, "entry_atr", 0.30, None),
    ("fixed", 0.60, 0.10, "entry_atr", 0.20, None),
    ("fixed", 0.60, 0.10, "entry_atr", 0.30, None),
    ("fixed", 0.60, 0.20, "entry_atr", 0.20, None),
    ("fixed", 0.60, 0.20, "entry_atr", 0.30, None),
    ("fixed", 0.80, 0.20, "entry_atr", 0.20, None),
    ("fixed", 0.80, 0.20, "entry_atr", 0.30, None),
    # Decoupled: real stop is outside the post-VSL local low/high by a small buffer.
    ("fixed", 0.40, 0.10, "extreme_buffer", None, 0.05),
    ("fixed", 0.40, 0.10, "extreme_buffer", None, 0.10),
    ("fixed", 0.40, 0.20, "extreme_buffer", None, 0.05),
    ("fixed", 0.40, 0.20, "extreme_buffer", None, 0.10),
    ("fixed", 0.60, 0.10, "extreme_buffer", None, 0.05),
    ("fixed", 0.60, 0.10, "extreme_buffer", None, 0.10),
    ("fixed", 0.60, 0.20, "extreme_buffer", None, 0.05),
    ("fixed", 0.60, 0.20, "extreme_buffer", None, 0.10),
    ("fixed", 0.80, 0.20, "extreme_buffer", None, 0.05),
    ("fixed", 0.80, 0.20, "extreme_buffer", None, 0.10),
]


def _fmt_param(value: float | None) -> str:
    if value is None:
        return "na"
    return str(value).replace(".", "p")


def _variant_name(
    turn_mode: str,
    sl_atr: float,
    entry_offset_atr: float,
    real_stop_mode: str,
    real_stop_atr: float | None,
    real_stop_buffer_atr: float | None,
) -> str:
    fmt = lambda v: str(v).replace(".", "p")
    if real_stop_mode == "entry_atr":
        stop_part = f"entrysl_{_fmt_param(real_stop_atr)}atr"
    elif real_stop_mode == "extreme_buffer":
        stop_part = f"extbuf_{_fmt_param(real_stop_buffer_atr)}atr"
    else:
        stop_part = "realsl_vsl"
    return f"{turn_mode}_vsl_{fmt(sl_atr)}atr_turn_{fmt(entry_offset_atr)}atr_{stop_part}"


def _entry_mix(summary: dict) -> str:
    return ", ".join(f"{k}:{v}" for k, v in summary["entry_reasons"].items())


def _make_variants() -> list[dict]:
    return [
        {
            "name": _variant_name(
                turn_mode,
                sl_atr,
                entry_offset_atr,
                real_stop_mode,
                real_stop_atr,
                real_stop_buffer_atr,
            ),
            "turn_mode": turn_mode,
            "virtual_sl_atr": float(sl_atr),
            "entry_offset_atr": float(entry_offset_atr),
            "real_stop_mode": real_stop_mode,
            "real_stop_atr": None if real_stop_atr is None else float(real_stop_atr),
            "real_stop_buffer_atr": None if real_stop_buffer_atr is None else float(real_stop_buffer_atr),
            "description": (
                f"Virtual Initial SL at structural breakout level +/- {sl_atr} ATR; "
                f"enter on {turn_mode} turn confirmation of {entry_offset_atr} ATR; "
                f"real stop mode {real_stop_mode}."
            ),
        }
        for turn_mode, sl_atr, entry_offset_atr, real_stop_mode, real_stop_atr, real_stop_buffer_atr in GRID
    ]


def _real_stop_from_level(side: str, entry_price: float, stop_level: float) -> float:
    if side == "long":
        return min(float(stop_level), float(entry_price))
    return max(float(stop_level), float(entry_price))


def _new_virtual(
    sig,
    side: str,
    variant: dict,
    shape_name: str,
    breakout_level: float,
    bar_index: int,
    lock_time,
    observed_lock_price: float,
):
    atr = float(sig["atr"])
    sl_atr = float(variant["virtual_sl_atr"])
    entry_raw = float(breakout_level)
    if side == "long":
        stop_level = entry_raw - sl_atr * atr
        trigger_level = stop_level + float(variant["entry_offset_atr"]) * atr
    else:
        stop_level = entry_raw + sl_atr * atr
        trigger_level = stop_level - float(variant["entry_offset_atr"]) * atr
    return {
        "side": side,
        "entry_p": entry_raw,
        "sl": float(stop_level),
        "trigger_level": float(trigger_level),
        "protected": False,
        "notional": 0.0,
        "breakout_shape_name": shape_name,
        "breakout_level": entry_raw,
        "bar_index": int(bar_index),
        "observed_lock_price": float(observed_lock_price),
        "lock_time": lock_time,
        "turn_mode": str(variant["turn_mode"]),
        "virtual_sl_atr": sl_atr,
        "entry_offset_atr": float(variant["entry_offset_atr"]),
        "real_stop_mode": str(variant["real_stop_mode"]),
        "real_stop_atr": variant.get("real_stop_atr"),
        "real_stop_buffer_atr": variant.get("real_stop_buffer_atr"),
        "atr_at_lock": atr,
        "hwm": float(entry_raw),
        "lwm": float(entry_raw),
    }


def _advance_virtual(virtual: dict, high_value: float, low_value: float):
    if virtual["side"] == "long" and low_value <= virtual["sl"]:
        return True, "SL"
    if virtual["side"] == "short" and high_value >= virtual["sl"]:
        return True, "SL"
    if virtual["side"] == "long":
        virtual["hwm"] = max(float(virtual.get("hwm", virtual["entry_p"])), float(high_value))
    else:
        virtual["lwm"] = min(float(virtual.get("lwm", virtual["entry_p"])), float(low_value))
    return False, ""


def _advance_real(position: dict, sig, high_value: float, low_value: float):
    trailing_stop_atr = float(replay.COMMON_REPLAY_KWARGS["trailing_stop_atr"])
    delayed_trailing_activation = float(replay.COMMON_REPLAY_KWARGS["delayed_trailing_activation"])
    profit_protect_atr = float(replay.COMMON_REPLAY_KWARGS["profit_protect_atr"])
    exit_triggered = False
    exit_p = 0.0
    reason = ""

    if position["side"] == "long":
        prev_hwm = position.get("hwm", position["entry_p"])
        protected_before_bar = bool(position.get("protected", False))
        profit_atr = (prev_hwm - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0.0
        if profit_atr >= delayed_trailing_activation:
            position["sl"] = max(position["sl"], prev_hwm - trailing_stop_atr * sig["atr"])
        if low_value <= position["sl"]:
            exit_p, reason, exit_triggered = position["sl"], "SL", True
        elif protected_before_bar and low_value <= sig["prev_low_1"]:
            exit_p, reason, exit_triggered = sig["prev_low_1"], "PT", True
        if not exit_triggered:
            position["hwm"] = max(prev_hwm, high_value)
            if not position["protected"] and high_value >= position["entry_p"] + profit_protect_atr * sig["atr"]:
                position["protected"] = True
    else:
        prev_lwm = position.get("lwm", position["entry_p"])
        protected_before_bar = bool(position.get("protected", False))
        profit_atr = (position["entry_p"] - prev_lwm) / sig["atr"] if sig["atr"] > 0 else 0.0
        if profit_atr >= delayed_trailing_activation:
            position["sl"] = min(position["sl"], prev_lwm + trailing_stop_atr * sig["atr"])
        if high_value >= position["sl"]:
            exit_p, reason, exit_triggered = position["sl"], "SL", True
        elif protected_before_bar and high_value >= sig["prev_high_1"]:
            exit_p, reason, exit_triggered = sig["prev_high_1"], "PT", True
        if not exit_triggered:
            position["lwm"] = min(prev_lwm, low_value)
            if not position["protected"] and low_value <= position["entry_p"] - profit_protect_atr * sig["atr"]:
                position["protected"] = True
    return exit_triggered, float(exit_p), reason


def _pending_from_virtual(virtual: dict, bar_index: int, exit_time, exit_extreme: float):
    return {
        "side": virtual["side"],
        "reason": "Zero-Initial-Reentry",
        "bar_index": int(bar_index),
        "turn_mode": str(virtual.get("turn_mode", "fixed")),
        "turn_extreme": float(exit_extreme),
        "trigger_level": float(virtual["trigger_level"]),
        "stop_level": float(virtual["sl"]),
        "breakout_shape_name": virtual["breakout_shape_name"],
        "breakout_level": float(virtual["breakout_level"]),
        "observed_lock_price": float(virtual.get("observed_lock_price", virtual["breakout_level"])),
        "lock_time": virtual["lock_time"],
        "source_exit_time": exit_time,
        "virtual_sl_atr": float(virtual["virtual_sl_atr"]),
        "entry_offset_atr": float(virtual["entry_offset_atr"]),
        "real_stop_mode": str(virtual.get("real_stop_mode", "vsl")),
        "real_stop_atr": virtual.get("real_stop_atr"),
        "real_stop_buffer_atr": virtual.get("real_stop_buffer_atr"),
        "atr_at_lock": float(virtual["atr_at_lock"]),
    }


def _pending_from_real_sl(position: dict, sig, bar_index: int, exit_time, stop_level: float, exit_extreme: float) -> dict:
    atr = float(sig["atr"])
    entry_offset_atr = float(position["entry_offset_atr"])
    stop_level = float(stop_level)
    if position["side"] == "long":
        trigger_level = stop_level + entry_offset_atr * atr
    else:
        trigger_level = stop_level - entry_offset_atr * atr
    return {
        "side": position["side"],
        "reason": "SL-Reentry",
        "bar_index": int(bar_index),
        "turn_mode": str(position.get("turn_mode", "fixed")),
        "turn_extreme": float(exit_extreme),
        "trigger_level": float(trigger_level),
        "stop_level": stop_level,
        "breakout_shape_name": position.get("breakout_shape_name", ""),
        "breakout_level": float(position.get("breakout_level", np.nan)),
        "observed_lock_price": float(position.get("observed_lock_price", np.nan)),
        "lock_time": position.get("lock_time", exit_time),
        "source_exit_time": exit_time,
        "virtual_sl_atr": float(position["virtual_sl_atr"]),
        "entry_offset_atr": entry_offset_atr,
        "real_stop_mode": str(position.get("real_stop_mode", "vsl")),
        "real_stop_atr": position.get("real_stop_atr"),
        "real_stop_buffer_atr": position.get("real_stop_buffer_atr"),
        "atr_at_lock": float(position.get("atr_at_lock", atr)),
    }


def _triggered(pending: dict, high_value: float, low_value: float, close_value: float, prev_close: float | None) -> bool:
    if prev_close is None or pd.isna(prev_close):
        return False
    if pending["side"] == "long":
        pending["turn_extreme"] = min(float(pending["turn_extreme"]), float(low_value))
    else:
        pending["turn_extreme"] = max(float(pending["turn_extreme"]), float(high_value))
    if pending.get("turn_mode") == "dynamic":
        offset = float(pending["entry_offset_atr"]) * float(pending["atr_at_lock"])
        if pending["side"] == "long":
            level = float(pending["turn_extreme"]) + offset
            pending["trigger_level"] = level
            return close_value >= level
        level = float(pending["turn_extreme"]) - offset
        pending["trigger_level"] = level
        return close_value <= level
    level = float(pending["trigger_level"])
    if pending["side"] == "long":
        return prev_close < level <= close_value
    return prev_close > level >= close_value


def _real_stop_from_pending(side: str, entry_price: float, sig, pending: dict) -> float:
    mode = str(pending.get("real_stop_mode", "vsl"))
    atr = float(sig["atr"])
    if mode == "entry_atr":
        stop_atr = float(pending["real_stop_atr"])
        raw_stop = entry_price - stop_atr * atr if side == "long" else entry_price + stop_atr * atr
    elif mode == "extreme_buffer":
        buffer_atr = float(pending["real_stop_buffer_atr"])
        extreme = float(pending.get("turn_extreme", pending["stop_level"]))
        raw_stop = extreme - buffer_atr * atr if side == "long" else extreme + buffer_atr * atr
    else:
        raw_stop = float(pending["stop_level"])
    return _real_stop_from_level(side, entry_price, raw_stop)


def _open_position(balance: float, sig, pending: dict, fill_raw: float, trade_slot: int):
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    commission = float(replay.COMMON_REPLAY_KWARGS.get("commission", 0.001))
    side = pending["side"]
    entry_price = float(fill_raw) * (1 + slippage if side == "long" else 1 - slippage)
    schedule = [float(v) for v in replay.COMMON_REPLAY_KWARGS["reentry_size_schedule"]]
    notional_share = replay._get_reentry_window_real_order_size(trade_slot, schedule)
    notional = balance * notional_share
    stop_level = _real_stop_from_pending(side, entry_price, sig, pending)
    position = {
        "side": side,
        "entry_p": entry_price,
        "sl": stop_level,
        "protected": False,
        "notional": notional,
        "breakout_shape_name": pending["breakout_shape_name"],
        "breakout_level": float(pending["breakout_level"]),
        "observed_lock_price": float(pending.get("observed_lock_price", np.nan)),
        "lock_time": pending.get("lock_time"),
        "trigger_level": float(pending["trigger_level"]),
        "stop_level": float(pending["stop_level"]),
        "turn_mode": str(pending.get("turn_mode", "fixed")),
        "turn_extreme": float(pending.get("turn_extreme", pending["stop_level"])),
        "virtual_sl_atr": float(pending["virtual_sl_atr"]),
        "entry_offset_atr": float(pending["entry_offset_atr"]),
        "real_stop_mode": str(pending.get("real_stop_mode", "vsl")),
        "real_stop_atr": pending.get("real_stop_atr"),
        "real_stop_buffer_atr": pending.get("real_stop_buffer_atr"),
        "atr_at_lock": float(pending.get("atr_at_lock", sig["atr"])),
    }
    if side == "long":
        position["hwm"] = entry_price
    else:
        position["lwm"] = entry_price
    balance -= notional * commission
    return balance, position


def _empty_state(initial_balance: float, variant: dict) -> dict:
    return {
        "variant": variant,
        "balance": float(initial_balance),
        "position": None,
        "virtual": None,
        "pending": None,
        "trades_in_bar": 0,
        "trade_logs": [],
        "diagnostics": {
            "breakout_locks": {"long": {}, "short": {}},
            "virtual_sl": 0,
            "pending_expired": 0,
            "pending_triggered": 0,
            "pending_armed_by_reason": {},
            "pending_triggered_by_reason": {},
            "pending_expired_by_reason": {},
            "real_sl": 0,
            "real_sl_reentry_armed": 0,
            "virtual_expired_without_sl": 0,
            "max_trades_blocked": 0,
        },
    }


def _record_lock(state: dict, side: str, shape_name: str):
    locks = state["diagnostics"]["breakout_locks"][side]
    locks[shape_name] = locks.get(shape_name, 0) + 1


def _record_diag_reason(state: dict, bucket: str, reason: str) -> None:
    counts = state["diagnostics"][bucket]
    counts[reason] = counts.get(reason, 0) + 1


def _finalize_state(state: dict, initial_balance: float, second_index, close_values, slippage: float) -> dict:
    if state["position"] is not None and len(close_values) > 0:
        position = state["position"]
        last_bar_time = second_index[-1]
        last_close = float(close_values[-1])
        side_mult = 1 if position["side"] == "long" else -1
        commission = float(replay.COMMON_REPLAY_KWARGS.get("commission", 0.001))
        final_exit_p = last_close * (1 - slippage) if position["side"] == "long" else last_close * (1 + slippage)
        pnl = side_mult * (final_exit_p - position["entry_p"]) / position["entry_p"] * position["notional"]
        state["balance"] += pnl - position["notional"] * commission
        state["trade_logs"].append(
            {
                "time": last_bar_time,
                "type": "EXIT",
                "price": final_exit_p,
                "reason": "FinalMarkToMarket",
                "notional": position["notional"],
                "bal": state["balance"],
                "breakout_shape_name": position.get("breakout_shape_name", ""),
                "breakout_level": position.get("breakout_level", np.nan),
                "trigger_level": position.get("trigger_level", np.nan),
                "stop_level": position.get("stop_level", np.nan),
            }
        )
    ledger = pd.DataFrame(state["trade_logs"])
    pairs = _paired_trades(ledger)
    return {
        "variant": state["variant"]["name"],
        "description": state["variant"]["description"],
        "params": state["variant"],
        "summary": replay.summarize_run(ledger, initial_balance),
        "pair_diagnostics": _pair_diagnostics(pairs),
        "level_diagnostics": _level_diagnostics(ledger, pairs),
        "attribution": replay.summarize_breakout_attribution(ledger),
        "diagnostics": state["diagnostics"],
        "ledger": ledger,
    }


def _level_diagnostics(ledger: pd.DataFrame, pairs: pd.DataFrame) -> dict:
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])] if not ledger.empty else ledger
    reason_stats = {}
    if not pairs.empty:
        for reason, group in pairs.groupby("entry_reason"):
            pnl = group["pnl_value"].astype("float64")
            reason_stats[str(reason)] = {
                "trades": int(len(group)),
                "win_rate_pct": round(float((pnl > 0).mean()) * 100.0, 2),
                "net_pnl": round(float(pnl.sum()), 2),
                "avg_pnl_pct": round(float(group["pnl_pct"].mean()) * 100.0, 4),
            }
    if entries.empty:
        return {"entry_reason_stats": reason_stats}
    return {
        "entry_reason_stats": reason_stats,
        "median_seconds_from_virtual_sl": round(float(entries["seconds_from_virtual_sl"].dropna().median()), 2)
        if "seconds_from_virtual_sl" in entries and entries["seconds_from_virtual_sl"].notna().any()
        else None,
        "median_entry_vs_stop_bps": round(float(entries["entry_vs_stop_bps"].dropna().median()), 4)
        if "entry_vs_stop_bps" in entries and entries["entry_vs_stop_bps"].notna().any()
        else None,
        "median_entry_vs_real_stop_bps": round(float(entries["entry_vs_real_stop_bps"].dropna().median()), 4)
        if "entry_vs_real_stop_bps" in entries and entries["entry_vs_real_stop_bps"].notna().any()
        else None,
        "median_entry_vs_trigger_bps": round(float(entries["entry_vs_trigger_bps"].dropna().median()), 4)
        if "entry_vs_trigger_bps" in entries and entries["entry_vs_trigger_bps"].notna().any()
        else None,
    }


def run_level_sweep(
    second_bars: pd.DataFrame,
    signal: pd.DataFrame,
    variants: list[dict],
    initial_balance: float,
    *,
    allow_downstream_sl_reentry: bool,
) -> list[dict]:
    started = time.time()
    states = [_empty_state(initial_balance, variant) for variant in variants]
    second_index = second_bars.index
    open_values = second_bars["open"].to_numpy(dtype="float64", copy=False)
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    commission = float(replay.COMMON_REPLAY_KWARGS.get("commission", 0.001))
    max_trades_per_bar = int(replay.COMMON_REPLAY_KWARGS["max_trades_per_bar"])
    reentry_timeout = 1

    for i in range(len(signal) - 1):
        start_t, end_t = signal.index[i], signal.index[i + 1]
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="right"))
        if start_pos >= end_pos:
            continue
        base_sig = signal.iloc[i]
        if pd.isna(base_sig["atr"]):
            continue

        for state in states:
            state["trades_in_bar"] = 0
            pending = state["pending"]
            if pending is not None and i - pending["bar_index"] > reentry_timeout:
                state["pending"] = None
                state["diagnostics"]["pending_expired"] += 1
                _record_diag_reason(state, "pending_expired_by_reason", pending["reason"])
            virtual = state["virtual"]
            if virtual is not None and i - virtual["bar_index"] > reentry_timeout:
                state["virtual"] = None
                state["diagnostics"]["virtual_expired_without_sl"] += 1

        bar_high_so_far = -np.inf
        bar_low_so_far = np.inf
        live_sig = base_sig.to_dict()
        live_sig["_closed_atr"] = float(base_sig["atr"])

        for current_pos in range(start_pos, end_pos):
            bar_time = second_index[current_pos]
            high_value = float(high_values[current_pos])
            low_value = float(low_values[current_pos])
            close_value = float(close_values[current_pos])
            prev_close = float(close_values[current_pos - 1]) if current_pos > 0 else None
            bar_high_so_far = max(bar_high_so_far, high_value)
            bar_low_so_far = min(bar_low_so_far, low_value)
            sig = replay._intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)
            long_ready, short_ready = replay._resolve_regime_ready(sig, "1d")

            long_break = replay._long_breakout(sig, high_value, "baseline_plus_t3") if long_ready else (False, np.nan, "")
            short_break = replay._short_breakout(sig, low_value, "baseline_plus_t3") if short_ready else (False, np.nan, "")

            for state in states:
                position = state["position"]
                if position is not None:
                    exit_triggered, exit_p, reason = _advance_real(position, sig, high_value, low_value)
                    if exit_triggered:
                        raw_exit_p = float(exit_p)
                        side_mult = 1 if position["side"] == "long" else -1
                        exit_p = exit_p * (1 - slippage) if position["side"] == "long" else exit_p * (1 + slippage)
                        pnl = side_mult * (exit_p - position["entry_p"]) / position["entry_p"] * position["notional"]
                        state["balance"] += pnl - position["notional"] * commission
                        state["trade_logs"].append(
                            {
                                "time": bar_time,
                                "type": "EXIT",
                                "price": exit_p,
                                "reason": reason,
                                "notional": position["notional"],
                                "bal": state["balance"],
                                "breakout_shape_name": position.get("breakout_shape_name", ""),
                                "breakout_level": position.get("breakout_level", np.nan),
                                "trigger_level": position.get("trigger_level", np.nan),
                                "stop_level": position.get("stop_level", np.nan),
                                "exit_stop_level": raw_exit_p,
                            }
                        )
                        if reason == "SL":
                            state["diagnostics"]["real_sl"] += 1
                            if allow_downstream_sl_reentry:
                                exit_extreme = low_value if position["side"] == "long" else high_value
                                state["pending"] = _pending_from_real_sl(position, sig, i, bar_time, raw_exit_p, exit_extreme)
                                state["diagnostics"]["real_sl_reentry_armed"] += 1
                                _record_diag_reason(state, "pending_armed_by_reason", "SL-Reentry")
                        state["position"] = None
                    continue

                virtual = state["virtual"]
                if virtual is not None:
                    exit_triggered, reason = _advance_virtual(virtual, high_value, low_value)
                    if exit_triggered:
                        state["diagnostics"]["virtual_sl"] += 1
                        exit_extreme = low_value if virtual["side"] == "long" else high_value
                        state["pending"] = _pending_from_virtual(virtual, i, bar_time, exit_extreme)
                        _record_diag_reason(state, "pending_armed_by_reason", "Zero-Initial-Reentry")
                        state["virtual"] = None
                    continue

                pending = state["pending"]
                if pending is not None:
                    if _triggered(pending, high_value, low_value, close_value, prev_close):
                        if state["trades_in_bar"] >= max_trades_per_bar:
                            state["pending"] = None
                            state["diagnostics"]["max_trades_blocked"] += 1
                            continue
                        fill_pos = current_pos + 1
                        if fill_pos >= len(open_values):
                            state["pending"] = None
                            continue
                        fill_raw = float(open_values[fill_pos])
                        state["balance"], state["position"] = _open_position(
                            state["balance"],
                            sig,
                            pending,
                            fill_raw,
                            state["trades_in_bar"],
                        )
                        entry_time = second_index[fill_pos]
                        position = state["position"]
                        side = pending["side"]
                        entry_vs_stop = (
                            (position["entry_p"] - pending["stop_level"]) / pending["stop_level"]
                            if side == "long"
                            else (pending["stop_level"] - position["entry_p"]) / pending["stop_level"]
                        )
                        entry_vs_real_stop = (
                            (position["entry_p"] - position["sl"]) / position["sl"]
                            if side == "long"
                            else (position["sl"] - position["entry_p"]) / position["sl"]
                        )
                        entry_vs_trigger = (
                            (position["entry_p"] - pending["trigger_level"]) / pending["trigger_level"]
                            if side == "long"
                            else (pending["trigger_level"] - position["entry_p"]) / pending["trigger_level"]
                        )
                        state["trade_logs"].append(
                            {
                                "time": entry_time,
                                "type": "BUY" if side == "long" else "SHORT",
                                "price": position["entry_p"],
                                "reason": pending["reason"],
                                "notional": position["notional"],
                                "bal": state["balance"],
                                "breakout_shape_name": pending["breakout_shape_name"],
                                "breakout_level": pending["breakout_level"],
                                "observed_lock_price": pending["observed_lock_price"],
                                "trigger_level": pending["trigger_level"],
                                "stop_level": pending["stop_level"],
                                "real_stop_price": position["sl"],
                                "real_stop_mode": position.get("real_stop_mode", "vsl"),
                                "real_stop_atr": position.get("real_stop_atr"),
                                "real_stop_buffer_atr": position.get("real_stop_buffer_atr"),
                                "turn_mode": pending.get("turn_mode", "fixed"),
                                "turn_extreme": pending.get("turn_extreme", np.nan),
                                "signal_bar_time": start_t,
                                "trade_slot": state["trades_in_bar"],
                                "seconds_from_lock": (pd.Timestamp(entry_time) - pd.Timestamp(pending["lock_time"])).total_seconds(),
                                "seconds_from_virtual_sl": (pd.Timestamp(entry_time) - pd.Timestamp(pending["source_exit_time"])).total_seconds(),
                                "entry_vs_stop_bps": entry_vs_stop * 10000.0,
                                "entry_vs_real_stop_bps": entry_vs_real_stop * 10000.0,
                                "entry_vs_trigger_bps": entry_vs_trigger * 10000.0,
                                "lock_extension_bps": (
                                    (pending["observed_lock_price"] - pending["breakout_level"]) / pending["breakout_level"]
                                    if side == "long"
                                    else (pending["breakout_level"] - pending["observed_lock_price"]) / pending["breakout_level"]
                                )
                                * 10000.0,
                                "virtual_sl_atr": pending["virtual_sl_atr"],
                                "entry_offset_atr": pending["entry_offset_atr"],
                            }
                        )
                        state["pending"] = None
                        state["trades_in_bar"] += 1
                        state["diagnostics"]["pending_triggered"] += 1
                        _record_diag_reason(state, "pending_triggered_by_reason", pending["reason"])
                    continue

                if state["trades_in_bar"] > 0:
                    continue

                if long_ready and long_break[0]:
                    _, breakout_level, shape_name = long_break
                    _record_lock(state, "long", shape_name)
                    state["virtual"] = _new_virtual(
                        sig,
                        "long",
                        state["variant"],
                        shape_name,
                        breakout_level,
                        i,
                        bar_time,
                        high_value,
                    )
                elif short_ready and short_break[0]:
                    _, breakout_level, shape_name = short_break
                    _record_lock(state, "short", shape_name)
                    state["virtual"] = _new_virtual(
                        sig,
                        "short",
                        state["variant"],
                        shape_name,
                        breakout_level,
                        i,
                        bar_time,
                        low_value,
                    )

    results = [_finalize_state(state, initial_balance, second_index, close_values, slippage) for state in states]
    for result in results:
        result["elapsed_seconds"] = round(time.time() - started, 2)
    return results


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        f"# {summary['symbol']} Q1 2026 {summary['timeframe']} Virtual-SL Decoupled Real-Stop Sweep",
        "",
        "Scope: research-only Python replay. No live or execution path is changed by this report.",
        "",
        "## Setup",
        "",
        f"- Symbol/window: `{summary['symbol']}`, `{summary['start']}` to `{summary['end']}`",
        "- Execution bars: continuous `1s` bars rebuilt from Binance trade archives",
        f"- Signal timeframe: `{summary['timeframe']}`",
        "- Breakout shape: `baseline_plus_t3`",
        "- `re_p` usage: none",
        "- Cooldown: none",
        "- Breakout arm: 1s high/low touching the structural breakout level",
        "- Virtual Initial reference price: structural breakout level, not the observed trigger-second close",
        "- Virtual SL is used only as fake-breakout filtering; real position stop is swept independently.",
        "- Real stop modes: `vsl` keeps the old tight stop at the virtual SL level; `entry_atr` stops from the observed entry by fixed ATR; `extreme_buffer` stops outside the post-VSL local low/high by a small ATR buffer.",
        "- Turn confirmation: `fixed` enters after price recovers from the SL level by offset ATR.",
        f"- Downstream `SL-Reentry`: {'enabled' if summary['allow_downstream_sl_reentry'] else 'disabled'}.",
        f"- Trailing stop retained for trend management: `trailing_stop_atr={summary['baseline_kwargs']['trailing_stop_atr']}`, activated after `{summary['baseline_kwargs']['delayed_trailing_activation']} ATR` unrealized profit",
        f"- Sizing: `reentry_size_schedule={summary['baseline_kwargs']['reentry_size_schedule']}`, `max_trades_per_bar={summary['baseline_kwargs']['max_trades_per_bar']}`",
        "",
        "## Results",
        "",
        "| Variant | Real Stop | VSL ATR | Turn Offset ATR | Stop ATR | Buffer ATR | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Median Real Stop bps |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        ld = result["level_diagnostics"]
        params = result["params"]
        stop_atr = params["real_stop_atr"] if params.get("real_stop_atr") is not None else ""
        buffer_atr = params["real_stop_buffer_atr"] if params.get("real_stop_buffer_atr") is not None else ""
        lines.append(
            f"| `{result['variant']}` | `{params['real_stop_mode']}` | {params['virtual_sl_atr']:.2f} | {params['entry_offset_atr']:.2f} | "
            f"{stop_atr} | {buffer_atr} | "
            f"{s['final_balance']:,.2f} | {s['return_pct']:.2f}% | {s['max_dd_pct']:.2f}% | "
            f"{s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | `{_entry_mix(s)}` | "
            f"{ld.get('median_entry_vs_real_stop_bps') if ld.get('median_entry_vs_real_stop_bps') is not None else ''} |"
        )

    best = summary["best_by_return"]
    lines.extend(
        [
            "",
            "## Best By Return",
            "",
            f"- `{best['variant']}`: return `{best['summary']['return_pct']:.2f}%`, trades `{best['summary']['trades']}`, win `{best['summary']['win_rate_pct']:.2f}%`, MaxDD `{best['summary']['max_dd_pct']:.2f}%`.",
            "",
            "## Entry Diagnostics",
            "",
            "This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.",
            "",
            "| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |",
            "|---|---|---:|---:|---:|---:|",
        ]
    )
    for result in summary["results"]:
        for reason, stats in result["level_diagnostics"].get("entry_reason_stats", {}).items():
            lines.append(
                f"| `{result['variant']}` | `{reason}` | {stats['trades']} | {stats['win_rate_pct']:.2f}% | "
                f"{stats['net_pnl']:,.2f} | {stats['avg_pnl_pct']:.4f}% |"
            )
    lines.extend(
        [
            "",
            "## Pending Diagnostics",
            "",
            "| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |",
            "|---|---:|---:|---|---|---|---:|",
        ]
    )
    for result in summary["results"]:
        diag = result["diagnostics"]
        armed = ", ".join(f"{k}:{v}" for k, v in diag.get("pending_armed_by_reason", {}).items())
        triggered = ", ".join(f"{k}:{v}" for k, v in diag.get("pending_triggered_by_reason", {}).items())
        expired = ", ".join(f"{k}:{v}" for k, v in diag.get("pending_expired_by_reason", {}).items())
        lines.append(
            f"| `{result['variant']}` | {diag.get('real_sl', 0)} | {diag.get('virtual_expired_without_sl', 0)} | `{armed}` | `{triggered}` | `{expired}` | {diag.get('max_trades_blocked', 0)} |"
        )
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC Q1 2026 30min virtual-SL decoupled real-stop sweep")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--timeframe", default="30min")
    parser.add_argument("--disable-downstream-sl-reentry", action="store_true")
    parser.add_argument("--summary-json", default="research/btc_2026_q1_30min_virtual_sl_decoupled_stop_summary.json")
    parser.add_argument("--markdown", default="research/20260505_btc_q1_30min_virtual_sl_decoupled_stop.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_2026_q1_30min_virtual_sl_decoupled_stop")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    replay.COMMON_REPLAY_KWARGS.clear()
    replay.COMMON_REPLAY_KWARGS.update(BTC_LIVE_LIKE_REPLAY_KWARGS)

    start = replay._as_utc_timestamp(args.start)
    end = replay._as_utc_timestamp(args.end)
    second_bars, build_stats = replay.build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    _, signal = replay.build_signal_frame(second_bars, args.timeframe)
    variants = _make_variants()
    allow_downstream_sl_reentry = not args.disable_downstream_sl_reentry
    results = run_level_sweep(
        second_bars,
        signal,
        variants,
        args.initial_balance,
        allow_downstream_sl_reentry=allow_downstream_sl_reentry,
    )

    for result in results:
        ledger_path = Path(f"{args.ledger_prefix}_{result['variant']}_ledger.csv")
        result["ledger"].to_csv(ledger_path, index=False)
        del result["ledger"]
        result["ledger_path"] = str(ledger_path)
        s = result["summary"]
        print(
            f"{result['variant']}: return={s['return_pct']:.2f}% trades={s['trades']} "
            f"win={s['win_rate_pct']:.2f}% max_dd={s['max_dd_pct']:.2f}%",
            flush=True,
        )

    best_by_return = max(results, key=lambda item: item["summary"]["return_pct"])
    base_summary = best_by_return["summary"]
    for result in results:
        result["delta_vs_best"] = _delta(base_summary, result["summary"])

    output = {
        "symbol": "BTCUSDT",
        "timeframe": args.timeframe,
        "allow_downstream_sl_reentry": allow_downstream_sl_reentry,
        "start": start.isoformat(),
        "end": end.isoformat(),
        "build_stats": build_stats,
        "signal_stats": {
            "signal_rows": int(len(signal)),
            "signal_start": signal.index[0].isoformat() if not signal.empty else "",
            "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
            "valid_sma5_rows": int(signal["sma5"].notna().sum()),
            "valid_atr_rows": int(signal["atr"].notna().sum()),
        },
        "baseline_kwargs": dict(BTC_LIVE_LIKE_REPLAY_KWARGS),
        "variants": variants,
        "best_by_return": best_by_return,
        "results": results,
        "note": "Research-only virtual-SL decoupled real-stop sweep. re_p and cooldown are not used.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
