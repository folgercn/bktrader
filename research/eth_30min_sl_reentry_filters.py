#!/usr/bin/env python3
"""ETH Q1 2026 30min 1s replay for SL-Reentry filters.

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
    _resolve_reentry_price,
    _resolve_regime_ready,
)
from eth_q1_breakout_t3_shape_compare import (
    COMMON_REPLAY_KWARGS,
    DEFAULT_TICK_FILES,
    _as_utc_timestamp,
    _intrabar_signal,
    _long_breakout,
    _open_position,
    _short_breakout,
    build_continuous_second_bars,
    build_signal_frame,
    summarize_run,
)


VARIANTS = [
    {
        "name": "baseline_sl_slot2",
        "description": "PR #261 sizing semantics only: SL-Reentry starts at schedule slot 2.",
    },
    {
        "name": "close_confirm",
        "description": "SL-Reentry requires reclaim by current 1s close.",
        "confirm": "close",
    },
    {
        "name": "close_confirm_buffer_0p02atr",
        "description": "SL-Reentry requires current 1s close reclaim with 0.02 ATR buffer.",
        "confirm": "close",
        "buffer_atr": 0.02,
    },
    {
        "name": "delay_10s",
        "description": "SL-Reentry and Zero-Initial-Reentry wait at least 10s after the SL exit.",
        "delay_seconds": 10,
    },
    {
        "name": "delay_20s",
        "description": "SL-Reentry and Zero-Initial-Reentry wait at least 20s after the SL exit.",
        "delay_seconds": 20,
    },
    {
        "name": "delay_30s",
        "description": "SL-Reentry and Zero-Initial-Reentry wait at least 30s after the SL exit.",
        "delay_seconds": 30,
    },
    {
        "name": "delay_60s",
        "description": "SL-Reentry and Zero-Initial-Reentry wait at least 60s after the SL exit.",
        "delay_seconds": 60,
    },
    {
        "name": "cooldown_1bar",
        "description": "SL-Reentry waits until the next signal bar after an SL exit.",
        "cooldown_bars": 1,
    },
    {
        "name": "close_confirm_delay_10s",
        "description": "SL-Reentry requires current 1s close reclaim and at least 10s delay.",
        "confirm": "close",
        "delay_seconds": 10,
    },
    {
        "name": "close_confirm_delay_30s",
        "description": "SL-Reentry requires current 1s close reclaim and at least 30s delay.",
        "confirm": "close",
        "delay_seconds": 30,
    },
]


def _sl_reentry_triggered(side: str, price: float, close_value: float, reentry_price: float, atr: float, variant: dict):
    buffer_value = float(variant.get("buffer_atr", 0.0) or 0.0) * float(atr)
    if str(variant.get("confirm", "")).lower() == "close":
        if side == "long":
            trigger = reentry_price + buffer_value
            return close_value >= trigger, close_value
        trigger = reentry_price - buffer_value
        return close_value <= trigger, close_value
    if side == "long":
        return price >= reentry_price, reentry_price
    return price <= reentry_price, reentry_price


def _zero_initial_triggered(side: str, high_value: float, low_value: float, reentry_price: float):
    if side == "long":
        return high_value >= reentry_price, reentry_price
    return low_value <= reentry_price, reentry_price


def _reentry_notional_share(reason: str, trades_in_bar: int, schedule: list[float]) -> float:
    if reason == "SL-Reentry" and len(schedule) > 1:
        return schedule[1]
    return _get_reentry_window_real_order_size(trades_in_bar, schedule)


def run_variant_replay(
    df_seconds: pd.DataFrame,
    signal: pd.DataFrame,
    *,
    initial_balance: float,
    variant: dict,
):
    balance = initial_balance
    position = None
    trade_logs = []
    diagnostics = {
        "breakout_locks": {"long": {}, "short": {}},
        "variant": variant,
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
    reentry_anchor_levels = str(COMMON_REPLAY_KWARGS["reentry_anchor_levels"])
    reentry_size_schedule = [float(v) for v in COMMON_REPLAY_KWARGS["reentry_size_schedule"]]
    delay_seconds = int(variant.get("delay_seconds", 0) or 0)
    cooldown_bars = int(variant.get("cooldown_bars", 0) or 0)

    last_exit_bar_index = -999
    reentry_timeout = 1
    last_exit_reason = None
    last_exit_side = None
    last_exit_time = None
    pending_zero_initial_side = None
    pending_zero_initial_bar_index = -999

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
        bar_high_so_far = -np.inf
        bar_low_so_far = np.inf
        live_sig = base_sig.to_dict()
        live_sig["_closed_atr"] = float(base_sig["atr"])

        if i - last_exit_bar_index > reentry_timeout:
            last_exit_side = None
            last_exit_time = None
        if i - pending_zero_initial_bar_index > reentry_timeout:
            pending_zero_initial_side = None

        while current_pos < end_pos:
            bar_time = second_index[current_pos]
            high_value = high_values[current_pos]
            low_value = low_values[current_pos]
            close_value = close_values[current_pos]
            bar_high_so_far = max(bar_high_so_far, high_value)
            bar_low_so_far = min(bar_low_so_far, low_value)
            sig = _intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)
            long_regime_ready, short_regime_ready = _resolve_regime_ready(sig, "1d")

            if position is None:
                if long_regime_ready:
                    triggered, _, shape_name = _long_breakout(sig, high_value, "baseline_plus_t3")
                    if trades_in_bar == 0 and triggered:
                        if not breakout_locked_this_bar:
                            diagnostics["breakout_locks"]["long"][shape_name] = (
                                diagnostics["breakout_locks"]["long"].get(shape_name, 0) + 1
                            )
                            breakout_locked_this_bar = True
                        pending_zero_initial_side = "long"
                        pending_zero_initial_bar_index = i

                    position, balance, trades_in_bar = _maybe_open_reentry(
                        side="long",
                        bar_time=bar_time,
                        high_value=high_value,
                        low_value=low_value,
                        close_value=close_value,
                        sig=sig,
                        has_zero_initial_window=(
                            pending_zero_initial_side == "long"
                            and (i - pending_zero_initial_bar_index <= reentry_timeout)
                        ),
                        has_exit_reentry_window=(
                            last_exit_side == "long"
                            and (i - last_exit_bar_index <= reentry_timeout)
                            and (i - last_exit_bar_index) >= cooldown_bars
                        ),
                        last_exit_reason=last_exit_reason,
                        last_exit_time=last_exit_time,
                        delay_seconds=delay_seconds,
                        trades_in_bar=trades_in_bar,
                        max_trades_per_bar=max_trades_per_bar,
                        reentry_size_schedule=reentry_size_schedule,
                        balance=balance,
                        slippage=slippage,
                        stop_mode=stop_mode,
                        stop_loss_atr=stop_loss_atr,
                        reentry_anchor_levels=reentry_anchor_levels,
                        reentry_atr=long_reentry_atr,
                        variant=variant,
                        trade_logs=trade_logs,
                    )
                    if position is not None:
                        if last_exit_side == "long":
                            last_exit_side = None
                        if pending_zero_initial_side == "long":
                            pending_zero_initial_side = None

                elif short_regime_ready:
                    triggered, _, shape_name = _short_breakout(sig, low_value, "baseline_plus_t3")
                    if trades_in_bar == 0 and triggered:
                        if not breakout_locked_this_bar:
                            diagnostics["breakout_locks"]["short"][shape_name] = (
                                diagnostics["breakout_locks"]["short"].get(shape_name, 0) + 1
                            )
                            breakout_locked_this_bar = True
                        pending_zero_initial_side = "short"
                        pending_zero_initial_bar_index = i

                    position, balance, trades_in_bar = _maybe_open_reentry(
                        side="short",
                        bar_time=bar_time,
                        high_value=high_value,
                        low_value=low_value,
                        close_value=close_value,
                        sig=sig,
                        has_zero_initial_window=(
                            pending_zero_initial_side == "short"
                            and (i - pending_zero_initial_bar_index <= reentry_timeout)
                        ),
                        has_exit_reentry_window=(
                            last_exit_side == "short"
                            and (i - last_exit_bar_index <= reentry_timeout)
                            and (i - last_exit_bar_index) >= cooldown_bars
                        ),
                        last_exit_reason=last_exit_reason,
                        last_exit_time=last_exit_time,
                        delay_seconds=delay_seconds,
                        trades_in_bar=trades_in_bar,
                        max_trades_per_bar=max_trades_per_bar,
                        reentry_size_schedule=reentry_size_schedule,
                        balance=balance,
                        slippage=slippage,
                        stop_mode=stop_mode,
                        stop_loss_atr=stop_loss_atr,
                        reentry_anchor_levels=reentry_anchor_levels,
                        reentry_atr=short_reentry_atr,
                        variant=variant,
                        trade_logs=trade_logs,
                    )
                    if position is not None:
                        if last_exit_side == "short":
                            last_exit_side = None
                        if pending_zero_initial_side == "short":
                            pending_zero_initial_side = None

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
                            "pnl": pnl,
                            "entry_reason": position.get("reason", ""),
                        }
                    )
                    last_exit_reason = reason
                    last_exit_side = position["side"]
                    last_exit_bar_index = i
                    last_exit_time = bar_time
                    position = None

            current_pos += 1

    if position is not None and not df_seconds.empty:
        last_bar_time = second_index[-1]
        last_close = float(close_values[-1])
        side_mult = 1 if position["side"] == "long" else -1
        final_exit_p = last_close * (1 - slippage) if position["side"] == "long" else last_close * (1 + slippage)
        pnl = (
            side_mult * (final_exit_p - position["entry_p"]) / position["entry_p"] * position["notional"]
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
                "pnl": pnl,
                "entry_reason": position.get("reason", ""),
            }
        )

    return pd.DataFrame(trade_logs), diagnostics


def _maybe_open_reentry(
    *,
    side: str,
    bar_time: pd.Timestamp,
    high_value: float,
    low_value: float,
    close_value: float,
    sig: dict,
    has_zero_initial_window: bool,
    has_exit_reentry_window: bool,
    last_exit_reason: str | None,
    last_exit_time: pd.Timestamp | None,
    delay_seconds: int,
    trades_in_bar: int,
    max_trades_per_bar: int,
    reentry_size_schedule: list[float],
    balance: float,
    slippage: float,
    stop_mode: str,
    stop_loss_atr: float,
    reentry_anchor_levels: str,
    reentry_atr: float,
    variant: dict,
    trade_logs: list[dict],
):
    if not has_zero_initial_window and not has_exit_reentry_window:
        return None, balance, trades_in_bar
    reentry_price = _resolve_reentry_price(sig, side, reentry_anchor_levels, reentry_atr)
    reason = "Zero-Initial-Reentry"
    is_triggered = False
    entry_p_raw = reentry_price
    seconds_since_exit = None
    if has_exit_reentry_window:
        reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
    if last_exit_time is not None:
        seconds_since_exit = (bar_time - last_exit_time).total_seconds()
        if seconds_since_exit < delay_seconds:
            return None, balance, trades_in_bar

    if reason == "SL-Reentry":
        trigger_price = high_value if side == "long" else low_value
        is_triggered, entry_p_raw = _sl_reentry_triggered(
            side,
            trigger_price,
            close_value,
            reentry_price,
            float(sig["atr"]),
            variant,
        )
    else:
        is_triggered, entry_p_raw = _zero_initial_triggered(side, high_value, low_value, reentry_price)
    if not is_triggered or trades_in_bar >= max_trades_per_bar:
        return None, balance, trades_in_bar

    notional_share = _reentry_notional_share(reason, trades_in_bar, reentry_size_schedule)
    entry_price = float(entry_p_raw) * (1 + slippage) if side == "long" else float(entry_p_raw) * (1 - slippage)
    balance, position = _open_position(
        balance,
        sig,
        side,
        entry_price,
        notional_share,
        reason,
        stop_mode,
        stop_loss_atr,
        "",
        "live_intrabar_sma5",
    )
    position["reason"] = reason
    trade_logs.append(
        {
            "time": bar_time,
            "type": "BUY" if side == "long" else "SHORT",
            "price": entry_price,
            "reason": reason,
            "notional": position["notional"],
            "bal": balance,
            "seconds_since_exit": seconds_since_exit,
        }
    )
    return position, balance, trades_in_bar + 1


def sl_reentry_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty:
        return {}
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])].reset_index(drop=True)
    exits = ledger[ledger["type"] == "EXIT"].reset_index(drop=True)
    sl_entries = entries[entries["reason"] == "SL-Reentry"].copy()
    if sl_entries.empty:
        return {
            "sl_reentry_count": 0,
            "fast_reentry_le_10s": 0,
            "fast_reentry_le_30s": 0,
            "fast_reentry_le_60s": 0,
            "sl_reentry_exit_sl_rate_pct": 0.0,
            "sl_reentry_avg_pnl": 0.0,
        }
    delays = pd.to_numeric(sl_entries.get("seconds_since_exit"), errors="coerce")
    paired = []
    entry_cursor = 0
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            entry_cursor += 1
            current_reason = row["reason"]
        elif row["type"] == "EXIT" and "current_reason" in locals():
            paired.append(
                {
                    "entry_reason": current_reason,
                    "exit_reason": row["reason"],
                    "pnl": float(row.get("pnl", 0.0) or 0.0),
                }
            )
    pairs = pd.DataFrame(paired)
    sl_pairs = pairs[pairs["entry_reason"] == "SL-Reentry"] if not pairs.empty else pairs
    sl_exit_count = int((sl_pairs["exit_reason"] == "SL").sum()) if not sl_pairs.empty else 0
    return {
        "sl_reentry_count": int(len(sl_entries)),
        "fast_reentry_le_10s": int((delays <= 10).sum()),
        "fast_reentry_le_30s": int((delays <= 30).sum()),
        "fast_reentry_le_60s": int((delays <= 60).sum()),
        "sl_reentry_exit_sl_rate_pct": round(sl_exit_count / len(sl_entries) * 100, 2),
        "sl_reentry_avg_pnl": round(float(sl_pairs["pnl"].mean()), 2) if not sl_pairs.empty else 0.0,
    }


def write_markdown(summary: dict, output_path: Path):
    lines = [
        "# ETH Q1 2026 30min SL-Reentry Filters, 1s Replay",
        "",
        "Scope: research-only backtest work. No live or execution path was changed.",
        "",
        "## Setup",
        "",
        "- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`",
        "- Execution bars: continuous `1s` bars rebuilt from raw Binance trades",
        "- Signal timeframe: `30min`",
        "- Replay mode: `live_intrabar_sma5` with `baseline_plus_t3` breakout shape",
        "- Sizing baseline includes PR #261 semantics: `Zero-Initial-Reentry=20%`, `SL-Reentry=10%` when schedule is `[0.20, 0.10]`.",
        "",
        "## Results",
        "",
        "| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | SL-Reentry | <=10s | <=30s | <=60s | SL-Reentry -> SL | Avg SL-Reentry PnL |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["sl_reentry_diagnostics"]
        lines.append(
            f"| `{result['variant']}` | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | "
            f"{d['sl_reentry_count']} | {d['fast_reentry_le_10s']} | {d['fast_reentry_le_30s']} | "
            f"{d['fast_reentry_le_60s']} | {d['sl_reentry_exit_sl_rate_pct']:.2f}% | {d['sl_reentry_avg_pnl']:,.2f} |"
        )
    lines.extend(["", "## Delta vs Baseline", ""])
    lines.append("| Variant | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta | SL-Reentry Delta |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|---:|")
    base = summary["results"][0]
    for result in summary["results"][1:]:
        s = result["summary"]
        b = base["summary"]
        d = result["sl_reentry_diagnostics"]
        bd = base["sl_reentry_diagnostics"]
        lines.append(
            f"| `{result['variant']}` | {s['final_balance'] - b['final_balance']:,.2f} | "
            f"{s['return_pct'] - b['return_pct']:.2f} pp | {s['max_dd_pct'] - b['max_dd_pct']:.2f} pp | "
            f"{s['trades'] - b['trades']} | {s['win_rate_pct'] - b['win_rate_pct']:.2f} pp | "
            f"{s['sharpe'] - b['sharpe']:.2f} | {d['sl_reentry_count'] - bd['sl_reentry_count']} |"
        )
    lines.extend(["", "## Variants", ""])
    for variant in VARIANTS:
        lines.append(f"- `{variant['name']}`: {variant['description']}")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH Q1 2026 30min SL-Reentry filter comparison")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--summary-json", default="research/eth_2026_q1_30min_sl_reentry_filters_summary.json")
    parser.add_argument("--markdown", default="research/20260428_eth_q1_30min_sl_reentry_filters.md")
    return parser.parse_args()


def main():
    args = parse_args()
    start = _as_utc_timestamp(args.start)
    end = _as_utc_timestamp(args.end)
    second_bars, build_stats = build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    _, signal = build_signal_frame(second_bars, "30min")

    results = []
    for variant in VARIANTS:
        started = time.time()
        ledger, diagnostics = run_variant_replay(
            second_bars,
            signal,
            initial_balance=args.initial_balance,
            variant=variant,
        )
        elapsed = round(time.time() - started, 2)
        ledger_path = Path(f"research/tmp_eth_2026_q1_30min_1s_sl_reentry_{variant['name']}_ledger.csv")
        ledger.to_csv(ledger_path, index=False)
        summary = summarize_run(ledger, args.initial_balance)
        sl_diag = sl_reentry_diagnostics(ledger)
        result = {
            "variant": variant["name"],
            "description": variant["description"],
            "params": variant,
            "summary": summary,
            "sl_reentry_diagnostics": sl_diag,
            "diagnostics": diagnostics,
            "ledger_path": str(ledger_path),
            "elapsed_seconds": elapsed,
        }
        results.append(result)
        print(
            f"{variant['name']}: return={summary['return_pct']:.2f}% trades={summary['trades']} "
            f"sl_reentry={sl_diag.get('sl_reentry_count', 0)} elapsed={elapsed}s",
            flush=True,
        )

    output = {
        "window": {"start": start.isoformat(), "end": end.isoformat()},
        "timeframe": "30min",
        "replay_mode": "live_intrabar_sma5",
        "breakout_shape": "baseline_plus_t3",
        "common_params": COMMON_REPLAY_KWARGS,
        "build_stats": build_stats,
        "signal_stats": {
            "signal_rows": int(len(signal)),
            "signal_start": signal.index[0].isoformat() if not signal.empty else "",
            "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
            "valid_sma5_rows": int(signal["sma5"].notna().sum()),
            "valid_atr_rows": int(signal["atr"].notna().sum()),
        },
        "results": results,
    }
    summary_path = Path(args.summary_json)
    summary_path.write_text(json.dumps(output, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    write_markdown(output, Path(args.markdown))
    print(json.dumps(output, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
