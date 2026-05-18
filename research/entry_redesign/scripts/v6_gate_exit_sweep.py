#!/usr/bin/env python3
"""V6 Gate + D=5 entry: exit parameter sweep.

Keeps V6 gate selection (116 events) and D=5 tick entry fixed.
Sweeps exit parameters to find combinations that produce positive calendar sum.

Usage:
    python research/entry_redesign/scripts/v6_gate_exit_sweep.py
"""

from __future__ import annotations

import itertools
import json
import sys
import time
from pathlib import Path

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[3]

V6_LEDGER_BASE = (
    PROJECT_ROOT / "research" / "probabilistic_v6_runs"
    / "walkforward_2025m06_2026apr_combo_baseline_short_speed"
    / "union_lifecycle_reentry_window_candidate_001_calendar_holdout"
    / "power0_fixed_1p30"
)
EVENTS_CSV = (
    PROJECT_ROOT / "research" / "probabilistic_v6_runs"
    / "2025m03_2026apr_original_t2_delay60" / "events_execution_labeled.csv"
)
BARS_CACHE_DIR = (
    PROJECT_ROOT / "research" / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest" / "bars_cache"
)
OUTPUT_DIR = PROJECT_ROOT / "research" / "entry_redesign" / "scripts" / "output"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

ENTRY_DELAY_SECONDS = 5
INITIAL_BALANCE = 100000.0
SIZING_SCALE = 1.3
REENTRY_SIZE_SCHEDULE = [0.20, 0.10]
MAX_TRADES_PER_BAR = 2

# Fixed params
FIXED_PARAMS = {
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.80,
    "min_stop_bps": 12.0,
    "cost_lock_bps": 10.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
}

# Sweep grid
SWEEP = {
    "initial_stop_atr": [0.30, 0.45, 0.60],
    "breakeven_at_r": [0.6, 0.8, 1.0],
    "trail_start_r": [0.9, 1.2, 1.5],
    "trail_buffer_atr": [0.05, 0.10, 0.20],
    "max_hold_hours": [4.0, 8.0, 12.0],
}


# ---------------------------------------------------------------------------
# Data loading (reuse from previous script)
# ---------------------------------------------------------------------------
def find_bars_cache(symbol: str, event_time: pd.Timestamp) -> Path | None:
    candidates = list(BARS_CACHE_DIR.glob(f"{symbol}_*_flow_1s.pkl"))
    for p in candidates:
        parts = p.stem.replace("_flow_1s", "").split("_")
        if len(parts) < 3:
            continue
        try:
            cache_start = pd.Timestamp(parts[1], tz="UTC")
            cache_end = pd.Timestamp(parts[2], tz="UTC")
            if cache_start <= event_time <= cache_end:
                return p
        except Exception:
            continue
    return None


def load_monthly_bars(symbol: str, month_start: pd.Timestamp) -> pd.DataFrame | None:
    month_end = (month_start + pd.offsets.MonthEnd(0)).replace(hour=23, minute=59, second=59)
    start_key = month_start.strftime("%Y%m%dT%H%M%S")
    end_key = month_end.strftime("%Y%m%dT%H%M%S")
    exact = BARS_CACHE_DIR / f"{symbol}_{start_key}_{end_key}_flow_1s.pkl"
    if exact.exists():
        return pd.read_pickle(exact)
    cache_path = find_bars_cache(symbol, month_start + pd.Timedelta(days=15))
    if cache_path is not None:
        return pd.read_pickle(cache_path)
    return None


def extract_v6_gate_entries() -> pd.DataFrame:
    entries = []
    for execute_dir in sorted(V6_LEDGER_BASE.glob("execute_*")):
        for symbol_dir in sorted(execute_dir.iterdir()):
            if not symbol_dir.is_dir():
                continue
            ledger_path = symbol_dir / "lifecycle_ledger.csv"
            if not ledger_path.exists():
                continue
            df = pd.read_csv(ledger_path, parse_dates=["time"])
            entry_rows = df[df["type"] != "EXIT"]
            for _, row in entry_rows.iterrows():
                entries.append({
                    "symbol": symbol_dir.name,
                    "touch_time": pd.Timestamp(row["time"]),
                    "side": "short" if "SHORT" in str(row["type"]) else "long",
                    "entry_reason": str(row["reason"]),
                })
    result = pd.DataFrame(entries)
    if not result.empty:
        result["touch_time"] = pd.to_datetime(result["touch_time"], utc=True)
    return result


