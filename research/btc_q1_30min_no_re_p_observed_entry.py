#!/usr/bin/env python3
"""BTC Q1 2026 30min no-re_p observed-entry research run.

Research-only script. It removes `re_p` from both fill price and primary
entry gating. Breakout creates only a virtual zero-notional state; the first
real exposure can happen only after that virtual state is stopped out and a
fresh observed breakout event appears after cooldown.
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


VARIANTS = [
    {
        "name": "virtual_sl_second_breakout_30s",
        "description": "Virtual Initial SL, then wait 30s and require a fresh close cross of the original breakout level; fill next 1s open.",
        "cooldown_seconds": 30,
        "trigger_mode": "fresh_cross",
        "fill_mode": "next_open",
    },
    {
        "name": "virtual_sl_second_breakout_60s",
        "description": "Virtual Initial SL, then wait 60s and require a fresh close cross of the original breakout level; fill next 1s open.",
        "cooldown_seconds": 60,
        "trigger_mode": "fresh_cross",
        "fill_mode": "next_open",
    },
    {
        "name": "virtual_sl_second_breakout_120s",
        "description": "Virtual Initial SL, then wait 120s and require a fresh close cross of the original breakout level; fill next 1s open.",
        "cooldown_seconds": 120,
        "trigger_mode": "fresh_cross",
        "fill_mode": "next_open",
    },
    {
        "name": "virtual_sl_acceptance_60s",
        "description": "Upper-bound variant: Virtual Initial SL, wait 60s, enter if close is still beyond the breakout level; fill next 1s open.",
        "cooldown_seconds": 60,
        "trigger_mode": "acceptance",
        "fill_mode": "next_open",
    },
]


def _entry_mix(summary: dict) -> str:
    return ", ".join(f"{k}:{v}" for k, v in summary["entry_reasons"].items())


def _shape_schedule(shape_name: str) -> list[float]:
    # Gates are intentionally removed for this baseline; t3 shares the same
    # real-order schedule unless future research explicitly changes sizing.
    _ = shape_name
    return [float(v) for v in replay.COMMON_REPLAY_KWARGS["reentry_size_schedule"]]


def _open_observed_position(
    balance: float,
    sig,
    side: str,
    fill_raw: float,
    notional_share: float,
    reason: str,
    breakout_shape_name: str,
    breakout_level: float,
):
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    entry_price = float(fill_raw) * (1 + slippage if side == "long" else 1 - slippage)
    notional = balance * notional_share
    position = {
        "side": side,
        "entry_p": entry_price,
        "sl": replay._resolve_stop_price(
            side,
            entry_price,
            sig,
            str(replay.COMMON_REPLAY_KWARGS["stop_mode"]),
            float(replay.COMMON_REPLAY_KWARGS["stop_loss_atr"]),
        ),
        "protected": reason == "PT-Reentry",
        "notional": notional,
        "breakout_shape_name": breakout_shape_name,
        "breakout_level": float(breakout_level),
    }
    if side == "long":
        position["hwm"] = entry_price
    else:
        position["lwm"] = entry_price
    balance -= notional * 0.001
    return balance, position


def _new_virtual_position(sig, side: str, entry_raw: float, breakout_shape_name: str, breakout_level: float, lock_time):
    entry_p = float(entry_raw)
    virtual = {
        "side": side,
        "entry_p": entry_p,
        "sl": replay._resolve_stop_price(
            side,
            entry_p,
            sig,
            str(replay.COMMON_REPLAY_KWARGS["stop_mode"]),
            float(replay.COMMON_REPLAY_KWARGS["stop_loss_atr"]),
        ),
        "protected": False,
        "notional": 0.0,
        "breakout_shape_name": breakout_shape_name,
        "breakout_level": float(breakout_level),
        "lock_time": lock_time,
    }
    if side == "long":
        virtual["hwm"] = entry_p
    else:
        virtual["lwm"] = entry_p
    return virtual


def _advance_position_state(position: dict, sig, high_value: float, low_value: float):
    trailing_stop_atr = float(replay.COMMON_REPLAY_KWARGS["trailing_stop_atr"])
    delayed_trailing_activation = float(replay.COMMON_REPLAY_KWARGS["delayed_trailing_activation"])
    profit_protect_atr = float(replay.COMMON_REPLAY_KWARGS["profit_protect_atr"])

    exit_triggered = False
    exit_p = 0.0
    reason = ""

    if position["side"] == "long":
        prev_hwm = position.get("hwm", position["entry_p"])
        protected_before_bar = bool(position.get("protected", False))

        profit_atr = (prev_hwm - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0
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
            profit_atr = (position["hwm"] - position["entry_p"]) / sig["atr"] if sig["atr"] > 0 else 0
            if profit_atr >= delayed_trailing_activation:
                position["sl"] = max(position["sl"], position["hwm"] - trailing_stop_atr * sig["atr"])

    else:
        prev_lwm = position.get("lwm", position["entry_p"])
        protected_before_bar = bool(position.get("protected", False))

        profit_atr = (position["entry_p"] - prev_lwm) / sig["atr"] if sig["atr"] > 0 else 0
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
            profit_atr = (position["entry_p"] - position["lwm"]) / sig["atr"] if sig["atr"] > 0 else 0
            if profit_atr >= delayed_trailing_activation:
                position["sl"] = min(position["sl"], position["lwm"] + trailing_stop_atr * sig["atr"])

    return exit_triggered, float(exit_p), reason


def _new_pending_from_exit(source: dict, reason: str, bar_index: int, exit_time, cooldown_seconds: int):
    return {
        "side": source["side"],
        "reason": "Zero-Initial-Reentry" if source.get("notional", 0.0) <= 0 else ("SL-Reentry" if reason == "SL" else "PT-Reentry"),
        "exit_reason": reason,
        "bar_index": int(bar_index),
        "ready_time": pd.Timestamp(exit_time) + pd.Timedelta(seconds=int(cooldown_seconds if reason == "SL" else 0)),
        "breakout_shape_name": source.get("breakout_shape_name", ""),
        "breakout_level": float(source["breakout_level"]),
        "lock_time": source.get("lock_time", exit_time),
        "source_exit_time": exit_time,
    }


def _pending_triggered(pending: dict, close_value: float, prev_close: float | None, trigger_mode: str) -> bool:
    level = float(pending["breakout_level"])
    side = pending["side"]
    if trigger_mode == "acceptance":
        return close_value >= level if side == "long" else close_value <= level

    if prev_close is None or pd.isna(prev_close):
        return False
    if side == "long":
        return prev_close < level <= close_value
    return prev_close > level >= close_value


def _fill_raw_for_mode(open_values, close_values, current_pos: int, fill_mode: str):
    if fill_mode == "same_close":
        return current_pos, float(close_values[current_pos])
    fill_pos = current_pos + 1
    if fill_pos >= len(open_values):
        return None, None
    return fill_pos, float(open_values[fill_pos])


def _no_re_p_diagnostics(ledger: pd.DataFrame, pairs: pd.DataFrame) -> dict:
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
        return {
            "entry_reason_stats": reason_stats,
            "median_seconds_from_lock": None,
            "median_seconds_from_exit": None,
            "median_entry_distance_bps": None,
            "median_entry_distance_atr": None,
            "side_pnl": {},
        }

    side_pnl = {}
    if not pairs.empty:
        for entry_type, group in pairs.groupby("entry_type"):
            side_pnl[str(entry_type)] = {
                "trades": int(len(group)),
                "win_rate_pct": round(float((group["pnl_value"] > 0).mean()) * 100.0, 2),
                "net_pnl": round(float(group["pnl_value"].sum()), 2),
            }

    return {
        "entry_reason_stats": reason_stats,
        "median_seconds_from_lock": round(float(entries["seconds_from_lock"].dropna().median()), 2)
        if "seconds_from_lock" in entries and entries["seconds_from_lock"].notna().any()
        else None,
        "median_seconds_from_exit": round(float(entries["seconds_from_exit"].dropna().median()), 2)
        if "seconds_from_exit" in entries and entries["seconds_from_exit"].notna().any()
        else None,
        "median_entry_distance_bps": round(float(entries["entry_distance_bps"].dropna().median()), 4)
        if "entry_distance_bps" in entries and entries["entry_distance_bps"].notna().any()
        else None,
        "median_entry_distance_atr": round(float(entries["entry_distance_atr"].dropna().median()), 4)
        if "entry_distance_atr" in entries and entries["entry_distance_atr"].notna().any()
        else None,
        "side_pnl": side_pnl,
    }


def run_no_re_p_variant(second_bars: pd.DataFrame, signal: pd.DataFrame, variant: dict, initial_balance: float) -> dict:
    started = time.time()
    balance = initial_balance
    position = None
    virtual = None
    pending = None
    trade_logs = []
    diagnostics = {
        "breakout_locks": {"long": {}, "short": {}},
        "virtual_exits": {"SL": 0, "PT": 0},
        "pending_expired": 0,
        "pending_triggered": 0,
        "pending_unfilled_end_of_data": 0,
        "real_exit_reentries_armed": {"SL": 0, "PT": 0},
    }

    second_index = second_bars.index
    open_values = second_bars["open"].to_numpy(dtype="float64", copy=False)
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)

    max_trades_per_bar = int(replay.COMMON_REPLAY_KWARGS["max_trades_per_bar"])
    slippage = float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"])
    reentry_timeout = 1
    cooldown_seconds = int(variant["cooldown_seconds"])
    trigger_mode = str(variant["trigger_mode"])
    fill_mode = str(variant["fill_mode"])

    for i in range(len(signal) - 1):
        start_t, end_t = signal.index[i], signal.index[i + 1]
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="right"))
        if start_pos >= end_pos:
            continue

        base_sig = signal.iloc[i]
        if pd.isna(base_sig["atr"]):
            continue

        if pending is not None and i - pending["bar_index"] > reentry_timeout:
            pending = None
            diagnostics["pending_expired"] += 1

        trades_in_bar = 0
        current_pos = start_pos
        bar_high_so_far = -np.inf
        bar_low_so_far = np.inf
        live_sig = base_sig.to_dict()
        live_sig["_closed_atr"] = float(base_sig["atr"])
        sig = base_sig

        while current_pos < end_pos:
            bar_time = second_index[current_pos]
            high_value = float(high_values[current_pos])
            low_value = float(low_values[current_pos])
            close_value = float(close_values[current_pos])
            prev_close = float(close_values[current_pos - 1]) if current_pos > 0 else None

            bar_high_so_far = max(bar_high_so_far, high_value)
            bar_low_so_far = min(bar_low_so_far, low_value)
            sig = replay._intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)
            long_regime_ready, short_regime_ready = replay._resolve_regime_ready(sig, "1d")

            if position is not None:
                exit_triggered, exit_p, reason = _advance_position_state(position, sig, high_value, low_value)
                if exit_triggered:
                    exit_shape = position.get("breakout_shape_name", "")
                    side_mult = 1 if position["side"] == "long" else -1
                    exit_p = exit_p * (1 - slippage) if position["side"] == "long" else exit_p * (1 + slippage)
                    pnl = side_mult * (exit_p - position["entry_p"]) / position["entry_p"] * position["notional"]
                    balance += pnl - position["notional"] * 0.001
                    trade_logs.append(
                        {
                            "time": bar_time,
                            "type": "EXIT",
                            "price": exit_p,
                            "reason": reason,
                            "notional": position["notional"],
                            "bal": balance,
                            "breakout_shape_name": exit_shape,
                            "breakout_level": position.get("breakout_level", np.nan),
                        }
                    )
                    pending = _new_pending_from_exit(position, reason, i, bar_time, cooldown_seconds)
                    diagnostics["real_exit_reentries_armed"][reason] = diagnostics["real_exit_reentries_armed"].get(reason, 0) + 1
                    position = None
                current_pos += 1
                continue

            if virtual is not None:
                exit_triggered, _, reason = _advance_position_state(virtual, sig, high_value, low_value)
                if exit_triggered:
                    diagnostics["virtual_exits"][reason] = diagnostics["virtual_exits"].get(reason, 0) + 1
                    if reason == "SL":
                        pending = _new_pending_from_exit(virtual, reason, i, bar_time, cooldown_seconds)
                    virtual = None
                current_pos += 1
                continue

            if pending is not None:
                if bar_time >= pending["ready_time"] and _pending_triggered(pending, close_value, prev_close, trigger_mode):
                    fill_pos, fill_raw = _fill_raw_for_mode(open_values, close_values, current_pos, fill_mode)
                    if fill_pos is None:
                        diagnostics["pending_unfilled_end_of_data"] += 1
                        pending = None
                        current_pos += 1
                        continue
                    active_schedule = _shape_schedule(pending["breakout_shape_name"])
                    if trades_in_bar < max_trades_per_bar:
                        notional_share = replay._get_reentry_window_real_order_size(trades_in_bar, active_schedule)
                        balance, position = _open_observed_position(
                            balance,
                            sig,
                            pending["side"],
                            fill_raw,
                            notional_share,
                            pending["reason"],
                            pending["breakout_shape_name"],
                            pending["breakout_level"],
                        )
                        entry_time = second_index[fill_pos]
                        entry_distance = (
                            (position["entry_p"] - pending["breakout_level"]) / pending["breakout_level"]
                            if pending["side"] == "long"
                            else (pending["breakout_level"] - position["entry_p"]) / pending["breakout_level"]
                        )
                        atr = float(sig.get("atr", np.nan))
                        trade_logs.append(
                            {
                                "time": entry_time,
                                "type": "BUY" if pending["side"] == "long" else "SHORT",
                                "price": position["entry_p"],
                                "reason": pending["reason"],
                                "notional": position["notional"],
                                "bal": balance,
                                "breakout_shape_name": position["breakout_shape_name"],
                                "breakout_level": pending["breakout_level"],
                                "lock_time": pending["lock_time"],
                                "source_exit_time": pending["source_exit_time"],
                                "seconds_from_lock": (pd.Timestamp(entry_time) - pd.Timestamp(pending["lock_time"])).total_seconds(),
                                "seconds_from_exit": (pd.Timestamp(entry_time) - pd.Timestamp(pending["source_exit_time"])).total_seconds(),
                                "entry_distance_bps": entry_distance * 10000.0,
                                "entry_distance_atr": abs(position["entry_p"] - pending["breakout_level"]) / atr
                                if np.isfinite(atr) and atr > 0
                                else np.nan,
                            }
                        )
                        trades_in_bar += 1
                        diagnostics["pending_triggered"] += 1
                    pending = None
                    current_pos = max(current_pos + 1, fill_pos + 1)
                    continue
                current_pos += 1
                continue

            if long_regime_ready:
                triggered, breakout_level, shape_name = replay._long_breakout(sig, close_value, "baseline_plus_t3")
                if triggered:
                    diagnostics["breakout_locks"]["long"][shape_name] = diagnostics["breakout_locks"]["long"].get(shape_name, 0) + 1
                    virtual = _new_virtual_position(sig, "long", close_value, shape_name, breakout_level, bar_time)
            elif short_regime_ready:
                triggered, breakout_level, shape_name = replay._short_breakout(sig, close_value, "baseline_plus_t3")
                if triggered:
                    diagnostics["breakout_locks"]["short"][shape_name] = diagnostics["breakout_locks"]["short"].get(shape_name, 0) + 1
                    virtual = _new_virtual_position(sig, "short", close_value, shape_name, breakout_level, bar_time)

            current_pos += 1

    if position is not None and not second_bars.empty:
        last_bar_time = second_index[-1]
        last_close = float(close_values[-1])
        side_mult = 1 if position["side"] == "long" else -1
        final_exit_p = last_close * (1 - slippage) if position["side"] == "long" else last_close * (1 + slippage)
        pnl = side_mult * (final_exit_p - position["entry_p"]) / position["entry_p"] * position["notional"]
        balance += pnl - position["notional"] * 0.001
        trade_logs.append(
            {
                "time": last_bar_time,
                "type": "EXIT",
                "price": final_exit_p,
                "reason": "FinalMarkToMarket",
                "notional": position["notional"],
                "bal": balance,
                "breakout_shape_name": position.get("breakout_shape_name", ""),
                "breakout_level": position.get("breakout_level", np.nan),
            }
        )

    ledger = pd.DataFrame(trade_logs)
    pairs = _paired_trades(ledger)
    return {
        "variant": variant["name"],
        "description": variant["description"],
        "params": variant,
        "summary": replay.summarize_run(ledger, initial_balance),
        "pair_diagnostics": _pair_diagnostics(pairs),
        "no_re_p_diagnostics": _no_re_p_diagnostics(ledger, pairs),
        "attribution": replay.summarize_breakout_attribution(ledger),
        "diagnostics": diagnostics,
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        "# BTCUSDT Q1 2026 30min No-re_p Observed Entry",
        "",
        "Scope: research-only Python replay. No live or execution path is changed by this report.",
        "",
        "## Setup",
        "",
        f"- Symbol/window: `{summary['symbol']}`, `{summary['start']}` to `{summary['end']}`",
        "- Execution bars: continuous `1s` bars rebuilt from Binance trade archives",
        "- Signal timeframe: `30min`",
        "- Breakout shape: `baseline_plus_t3`",
        f"- Fixed stop: `stop_mode={summary['baseline_kwargs']['stop_mode']}`, `stop_loss_atr={summary['baseline_kwargs']['stop_loss_atr']}`",
        f"- Trailing stop: `trailing_stop_atr={summary['baseline_kwargs']['trailing_stop_atr']}`, activated after `{summary['baseline_kwargs']['delayed_trailing_activation']} ATR` unrealized profit",
        f"- Profit protection: `profit_protect_atr={summary['baseline_kwargs']['profit_protect_atr']}`",
        f"- Sizing: `reentry_size_schedule={summary['baseline_kwargs']['reentry_size_schedule']}`, `max_trades_per_bar={summary['baseline_kwargs']['max_trades_per_bar']}`",
        "- Optimization gates: removed",
        "- Entry semantics: first breakout creates only a virtual zero-notional Initial state; real entry requires virtual/real SL, cooldown, and a post-cooldown observed breakout-level event",
        "- `re_p` usage: none for fill, none for entry trigger, none for actionability",
        "",
        "## Results",
        "",
        "| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Lock->Entry Median | Exit->Entry Median |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        nd = result["no_re_p_diagnostics"]
        lock_med = nd["median_seconds_from_lock"]
        exit_med = nd["median_seconds_from_exit"]
        lines.append(
            f"| `{result['variant']}` | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | "
            f"{d['avg_loss_pct']:.4f}% | {d['worst_loss_pct']:.4f}% | `{_entry_mix(s)}` | "
            f"{lock_med if lock_med is not None else ''} | {exit_med if exit_med is not None else ''} |"
        )

    lines.extend(["", "## Delta vs 60s Fresh-cross", ""])
    lines.append("| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"]:
        if result["variant"] == summary["comparison_base"]:
            continue
        delta = result["delta_vs_base"]
        lines.append(
            f"| `{result['variant']}` | {delta['final_balance_delta']:,.2f} | "
            f"{delta['return_pct_delta']:.2f} pp | {delta['max_dd_pct_delta']:.2f} pp | "
            f"{delta['trades_delta']} | {delta['win_rate_pct_delta']:.2f} pp | {delta['sharpe_delta']:.2f} |"
        )

    lines.extend(["", "## Entry Reason Diagnostics", ""])
    lines.append("| Variant | Reason | Trades | Win Rate | Net PnL | Avg PnL |")
    lines.append("|---|---|---:|---:|---:|---:|")
    for result in summary["results"]:
        for reason, stats in result["no_re_p_diagnostics"]["entry_reason_stats"].items():
            lines.append(
                f"| `{result['variant']}` | `{reason}` | {stats['trades']} | {stats['win_rate_pct']:.2f}% | "
                f"{stats['net_pnl']:,.2f} | {stats['avg_pnl_pct']:.4f}% |"
            )

    lines.extend(["", "## Diagnostics", ""])
    for result in summary["results"]:
        diag = result["diagnostics"]
        lines.append(f"- `{result['variant']}`: locks={diag['breakout_locks']}, virtual_exits={diag['virtual_exits']}, pending_expired={diag['pending_expired']}, pending_triggered={diag['pending_triggered']}")

    lines.extend(
        [
            "",
            "## Read",
            "",
            "These rows intentionally do not test `re_p`. They test whether the old lesson can be kept: first breakout is structure proof only, and real exposure waits for a later observed event.",
            "",
        ]
    )
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC Q1 2026 30min no-re_p observed-entry replay")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--variants", nargs="+", default=[item["name"] for item in VARIANTS])
    parser.add_argument("--summary-json", default="research/btc_2026_q1_30min_no_re_p_observed_entry_summary.json")
    parser.add_argument("--markdown", default="research/20260505_btc_q1_30min_no_re_p_observed_entry.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_2026_q1_30min_no_re_p_observed_entry")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    selected = [item for item in VARIANTS if item["name"] in set(args.variants)]
    if not selected:
        raise ValueError("no variants selected")

    replay.COMMON_REPLAY_KWARGS.clear()
    replay.COMMON_REPLAY_KWARGS.update(BTC_LIVE_LIKE_REPLAY_KWARGS)

    start = replay._as_utc_timestamp(args.start)
    end = replay._as_utc_timestamp(args.end)
    second_bars, build_stats = replay.build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    _, signal = replay.build_signal_frame(second_bars, "30min")

    results = []
    for variant in selected:
        result = run_no_re_p_variant(second_bars, signal, variant, args.initial_balance)
        ledger_path = Path(f"{args.ledger_prefix}_{variant['name']}_ledger.csv")
        result["ledger"].to_csv(ledger_path, index=False)
        del result["ledger"]
        result["ledger_path"] = str(ledger_path)
        results.append(result)
        s = result["summary"]
        print(
            f"{variant['name']}: return={s['return_pct']:.2f}% trades={s['trades']} "
            f"win={s['win_rate_pct']:.2f}% max_dd={s['max_dd_pct']:.2f}% "
            f"elapsed={result['elapsed_seconds']}s",
            flush=True,
        )

    base_name = "virtual_sl_second_breakout_60s"
    base_summary = next((r["summary"] for r in results if r["variant"] == base_name), results[0]["summary"])
    for result in results:
        result["delta_vs_base"] = _delta(base_summary, result["summary"])

    output = {
        "symbol": "BTCUSDT",
        "start": start.isoformat(),
        "end": end.isoformat(),
        "build_stats": build_stats,
        "signal_stats": {
            "signal_rows": int(len(signal)),
            "signal_start": signal.index[0].isoformat() if not signal.empty else "",
            "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
            "valid_sma5_rows": int(signal["sma5"].notna().sum()),
            "valid_atr_rows": int(signal["atr"].notna().sum()),
            "valid_atr_percentile_rows": int(signal["atr_percentile"].notna().sum()),
        },
        "baseline_kwargs": dict(BTC_LIVE_LIKE_REPLAY_KWARGS),
        "comparison_base": base_name,
        "variants": selected,
        "results": results,
        "note": "Research-only no-re_p observed-entry run. re_p is not used for fill, trigger, or actionability.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
