#!/usr/bin/env python3
"""V6 Gate-filtered events → D=5 tick backtest.

Experiment: Take the events that V6 candidate_001 gate selected (116 unique),
run them through V4 tick execution with D=5, and compare calendar sum against
V6's 33.02% baseline.

Key differences from V6 baseline:
- V6 uses reentry-trigger entry (price returns to level) + 0.05 ATR stop
- This experiment uses D=5 fixed delay entry + 0.45 ATR stop (V4 model)
- Both use 1s bar simulation, same events, same sizing

Usage:
    python research/entry_redesign/scripts/v6_gate_d5_tick_backtest.py
"""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path

import numpy as np
import pandas as pd

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parents[3]

V6_LEDGER_BASE = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_2025m06_2026apr_combo_baseline_short_speed"
    / "union_lifecycle_reentry_window_candidate_001_calendar_holdout"
    / "power0_fixed_1p30"
)

EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "2025m03_2026apr_original_t2_delay60"
    / "events_execution_labeled.csv"
)

BARS_CACHE_DIR = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest"
    / "bars_cache"
)

OUTPUT_DIR = PROJECT_ROOT / "research" / "entry_redesign" / "scripts" / "output"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

# ---------------------------------------------------------------------------
# V4 Execution parameters (from NEXT_EXPERIMENT.md)
# ---------------------------------------------------------------------------
EXEC_PARAMS = {
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.80,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "trail_start_r": 0.9,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 4.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
}

ENTRY_DELAY_SECONDS = 5
INITIAL_BALANCE = 100000.0

# Sizing: reentry_window with V6's sizing_scale=1.3
SIZING_SCALE = 1.3
REENTRY_SIZE_SCHEDULE = [0.20, 0.10]  # effective: [0.26, 0.13]
MAX_TRADES_PER_BAR = 2


# ---------------------------------------------------------------------------
# Cached 1s bars loading
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