def match_events(gate_entries: pd.DataFrame, all_events: pd.DataFrame) -> pd.DataFrame:
    all_events = all_events.copy()
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)
    matched = []
    for _, entry in gate_entries.iterrows():
        candidates = all_events[
            (all_events["symbol"] == entry["symbol"])
            & (all_events["side"] == entry["side"])
            & ((all_events["touch_time"] - entry["touch_time"]).abs() <= pd.Timedelta(seconds=2))
        ]
        if not candidates.empty:
            idx = (candidates["touch_time"] - entry["touch_time"]).abs().idxmin()
            matched.append(candidates.loc[idx])
    if not matched:
        return pd.DataFrame()
    return pd.DataFrame(matched).drop_duplicates(subset=["event_id"])


# ---------------------------------------------------------------------------
# Execution
# ---------------------------------------------------------------------------
def open_position(event, entry_price, entry_time, balance, notional_share, params):
    atr = float(event["atr"])
    side = str(event["side"])
    slippage = params["slippage"]
    entry_p = entry_price * (1.0 + slippage) if side == "long" else entry_price * (1.0 - slippage)

    sig_low = float(event.get("touch_low_so_far", event.get("signal_low", 0)))
    sig_high = float(event.get("touch_high_so_far", event.get("signal_high", 0)))

    if side == "long":
        raw_stop = min(sig_low - params["stop_buffer_atr"] * atr, entry_p - params["initial_stop_atr"] * atr)
        capped_stop = entry_p - params["stop_cap_atr"] * atr
        stop = max(raw_stop, capped_stop)
        risk = entry_p - stop
    else:
        raw_stop = max(sig_high + params["stop_buffer_atr"] * atr, entry_p + params["initial_stop_atr"] * atr)
        capped_stop = entry_p + params["stop_cap_atr"] * atr
        stop = min(raw_stop, capped_stop)
        risk = stop - entry_p

    if risk <= 0 or risk < entry_p * params["min_stop_bps"] / 10000.0:
        return None
    return {
        "side": side, "entry_time": entry_time, "entry_p": entry_p,
        "sl": stop, "risk": risk, "atr": atr,
        "notional_share": notional_share,
        "protected": False, "trailing_active": False,
        "hwm": entry_p, "lwm": entry_p, "mfe_r": 0.0, "mae_r": 0.0,
    }


def simulate_position(pos, second_bars, params):
    entry_time = pos["entry_time"]
    end_time = entry_time + pd.Timedelta(hours=params["max_hold_hours"])
    mask = (second_bars.index > entry_time) & (second_bars.index <= end_time)
    bars = second_bars.loc[mask]
    if bars.empty:
        return _exit(pos, pos["entry_p"], entry_time, "NoData", params)

    entry_p, risk = pos["entry_p"], pos["risk"]
    highs = bars["high"].values.astype(np.float64)
    lows = bars["low"].values.astype(np.float64)
    closes = bars["close"].values.astype(np.float64)
    timestamps = bars.index

    for i in range(len(bars)):
        h, l = highs[i], lows[i]
        if pos["side"] == "long":
            pos["hwm"] = max(pos["hwm"], h)
            favorable = max(0.0, pos["hwm"] - entry_p)
            adverse = max(0.0, entry_p - l)
        else:
            pos["lwm"] = min(pos["lwm"], l)
            favorable = max(0.0, entry_p - pos["lwm"])
            adverse = max(0.0, h - entry_p)
        pos["mfe_r"] = max(pos["mfe_r"], favorable / risk)
        pos["mae_r"] = max(pos["mae_r"], adverse / risk)

        # Breakeven
        if favorable / risk >= params["breakeven_at_r"]:
            if pos["side"] == "long":
                be_sl = entry_p * (1.0 + params["cost_lock_bps"] / 10000.0)
                if be_sl > pos["sl"]:
                    pos["sl"] = be_sl
                    pos["protected"] = True
            else:
                be_sl = entry_p * (1.0 - params["cost_lock_bps"] / 10000.0)
                if be_sl < pos["sl"]:
                    pos["sl"] = be_sl
                    pos["protected"] = True

        # Trailing
        if pos["mfe_r"] >= params["trail_start_r"]:
            atr = pos["atr"]
            if pos["side"] == "long":
                trail = pos["hwm"] - params["trail_buffer_atr"] * atr
                if trail > pos["sl"]:
                    pos["sl"] = trail
                    pos["trailing_active"] = True
            else:
                trail = pos["lwm"] + params["trail_buffer_atr"] * atr
                if trail < pos["sl"]:
                    pos["sl"] = trail
                    pos["trailing_active"] = True

        # Stop
        if pos["side"] == "long" and l <= pos["sl"]:
            reason = "TrailingSL" if pos["trailing_active"] else ("BreakevenSL" if pos["protected"] else "InitialSL")
            return _exit(pos, pos["sl"], timestamps[i], reason, params)
        if pos["side"] == "short" and h >= pos["sl"]:
            reason = "TrailingSL" if pos["trailing_active"] else ("BreakevenSL" if pos["protected"] else "InitialSL")
            return _exit(pos, pos["sl"], timestamps[i], reason, params)

    return _exit(pos, closes[-1], timestamps[-1], "MaxHoldExit", params)


