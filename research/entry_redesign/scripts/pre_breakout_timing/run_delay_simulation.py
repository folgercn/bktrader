#!/usr/bin/env python3
"""
run_delay_simulation — 多延迟模拟运行脚本

对 116 V6 gate events 运行多延迟模拟（D=0/5/10/15/pullback），
产出 delay_pnl_matrix.csv，并验证各 delay 的 baseline calendar_sum。

已知 baseline：D=5s ≈ 2.74%（来自 v6_gate_d5_tick_backtest.py 结果）

Usage:
    cd research/entry_redesign/scripts
    python -m pre_breakout_timing.run_delay_simulation
"""

from __future__ import annotations

import sys
import time
from pathlib import Path

import pandas as pd

# Ensure the scripts directory is on sys.path for imports
SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from pre_breakout_timing.data_layer import (
    load_bars_cache,
    load_v6_gate_events,
    time_split_events,
)
from pre_breakout_timing.delay_simulator import (
    DELAY_VALUES,
    DelayResult,
    simulate_all_delays,
)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

OUTPUT_DIR = SCRIPTS_DIR / "output" / "pre_breakout_timing"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

INITIAL_BALANCE = 100_000.0

# Default pullback params (from design doc)
DEFAULT_PULLBACK_PARAMS = {
    "pullback_target_atr": 0.05,
    "pullback_window_seconds": 60,
    "start_offset_seconds": 5,
}

# Known baseline for validation
KNOWN_D5_CALENDAR_SUM = 2.74  # %


# ---------------------------------------------------------------------------
# Calendar sum computation (silo-based)
# ---------------------------------------------------------------------------


