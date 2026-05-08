#!/usr/bin/env python3
"""BTC Jan-Apr 2026 direct-breakout research replay.

This is a research-only comparison for the "no VSL / no reclaim" idea:
the first structural breakout in each signal bar opens real exposure
immediately at the observed 1s close. It deliberately does not arm a virtual
SL, does not use re_p, and does not use downstream reclaim reentry.
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
    _pair_diagnostics,
    _paired_trades,
)


DEFAULT_TICK_FILES = [
    "dataset/archive/BTCUSDT-trades-2026-01/BTCUSDT-trades-2026-01.csv",
    "dataset/archive/BTCUSDT-trades-2026-02/BTCUSDT-trades-2026-02.csv",
    "dataset/archive/BTCUSDT-trades-2026-03/BTCUSDT-trades-2026-03.csv",
    "dataset/archive/BTCUSDT-trades-2026-04/BTCUSDT-trades-2026-04.zip",
]


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


def _base_exit_policy() -> dict:
    return {
        "name": "baseline",
        "trailing_stop_atr": float(replay.COMMON_REPLAY_KWARGS["trailing_stop_atr"]),
        "delayed_trailing_activation": float(replay.COMMON_REPLAY_KWARGS["delayed_trailing_activation"]),
        "profit_protect_atr": float(replay.COMMON_REPLAY_KWARGS["profit_protect_atr"]),
        "profit_lock_activation_atr": None,
        "profit_lock_bps": 0.0,
        "take_profit_atr": None,
    }


def _open_direct_position(
    balance: float,
    sig,
    *,
    side: str,
    fill_raw: float,
    notional_share: float,
    shape_name: str,
    breakout_level: float,
    signal_bar_time,
    trade_slot: int,
    exit_policy: dict,
):
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    stop_loss_atr = float(replay.COMMON_REPLAY_KWARGS["stop_loss_atr"])
    stop_mode = str(replay.COMMON_REPLAY_KWARGS["stop_mode"])
    entry_price = float(fill_raw) * (1 + slippage if side == "long" else 1 - slippage)
    notional = balance * float(notional_share)
    position = {
        "side": side,
        "entry_p": entry_price,
        "sl": replay._resolve_stop_price(side, entry_price, sig, stop_mode, stop_loss_atr),
        "protected": False,
        "notional": notional,
        "notional_share": float(notional_share),
        "breakout_shape_name": shape_name,
        "breakout_level": float(breakout_level),
        "observed_fill_raw": float(fill_raw),
        "signal_bar_time": signal_bar_time,
        "trade_slot": int(trade_slot),
        "trailing_stop_active": False,
        "atr_at_entry": float(sig["atr"]),
        "mfe_bps": 0.0,
        "mae_bps": 0.0,
        "mfe_atr": 0.0,
        "mae_atr": 0.0,
        "exit_policy": dict(exit_policy),
        "profit_lock_active": False,
    }
    position["initial_stop_price"] = float(position["sl"])
    if side == "long":
        position["hwm"] = entry_price
    else:
        position["lwm"] = entry_price
    return balance, position


def _update_excursions(position: dict, high_value: float, low_value: float) -> None:
    entry_price = float(position["entry_p"])
    if entry_price <= 0:
        return
    atr_at_entry = float(position.get("atr_at_entry", np.nan))
    if position["side"] == "long":
        favorable = max(0.0, float(high_value) - entry_price)
        adverse = max(0.0, entry_price - float(low_value))
    else:
        favorable = max(0.0, entry_price - float(low_value))
        adverse = max(0.0, float(high_value) - entry_price)
    position["mfe_bps"] = max(float(position.get("mfe_bps", 0.0)), favorable / entry_price * 10000.0)
    position["mae_bps"] = max(float(position.get("mae_bps", 0.0)), adverse / entry_price * 10000.0)
    if np.isfinite(atr_at_entry) and atr_at_entry > 0:
        position["mfe_atr"] = max(float(position.get("mfe_atr", 0.0)), favorable / atr_at_entry)
        position["mae_atr"] = max(float(position.get("mae_atr", 0.0)), adverse / atr_at_entry)


def _exit_reason_for_stop(position: dict) -> str:
    if position.get("trailing_stop_active", False):
        return "TrailingSL"
    if position.get("profit_lock_active", False):
        return "ProfitLockSL"
    return "InitialSL"


def _advance_position(position: dict, sig, high_value: float, low_value: float):
    exit_policy = position.get("exit_policy", {})
    trailing_stop_atr = float(exit_policy.get("trailing_stop_atr", replay.COMMON_REPLAY_KWARGS["trailing_stop_atr"]))
    delayed_trailing_activation = float(
        exit_policy.get("delayed_trailing_activation", replay.COMMON_REPLAY_KWARGS["delayed_trailing_activation"])
    )
    profit_protect_atr = float(exit_policy.get("profit_protect_atr", replay.COMMON_REPLAY_KWARGS["profit_protect_atr"]))
    profit_lock_activation_atr = exit_policy.get("profit_lock_activation_atr")
    profit_lock_bps = float(exit_policy.get("profit_lock_bps", 0.0))
    take_profit_atr = exit_policy.get("take_profit_atr")
    _update_excursions(position, high_value, low_value)

    if position["side"] == "long":
        prev_hwm = float(position.get("hwm", position["entry_p"]))
        protected_before_bar = bool(position.get("protected", False))
        profit_atr = (prev_hwm - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0.0
        if profit_lock_activation_atr is not None and profit_atr >= float(profit_lock_activation_atr):
            locked_sl = float(position["entry_p"]) * (1.0 + profit_lock_bps / 10000.0)
            if locked_sl > float(position["sl"]):
                position["sl"] = locked_sl
                position["profit_lock_active"] = True
        if profit_atr >= delayed_trailing_activation:
            trailing_sl = prev_hwm - trailing_stop_atr * float(sig["atr"])
            if trailing_sl > float(position["sl"]):
                position["sl"] = trailing_sl
                position["trailing_stop_active"] = True
        if low_value <= position["sl"]:
            return True, float(position["sl"]), _exit_reason_for_stop(position)
        if take_profit_atr is not None:
            target_price = float(position["entry_p"]) + float(take_profit_atr) * float(sig["atr"])
            if high_value >= target_price:
                return True, target_price, "TPxATR"
        if protected_before_bar and low_value <= sig["prev_low_1"]:
            return True, float(sig["prev_low_1"]), "PT"
        position["hwm"] = max(prev_hwm, float(high_value))
        if not position["protected"] and high_value >= position["entry_p"] + profit_protect_atr * float(sig["atr"]):
            position["protected"] = True
        profit_atr = (position["hwm"] - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0.0
        if profit_lock_activation_atr is not None and profit_atr >= float(profit_lock_activation_atr):
            locked_sl = float(position["entry_p"]) * (1.0 + profit_lock_bps / 10000.0)
            if locked_sl > float(position["sl"]):
                position["sl"] = locked_sl
                position["profit_lock_active"] = True
        if profit_atr >= delayed_trailing_activation:
            trailing_sl = position["hwm"] - trailing_stop_atr * float(sig["atr"])
            if trailing_sl > float(position["sl"]):
                position["sl"] = trailing_sl
                position["trailing_stop_active"] = True
    else:
        prev_lwm = float(position.get("lwm", position["entry_p"]))
        protected_before_bar = bool(position.get("protected", False))
        profit_atr = (position["entry_p"] - prev_lwm) / sig["atr"] if sig["atr"] > 0 else 0.0
        if profit_lock_activation_atr is not None and profit_atr >= float(profit_lock_activation_atr):
            locked_sl = float(position["entry_p"]) * (1.0 - profit_lock_bps / 10000.0)
            if locked_sl < float(position["sl"]):
                position["sl"] = locked_sl
                position["profit_lock_active"] = True
        if profit_atr >= delayed_trailing_activation:
            trailing_sl = prev_lwm + trailing_stop_atr * float(sig["atr"])
            if trailing_sl < float(position["sl"]):
                position["sl"] = trailing_sl
                position["trailing_stop_active"] = True
        if high_value >= position["sl"]:
            return True, float(position["sl"]), _exit_reason_for_stop(position)
        if take_profit_atr is not None:
            target_price = float(position["entry_p"]) - float(take_profit_atr) * float(sig["atr"])
            if low_value <= target_price:
                return True, target_price, "TPxATR"
        if protected_before_bar and high_value >= sig["prev_high_1"]:
            return True, float(sig["prev_high_1"]), "PT"
        position["lwm"] = min(prev_lwm, float(low_value))
        if not position["protected"] and low_value <= position["entry_p"] - profit_protect_atr * float(sig["atr"]):
            position["protected"] = True
        profit_atr = (position["entry_p"] - position["lwm"]) / sig["atr"] if sig["atr"] > 0 else 0.0
        if profit_lock_activation_atr is not None and profit_atr >= float(profit_lock_activation_atr):
            locked_sl = float(position["entry_p"]) * (1.0 - profit_lock_bps / 10000.0)
            if locked_sl < float(position["sl"]):
                position["sl"] = locked_sl
                position["profit_lock_active"] = True
        if profit_atr >= delayed_trailing_activation:
            trailing_sl = position["lwm"] + trailing_stop_atr * float(sig["atr"])
            if trailing_sl < float(position["sl"]):
                position["sl"] = trailing_sl
                position["trailing_stop_active"] = True
    return False, 0.0, ""


def _append_exit(state: dict, position: dict, *, bar_time, raw_exit_price: float, reason: str) -> None:
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    side_mult = 1.0 if position["side"] == "long" else -1.0
    exit_price = raw_exit_price * (1 - slippage) if position["side"] == "long" else raw_exit_price * (1 + slippage)
    pnl = side_mult * (exit_price - position["entry_p"]) / position["entry_p"] * position["notional"]
    state["balance"] += pnl
    state["trade_logs"].append(
        {
            "time": bar_time,
            "type": "EXIT",
            "price": exit_price,
            "reason": reason,
            "notional": position["notional"],
            "notional_share": position["notional_share"],
            "bal": state["balance"],
            "breakout_shape_name": position.get("breakout_shape_name", ""),
            "breakout_level": position.get("breakout_level", np.nan),
            "observed_fill_raw": np.nan,
            "signal_bar_time": position.get("signal_bar_time"),
            "trade_slot": position.get("trade_slot", 0),
            "raw_exit_price": float(raw_exit_price),
            "real_stop_price": position.get("sl", np.nan),
            "initial_stop_price": position.get("initial_stop_price", np.nan),
            "trailing_stop_active": bool(position.get("trailing_stop_active", False)),
            "profit_lock_active": bool(position.get("profit_lock_active", False)),
            "exit_policy_name": str(position.get("exit_policy", {}).get("name", "")),
            "mfe_bps": float(position.get("mfe_bps", 0.0)),
            "mae_bps": float(position.get("mae_bps", 0.0)),
            "mfe_atr": float(position.get("mfe_atr", 0.0)),
            "mae_atr": float(position.get("mae_atr", 0.0)),
        }
    )


def run_direct_breakout(
    second_bars: pd.DataFrame,
    signal: pd.DataFrame,
    *,
    initial_balance: float,
    timeframe: str,
    variant_name: str = "",
    breakout_shape: str = "baseline_plus_t3",
    allow_same_bar_second_breakout: bool = False,
    exit_policy: dict | None = None,
) -> dict:
    started = time.time()
    second_index = second_bars.index
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)

    max_trades_per_bar = int(replay.COMMON_REPLAY_KWARGS["max_trades_per_bar"])
    schedule = [float(v) for v in replay.COMMON_REPLAY_KWARGS["reentry_size_schedule"]]
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    exit_policy = _base_exit_policy() if exit_policy is None else dict(exit_policy)

    state = {
        "balance": float(initial_balance),
        "position": None,
        "trade_logs": [],
        "diagnostics": {
            "breakout_locks": {"long": {}, "short": {}},
            "direct_entries": 0,
            "max_trades_blocked": 0,
            "bars_with_entry": 0,
            "allow_same_bar_second_breakout": bool(allow_same_bar_second_breakout),
        },
    }
    entries_by_bar = {}

    for i in range(len(signal) - 1):
        start_t, end_t = signal.index[i], signal.index[i + 1]
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="left"))
        if start_pos >= end_pos:
            continue
        base_sig = signal.iloc[i]
        if pd.isna(base_sig["atr"]):
            continue

        trades_in_bar = 0
        direct_breakouts_taken = 0
        bar_high_so_far = -np.inf
        bar_low_so_far = np.inf
        live_sig = base_sig.to_dict()
        live_sig["_closed_atr"] = float(base_sig["atr"])

        for current_pos in range(start_pos, end_pos):
            bar_time = second_index[current_pos]
            high_value = float(high_values[current_pos])
            low_value = float(low_values[current_pos])
            close_value = float(close_values[current_pos])
            bar_high_so_far = max(bar_high_so_far, high_value)
            bar_low_so_far = min(bar_low_so_far, low_value)
            sig = replay._intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)
            long_ready, short_ready = replay._resolve_regime_ready(sig, "1d")

            position = state["position"]
            if position is not None:
                exit_triggered, raw_exit_price, reason = _advance_position(position, sig, high_value, low_value)
                if exit_triggered:
                    _append_exit(state, position, bar_time=bar_time, raw_exit_price=raw_exit_price, reason=reason)
                    state["position"] = None
                continue

            if direct_breakouts_taken > 0 and not allow_same_bar_second_breakout:
                continue

            if trades_in_bar >= max_trades_per_bar:
                state["diagnostics"]["max_trades_blocked"] += 1
                continue

            side = ""
            breakout_level = np.nan
            shape_name = ""
            if long_ready:
                triggered, breakout_level, shape_name = replay._long_breakout(sig, high_value, breakout_shape)
                if triggered:
                    side = "long"
            elif short_ready:
                triggered, breakout_level, shape_name = replay._short_breakout(sig, low_value, breakout_shape)
                if triggered:
                    side = "short"

            if not side:
                continue

            notional_share = replay._get_reentry_window_real_order_size(trades_in_bar, schedule)
            state["balance"], state["position"] = _open_direct_position(
                state["balance"],
                sig,
                side=side,
                fill_raw=close_value,
                notional_share=notional_share,
                shape_name=shape_name,
                breakout_level=breakout_level,
                signal_bar_time=start_t,
                trade_slot=trades_in_bar,
                exit_policy=exit_policy,
            )
            position = state["position"]
            locks = state["diagnostics"]["breakout_locks"][side]
            locks[shape_name] = locks.get(shape_name, 0) + 1
            state["diagnostics"]["direct_entries"] += 1
            entries_by_bar[start_t] = entries_by_bar.get(start_t, 0) + 1
            direct_breakouts_taken += 1
            state["trade_logs"].append(
                {
                    "time": bar_time,
                    "type": "BUY" if side == "long" else "SHORT",
                    "price": position["entry_p"],
                    "reason": "Direct-Breakout",
                    "notional": position["notional"],
                    "notional_share": notional_share,
                    "bal": state["balance"],
                    "breakout_shape_name": shape_name,
                    "breakout_level": float(breakout_level),
                    "observed_fill_raw": float(close_value),
                    "signal_bar_time": start_t,
                    "trade_slot": trades_in_bar,
                    "raw_exit_price": np.nan,
                    "real_stop_price": position["sl"],
                    "entry_vs_breakout_bps": (
                        (close_value - breakout_level) / breakout_level
                        if side == "long"
                        else (breakout_level - close_value) / breakout_level
                    )
                    * 10000.0,
                    "exit_policy_name": str(exit_policy.get("name", "")),
                }
            )
            trades_in_bar += 1

    if state["position"] is not None and len(close_values) > 0:
        _append_exit(
            state,
            state["position"],
            bar_time=second_index[-1],
            raw_exit_price=float(close_values[-1]),
            reason="FinalMarkToMarket",
        )
        state["position"] = None

    ledger = pd.DataFrame(state["trade_logs"])
    pairs = _paired_trades(ledger)
    state["diagnostics"]["bars_with_entry"] = int(len(entries_by_bar))
    state["bar_guard_diagnostics"] = {
        "bars_with_multi_entries": int(sum(1 for v in entries_by_bar.values() if v > 1)),
        "extra_entries": int(sum(max(0, v - 1) for v in entries_by_bar.values())),
        "max_entries_per_bar": int(max(entries_by_bar.values()) if entries_by_bar else 0),
    }
    return {
        "timeframe": timeframe,
        "variant": variant_name,
        "breakout_shape": breakout_shape,
        "exit_policy": exit_policy,
        "reentry_size_schedule": [float(v) for v in replay.COMMON_REPLAY_KWARGS["reentry_size_schedule"]],
        "summary": replay.summarize_run(ledger, initial_balance),
        "pair_diagnostics": _pair_diagnostics(pairs),
        "attribution": replay.summarize_breakout_attribution(ledger),
        "diagnostics": state["diagnostics"],
        "bar_guard_diagnostics": state["bar_guard_diagnostics"],
        "entry_distance_diagnostics": _entry_distance_diagnostics(ledger),
        "exit_hold_diagnostics": _exit_hold_diagnostics(pairs),
        "trade_slot_diagnostics": _trade_slot_diagnostics(ledger),
        "mfe_mae_diagnostics": _mfe_mae_diagnostics(ledger),
        "accounting_2bps_maker_entry_market_exit": _accounting_from_ledger(
            ledger,
            initial_balance=initial_balance,
            slippage=slippage,
            entry_fee=0.0002,
            exit_fee=0.0004,
        ),
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def _exit_hold_diagnostics(pairs: pd.DataFrame) -> dict:
    if pairs.empty:
        return {}
    diagnostics = {}
    for reason, group in pairs.groupby("exit_reason", dropna=False):
        hold = group["hold_seconds"].astype("float64")
        pnl = group["pnl_value"].astype("float64")
        diagnostics[str(reason)] = {
            "trades": int(len(group)),
            "avg_hold_seconds": round(float(hold.mean()), 2),
            "median_hold_seconds": round(float(hold.median()), 2),
            "win_rate_pct": round(float((pnl > 0).mean()) * 100.0, 2),
        }
    return diagnostics


def _trade_slot_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty:
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
        rows.append(
            {
                "trade_slot": int(float(open_entry.get("trade_slot", 0))),
                "exit_reason": str(row.get("reason", "")),
                "pnl_pct": pnl_pct,
                "pnl_value": pnl_pct * float(open_entry.get("notional", 0.0)),
                "hold_seconds": float((pd.Timestamp(row["time"]) - pd.Timestamp(open_entry["time"])).total_seconds()),
            }
        )
        open_entry = None
    if not rows:
        return {}
    pairs = pd.DataFrame(rows)
    diagnostics = {}
    for slot, group in pairs.groupby("trade_slot"):
        pnl = group["pnl_value"].astype("float64")
        pnl_pct = group["pnl_pct"].astype("float64")
        hold = group["hold_seconds"].astype("float64")
        diagnostics[str(int(slot))] = {
            "trades": int(len(group)),
            "win_rate_pct": round(float((pnl > 0).mean()) * 100.0, 2),
            "avg_pnl_pct": round(float(pnl_pct.mean()) * 100.0, 4),
            "net_pnl": round(float(pnl.sum()), 2),
            "avg_hold_seconds": round(float(hold.mean()), 2),
            "median_hold_seconds": round(float(hold.median()), 2),
            "exit_reasons": {str(k): int(v) for k, v in group["exit_reason"].value_counts().items()},
        }
    return diagnostics


def _mfe_mae_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty:
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
        realized_bps = side_mult * (exit_price - entry_price) / entry_price * 10000.0 if entry_price > 0 else 0.0
        rows.append(
            {
                "exit_reason": str(row.get("reason", "")),
                "breakout_shape_name": str(open_entry.get("breakout_shape_name", "")),
                "trade_slot": int(float(open_entry.get("trade_slot", 0))),
                "mfe_bps": float(row.get("mfe_bps", 0.0)),
                "mae_bps": float(row.get("mae_bps", 0.0)),
                "mfe_atr": float(row.get("mfe_atr", 0.0)),
                "mae_atr": float(row.get("mae_atr", 0.0)),
                "realized_bps": realized_bps,
            }
        )
        open_entry = None
    if not rows:
        return {}

    pairs = pd.DataFrame(rows)

    def summarize(group: pd.DataFrame) -> dict:
        mfe_bps = group["mfe_bps"].astype("float64")
        mae_bps = group["mae_bps"].astype("float64")
        mfe_atr = group["mfe_atr"].astype("float64")
        mae_atr = group["mae_atr"].astype("float64")
        realized_bps = group["realized_bps"].astype("float64")
        return {
            "trades": int(len(group)),
            "median_mfe_bps": round(float(mfe_bps.median()), 4),
            "median_mae_bps": round(float(mae_bps.median()), 4),
            "avg_mfe_bps": round(float(mfe_bps.mean()), 4),
            "avg_mae_bps": round(float(mae_bps.mean()), 4),
            "median_mfe_atr": round(float(mfe_atr.median()), 4),
            "median_mae_atr": round(float(mae_atr.median()), 4),
            "median_realized_bps": round(float(realized_bps.median()), 4),
            "mfe_ge_10bps_pct": round(float((mfe_bps >= 10.0).mean()) * 100.0, 2),
            "mfe_ge_20bps_pct": round(float((mfe_bps >= 20.0).mean()) * 100.0, 2),
            "mae_ge_10bps_pct": round(float((mae_bps >= 10.0).mean()) * 100.0, 2),
            "mae_ge_20bps_pct": round(float((mae_bps >= 20.0).mean()) * 100.0, 2),
        }

    return {
        "overall": summarize(pairs),
        "by_exit_reason": {str(reason): summarize(group) for reason, group in pairs.groupby("exit_reason")},
        "by_breakout_shape": {
            str(shape): summarize(group) for shape, group in pairs.groupby("breakout_shape_name")
        },
        "by_trade_slot": {str(int(slot)): summarize(group) for slot, group in pairs.groupby("trade_slot")},
    }


def _entry_distance_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty:
        return {"entries": 0}
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])].copy()
    if entries.empty:
        return {"entries": 0}
    distances = entries["entry_vs_breakout_bps"].astype("float64").dropna()
    return {
        "entries": int(len(entries)),
        "median_entry_vs_breakout_bps": round(float(distances.median()), 4) if not distances.empty else None,
        "entry_within_5bps_of_breakout_pct": round(float((distances.abs() <= 5.0).mean()) * 100.0, 2)
        if not distances.empty
        else None,
        "worst_entry_extension_bps": round(float(distances.max()), 4) if not distances.empty else None,
    }


def _accounting_from_ledger(
    ledger: pd.DataFrame,
    *,
    initial_balance: float,
    slippage: float,
    entry_fee: float,
    exit_fee: float,
) -> dict:
    raw_balance = float(initial_balance)
    slip_balance = float(initial_balance)
    realistic_balance = float(initial_balance)
    fees_paid = 0.0
    round_trips = 0
    entry_counts = {}
    open_entry = None

    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue

        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        share = float(open_entry.get("notional_share", 0.0))
        if not np.isfinite(share) or share <= 0:
            share = 0.0

        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        if open_entry["type"] == "BUY":
            entry_raw = entry_price / (1.0 + slippage)
            exit_raw = exit_price / (1.0 - slippage)
        else:
            entry_raw = entry_price / (1.0 - slippage)
            exit_raw = exit_price / (1.0 + slippage)

        raw_pnl_pct = side_mult * (exit_raw - entry_raw) / entry_raw
        slip_pnl_pct = side_mult * (exit_price - entry_price) / entry_price

        raw_notional = raw_balance * share
        raw_balance += raw_notional * raw_pnl_pct

        slip_notional = slip_balance * share
        slip_balance += slip_notional * slip_pnl_pct

        realistic_notional = realistic_balance * share
        entry_fee_value = realistic_notional * entry_fee
        exit_fee_value = realistic_notional * exit_fee
        realistic_balance += realistic_notional * slip_pnl_pct - entry_fee_value - exit_fee_value
        fees_paid += entry_fee_value + exit_fee_value

        reason = str(open_entry.get("reason", ""))
        entry_counts[reason] = entry_counts.get(reason, 0) + 1
        round_trips += 1
        open_entry = None

    return {
        "round_trips": int(round_trips),
        "entry_reasons": {str(k): int(v) for k, v in entry_counts.items()},
        "raw_no_fee_no_slip_return_pct": round((raw_balance / initial_balance - 1.0) * 100.0, 4),
        "price_pnl_with_2bps_slip_no_fee_return_pct": round((slip_balance / initial_balance - 1.0) * 100.0, 4),
        "realistic_return_pct": round((realistic_balance / initial_balance - 1.0) * 100.0, 4),
        "realistic_final_balance": round(realistic_balance, 2),
        "realistic_fees_pct": round(fees_paid / initial_balance * 100.0, 4),
        "entry_fee_bps": round(entry_fee * 10000.0, 4),
        "exit_fee_bps": round(exit_fee * 10000.0, 4),
        "slippage_bps_per_side": round(slippage * 10000.0, 4),
        "by_trade_slot": _accounting_by_trade_slot(
            ledger,
            initial_balance=initial_balance,
            slippage=slippage,
            entry_fee=entry_fee,
            exit_fee=exit_fee,
        ),
    }


def _accounting_by_trade_slot(
    ledger: pd.DataFrame,
    *,
    initial_balance: float,
    slippage: float,
    entry_fee: float,
    exit_fee: float,
) -> dict:
    raw_balance = float(initial_balance)
    slip_balance = float(initial_balance)
    realistic_balance = float(initial_balance)
    buckets = {}
    open_entry = None

    def bucket_for(slot: int) -> dict:
        key = str(int(slot))
        if key not in buckets:
            buckets[key] = {
                "round_trips": 0,
                "raw_pnl": 0.0,
                "slip_pnl": 0.0,
                "realistic_pnl": 0.0,
                "fees": 0.0,
            }
        return buckets[key]

    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue

        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        share = float(open_entry.get("notional_share", 0.0))
        if not np.isfinite(share) or share <= 0:
            share = 0.0
        slot = int(float(open_entry.get("trade_slot", 0)))
        bucket = bucket_for(slot)

        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        if open_entry["type"] == "BUY":
            entry_raw = entry_price / (1.0 + slippage)
            exit_raw = exit_price / (1.0 - slippage)
        else:
            entry_raw = entry_price / (1.0 - slippage)
            exit_raw = exit_price / (1.0 + slippage)

        raw_pnl_pct = side_mult * (exit_raw - entry_raw) / entry_raw
        slip_pnl_pct = side_mult * (exit_price - entry_price) / entry_price

        raw_notional = raw_balance * share
        raw_pnl = raw_notional * raw_pnl_pct
        raw_balance += raw_pnl

        slip_notional = slip_balance * share
        slip_pnl = slip_notional * slip_pnl_pct
        slip_balance += slip_pnl

        realistic_notional = realistic_balance * share
        fee_value = realistic_notional * (entry_fee + exit_fee)
        realistic_pnl = realistic_notional * slip_pnl_pct - fee_value
        realistic_balance += realistic_pnl

        bucket["round_trips"] += 1
        bucket["raw_pnl"] += raw_pnl
        bucket["slip_pnl"] += slip_pnl
        bucket["realistic_pnl"] += realistic_pnl
        bucket["fees"] += fee_value
        open_entry = None

    return {
        slot: {
            "round_trips": int(values["round_trips"]),
            "raw_no_fee_no_slip_contribution_pct": round(values["raw_pnl"] / initial_balance * 100.0, 4),
            "price_pnl_with_2bps_slip_no_fee_contribution_pct": round(values["slip_pnl"] / initial_balance * 100.0, 4),
            "realistic_contribution_pct": round(values["realistic_pnl"] / initial_balance * 100.0, 4),
            "fees_pct": round(values["fees"] / initial_balance * 100.0, 4),
        }
        for slot, values in sorted(buckets.items(), key=lambda item: int(item[0]))
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    exit_reason_order = ["InitialSL", "ProfitLockSL", "TrailingSL", "PT", "TPxATR", "FinalMarkToMarket"]

    def exit_reason_text(counts: dict) -> str:
        parts = []
        for reason in exit_reason_order:
            if reason in counts or reason in {"InitialSL", "TrailingSL", "PT"}:
                parts.append(f"{reason}:{int(counts.get(reason, 0))}")
        for reason, count in counts.items():
            if reason not in exit_reason_order:
                parts.append(f"{reason}:{int(count)}")
        return ", ".join(parts)

    lines = [
        f"# {summary['symbol']} 2026 Jan-Apr Direct Breakout",
        "",
        "Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first configured structural breakout in each signal bar opens real exposure immediately at the observed 1s close. Exit variants keep the entry semantics fixed and only change stop/target handling after entry.",
        "",
        "Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.",
        "",
        "| Timeframe | Shape | Variant | Exit Policy | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |",
        "|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        acct = result["accounting_2bps_maker_entry_market_exit"]
        dist = result["entry_distance_diagnostics"]
        schedule = result.get("reentry_size_schedule", [])
        lines.append(
            f"| `{result['timeframe']}` | `{result.get('breakout_shape', '')}` | `{result.get('variant', '')}` | `{result.get('exit_policy', {}).get('name', '')}` | `{schedule}` | "
            f"{acct['realistic_return_pct']:.4f}% | {acct['round_trips']} | "
            f"{acct['raw_no_fee_no_slip_return_pct']:.4f}% | {acct['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{acct['realistic_fees_pct']:.4f}% | {s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.2f}% | "
            f"`{exit_reason_text(s['exit_reasons'])}` | {d['avg_hold_seconds']:.2f}s | {d['median_hold_seconds']:.2f}s | "
            f"{dist.get('median_entry_vs_breakout_bps')} | "
            f"{result['bar_guard_diagnostics']['max_entries_per_bar']} |"
        )

    lines.extend(["", "## Exit Hold Diagnostics", ""])
    lines.append("| Timeframe | Shape | Variant | Exit Policy | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |")
    lines.append("|---|---|---|---|---|---:|---:|---:|---:|")
    for result in summary["results"]:
        diagnostics = result.get("exit_hold_diagnostics", {})
        reason_names = [reason for reason in exit_reason_order if reason in diagnostics or reason in {"InitialSL", "TrailingSL", "PT"}]
        reason_names.extend(reason for reason in diagnostics.keys() if reason not in exit_reason_order)
        for reason in reason_names:
            stats = diagnostics.get(reason, {"trades": 0, "avg_hold_seconds": 0.0, "median_hold_seconds": 0.0, "win_rate_pct": 0.0})
            lines.append(
                f"| `{result['timeframe']}` | `{result.get('breakout_shape', '')}` | `{result.get('variant', '')}` | `{result.get('exit_policy', {}).get('name', '')}` | `{reason}` | {stats['trades']} | "
                f"{stats['avg_hold_seconds']:.2f}s | {stats['median_hold_seconds']:.2f}s | "
                f"{stats['win_rate_pct']:.2f}% |"
            )

    lines.extend(["", "## Trade Slot Diagnostics", ""])
    lines.append("| Timeframe | Shape | Variant | Exit Policy | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |")
    lines.append("|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|")
    for result in summary["results"]:
        by_slot = result["accounting_2bps_maker_entry_market_exit"].get("by_trade_slot", {})
        slot_diag = result.get("trade_slot_diagnostics", {})
        for slot, acct in by_slot.items():
            diag = slot_diag.get(slot, {})
            lines.append(
                f"| `{result['timeframe']}` | `{result.get('breakout_shape', '')}` | `{result.get('variant', '')}` | `{result.get('exit_policy', {}).get('name', '')}` | {slot} | {acct['round_trips']} | "
                f"{acct['realistic_contribution_pct']:.4f}% | "
                f"{acct['raw_no_fee_no_slip_contribution_pct']:.4f}% | "
                f"{acct['price_pnl_with_2bps_slip_no_fee_contribution_pct']:.4f}% | "
                f"{acct['fees_pct']:.4f}% | "
                f"{diag.get('win_rate_pct', 0.0):.2f}% | "
                f"{diag.get('avg_hold_seconds', 0.0):.2f}s | {diag.get('median_hold_seconds', 0.0):.2f}s | "
                f"`{diag.get('exit_reasons', {})}` |"
            )

    lines.extend(["", "## MFE/MAE Diagnostics", ""])
    lines.append("| Timeframe | Shape | Variant | Exit Policy | Group | Trades | Median MFE | Median MAE | Median MFE ATR | Median MAE ATR | MFE >= 10bps | MFE >= 20bps | Median Realized |")
    lines.append("|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"]:
        diagnostics = result.get("mfe_mae_diagnostics", {})
        rows = [("overall", diagnostics.get("overall"))]
        rows.extend((f"exit:{reason}", stats) for reason, stats in diagnostics.get("by_exit_reason", {}).items())
        rows.extend((f"filled:{shape}", stats) for shape, stats in diagnostics.get("by_breakout_shape", {}).items())
        for group_name, stats in rows:
            if not stats:
                continue
            lines.append(
                f"| `{result['timeframe']}` | `{result.get('breakout_shape', '')}` | `{result.get('variant', '')}` | `{result.get('exit_policy', {}).get('name', '')}` | `{group_name}` | "
                f"{stats['trades']} | {stats['median_mfe_bps']:.4f}bps | {stats['median_mae_bps']:.4f}bps | "
                f"{stats['median_mfe_atr']:.4f} | {stats['median_mae_atr']:.4f} | "
                f"{stats['mfe_ge_10bps_pct']:.2f}% | {stats['mfe_ge_20bps_pct']:.2f}% | "
                f"{stats['median_realized_bps']:.4f}bps |"
            )

    lines.extend(["", "## Breakout Attribution", ""])
    lines.append("| Timeframe | Configured Shape | Variant | Exit Policy | Filled Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |")
    lines.append("|---|---|---|---|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"]:
        for shape_name, stats in result.get("attribution", {}).items():
            lines.append(
                f"| `{result['timeframe']}` | `{result.get('breakout_shape', '')}` | `{result.get('variant', '')}` | `{result.get('exit_policy', {}).get('name', '')}` | `{shape_name}` | {stats['trades']} | {stats['win_rate_pct']:.2f}% | "
                f"{stats['avg_pnl_pct']:.4f}% | {stats['median_pnl_pct']:.4f}% | "
                f"{stats['net_pnl_value']:,.2f} | {stats['profit_factor']} |"
            )

    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['timeframe']}` ledger: `{result['ledger_path']}`")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC 2026 Jan-Apr direct-breakout replay")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--symbol", default="BTCUSDT")
    parser.add_argument("--timeframes", nargs="+", default=["1h", "4h", "1d"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--stop-loss-atr", type=float, default=0.30)
    parser.add_argument(
        "--summary-json",
        default="research/btc_2026_jan_apr_direct_breakout_summary.json",
    )
    parser.add_argument(
        "--markdown",
        default="research/20260506_btc_2026_jan_apr_direct_breakout.md",
    )
    parser.add_argument(
        "--ledger-prefix",
        default="research/tmp_btc_2026_jan_apr_direct_breakout",
    )
    parser.add_argument(
        "--allow-same-bar-second-breakout",
        action="store_true",
        help="Allow a second direct-breakout entry in the same signal bar after the first position exits.",
    )
    parser.add_argument(
        "--schedule-variants",
        nargs="+",
        default=None,
        help="Optional variants like name=0.20,0.10. Runs all variants against the same rebuilt 1s bars.",
    )
    parser.add_argument(
        "--breakout-shapes",
        nargs="+",
        default=["baseline_plus_t3"],
        choices=["original_t2", "baseline_plus_t3"],
        help="Breakout shape configurations to run against the same rebuilt 1s bars.",
    )
    parser.add_argument(
        "--exit-variants",
        nargs="+",
        default=["baseline"],
        help=(
            "Exit policy variants. Presets: baseline, be0p5, cost0p5, "
            "lock1p0_20bps, trail0p2_act0p5, trail0p5_act1p0, tp1p0. "
            "Custom format: name=lock_atr:0.5,lock_bps:10,trailing_atr:0.3,activation_atr:0.5,take_profit_atr:1.0"
        ),
    )
    return parser.parse_args()


def _parse_schedule_variants(raw_values) -> list[tuple[str, list[float]]]:
    if not raw_values:
        return [("default", [0.20, 0.10])]
    variants = []
    for raw in raw_values:
        if "=" in raw:
            name, values = raw.split("=", 1)
        else:
            values = raw
            name = raw.replace(",", "_").replace(".", "p")
        schedule = [float(value) for value in values.split(",") if value.strip()]
        if not schedule:
            raise ValueError(f"empty schedule variant: {raw}")
        variants.append((name.strip(), schedule))
    return variants


def _parse_exit_variants(raw_values) -> list[dict]:
    presets = {
        "baseline": {},
        "be0p5": {"profit_lock_activation_atr": 0.5, "profit_lock_bps": 0.0},
        "cost0p5": {"profit_lock_activation_atr": 0.5, "profit_lock_bps": 10.0},
        "lock1p0_20bps": {"profit_lock_activation_atr": 1.0, "profit_lock_bps": 20.0},
        "trail0p2_act0p5": {"trailing_stop_atr": 0.2, "delayed_trailing_activation": 0.5},
        "trail0p5_act1p0": {"trailing_stop_atr": 0.5, "delayed_trailing_activation": 1.0},
        "tp1p0": {"take_profit_atr": 1.0},
    }
    variants = []
    for raw in raw_values or ["baseline"]:
        base = _base_exit_policy()
        if raw in presets:
            base.update(presets[raw])
            base["name"] = raw
            variants.append(base)
            continue
        if "=" not in raw:
            raise ValueError(f"unknown exit variant: {raw}")
        name, values = raw.split("=", 1)
        base["name"] = name.strip()
        key_map = {
            "lock_atr": "profit_lock_activation_atr",
            "lock_bps": "profit_lock_bps",
            "trailing_atr": "trailing_stop_atr",
            "activation_atr": "delayed_trailing_activation",
            "profit_protect_atr": "profit_protect_atr",
            "take_profit_atr": "take_profit_atr",
        }
        for item in values.split(","):
            if not item.strip():
                continue
            key, value = item.split(":", 1)
            mapped_key = key_map.get(key.strip())
            if mapped_key is None:
                raise ValueError(f"unknown exit variant key in {raw}: {key}")
            base[mapped_key] = float(value)
        variants.append(base)
    return variants


def main() -> None:
    args = parse_args()
    started = time.time()
    start = _as_utc(args.start)
    end = _as_utc(args.end)

    replay.COMMON_REPLAY_KWARGS.update(BTC_LIVE_LIKE_REPLAY_KWARGS)
    replay.COMMON_REPLAY_KWARGS.update(
        {
            "fixed_slippage": 0.0002,
            "commission": 0.0,
            "stop_loss_atr": float(args.stop_loss_atr),
            "stop_mode": "atr",
            "max_trades_per_bar": 2,
        }
    )
    schedule_variants = _parse_schedule_variants(args.schedule_variants)
    exit_variants = _parse_exit_variants(args.exit_variants)

    second_bars, build_stats = replay.build_continuous_second_bars(args.tick_files, start, end, args.chunksize)

    results = []
    for timeframe in args.timeframes:
        _, signal = replay.build_signal_frame(second_bars, timeframe)
        run_signal = _add_right_boundary(signal, timeframe)
        print(
            f"running timeframe={timeframe} real_signal_rows={len(signal)} run_rows={len(run_signal)}",
            flush=True,
        )
        for breakout_shape in args.breakout_shapes:
            for variant_name, schedule in schedule_variants:
                replay.COMMON_REPLAY_KWARGS["reentry_size_schedule"] = [float(v) for v in schedule]
                for exit_policy in exit_variants:
                    result = run_direct_breakout(
                        second_bars,
                        run_signal,
                        initial_balance=args.initial_balance,
                        timeframe=timeframe,
                        variant_name=variant_name,
                        breakout_shape=breakout_shape,
                        allow_same_bar_second_breakout=args.allow_same_bar_second_breakout,
                        exit_policy=exit_policy,
                    )
                    exit_policy_name = str(exit_policy.get("name", ""))
                    ledger_path = Path(
                        f"{args.ledger_prefix}_{timeframe}_{breakout_shape}_{variant_name}_{exit_policy_name}_observed_close_ledger.csv"
                    )
                    result["ledger"].to_csv(ledger_path, index=False)
                    result["ledger_path"] = str(ledger_path)
                    del result["ledger"]
                    result["signal_stats"] = {
                        "signal_rows": int(len(signal)),
                        "run_signal_rows_with_right_boundary": int(len(run_signal)),
                        "signal_start": signal.index[0].isoformat() if not signal.empty else "",
                        "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
                        "valid_sma5_rows": int(signal["sma5"].notna().sum()) if "sma5" in signal else 0,
                        "valid_atr_rows": int(signal["atr"].notna().sum()) if "atr" in signal else 0,
                    }
                    acct = result["accounting_2bps_maker_entry_market_exit"]
                    print(
                        f"{timeframe} {breakout_shape} {variant_name} {exit_policy_name}: "
                        f"realistic={acct['realistic_return_pct']:.4f}% "
                        f"raw={acct['raw_no_fee_no_slip_return_pct']:.4f}% trades={acct['round_trips']} "
                        f"slots={acct.get('by_trade_slot', {})} guard={result['bar_guard_diagnostics']}",
                        flush=True,
                    )
                    results.append(result)

    summary_path = Path(args.summary_json)
    markdown_path = Path(args.markdown)
    summary = {
        "symbol": args.symbol,
        "start": start.isoformat(),
        "end": end.isoformat(),
        "timeframes": args.timeframes,
        "schedule_variants": [{"name": name, "schedule": schedule} for name, schedule in schedule_variants],
        "breakout_shapes": args.breakout_shapes,
        "exit_variants": exit_variants,
        "tick_files": args.tick_files,
        "build_stats": build_stats,
        "mode": {
            "name": "direct_breakout_observed_close",
            "breakout_shape": args.breakout_shapes,
            "entry": "first breakout per signal bar, filled at triggering 1s close",
            "allow_same_bar_second_breakout": bool(args.allow_same_bar_second_breakout),
            "vsl": "disabled",
            "re_p": "disabled",
            "reclaim_reentry": "disabled",
            "real_stop_atr": replay.COMMON_REPLAY_KWARGS["stop_loss_atr"],
            "exit_labels": {
                "InitialSL": f"initial {replay.COMMON_REPLAY_KWARGS['stop_loss_atr']:.2f} ATR stop",
                "ProfitLockSL": "stop after an MFE threshold moves the stop to entry or a configured positive bps lock",
                "TrailingSL": "stop after trailing logic moves the original stop",
                "PT": "structure profit-taking after profit protection is armed",
                "TPxATR": "full take-profit target at a configured ATR multiple",
            },
            "trailing_stop_atr": replay.COMMON_REPLAY_KWARGS["trailing_stop_atr"],
            "delayed_trailing_activation": replay.COMMON_REPLAY_KWARGS["delayed_trailing_activation"],
            "profit_protect_atr": replay.COMMON_REPLAY_KWARGS["profit_protect_atr"],
        },
        "baseline_kwargs": dict(replay.COMMON_REPLAY_KWARGS),
        "accounting_assumption": "Event path generated with fixed_slippage=2bps/side and commission=0; accounting recomputed as maker entry fee 2bps + market SL/exit fee 4bps.",
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