def _exit(pos, raw_exit, exit_time, reason, params):
    slippage = params["slippage"]
    if pos["side"] == "long":
        exit_p = raw_exit * (1.0 - slippage)
        pnl_pct = (exit_p - pos["entry_p"]) / pos["entry_p"]
    else:
        exit_p = raw_exit * (1.0 + slippage)
        pnl_pct = (pos["entry_p"] - exit_p) / pos["entry_p"]
    fee_rate = params["entry_fee"] + params["exit_fee"]
    return {
        "pnl_pct": pnl_pct,
        "realistic_pnl_pct": pnl_pct - fee_rate,
        "exit_reason": reason,
        "notional_share": pos["notional_share"],
        "exit_time": exit_time,
        "entry_time": pos["entry_time"],
        "side": pos["side"],
        "mfe_r": pos["mfe_r"],
    }


def run_backtest(events, bars_cache, params):
    """Run backtest with given params, return list of trades."""
    all_trades = []
    for symbol, sym_events in events.groupby("symbol"):
        balance = INITIAL_BALANCE
        last_exit_time = pd.Timestamp.min.tz_localize("UTC")

        for _, event in sym_events.sort_values("touch_time").iterrows():
            touch_time = pd.Timestamp(event["touch_time"])
            if touch_time.tzinfo is None:
                touch_time = touch_time.tz_localize("UTC")
            if touch_time <= last_exit_time:
                continue

            month_key = f"{symbol}_{touch_time.strftime('%Y-%m')}"
            second_bars = bars_cache.get(month_key)
            if second_bars is None or second_bars.empty:
                continue

            trades_in_event = 0
            current_entry_time = touch_time + pd.Timedelta(seconds=ENTRY_DELAY_SECONDS)

            while trades_in_event < MAX_TRADES_PER_BAR:
                entry_mask = second_bars.index >= current_entry_time
                if not entry_mask.any():
                    break
                entry_idx = second_bars.index[entry_mask][0]
                entry_price = float(second_bars.loc[entry_idx, "close"])
                notional_share = REENTRY_SIZE_SCHEDULE[min(trades_in_event, len(REENTRY_SIZE_SCHEDULE) - 1)] * SIZING_SCALE

                pos = open_position(event, entry_price, entry_idx, balance, notional_share, params)
                if pos is None:
                    break

                trade = simulate_position(pos, second_bars, params)
                trade["symbol"] = symbol
                balance += balance * trade["notional_share"] * trade["realistic_pnl_pct"]
                all_trades.append(trade)
                last_exit_time = pd.Timestamp(trade["exit_time"])
                trades_in_event += 1

                if trade["exit_reason"] != "InitialSL":
                    break
                current_entry_time = pd.Timestamp(trade["exit_time"]) + pd.Timedelta(seconds=1)

    return all_trades