def compute_calendar_sum_from_results(
    results: list[DelayResult],
    events: pd.DataFrame,
) -> float:
    """计算 silo-based calendar sum (%).

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。

    Parameters
    ----------
    results : list[DelayResult]
        某个 delay 下所有 event 的执行结果。
    events : pd.DataFrame
        原始 events DataFrame，用于获取 symbol 信息。

    Returns
    -------
    float
        Calendar sum (%)。
    """
    # Build event_id -> symbol mapping
    event_symbol_map: dict[str, str] = {}
    for _, row in events.iterrows():
        eid = str(row.get("event_id", ""))
        event_symbol_map[eid] = str(row["symbol"])

    # Group trades by (symbol, month)
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        symbol = event_symbol_map.get(r.event_id, "unknown")
        entry_time = pd.Timestamp(r.entry_time)
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(r)

    total_return_pct = 0.0
    for silo_key, silo_results in silos.items():
        balance = INITIAL_BALANCE
        # Sort by entry_time
        sorted_results = sorted(silo_results, key=lambda r: r.entry_time)
        for r in sorted_results:
            # notional_share = 0.26 (default in execute_trade)
            notional = balance * 0.26
            # realistic_pnl_pct already includes fees
            pnl = notional * r.pnl_pct
            balance += pnl
        silo_return = (balance - INITIAL_BALANCE) / INITIAL_BALANCE * 100.0
        total_return_pct += silo_return

    return total_return_pct


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main():
    """运行多延迟模拟，产出 delay_pnl_matrix.csv 并验证 calendar_sum。"""
    print("=" * 70)
    print("Pre-Breakout Timing: Multi-Delay Simulation")
    print("=" * 70)

    # --- Step 1: Load data ---
    print("\n[1/4] Loading V6 gate events...")
    t0 = time.time()
    events = load_v6_gate_events()
    print(f"  Loaded {len(events)} events in {time.time() - t0:.1f}s")

    print("\n[2/4] Loading 1s bar cache...")
    t0 = time.time()
    bars_cache = load_bars_cache(events)
    print(f"  Loaded {len(bars_cache)} month caches in {time.time() - t0:.1f}s")

    # --- Step 2: Run multi-delay simulation ---
    print(f"\n[3/4] Running multi-delay simulation on {len(events)} events...")
    print(f"  Delays: D=0s, D=5s, D=10s, D=15s, pullback")
    print(f"  Pullback params: {DEFAULT_PULLBACK_PARAMS}")

    all_delay_results: list[list[DelayResult]] = []
    t0 = time.time()

    for idx, (_, event) in enumerate(events.iterrows()):
        symbol = event["symbol"]
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        # Get bars for this event's month
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)

        if second_bars is None or second_bars.empty:
            # No data: create empty results for all delays
            event_id = str(event.get("event_id", ""))
            empty_results = []
            for delay in DELAY_VALUES:
                label = f"D{delay}"
                empty_results.append(DelayResult(
                    event_id=event_id,
                    delay_label=label,
                    delay_seconds=delay,
                    entry_time=None,
                    entry_price=None,
                    pnl_pct=None,
                    exit_reason="NoData",
                    exit_time=None,
                    hold_seconds=None,
                    mfe_r=None,
                    mae_r=None,
                    traded=False,
                ))
            empty_results.append(DelayResult(
                event_id=event_id,
                delay_label="pullback",
                delay_seconds=0,
                entry_time=None,
                entry_price=None,
                pnl_pct=None,
                exit_reason="NoData",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            ))
            all_delay_results.append(empty_results)
            continue

        # Run simulation for all delays
        event_results = simulate_all_delays(
            event, second_bars, pullback_params=DEFAULT_PULLBACK_PARAMS
        )
        all_delay_results.append(event_results)

        # Progress
        if (idx + 1) % 20 == 0 or idx == len(events) - 1:
            elapsed = time.time() - t0
            print(f"  Processed {idx + 1}/{len(events)} events ({elapsed:.1f}s)")

    print(f"  Simulation complete in {time.time() - t0:.1f}s")

    # --- Step 3: Build DataFrame and save CSV ---
    print("\n[4/4] Building delay_pnl_matrix.csv...")

    rows = []
    for event_idx, event_results in enumerate(all_delay_results):
        event_row = events.iloc[event_idx]
        for dr in event_results:
            rows.append({
                "event_id": dr.event_id,
                "symbol": event_row["symbol"],
                "side": event_row["side"],
                "touch_time": event_row["touch_time"],
                "delay_label": dr.delay_label,
                "delay_seconds": dr.delay_seconds,
                "entry_time": dr.entry_time,
                "entry_price": dr.entry_price,
                "pnl_pct": dr.pnl_pct,
                "exit_reason": dr.exit_reason,
                "exit_time": dr.exit_time,
                "hold_seconds": dr.hold_seconds,
                "mfe_r": dr.mfe_r,
                "mae_r": dr.mae_r,
                "traded": dr.traded,
            })

    matrix_df = pd.DataFrame(rows)
    output_path = OUTPUT_DIR / "delay_pnl_matrix.csv"
    matrix_df.to_csv(output_path, index=False)
    print(f"  Saved: {output_path}")
    print(f"  Shape: {matrix_df.shape}")

    # --- Step 4: Compute calendar_sum for each delay (all 116 events) ---
    print("\n" + "=" * 70)
    print("Calendar Sum Results — ALL 116 events (silo-based, per delay)")
    print("=" * 70)

    delay_labels = ["D0", "D5", "D10", "D15", "pullback"]
    calendar_sums: dict[str, float] = {}

    for label in delay_labels:
        # Filter results for this delay
        label_results = [
            dr
            for event_results in all_delay_results
            for dr in event_results
            if dr.delay_label == label
        ]
        cal_sum = compute_calendar_sum_from_results(label_results, events)
        calendar_sums[label] = cal_sum

        # Also compute win rate and trade count
        traded = [dr for dr in label_results if dr.traded]
        wins = [dr for dr in traded if dr.pnl_pct is not None and dr.pnl_pct > 0]
        win_rate = len(wins) / len(traded) * 100.0 if traded else 0.0

        print(f"\n  {label:>10s}: calendar_sum = {cal_sum:+.4f}%")
        print(f"             trades = {len(traded)}, win_rate = {win_rate:.1f}%")

    # --- Step 5: Compute calendar_sum for test set (for validation) ---
    print("\n" + "=" * 70)
    print("Calendar Sum Results — TEST SET (for validation vs known baseline)")
    print("=" * 70)

    # Time-split: 60/40
    train_events, test_events = time_split_events(events)
    print(f"\n  Time split: train={len(train_events)}, test={len(test_events)}")

    # We need to map test events back to their delay results
    # Build event_id -> index mapping from original events
    event_id_to_idx: dict[str, int] = {}
    for idx, (_, row) in enumerate(events.iterrows()):
        eid = str(row.get("event_id", ""))
        event_id_to_idx[eid] = idx

    # Get test event indices
    test_event_ids = set(str(row.get("event_id", "")) for _, row in test_events.iterrows())

    test_calendar_sums: dict[str, float] = {}
    for label in delay_labels:
        # Filter results for this delay, test set only
        label_results = []
        for event_idx, event_results in enumerate(all_delay_results):
            event_row = events.iloc[event_idx]
            eid = str(event_row.get("event_id", ""))
            if eid in test_event_ids:
                for dr in event_results:
                    if dr.delay_label == label:
                        label_results.append(dr)

        cal_sum = compute_calendar_sum_from_results(label_results, test_events)
        test_calendar_sums[label] = cal_sum

        traded = [dr for dr in label_results if dr.traded]
        wins = [dr for dr in traded if dr.pnl_pct is not None and dr.pnl_pct > 0]
        win_rate = len(wins) / len(traded) * 100.0 if traded else 0.0

        print(f"\n  {label:>10s}: calendar_sum = {cal_sum:+.4f}%")
        print(f"             trades = {len(traded)}, win_rate = {win_rate:.1f}%")

    # --- Step 6: Validate against known baseline ---
    print("\n" + "=" * 70)
    print("Validation: D=5s vs Known Baseline (≈ 2.74%)")
    print("=" * 70)

    # The known 2.74% is from dynamic_timing's baseline_a on test set (47 events)
    d5_all = calendar_sums.get("D5", 0.0)
    d5_test = test_calendar_sums.get("D5", 0.0)

    print(f"\n  Known D=5s baseline (test set, dynamic_timing): {KNOWN_D5_CALENDAR_SUM:.4f}%")
    print(f"  Computed D=5s (all 116 events):                  {d5_all:.4f}%")
    print(f"  Computed D=5s (test set, {len(test_events)} events):             {d5_test:.4f}%")

    diff_test = d5_test - KNOWN_D5_CALENDAR_SUM
    print(f"\n  Test set difference: {diff_test:+.4f}%")

    if abs(diff_test) < 0.5:
        print(f"  ✓ PASS: D=5s test set result is within 0.5% of known baseline")
    elif abs(diff_test) < 1.0:
        print(f"  ~ CLOSE: D=5s test set result is within 1.0% of known baseline")
        print(f"    Minor differences expected due to implementation details")
    else:
        print(f"  ⚠ WARNING: D=5s test set result differs by {abs(diff_test):.4f}%")
        print(f"    Possible reasons:")
        print(f"    - v6_gate_d5_tick_backtest uses reentry (2 trades/event)")
        print(f"    - dynamic_timing baseline_a uses single trade (same as us)")
        print(f"    - Slight differences in event matching or bar loading")

    # --- Summary ---
    print("\n" + "=" * 70)
    print("Summary")
    print("=" * 70)
    print(f"\n  Total events: {len(events)}")
    print(f"  Train/Test split: {len(train_events)}/{len(test_events)}")
    print(f"  Delay values tested: {delay_labels}")

    print(f"\n  Calendar Sum Ranking (all 116 events):")
    sorted_delays = sorted(calendar_sums.items(), key=lambda x: x[1], reverse=True)
    for rank, (label, cal_sum) in enumerate(sorted_delays, 1):
        print(f"    {rank}. {label:>10s}: {cal_sum:+.4f}%")

    print(f"\n  Calendar Sum Ranking (test set, {len(test_events)} events):")
    sorted_test = sorted(test_calendar_sums.items(), key=lambda x: x[1], reverse=True)
    for rank, (label, cal_sum) in enumerate(sorted_test, 1):
        marker = " ← compare to known 2.74%" if label == "D5" else ""
        print(f"    {rank}. {label:>10s}: {cal_sum:+.4f}%{marker}")

    print(f"\n  Output: {output_path}")
    print("=" * 70)

    return calendar_sums


if __name__ == "__main__":
    main()
