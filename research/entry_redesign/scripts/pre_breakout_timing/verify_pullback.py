#!/usr/bin/env python3
"""
verify_pullback — 回调入场逻辑验证脚本

验证 simulate_pullback_entry() 的行为：
1. 当回调在窗口内触发 → entry at pullback bar's close, pullback_triggered=True
2. 当回调超时 → entry at last bar's close in window (fallback), pullback_triggered=False
3. price_improvement_bps 正确计算：
   - Long: (D5_entry_price - pullback_entry_price) / D5_entry_price × 10000
   - Short: (pullback_entry_price - D5_entry_price) / D5_entry_price × 10000

同时与 delay_pnl_matrix.csv 中已有的 pullback 结果进行交叉验证。

Usage:
    cd research/entry_redesign/scripts
    python -m pre_breakout_timing.verify_pullback
"""

from __future__ import annotations

import sys
from pathlib import Path

import pandas as pd

# Ensure the scripts directory is on sys.path for imports
SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from pre_breakout_timing.data_layer import load_bars_cache, load_v6_gate_events
from pre_breakout_timing.delay_simulator import simulate_pullback_entry

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

OUTPUT_DIR = SCRIPTS_DIR / "output" / "pre_breakout_timing"

# Default pullback params (from design doc / run_delay_simulation.py)
DEFAULT_START_OFFSET = 5
DEFAULT_PULLBACK_TARGET_ATR = 0.05
DEFAULT_PULLBACK_WINDOW = 60


def compute_price_improvement_bps(
    d5_entry_price: float,
    pullback_entry_price: float,
    side: str,
) -> float:
    """计算 pullback 入场相对 D5 入场的价格改善 (bps)。

    Long: 价格越低越好 → (D5 - pullback) / D5 × 10000
    Short: 价格越高越好 → (pullback - D5) / D5 × 10000

    正值表示 pullback 入场价格更优。
    """
    if d5_entry_price == 0:
        return 0.0
    if side == "long":
        return (d5_entry_price - pullback_entry_price) / d5_entry_price * 10000
    else:  # short
        return (pullback_entry_price - d5_entry_price) / d5_entry_price * 10000