# ---------------------------------------------------------------------------
# V6 gate event extraction
# ---------------------------------------------------------------------------
def extract_v6_gate_entries() -> pd.DataFrame:
    entries = []
    for execute_dir in sorted(V6_LEDGER_BASE.glob("execute_*")):
        for symbol_dir in sorted(execute_dir.iterdir()):
            if not symbol_dir.is_dir():
                continue
            ledger_path = symbol_dir / "lifecycle_ledger.csv"
            if not ledger_path.exists():
                continue
            symbol = symbol_dir.name
            df = pd.read_csv(ledger_path, parse_dates=["time"])
            entry_rows = df[df["type"] != "EXIT"].copy()
            for _, row in entry_rows.iterrows():
                entries.append({
                    "symbol": symbol,
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
        symbol = entry["symbol"]
        touch = entry["touch_time"]
        side = entry["side"]

        candidates = all_events[
            (all_events["symbol"] == symbol)
            & (all_events["side"] == side)
            & ((all_events["touch_time"] - touch).abs() <= pd.Timedelta(seconds=2))
        ]
        if not candidates.empty:
            idx = (candidates["touch_time"] - touch).abs().idxmin()
            matched.append(candidates.loc[idx])

    if not matched:
        return pd.DataFrame()
    return pd.DataFrame(matched).drop_duplicates(subset=["event_id"])


# ---------------------------------------------------------------------------
# V4 Execution simulation
# ---------------------------------------------------------------------------
def open_position(event: pd.Series, entry_price: float, entry_time,
                  balance: float, notional_share: float) -> dict | None:
    atr = float(event["atr"])
    side = str(event["side"])
    slippage = EXEC_PARAMS["slippage"]

    if side == "long":
        entry_p = entry_price * (1.0 + slippage)
    else:
        entry_p = entry_price * (1.0 - slippage)

    sig_low = float(event.get("touch_low_so_far", event.get("signal_low", 0)))
    sig_high = float(event.get("touch_high_so_far", event.get("signal_high", 0)))

    if side == "long":
        raw_stop = min(
            sig_low - EXEC_PARAMS["stop_buffer_atr"] * atr,
            entry_p - EXEC_PARAMS["initial_stop_atr"] * atr,
        )
        capped_stop = entry_p - EXEC_PARAMS["stop_cap_atr"] * atr
        stop = max(raw_stop, capped_stop)
        risk = entry_p - stop
    else:
        raw_stop = max(
            sig_high + EXEC_PARAMS["stop_buffer_atr"] * atr,
            entry_p + EXEC_PARAMS["initial_stop_atr"] * atr,
        )
        capped_stop = entry_p + EXEC_PARAMS["stop_cap_atr"] * atr
        stop = min(raw_stop, capped_stop)
        risk = stop - entry_p

    if risk <= 0 or risk < entry_p * EXEC_PARAMS["min_stop_bps"] / 10000.0:
        return None

    return {
        "side": side,
        "entry_time": entry_time,
        "entry_p": entry_p,
        "entry_raw": entry_price,
        "sl": stop,
        "risk": risk,
        "atr": atr,
        "notional": balance * notional_share,
        "notional_share": notional_share,
        "protected": False,
        "trailing_active": False,
        "hwm": entry_p,
        "lwm": entry_p,
        "mfe_r": 0.0,
        "mae_r": 0.0,
    }


def simulate_position(pos: dict, second_bars: pd.DataFrame) -> dict:
    entry_time = pos["entry_time"]
    max_hold = pd.Timedelta(hours=EXEC_PARAMS["max_hold_hours"])
    end_time = entry_time + max_hold

    mask = (second_bars.index > entry_time) & (second_bars.index <= end_time)
    bars = second_bars.loc[mask]

    if bars.empty:
        return _make_exit(pos, pos["entry_raw"], entry_time, "NoData")

    entry_p = pos["entry_p"]
    risk = pos["risk"]
    highs = bars["high"].values.astype(np.float64)
    lows = bars["low"].values.astype(np.float64)
    closes = bars["close"].values.astype(np.float64)
    timestamps = bars.index

    for i in range(len(bars)):
        h = highs[i]
        l = lows[i]

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
        if favorable / risk >= EXEC_PARAMS["breakeven_at_r"]:
            if pos["side"] == "long":
                be_sl = entry_p * (1.0 + EXEC_PARAMS["cost_lock_bps"] / 10000.0)
                if be_sl > pos["sl"]:
                    pos["sl"] = be_sl
                    pos["protected"] = True
            else:
                be_sl = entry_p * (1.0 - EXEC_PARAMS["cost_lock_bps"] / 10000.0)
                if be_sl < pos["sl"]:
                    pos["sl"] = be_sl
                    pos["protected"] = True

        # Trailing
        if pos["mfe_r"] >= EXEC_PARAMS["trail_start_r"]:
            atr = pos["atr"]
            if pos["side"] == "long":
                trail = pos["hwm"] - EXEC_PARAMS["trail_buffer_atr"] * atr
                if trail > pos["sl"]:
                    pos["sl"] = trail
                    pos["trailing_active"] = True
            else:
                trail = pos["lwm"] + EXEC_PARAMS["trail_buffer_atr"] * atr
                if trail < pos["sl"]:
                    pos["sl"] = trail
                    pos["trailing_active"] = True

        # Stop trigger
        if pos["side"] == "long":
            if l <= pos["sl"]:
                reason = "TrailingSL" if pos["trailing_active"] else ("BreakevenSL" if pos["protected"] else "InitialSL")
                return _make_exit(pos, pos["sl"], timestamps[i], reason)
        else:
            if h >= pos["sl"]:
                reason = "TrailingSL" if pos["trailing_active"] else ("BreakevenSL" if pos["protected"] else "InitialSL")
                return _make_exit(pos, pos["sl"], timestamps[i], reason)

    # Max hold
    return _make_exit(pos, closes[-1], timestamps[-1], "MaxHoldExit")


def _make_exit(pos: dict, raw_exit_price: float, exit_time, reason: str) -> dict:
    slippage = EXEC_PARAMS["slippage"]
    if pos["side"] == "long":
        exit_p = raw_exit_price * (1.0 - slippage)
        pnl_pct = (exit_p - pos["entry_p"]) / pos["entry_p"]
    else:
        exit_p = raw_exit_price * (1.0 + slippage)
        pnl_pct = (pos["entry_p"] - exit_p) / pos["entry_p"]

    fee_rate = EXEC_PARAMS["entry_fee"] + EXEC_PARAMS["exit_fee"]

    return {
        "entry_time": pos["entry_time"],
        "exit_time": exit_time,
        "side": pos["side"],
        "entry_p": pos["entry_p"],
        "exit_p": exit_p,
        "notional_share": pos["notional_share"],
        "pnl_pct": pnl_pct,
        "realistic_pnl_pct": pnl_pct - fee_rate,
        "exit_reason": reason,
        "mfe_r": pos["mfe_r"],
        "mae_r": pos["mae_r"],
        "hold_seconds": (pd.Timestamp(exit_time) - pd.Timestamp(pos["entry_time"])).total_seconds(),
    }


# ---------------------------------------------------------------------------
# Lifecycle backtest with SL-Reentry
# ---------------------------------------------------------------------------
def run_lifecycle_backtest(events: pd.DataFrame) -> pd.DataFrame:
    """Run backtest with reentry_window sizing.

    For each event: enter at touch_time + D=5s.
    On InitialSL: reentry 1s later with slot1 sizing.
    Max 2 trades per event.
    """
    events = events.sort_values("touch_time").reset_index(drop=True)
    all_trades = []
    bars_cache: dict[str, pd.DataFrame] = {}

    for symbol, sym_events in events.groupby("symbol"):
        print(f"\n{'='*60}")
        print(f"Processing {symbol}: {len(sym_events)} gate-selected events")
        print(f"{'='*60}")

        balance = INITIAL_BALANCE
        last_exit_time = pd.Timestamp.min.tz_localize("UTC")
        sym_trades = []

        for _, event in sym_events.iterrows():
            touch_time = pd.Timestamp(event["touch_time"])
            if touch_time.tzinfo is None:
                touch_time = touch_time.tz_localize("UTC")

            if touch_time <= last_exit_time:
                continue

            month_key = f"{symbol}_{touch_time.strftime('%Y-%m')}"
            if month_key not in bars_cache:
                month_start = touch_time.replace(day=1, hour=0, minute=0, second=0, microsecond=0)
                print(f"  Loading 1s bars for {month_key}...")
                t0 = time.time()
                bars = load_monthly_bars(symbol, month_start)
                if bars is not None:
                    bars_cache[month_key] = bars
                    print(f"    Loaded {len(bars):,} bars in {time.time()-t0:.1f}s")
                else:
                    print(f"    WARNING: No cached bars found for {month_key}")
                    bars_cache[month_key] = pd.DataFrame()

            second_bars = bars_cache[month_key]
            if second_bars.empty:
                continue

            signal_start = str(event["signal_start"])

            # --- Execute with reentry ---
            trades_in_event = 0
            current_entry_time = touch_time + pd.Timedelta(seconds=ENTRY_DELAY_SECONDS)

            while trades_in_event < MAX_TRADES_PER_BAR:
                entry_mask = second_bars.index >= current_entry_time
                if not entry_mask.any():
                    break
                entry_idx = second_bars.index[entry_mask][0]
                entry_price = float(second_bars.loc[entry_idx, "close"])

                notional_share = REENTRY_SIZE_SCHEDULE[min(trades_in_event, len(REENTRY_SIZE_SCHEDULE) - 1)] * SIZING_SCALE

                pos = open_position(event, entry_price, entry_idx, balance, notional_share)
                if pos is None:
                    break

                trade = simulate_position(pos, second_bars)
                trade["symbol"] = symbol
                trade["signal_start"] = signal_start
                trade["event_id"] = event.get("event_id", "")
                trade["slot_idx"] = trades_in_event

                # Update balance
                balance += balance * trade["notional_share"] * trade["realistic_pnl_pct"]
                trade["balance_after"] = balance

                sym_trades.append(trade)
                all_trades.append(trade)
                last_exit_time = pd.Timestamp(trade["exit_time"])
                trades_in_event += 1

                # Reentry only on InitialSL
                if trade["exit_reason"] != "InitialSL":
                    break
                current_entry_time = pd.Timestamp(trade["exit_time"]) + pd.Timedelta(seconds=1)

        print(f"  {symbol}: {len(sym_trades)} trades, final balance: {balance:,.2f}")

    return pd.DataFrame(all_trades)


# ---------------------------------------------------------------------------
# Calendar sum (silo-based)
# ---------------------------------------------------------------------------
def compute_calendar_sum(trades: pd.DataFrame) -> dict:
    """Each (symbol, month) is an independent silo starting at 100k."""
    if trades.empty:
        return {"calendar_sum_pct": 0.0, "silos": [], "total_trades": 0}

    trades = trades.copy()
    trades["entry_time"] = pd.to_datetime(trades["entry_time"], utc=True)
    trades["month"] = trades["entry_time"].dt.to_period("M").astype(str)

    fee_rate = EXEC_PARAMS["entry_fee"] + EXEC_PARAMS["exit_fee"]

    silos = []
    for (symbol, month), group in trades.groupby(["symbol", "month"]):
        balance = INITIAL_BALANCE
        for _, trade in group.sort_values("entry_time").iterrows():
            notional = balance * trade["notional_share"]
            pnl = notional * trade["pnl_pct"] - notional * fee_rate
            balance += pnl

        return_pct = (balance / INITIAL_BALANCE - 1.0) * 100.0
        silos.append({
            "symbol": symbol,
            "month": month,
            "trades": len(group),
            "return_pct": round(return_pct, 4),
            "win_rate_pct": round(float((group["realistic_pnl_pct"] > 0).mean()) * 100.0, 2),
        })

    calendar_sum = sum(s["return_pct"] for s in silos)
    return {
        "calendar_sum_pct": round(calendar_sum, 4),
        "silos": silos,
        "total_trades": len(trades),
    }


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main():
    print("=" * 70)
    print("V6 Gate D=5 Tick Backtest (V4 Execution Params)")
    print("=" * 70)
    print(f"Entry: D={ENTRY_DELAY_SECONDS}s after touch_time")
    print(f"Stop: initial_stop_atr={EXEC_PARAMS['initial_stop_atr']}, min_stop_bps={EXEC_PARAMS['min_stop_bps']}")
    print(f"Trail: breakeven_at_r={EXEC_PARAMS['breakeven_at_r']}, trail_start_r={EXEC_PARAMS['trail_start_r']}")
    print(f"Max hold: {EXEC_PARAMS['max_hold_hours']}h")
    effective = [f"{x*SIZING_SCALE:.2f}" for x in REENTRY_SIZE_SCHEDULE]
    print(f"Sizing: {REENTRY_SIZE_SCHEDULE} x scale={SIZING_SCALE} -> [{', '.join(effective)}]")
    print(f"Fees: slip={EXEC_PARAMS['slippage']*10000:.0f}bps, entry={EXEC_PARAMS['entry_fee']*10000:.0f}bps, exit={EXEC_PARAMS['exit_fee']*10000:.0f}bps")
    print()

    # Step 1
    print("Step 1: Extracting V6 gate-selected entries...")
    gate_entries = extract_v6_gate_entries()
    initial_entries = gate_entries[gate_entries["entry_reason"] == "Zero-Initial-Reentry"]
    print(f"  {len(initial_entries)} unique events (Zero-Initial)")
    print(f"  Symbols: {initial_entries['symbol'].value_counts().to_dict()}")
    print()

    # Step 2
    print("Step 2: Matching to full events CSV...")
    all_events = pd.read_csv(EVENTS_CSV)
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)
    matched = match_events(initial_entries, all_events)
    print(f"  Matched {len(matched)} unique events")
    print(f"  Symbols: {matched['symbol'].value_counts().to_dict()}")
    print()

    if matched.empty:
        print("ERROR: No events matched.")
        return

    # Step 3
    print("Step 3: Running D=5 tick execution...")
    t0 = time.time()
    trades = run_lifecycle_backtest(matched)
    print(f"\nCompleted in {time.time()-t0:.1f}s, {len(trades)} trades")
    print()

    if trades.empty:
        print("ERROR: No trades.")
        return

    # Step 4
    print("Step 4: Computing calendar sum (silo-based)...")
    result = compute_calendar_sum(trades)

    # V6 baseline silos for comparison
    v6_silos = {
        ("ETHUSDT", "2025-06"): 0.84, ("BTCUSDT", "2025-07"): 2.36,
        ("BTCUSDT", "2025-08"): -0.40, ("ETHUSDT", "2025-09"): 0.73,
        ("BTCUSDT", "2025-11"): 2.16, ("ETHUSDT", "2025-12"): 7.91,
        ("BTCUSDT", "2026-01"): 1.45, ("BTCUSDT", "2026-02"): 5.58,
        ("ETHUSDT", "2026-02"): 8.12, ("ETHUSDT", "2026-03"): 4.27,
    }

    print()
    print("=" * 70)
    print("RESULTS")
    print("=" * 70)
    print(f"  Calendar Sum (D=5):   {result['calendar_sum_pct']:.2f}%")
    print(f"  V6 Baseline (D=60):   33.02%")
    print(f"  Delta:                {result['calendar_sum_pct'] - 33.02:+.2f}%")
    print(f"  Total Trades:         {result['total_trades']}")
    print(f"  Active Silos:         {len(result['silos'])}")
    print()

    print("Silo comparison:")
    print(f"  {'Symbol':<10} {'Month':<10} {'D=5%':>10} {'V6%':>10} {'Delta':>10} {'Trades':>6} {'Win%':>6}")
    print(f"  {'-'*10} {'-'*10} {'-'*10} {'-'*10} {'-'*10} {'-'*6} {'-'*6}")
    for s in sorted(result["silos"], key=lambda x: x["month"]):
        v6_val = v6_silos.get((s["symbol"], s["month"]), 0.0)
        delta = s["return_pct"] - v6_val
        print(f"  {s['symbol']:<10} {s['month']:<10} {s['return_pct']:>+10.2f}% {v6_val:>+10.2f}% {delta:>+10.2f}% {s['trades']:>6} {s['win_rate_pct']:>5.0f}%")
    print()

    # Exit reasons
    print("Exit reasons:")
    for reason, count in trades["exit_reason"].value_counts().items():
        subset = trades[trades["exit_reason"] == reason]
        avg_pnl = subset["realistic_pnl_pct"].mean() * 100
        print(f"  {reason:<15} {count:>4} trades, avg pnl: {avg_pnl:+.4f}%")
    print()

    win_rate = (trades["realistic_pnl_pct"] > 0).mean() * 100
    avg_win = trades.loc[trades["realistic_pnl_pct"] > 0, "realistic_pnl_pct"].mean() * 100 if (trades["realistic_pnl_pct"] > 0).any() else 0
    avg_loss = trades.loc[trades["realistic_pnl_pct"] <= 0, "realistic_pnl_pct"].mean() * 100 if (trades["realistic_pnl_pct"] <= 0).any() else 0
    print(f"  Win rate: {win_rate:.1f}%")
    print(f"  Avg win:  {avg_win:+.4f}%")
    print(f"  Avg loss: {avg_loss:+.4f}%")
    print(f"  Median hold: {trades['hold_seconds'].median():.0f}s")
    print(f"  Avg MFE_R: {trades['mfe_r'].mean():.2f}")

    # Save
    trades.to_csv(OUTPUT_DIR / "v6_gate_d5_trades.csv", index=False)
    summary = {
        "experiment": "V6 Gate D=5 Tick Backtest",
        "note": "V4 execution params on V6 gate-filtered events, D=5 entry",
        "execution_params": EXEC_PARAMS,
        "entry_delay_seconds": ENTRY_DELAY_SECONDS,
        "sizing": {
            "schedule": REENTRY_SIZE_SCHEDULE,
            "sizing_scale": SIZING_SCALE,
            "effective": [x * SIZING_SCALE for x in REENTRY_SIZE_SCHEDULE],
            "max_trades_per_bar": MAX_TRADES_PER_BAR,
        },
        "result": result,
        "v6_baseline_pct": 33.02,
        "delta_pct": round(result["calendar_sum_pct"] - 33.02, 4),
    }
    (OUTPUT_DIR / "v6_gate_d5_summary.json").write_text(
        json.dumps(summary, indent=2, default=str), encoding="utf-8"
    )
    print(f"\nSaved: {OUTPUT_DIR / 'v6_gate_d5_trades.csv'}")
    print(f"Saved: {OUTPUT_DIR / 'v6_gate_d5_summary.json'}")


if __name__ == "__main__":
    main()