def compute_calendar_sum(trades):
    if not trades:
        return 0.0, 0
    df = pd.DataFrame(trades)
    df["entry_time"] = pd.to_datetime(df["entry_time"], utc=True)
    df["month"] = df["entry_time"].dt.to_period("M").astype(str)
    fee_rate = FIXED_PARAMS["entry_fee"] + FIXED_PARAMS["exit_fee"]

    total = 0.0
    for (symbol, month), group in df.groupby(["symbol", "month"]):
        balance = INITIAL_BALANCE
        for _, t in group.sort_values("entry_time").iterrows():
            notional = balance * t["notional_share"]
            pnl = notional * t["pnl_pct"] - notional * fee_rate
            balance += pnl
        total += (balance / INITIAL_BALANCE - 1.0) * 100.0
    return round(total, 4), len(df)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main():
    print("Loading data...")
    gate_entries = extract_v6_gate_entries()
    initial_entries = gate_entries[gate_entries["entry_reason"] == "Zero-Initial-Reentry"]
    all_events = pd.read_csv(EVENTS_CSV)
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)
    matched = match_events(initial_entries, all_events)
    print(f"  {len(matched)} events matched")

    # Pre-load all bars
    bars_cache = {}
    for symbol in matched["symbol"].unique():
        sym_events = matched[matched["symbol"] == symbol]
        for month in sym_events["touch_time"].dt.to_period("M").unique():
            month_start = pd.Timestamp(str(month), tz="UTC")
            key = f"{symbol}_{month_start.strftime('%Y-%m')}"
            if key not in bars_cache:
                bars = load_monthly_bars(symbol, month_start)
                bars_cache[key] = bars if bars is not None else pd.DataFrame()
    print(f"  {len(bars_cache)} month-bars loaded")

    # Generate sweep combinations
    keys = list(SWEEP.keys())
    values = list(SWEEP.values())
    combos = list(itertools.product(*values))
    print(f"\nSweeping {len(combos)} parameter combinations...")
    print(f"  Grid: {' x '.join(f'{k}({len(v)})' for k, v in SWEEP.items())}")
    print()

    results = []
    best_sum = -999.0
    best_params = None

    for idx, combo in enumerate(combos):
        params = dict(FIXED_PARAMS)
        for k, v in zip(keys, combo):
            params[k] = v

        trades = run_backtest(matched, bars_cache, params)
        cal_sum, n_trades = compute_calendar_sum(trades)
        results.append({**{k: v for k, v in zip(keys, combo)}, "calendar_sum_pct": cal_sum, "trades": n_trades})

        if cal_sum > best_sum:
            best_sum = cal_sum
            best_params = dict(zip(keys, combo))
            print(f"  [{idx+1}/{len(combos)}] NEW BEST: {cal_sum:+.2f}% ({n_trades} trades) | {best_params}")
        elif (idx + 1) % 50 == 0:
            print(f"  [{idx+1}/{len(combos)}] current best: {best_sum:+.2f}%")

    # Sort and display top results
    results.sort(key=lambda x: x["calendar_sum_pct"], reverse=True)
    print()
    print("=" * 70)
    print("TOP 10 PARAMETER COMBINATIONS")
    print("=" * 70)
    for i, r in enumerate(results[:10]):
        print(f"  #{i+1}: {r['calendar_sum_pct']:+.2f}% ({r['trades']} trades) | "
              f"stop={r['initial_stop_atr']}, be={r['breakeven_at_r']}, "
              f"trail_start={r['trail_start_r']}, trail_buf={r['trail_buffer_atr']}, "
              f"hold={r['max_hold_hours']}h")

    print()
    print("WORST 5:")
    for r in results[-5:]:
        print(f"  {r['calendar_sum_pct']:+.2f}% | stop={r['initial_stop_atr']}, be={r['breakeven_at_r']}, "
              f"trail_start={r['trail_start_r']}, trail_buf={r['trail_buffer_atr']}, hold={r['max_hold_hours']}h")

    # Save
    pd.DataFrame(results).to_csv(OUTPUT_DIR / "v6_gate_exit_sweep_results.csv", index=False)
    summary = {
        "experiment": "V6 Gate Exit Parameter Sweep",
        "entry": {"delay_seconds": ENTRY_DELAY_SECONDS, "sizing_scale": SIZING_SCALE},
        "sweep_grid": SWEEP,
        "total_combos": len(combos),
        "best": {"params": best_params, "calendar_sum_pct": best_sum},
        "top10": results[:10],
    }
    (OUTPUT_DIR / "v6_gate_exit_sweep_summary.json").write_text(
        json.dumps(summary, indent=2, default=str), encoding="utf-8"
    )
    print(f"\nSaved: {OUTPUT_DIR / 'v6_gate_exit_sweep_results.csv'}")


if __name__ == "__main__":
    main()