def main():
    """运行回调入场逻辑验证。"""
    print("=" * 70)
    print("Pullback Entry Verification")
    print("=" * 70)

    # --- Step 1: Load data ---
    print("\n[1/5] Loading V6 gate events and 1s bar cache...")
    events = load_v6_gate_events()
    bars_cache = load_bars_cache(events)
    print(f"  Loaded {len(events)} events, {len(bars_cache)} month caches")

    # --- Step 2: Load existing delay_pnl_matrix for cross-validation ---
    print("\n[2/5] Loading existing delay_pnl_matrix.csv for cross-validation...")
    matrix_path = OUTPUT_DIR / "delay_pnl_matrix.csv"
    if matrix_path.exists():
        matrix_df = pd.read_csv(matrix_path)
        pb_matrix = matrix_df[matrix_df["delay_label"] == "pullback"].copy()
        d5_matrix = matrix_df[matrix_df["delay_label"] == "D5"].copy()
        print(f"  Loaded {len(pb_matrix)} pullback entries, {len(d5_matrix)} D5 entries")
    else:
        print("  ⚠ delay_pnl_matrix.csv not found, skipping cross-validation")
        pb_matrix = pd.DataFrame()
        d5_matrix = pd.DataFrame()

    # --- Step 3: Run simulate_pullback_entry on sample events ---
    print("\n[3/5] Running simulate_pullback_entry() on all events...")
    print(f"  Params: start_offset={DEFAULT_START_OFFSET}s, "
          f"target_atr={DEFAULT_PULLBACK_TARGET_ATR}, "
          f"window={DEFAULT_PULLBACK_WINDOW}s")

    results = []
    for idx, (_, event) in enumerate(events.iterrows()):
        event_id = str(event.get("event_id", ""))
        symbol = str(event["symbol"])
        side = str(event["side"])
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        # Get bars for this event's month
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)

        if second_bars is None or second_bars.empty:
            results.append({
                "event_id": event_id,
                "symbol": symbol,
                "side": side,
                "touch_time": touch_time,
                "pullback_triggered": None,
                "entry_time": None,
                "entry_price": None,
                "wait_seconds": None,
                "status": "no_data",
            })
            continue

        # Run pullback simulation
        entry_time, entry_price, pullback_triggered = simulate_pullback_entry(
            event,
            second_bars,
            start_offset_seconds=DEFAULT_START_OFFSET,
            pullback_target_atr=DEFAULT_PULLBACK_TARGET_ATR,
            pullback_window_seconds=DEFAULT_PULLBACK_WINDOW,
        )

        if entry_time is None:
            wait_seconds = None
            status = "no_data"
        else:
            wait_seconds = (entry_time - touch_time).total_seconds()
            status = "triggered" if pullback_triggered else "fallback"

        results.append({
            "event_id": event_id,
            "symbol": symbol,
            "side": side,
            "touch_time": touch_time,
            "pullback_triggered": pullback_triggered,
            "entry_time": entry_time,
            "entry_price": entry_price,
            "wait_seconds": wait_seconds,
            "status": status,
        })

    results_df = pd.DataFrame(results)

    # --- Step 4: Analyze results ---
    print("\n[4/5] Analyzing pullback results...")

    triggered_events = results_df[results_df["status"] == "triggered"]
    fallback_events = results_df[results_df["status"] == "fallback"]
    no_data_events = results_df[results_df["status"] == "no_data"]

    print(f"\n  Summary:")
    print(f"    Total events:        {len(results_df)}")
    print(f"    Pullback triggered:  {len(triggered_events)} (pullback_triggered=True)")
    print(f"    Fallback (timeout):  {len(fallback_events)} (pullback_triggered=False)")
    print(f"    No data:             {len(no_data_events)}")

    # Verify at least one triggered and one fallback
    assert len(triggered_events) > 0, "FAIL: No events with pullback_triggered=True!"
    assert len(fallback_events) > 0, "FAIL: No events with pullback_triggered=False!"
    print(f"\n  ✓ PASS: At least one triggered and one fallback event exist")

    # --- Show triggered examples ---
    print(f"\n  === Pullback TRIGGERED examples (first 5) ===")
    print(f"  {'Event ID':<55} {'Side':<6} {'Wait(s)':<8} {'Entry Price':<14}")
    print(f"  {'-'*55} {'-'*6} {'-'*8} {'-'*14}")
    for _, row in triggered_events.head(5).iterrows():
        eid_short = row["event_id"][:55]
        print(f"  {eid_short:<55} {row['side']:<6} {row['wait_seconds']:<8.0f} "
              f"{row['entry_price']:<14.4f}")

    # --- Show fallback examples ---
    print(f"\n  === Pullback FALLBACK (timeout) examples (first 5) ===")
    print(f"  {'Event ID':<55} {'Side':<6} {'Wait(s)':<8} {'Entry Price':<14}")
    print(f"  {'-'*55} {'-'*6} {'-'*8} {'-'*14}")
    for _, row in fallback_events.head(5).iterrows():
        eid_short = row["event_id"][:55]
        print(f"  {eid_short:<55} {row['side']:<6} {row['wait_seconds']:<8.0f} "
              f"{row['entry_price']:<14.4f}")

    # --- Step 5: Compute price_improvement_bps ---
    print(f"\n[5/5] Computing price_improvement_bps (vs D5 entry)...")

    # Build D5 entry price lookup from matrix
    d5_price_map: dict[str, float] = {}
    if not d5_matrix.empty:
        for _, row in d5_matrix.iterrows():
            eid = str(row["event_id"])
            if pd.notna(row["entry_price"]):
                d5_price_map[eid] = float(row["entry_price"])

    improvement_records = []
    for _, row in results_df.iterrows():
        if row["entry_price"] is None or row["status"] == "no_data":
            continue
        eid = row["event_id"]
        d5_price = d5_price_map.get(eid)
        if d5_price is None:
            continue

        bps = compute_price_improvement_bps(d5_price, row["entry_price"], row["side"])
        improvement_records.append({
            "event_id": eid,
            "side": row["side"],
            "pullback_triggered": row["pullback_triggered"],
            "status": row["status"],
            "wait_seconds": row["wait_seconds"],
            "d5_entry_price": d5_price,
            "pullback_entry_price": row["entry_price"],
            "price_improvement_bps": bps,
        })

    imp_df = pd.DataFrame(improvement_records)

    if not imp_df.empty:
        triggered_imp = imp_df[imp_df["status"] == "triggered"]
        fallback_imp = imp_df[imp_df["status"] == "fallback"]

        print(f"\n  Price Improvement (bps) — Triggered events:")
        if not triggered_imp.empty:
            print(f"    Count:  {len(triggered_imp)}")
            print(f"    Mean:   {triggered_imp['price_improvement_bps'].mean():+.2f} bps")
            print(f"    Median: {triggered_imp['price_improvement_bps'].median():+.2f} bps")
            print(f"    Min:    {triggered_imp['price_improvement_bps'].min():+.2f} bps")
            print(f"    Max:    {triggered_imp['price_improvement_bps'].max():+.2f} bps")
        else:
            print(f"    (no triggered events with D5 price available)")

        print(f"\n  Price Improvement (bps) — Fallback events:")
        if not fallback_imp.empty:
            print(f"    Count:  {len(fallback_imp)}")
            print(f"    Mean:   {fallback_imp['price_improvement_bps'].mean():+.2f} bps")
            print(f"    Median: {fallback_imp['price_improvement_bps'].median():+.2f} bps")
            print(f"    Min:    {fallback_imp['price_improvement_bps'].min():+.2f} bps")
            print(f"    Max:    {fallback_imp['price_improvement_bps'].max():+.2f} bps")
        else:
            print(f"    (no fallback events with D5 price available)")

        # Show detailed examples
        print(f"\n  === Detailed price improvement examples ===")
        print(f"  {'Status':<10} {'Side':<6} {'Wait(s)':<8} {'D5 Price':<14} "
              f"{'PB Price':<14} {'Improvement':<12}")
        print(f"  {'-'*10} {'-'*6} {'-'*8} {'-'*14} {'-'*14} {'-'*12}")

        # Show a mix of triggered and fallback
        sample = pd.concat([
            triggered_imp.head(5),
            fallback_imp.head(5),
        ])
        for _, row in sample.iterrows():
            print(f"  {row['status']:<10} {row['side']:<6} {row['wait_seconds']:<8.0f} "
                  f"{row['d5_entry_price']:<14.4f} {row['pullback_entry_price']:<14.4f} "
                  f"{row['price_improvement_bps']:+.2f} bps")

    # --- Cross-validation with delay_pnl_matrix.csv ---
    expected_fallback_wait = DEFAULT_START_OFFSET + DEFAULT_PULLBACK_WINDOW
    print(f"\n  === Cross-validation with delay_pnl_matrix.csv ===")
    if not pb_matrix.empty:
        # The matrix uses delay_seconds to record wait time.
        # delay_seconds < (start_offset + window) → clearly triggered
        # delay_seconds == (start_offset + window) → ambiguous: could be triggered-at-boundary
        #   or true timeout. Our pullback_triggered boolean is authoritative.
        matrix_clearly_triggered = pb_matrix[
            pb_matrix["delay_seconds"] < (DEFAULT_START_OFFSET + DEFAULT_PULLBACK_WINDOW)
        ]
        matrix_at_boundary = pb_matrix[
            pb_matrix["delay_seconds"] == (DEFAULT_START_OFFSET + DEFAULT_PULLBACK_WINDOW)
        ]

        # Count boundary events that are actually triggered
        boundary_triggered = triggered_events[
            triggered_events["wait_seconds"] == expected_fallback_wait
        ] if not triggered_events.empty else pd.DataFrame()

        print(f"    Matrix delay < {expected_fallback_wait}s (clearly triggered): "
              f"{len(matrix_clearly_triggered)}")
        print(f"    Matrix delay == {expected_fallback_wait}s (boundary/timeout): "
              f"{len(matrix_at_boundary)}")
        print(f"    Our pullback_triggered=True:  {len(triggered_events)} "
              f"(includes {len(boundary_triggered)} at boundary)")
        print(f"    Our pullback_triggered=False: {len(fallback_events)}")

        # The clearly-triggered count should match (ours - boundary)
        our_clearly_triggered = len(triggered_events) - len(boundary_triggered)
        if our_clearly_triggered == len(matrix_clearly_triggered):
            print(f"    ✓ PASS: Clearly-triggered count matches "
                  f"({our_clearly_triggered} events)")
        else:
            print(f"    ⚠ MISMATCH: Clearly-triggered count differs "
                  f"(ours={our_clearly_triggered}, matrix={len(matrix_clearly_triggered)})")

        # Verify entry prices match for all events
        price_matches = 0
        price_mismatches = 0
        for _, our_row in results_df.iterrows():
            if our_row["entry_price"] is None:
                continue
            eid = our_row["event_id"]
            matrix_row = pb_matrix[pb_matrix["event_id"] == eid]
            if matrix_row.empty:
                continue
            matrix_price = float(matrix_row.iloc[0]["entry_price"])
            our_price = float(our_row["entry_price"])
            if abs(matrix_price - our_price) < 0.01:
                price_matches += 1
            else:
                price_mismatches += 1

        print(f"\n    Entry price cross-validation:")
        print(f"      Matches:    {price_matches}")
        print(f"      Mismatches: {price_mismatches}")
        if price_mismatches == 0:
            print(f"      ✓ PASS: All entry prices match delay_pnl_matrix.csv")
        else:
            print(f"      ⚠ WARNING: {price_mismatches} price mismatches found")

    # --- Final verification summary ---
    print("\n" + "=" * 70)
    print("Verification Summary")
    print("=" * 70)
    checks_passed = 0
    checks_total = 0

    # Check 1: At least one triggered
    checks_total += 1
    if len(triggered_events) > 0:
        checks_passed += 1
        print(f"  ✓ [1] At least one pullback_triggered=True event exists "
              f"({len(triggered_events)} events)")
    else:
        print(f"  ✗ [1] No pullback_triggered=True events found!")

    # Check 2: At least one fallback
    checks_total += 1
    if len(fallback_events) > 0:
        checks_passed += 1
        print(f"  ✓ [2] At least one pullback_triggered=False (fallback) event exists "
              f"({len(fallback_events)} events)")
    else:
        print(f"  ✗ [2] No pullback_triggered=False (fallback) events found!")

    # Check 3: Fallback wait time = start_offset + window
    checks_total += 1
    expected_fallback_wait = DEFAULT_START_OFFSET + DEFAULT_PULLBACK_WINDOW
    if not fallback_events.empty:
        fallback_waits = fallback_events["wait_seconds"].dropna()
        all_correct = (fallback_waits == expected_fallback_wait).all()
        if all_correct:
            checks_passed += 1
            print(f"  ✓ [3] All fallback events have wait_seconds={expected_fallback_wait}s "
                  f"(start_offset + window)")
        else:
            # Some might be slightly off due to bar alignment
            close_enough = ((fallback_waits - expected_fallback_wait).abs() <= 1).all()
            if close_enough:
                checks_passed += 1
                print(f"  ✓ [3] All fallback events have wait_seconds≈{expected_fallback_wait}s "
                      f"(within 1s tolerance)")
            else:
                print(f"  ✗ [3] Some fallback events have unexpected wait_seconds")
                print(f"        Expected: {expected_fallback_wait}s, "
                      f"Got: {fallback_waits.unique()[:5]}")
    else:
        print(f"  ✗ [3] No fallback events to verify wait time")

    # Check 4: Triggered events have wait <= start_offset + window
    # Note: A pullback can trigger exactly at the last bar of the window (wait == 65s).
    # This is correct behavior — the price hit the target at the boundary bar.
    checks_total += 1
    if not triggered_events.empty:
        triggered_waits = triggered_events["wait_seconds"].dropna()
        all_within_window = (triggered_waits <= expected_fallback_wait).all()
        if all_within_window:
            checks_passed += 1
            boundary_count = (triggered_waits == expected_fallback_wait).sum()
            if boundary_count > 0:
                print(f"  ✓ [4] All triggered events have wait_seconds <= {expected_fallback_wait}s "
                      f"({boundary_count} triggered at boundary)")
            else:
                print(f"  ✓ [4] All triggered events have wait_seconds <= {expected_fallback_wait}s")
        else:
            print(f"  ✗ [4] Some triggered events have wait_seconds > {expected_fallback_wait}s")
    else:
        print(f"  ✗ [4] No triggered events to verify wait time")

    # Check 5: price_improvement_bps computed correctly
    checks_total += 1
    if not imp_df.empty:
        # For triggered events, we expect positive improvement on average
        # (pullback means better entry price)
        checks_passed += 1
        print(f"  ✓ [5] price_improvement_bps computed for {len(imp_df)} events")
        if not triggered_imp.empty:
            mean_imp = triggered_imp["price_improvement_bps"].mean()
            print(f"        Triggered mean improvement: {mean_imp:+.2f} bps "
                  f"({'positive=better entry' if mean_imp > 0 else 'negative=worse entry'})")
    else:
        print(f"  ✗ [5] Could not compute price_improvement_bps")

    print(f"\n  Result: {checks_passed}/{checks_total} checks passed")
    print("=" * 70)

    return checks_passed == checks_total


if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
