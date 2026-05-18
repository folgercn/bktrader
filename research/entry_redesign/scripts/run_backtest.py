#!/usr/bin/env python3
"""Entry Redesign Backtest Runner — 36 candidates × 1303 events.

Loads events from the labeled CSV, finds matching 1s bar pkl caches,
simulates execution for each of the 36 DEFAULT_SUBSET candidates,
and outputs per-candidate metrics to JSON.

Usage (from project root):
    python3 research/entry_redesign/scripts/run_backtest.py

Output:
    research/tmp_entry_redesign_backtest_results.json
"""

from __future__ import annotations

import json
import pathlib
import sys
import time
from dataclasses import asdict
from datetime import datetime, timezone
from typing import Any

import numpy as np
import pandas as pd

# ---------------------------------------------------------------------------
# Path setup
# ---------------------------------------------------------------------------

_SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
_ENTRY_REDESIGN_ROOT = _SCRIPT_DIR.parent
_RESEARCH_DIR = _ENTRY_REDESIGN_ROOT.parent
_PROJECT_ROOT = _RESEARCH_DIR.parent

if str(_PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(_PROJECT_ROOT))

from research.entry_redesign.cost.cost_model_applier import (
    BASELINE_COST_PARAMS,
    CostModelApplier,
    RawTrade,
)
from research.entry_redesign.metrics.metrics_aggregator import MetricsAggregator
from research.entry_redesign.scheduler.default_subset import BASELINE, DEFAULT_SUBSET
from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

EVENTS_CSV = _RESEARCH_DIR / "probabilistic_v6_runs" / "2025m03_2026apr_original_t2_delay60" / "events_execution_labeled.csv"
OUTPUT_JSON = _RESEARCH_DIR / "tmp_entry_redesign_backtest_results.json"

# Execution model parameters (matching the task description)
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
    "notional_share": 0.20,
    "slippage": 0.0002,
}

# All bars_cache directories to search for pkl files
_BARS_CACHE_DIRS: list[pathlib.Path] = []


def _discover_bars_cache_dirs() -> list[pathlib.Path]:
    """Find all bars_cache directories under research/."""
    global _BARS_CACHE_DIRS
    if _BARS_CACHE_DIRS:
        return _BARS_CACHE_DIRS
    _BARS_CACHE_DIRS = sorted(_RESEARCH_DIR.glob("**/bars_cache"))
    return _BARS_CACHE_DIRS


# ---------------------------------------------------------------------------
# Bars loading — find best matching pkl for a (symbol, time_range)
# ---------------------------------------------------------------------------

# Cache: (symbol, month_key) -> DataFrame
_BARS_CACHE: dict[str, pd.DataFrame] = {}


def _find_pkl_for_event(symbol: str, touch_time: pd.Timestamp) -> pathlib.Path | None:
    """Find a pkl file that covers the given touch_time for the symbol.

    Strategy: search all bars_cache dirs for files matching the symbol,
    then check if the time range covers touch_time to touch_time + max_hold.
    """
    needed_start = touch_time
    needed_end = touch_time + pd.Timedelta(hours=EXEC_PARAMS["max_hold_hours"] + 0.5)

    best_path: pathlib.Path | None = None
    best_span: float = float("inf")  # prefer smallest covering file

    for cache_dir in _discover_bars_cache_dirs():
        for pkl_path in cache_dir.glob(f"{symbol}_*_1s.pkl"):
            # Parse start/end from filename: {SYMBOL}_{START}_{END}_{type}_1s.pkl
            parts = pkl_path.stem.split("_")
            # e.g. BTCUSDT_20250301T000000_20250630T235959_flow_1s
            if len(parts) < 4:
                continue
            try:
                start_str = parts[1]  # 20250301T000000
                end_str = parts[2]    # 20250630T235959
                file_start = pd.Timestamp(start_str, tz="UTC")
                file_end = pd.Timestamp(end_str, tz="UTC")
            except Exception:
                continue

            if file_start <= needed_start and file_end >= needed_end:
                span = (file_end - file_start).total_seconds()
                if span < best_span:
                    best_span = span
                    best_path = pkl_path

    return best_path


def _load_bars_for_month(symbol: str, month_start: pd.Timestamp) -> pd.DataFrame | None:
    """Load 1s bars covering an entire month for a symbol.

    Uses a cache keyed by (symbol, YYYY-MM) to avoid reloading.
    Falls back to finding the best pkl that covers the month.
    """
    month_key = f"{symbol}_{month_start.strftime('%Y%m')}"
    if month_key in _BARS_CACHE:
        return _BARS_CACHE[month_key]

    month_end = (month_start + pd.offsets.MonthEnd(1)).normalize() + pd.Timedelta(hours=23, minutes=59, seconds=59)
    if month_end.tzinfo is None:
        month_end = month_end.tz_localize("UTC")

    # Find best pkl covering this month
    best_path: pathlib.Path | None = None
    best_span: float = float("inf")

    for cache_dir in _discover_bars_cache_dirs():
        for pkl_path in cache_dir.glob(f"{symbol}_*_1s.pkl"):
            parts = pkl_path.stem.split("_")
            if len(parts) < 4:
                continue
            try:
                file_start = pd.Timestamp(parts[1], tz="UTC")
                file_end = pd.Timestamp(parts[2], tz="UTC")
            except Exception:
                continue

            if file_start <= month_start and file_end >= month_end:
                span = (file_end - file_start).total_seconds()
                if span < best_span:
                    best_span = span
                    best_path = pkl_path

    if best_path is None:
        # Try to find any pkl that at least partially covers
        for cache_dir in _discover_bars_cache_dirs():
            for pkl_path in cache_dir.glob(f"{symbol}_*_1s.pkl"):
                parts = pkl_path.stem.split("_")
                if len(parts) < 4:
                    continue
                try:
                    file_start = pd.Timestamp(parts[1], tz="UTC")
                    file_end = pd.Timestamp(parts[2], tz="UTC")
                except Exception:
                    continue
                # Partial overlap is acceptable
                if file_start <= month_end and file_end >= month_start:
                    span = (file_end - file_start).total_seconds()
                    if span < best_span:
                        best_span = span
                        best_path = pkl_path

    if best_path is None:
        _BARS_CACHE[month_key] = None  # type: ignore
        return None

    print(f"  Loading bars: {best_path.name} for {month_key}")
    df = pd.read_pickle(best_path)
    if df.index.tzinfo is None:
        df.index = df.index.tz_localize("UTC")
    _BARS_CACHE[month_key] = df
    return df


def _get_bars_for_event(symbol: str, touch_time: pd.Timestamp) -> pd.DataFrame | None:
    """Get 1s bars DataFrame covering the event's execution window."""
    month_start = touch_time.normalize().replace(day=1)
    if month_start.tzinfo is None:
        month_start = month_start.tz_localize("UTC")

    bars = _load_bars_for_month(symbol, month_start)
    if bars is not None:
        return bars

    # If event is near month boundary, try next month too
    next_month = month_start + pd.offsets.MonthBegin(1)
    if next_month.tzinfo is None:
        next_month = next_month.tz_localize("UTC")
    bars = _load_bars_for_month(symbol, next_month)
    return bars


# ---------------------------------------------------------------------------
# Execution simulation (simplified from probabilistic_v4_execution_runner.py)
# ---------------------------------------------------------------------------


def _simulate_single_event(
    bars: pd.DataFrame,
    row: pd.Series,
    entry_delay_seconds: int = 0,
) -> dict | None:
    """Simulate a single event execution on 1s bars.

    Returns a dict with trade results or None if entry not possible.
    """
    touch_time = pd.Timestamp(row["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    entry_time = touch_time + pd.Timedelta(seconds=entry_delay_seconds)
    idx = bars.index
    start_pos = int(idx.searchsorted(entry_time, side="left"))
    if start_pos >= len(idx):
        return None

    # Entry price = close at entry second
    entry_price = float(bars["close"].iloc[start_pos])
    side = str(row["side"])
    atr = float(row["atr"])

    # Compute initial stop
    signal_high = float(row.get("touch_high_so_far", row.get("signal_high", row.get("level", entry_price))))
    signal_low = float(row.get("touch_low_so_far", row.get("signal_low", row.get("level", entry_price))))

    if side == "long":
        raw_stop = min(
            signal_low - EXEC_PARAMS["stop_buffer_atr"] * atr,
            entry_price - EXEC_PARAMS["initial_stop_atr"] * atr,
        )
        capped_stop = entry_price - EXEC_PARAMS["stop_cap_atr"] * atr
        stop = max(raw_stop, capped_stop)
        risk = entry_price - stop
    else:
        raw_stop = max(
            signal_high + EXEC_PARAMS["stop_buffer_atr"] * atr,
            entry_price + EXEC_PARAMS["initial_stop_atr"] * atr,
        )
        capped_stop = entry_price + EXEC_PARAMS["stop_cap_atr"] * atr
        stop = min(raw_stop, capped_stop)
        risk = stop - entry_price

    if risk <= 0 or risk < entry_price * EXEC_PARAMS["min_stop_bps"] / 10000.0:
        return None

    # Position state
    hwm = entry_price
    lwm = entry_price
    mfe_r = 0.0
    mae_r = 0.0
    protected = False
    trailing_active = False
    sl = stop

    # Simulate bar by bar
    end_time = idx[start_pos] + pd.Timedelta(hours=EXEC_PARAMS["max_hold_hours"])
    end_pos = min(int(idx.searchsorted(end_time, side="left")), len(idx) - 1)

    high_arr = bars["high"].to_numpy(dtype="float64", copy=False)
    low_arr = bars["low"].to_numpy(dtype="float64", copy=False)
    close_arr = bars["close"].to_numpy(dtype="float64", copy=False)

    exit_price = None
    exit_reason = None
    exit_time = None

    for pos in range(start_pos + 1, end_pos + 1):
        h = float(high_arr[pos])
        l = float(low_arr[pos])

        # Update excursion
        if side == "long":
            hwm = max(hwm, h)
            favorable = max(0.0, hwm - entry_price)
            adverse = max(0.0, entry_price - l)
            # Breakeven check
            if risk > 0 and favorable / risk >= EXEC_PARAMS["breakeven_at_r"]:
                be_sl = entry_price * (1.0 + EXEC_PARAMS["cost_lock_bps"] / 10000.0)
                if be_sl > sl:
                    sl = be_sl
                    protected = True
            # Trailing check
            if risk > 0 and favorable / risk >= EXEC_PARAMS["trail_start_r"]:
                trail = hwm - EXEC_PARAMS["trail_buffer_atr"] * atr
                if trail > sl:
                    sl = trail
                    trailing_active = True
        else:
            lwm = min(lwm, l)
            favorable = max(0.0, entry_price - lwm)
            adverse = max(0.0, h - entry_price)
            # Breakeven check
            if risk > 0 and favorable / risk >= EXEC_PARAMS["breakeven_at_r"]:
                be_sl = entry_price * (1.0 - EXEC_PARAMS["cost_lock_bps"] / 10000.0)
                if be_sl < sl:
                    sl = be_sl
                    protected = True
            # Trailing check
            if risk > 0 and favorable / risk >= EXEC_PARAMS["trail_start_r"]:
                trail = lwm + EXEC_PARAMS["trail_buffer_atr"] * atr
                if trail < sl:
                    sl = trail
                    trailing_active = True

        if risk > 0:
            mfe_r = max(mfe_r, favorable / risk)
            mae_r = max(mae_r, adverse / risk)

        # Stop trigger check
        triggered = False
        if side == "long" and l <= sl:
            triggered = True
            exit_price = sl
        elif side == "short" and h >= sl:
            triggered = True
            exit_price = sl

        if triggered:
            if trailing_active:
                exit_reason = "TrailingSL"
            elif protected:
                exit_reason = "BreakevenSL"
            else:
                exit_reason = "InitialSL"
            exit_time = idx[pos]
            break

        # Max hold check
        if idx[pos] >= end_time:
            exit_price = float(close_arr[pos])
            exit_reason = "MaxHoldExit"
            exit_time = idx[pos]
            break

    # If loop ended without exit (shouldn't happen but safety)
    if exit_price is None:
        exit_price = float(close_arr[end_pos])
        exit_reason = "FinalMarkToMarket"
        exit_time = idx[end_pos]

    # Compute raw PnL
    notional = 100000.0 * EXEC_PARAMS["notional_share"]  # fixed notional for comparison
    if side == "long":
        raw_pnl = (exit_price - entry_price) / entry_price * notional
    else:
        raw_pnl = (entry_price - exit_price) / entry_price * notional

    return {
        "entry_time": entry_time,
        "exit_time": exit_time,
        "entry_price": entry_price,
        "exit_price": exit_price,
        "side": side,
        "symbol": str(row["symbol"]),
        "notional": notional,
        "raw_pnl": raw_pnl,
        "exit_reason": exit_reason,
        "mfe_r": mfe_r,
        "mae_r": mae_r,
        "signal_start": row["signal_start"],
        "atr": atr,
    }


# ---------------------------------------------------------------------------
# Entry layer modifications for non-baseline candidates
# ---------------------------------------------------------------------------


def _check_trigger_confirmation(
    bars: pd.DataFrame,
    row: pd.Series,
    tc_id: str,
) -> bool:
    """Check if trigger confirmation condition holds in 1s bars after touch.

    Returns True if the event passes the confirmation filter.
    """
    if tc_id == "none":
        return True

    touch_time = pd.Timestamp(row["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    side = str(row["side"])
    level = float(row["level"])
    idx = bars.index
    touch_pos = int(idx.searchsorted(touch_time, side="left"))

    if touch_pos >= len(idx):
        return False

    close_arr = bars["close"].to_numpy(dtype="float64", copy=False)
    high_arr = bars["high"].to_numpy(dtype="float64", copy=False)
    low_arr = bars["low"].to_numpy(dtype="float64", copy=False)
    vol_arr = bars["volume"].to_numpy(dtype="float64", copy=False) if "volume" in bars.columns else None

    # persistence_nX: price stays beyond level for X consecutive seconds
    if tc_id.startswith("persistence_n"):
        n = int(tc_id.split("n")[1])
        end_check = min(touch_pos + n + 5, len(idx))  # small buffer
        consecutive = 0
        for pos in range(touch_pos, end_check):
            if side == "long" and close_arr[pos] >= level:
                consecutive += 1
            elif side == "short" and close_arr[pos] <= level:
                consecutive += 1
            else:
                consecutive = 0
            if consecutive >= n:
                return True
        return False

    # retest_tbX: price retests level within X seconds (touches back)
    if tc_id.startswith("retest_tb"):
        tb = int(tc_id.split("tb")[1])
        # Check if price pulls back to level within tb+5 seconds
        end_check = min(touch_pos + tb + 10, len(idx))
        for pos in range(touch_pos + 1, end_check):
            if side == "long" and low_arr[pos] <= level:
                return True
            elif side == "short" and high_arr[pos] >= level:
                return True
        return False

    # minvol_bpsX: minimum volume in bps of price within first few seconds
    if tc_id.startswith("minvol_bps"):
        bps_threshold = int(tc_id.split("bps")[1])
        if vol_arr is None:
            return True  # no volume data, pass by default
        # Check volume in first 5 seconds after touch
        end_check = min(touch_pos + 5, len(idx))
        total_vol = float(np.sum(vol_arr[touch_pos:end_check]))
        price = float(close_arr[touch_pos]) if touch_pos < len(close_arr) else level
        # Volume threshold: bps of notional
        threshold = price * bps_threshold / 10000.0
        return total_vol >= threshold

    return True


def _compute_entry_price(
    bars: pd.DataFrame,
    row: pd.Series,
    epm_id: str,
    entry_delay_seconds: int,
) -> float | None:
    """Compute adjusted entry price based on entry_price_mode.

    Returns the entry price or None if limit order not filled.
    """
    touch_time = pd.Timestamp(row["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    entry_time = touch_time + pd.Timedelta(seconds=entry_delay_seconds)
    idx = bars.index
    start_pos = int(idx.searchsorted(entry_time, side="left"))
    if start_pos >= len(idx):
        return None

    side = str(row["side"])
    level = float(row["level"])
    atr = float(row["atr"])

    if epm_id == "market_on_touch":
        return float(bars["close"].iloc[start_pos])

    if epm_id == "limit_at_level":
        # Limit order at the breakout level — check if filled within 60s
        end_check = min(start_pos + 60, len(idx))
        low_arr = bars["low"].to_numpy(dtype="float64", copy=False)
        high_arr = bars["high"].to_numpy(dtype="float64", copy=False)
        for pos in range(start_pos, end_check):
            if side == "long" and low_arr[pos] <= level:
                return level
            elif side == "short" and high_arr[pos] >= level:
                return level
        return None  # not filled

    # limit_tb_kX: limit at level + k ticks buffer
    if epm_id.startswith("limit_tb_k"):
        k = int(epm_id.split("k")[1])
        tick_size = atr * 0.001  # approximate tick as 0.1% of ATR
        if side == "long":
            limit_price = level + k * tick_size
        else:
            limit_price = level - k * tick_size
        # Check fill within 60s
        end_check = min(start_pos + 60, len(idx))
        low_arr = bars["low"].to_numpy(dtype="float64", copy=False)
        high_arr = bars["high"].to_numpy(dtype="float64", copy=False)
        for pos in range(start_pos, end_check):
            if side == "long" and low_arr[pos] <= limit_price:
                return limit_price
            elif side == "short" and high_arr[pos] >= limit_price:
                return limit_price
        return None

    # pullback_pXXX: wait for pullback of X% of ATR, then market entry
    if epm_id.startswith("pullback_p"):
        pct_str = epm_id.split("p")[1]
        pct = int(pct_str) / 1000.0  # e.g. p002 = 0.002 ATR
        pullback_dist = pct * atr
        end_check = min(start_pos + 120, len(idx))  # 2 min window
        low_arr = bars["low"].to_numpy(dtype="float64", copy=False)
        high_arr = bars["high"].to_numpy(dtype="float64", copy=False)
        close_arr = bars["close"].to_numpy(dtype="float64", copy=False)
        ref_price = float(close_arr[start_pos])
        for pos in range(start_pos + 1, end_check):
            if side == "long" and low_arr[pos] <= ref_price - pullback_dist:
                return ref_price - pullback_dist
            elif side == "short" and high_arr[pos] >= ref_price + pullback_dist:
                return ref_price + pullback_dist
        return None

    # Fallback: market on touch
    return float(bars["close"].iloc[start_pos])


def _check_pretouch_state_band(row: pd.Series, psb_id: str) -> bool:
    """Filter events based on pretouch state bands using event's existing features.

    fast_clean: speed_300s_atr >= 0.1 AND pullback_60s_atr <= 0.1
    fast_clean_strict: speed_300s_atr >= 0.15 AND pullback_60s_atr <= 0.05
    """
    if psb_id == "none":
        return True

    speed_300s = float(row.get("speed_300s_atr", 0.0))
    pullback_60s = float(row.get("pullback_60s_atr", 999.0))

    if psb_id == "fast_clean":
        return speed_300s >= 0.1 and pullback_60s <= 0.1
    elif psb_id == "fast_clean_strict":
        return speed_300s >= 0.15 and pullback_60s <= 0.05

    return True


def _check_posttouch_quality_band(
    bars: pd.DataFrame,
    row: pd.Series,
    pqb_id: str,
) -> bool:
    """Filter events based on post-touch quality in 1s bars.

    cont1s_rXXX: continuation ratio in first X seconds after touch
    tickimb_bXXX: tick imbalance (buy_volume / total_volume) threshold
    spread_sX: spread quality (not directly available, use volume proxy)
    """
    if pqb_id == "none":
        return True

    touch_time = pd.Timestamp(row["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    side = str(row["side"])
    idx = bars.index
    touch_pos = int(idx.searchsorted(touch_time, side="left"))
    if touch_pos >= len(idx):
        return False

    # cont1s_rXXX: check if price continues in direction for X/10 of ATR
    if pqb_id.startswith("cont1s_r"):
        r_str = pqb_id.split("r")[1]
        r_threshold = int(r_str) / 1000.0  # e.g. r003 = 0.003
        atr = float(row["atr"])
        target_move = r_threshold * atr
        end_check = min(touch_pos + 10, len(idx))  # check first 10 seconds
        close_arr = bars["close"].to_numpy(dtype="float64", copy=False)
        ref_price = float(close_arr[touch_pos])
        for pos in range(touch_pos + 1, end_check):
            if side == "long" and close_arr[pos] - ref_price >= target_move:
                return True
            elif side == "short" and ref_price - close_arr[pos] >= target_move:
                return True
        return False

    # tickimb_bXXX: buy_volume / total_volume >= threshold
    if pqb_id.startswith("tickimb_b"):
        threshold_str = pqb_id.split("b")[1]
        threshold = int(threshold_str) / 100.0  # e.g. b055 = 0.55
        if "buy_volume" not in bars.columns or "volume" not in bars.columns:
            return True  # no data, pass
        end_check = min(touch_pos + 5, len(idx))
        buy_vol = float(bars["buy_volume"].iloc[touch_pos:end_check].sum())
        total_vol = float(bars["volume"].iloc[touch_pos:end_check].sum())
        if total_vol <= 0:
            return False
        ratio = buy_vol / total_vol
        if side == "long":
            return ratio >= threshold
        else:
            return (1.0 - ratio) >= threshold  # sell imbalance for shorts

    # spread_sX: use trade_count as proxy for spread quality
    if pqb_id.startswith("spread_s"):
        s_val = int(pqb_id.split("s")[1])
        if "trade_count" not in bars.columns:
            return True
        end_check = min(touch_pos + 5, len(idx))
        avg_trades = float(bars["trade_count"].iloc[touch_pos:end_check].mean())
        # Higher s_val = stricter filter (more trades required)
        return avg_trades >= s_val * 10.0

    return True


# ---------------------------------------------------------------------------
# Run a single candidate across all events
# ---------------------------------------------------------------------------


def _run_candidate(
    candidate: EntryCandidateSpec,
    events: pd.DataFrame,
    bars_by_key: dict[str, pd.DataFrame | None],
) -> dict[str, Any]:
    """Run a single candidate across all events, return metrics dict."""
    cost_applier = CostModelApplier()
    trades: list[dict] = []
    skipped = {"no_bars": 0, "pretouch_filter": 0, "posttouch_filter": 0,
               "trigger_filter": 0, "no_fill": 0, "sim_fail": 0}

    for _, row in events.iterrows():
        symbol = str(row["symbol"])
        touch_time = pd.Timestamp(row["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        bars = bars_by_key.get(month_key)
        if bars is None:
            skipped["no_bars"] += 1
            continue

        # 1. Pretouch state band filter
        if not _check_pretouch_state_band(row, candidate.pretouch_state_band_id):
            skipped["pretouch_filter"] += 1
            continue

        # 2. Posttouch quality band filter
        if candidate.posttouch_quality_band_id != "none":
            if not _check_posttouch_quality_band(bars, row, candidate.posttouch_quality_band_id):
                skipped["posttouch_filter"] += 1
                continue

        # 3. Trigger confirmation filter
        if candidate.trigger_confirmation_id != "none":
            if not _check_trigger_confirmation(bars, row, candidate.trigger_confirmation_id):
                skipped["trigger_filter"] += 1
                continue

        # 4. Entry price mode
        if candidate.entry_price_mode_id != "market_on_touch":
            entry_price = _compute_entry_price(
                bars, row, candidate.entry_price_mode_id, candidate.entry_delay_seconds
            )
            if entry_price is None:
                skipped["no_fill"] += 1
                continue

        # 5. Simulate execution
        result = _simulate_single_event(
            bars, row, entry_delay_seconds=candidate.entry_delay_seconds
        )
        if result is None:
            skipped["sim_fail"] += 1
            continue

        # If entry_price_mode is not market_on_touch, adjust the result
        if candidate.entry_price_mode_id != "market_on_touch":
            # Recompute raw_pnl with adjusted entry price
            side = result["side"]
            notional = result["notional"]
            if side == "long":
                result["raw_pnl"] = (result["exit_price"] - entry_price) / entry_price * notional
            else:
                result["raw_pnl"] = (entry_price - result["exit_price"]) / entry_price * notional
            result["entry_price"] = entry_price

        # Apply cost model
        raw_trade = RawTrade(
            raw_pnl=result["raw_pnl"],
            notional=result["notional"],
            entry_price=result["entry_price"],
            exit_price=result["exit_price"],
            symbol=result["symbol"],
            side=result["side"],
        )
        priced = cost_applier.apply(raw_trade, BASELINE_COST_PARAMS)

        trades.append({
            "entry_ts": result["entry_time"],
            "exit_ts": result["exit_time"],
            "symbol": result["symbol"],
            "side": result["side"],
            "entry_price": priced.entry_price,
            "exit_price": priced.exit_price,
            "notional": priced.notional,
            "raw_pnl": priced.raw_pnl,
            "slip_pnl": priced.slip_pnl,
            "realistic_pnl": priced.realistic_pnl,
            "realistic_taker_both_pnl": priced.realistic_taker_both_pnl,
            "exit_reason": result["exit_reason"],
            "gate_mode": "nogate",
            "signal_bar_start_ts": result["signal_start"],
        })

    # Build ledger DataFrame for MetricsAggregator
    if trades:
        ledger = pd.DataFrame(trades)
        ledger["entry_ts"] = pd.to_datetime(ledger["entry_ts"], utc=True)
        ledger["exit_ts"] = pd.to_datetime(ledger["exit_ts"], utc=True)
        ledger["signal_bar_start_ts"] = pd.to_datetime(ledger["signal_bar_start_ts"], utc=True)
    else:
        ledger = pd.DataFrame(columns=[
            "entry_ts", "exit_ts", "symbol", "side", "entry_price", "exit_price",
            "notional", "raw_pnl", "slip_pnl", "realistic_pnl",
            "realistic_taker_both_pnl", "exit_reason", "gate_mode",
            "signal_bar_start_ts",
        ])

    # Compute metrics (nogate only for this runner)
    aggregator = MetricsAggregator()
    # MetricsAggregator expects gate_mode column; we only have "nogate"
    metrics = aggregator.aggregate(ledger, total_silos=22)

    return {
        "candidate": {
            "entry_delay_seconds": candidate.entry_delay_seconds,
            "feature_horizon_seconds": candidate.feature_horizon_seconds,
            "trigger_confirmation_id": candidate.trigger_confirmation_id,
            "entry_price_mode_id": candidate.entry_price_mode_id,
            "pretouch_state_band_id": candidate.pretouch_state_band_id,
            "posttouch_quality_band_id": candidate.posttouch_quality_band_id,
        },
        "metrics": metrics,
        "skipped": skipped,
        "trade_count": len(trades),
    }


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main() -> None:
    print("=" * 70)
    print("Entry Redesign Backtest Runner")
    print(f"Events CSV: {EVENTS_CSV}")
    print(f"Output: {OUTPUT_JSON}")
    print("=" * 70)

    t0 = time.time()

    # 1. Load events
    print("\n[1/4] Loading events...")
    events = pd.read_csv(
        EVENTS_CSV,
        parse_dates=["signal_start", "signal_end", "touch_time"],
    )
    print(f"  Loaded {len(events)} events ({events['symbol'].value_counts().to_dict()})")

    # 2. Pre-load bars by (symbol, month) — batch by symbol and month
    print("\n[2/4] Loading 1s bars (by symbol × month)...")
    bars_by_key: dict[str, pd.DataFrame | None] = {}

    for symbol in events["symbol"].unique():
        symbol_events = events[events["symbol"] == symbol]
        touch_times = pd.to_datetime(symbol_events["touch_time"], utc=True)
        months = touch_times.dt.to_period("M").unique()

        for month in sorted(months):
            month_start = month.to_timestamp(freq="s").tz_localize("UTC")
            month_key = f"{symbol}_{month_start.strftime('%Y%m')}"
            if month_key not in bars_by_key:
                bars = _load_bars_for_month(symbol, month_start)
                bars_by_key[month_key] = bars
                if bars is None:
                    print(f"  WARNING: No bars found for {month_key}")
                else:
                    print(f"  {month_key}: {len(bars)} rows")

    loaded_count = sum(1 for v in bars_by_key.values() if v is not None)
    print(f"  Loaded {loaded_count}/{len(bars_by_key)} month-symbol combinations")

    # 3. Run BASELINE first as smoke test
    print("\n[3/4] Running BASELINE candidate (smoke test)...")
    baseline_result = _run_candidate(BASELINE, events, bars_by_key)
    print(f"  BASELINE: {baseline_result['trade_count']} trades")
    print(f"  Metrics (nogate): win_rate={baseline_result['metrics'].get('nogate_win_rate')}, "
          f"realistic_pnl_pct={baseline_result['metrics'].get('nogate_realistic_pnl_pct'):.4f}")
    print(f"  Skipped: {baseline_result['skipped']}")

    # 4. Run all 36 candidates
    print(f"\n[4/4] Running all {len(DEFAULT_SUBSET)} candidates...")
    all_results: list[dict] = []

    for i, candidate in enumerate(DEFAULT_SUBSET):
        is_baseline = (candidate == BASELINE)
        if is_baseline:
            result = baseline_result
        else:
            result = _run_candidate(candidate, events, bars_by_key)

        cand_desc = (
            f"D={candidate.entry_delay_seconds} "
            f"H={candidate.feature_horizon_seconds} "
            f"TC={candidate.trigger_confirmation_id} "
            f"EPM={candidate.entry_price_mode_id} "
            f"PSB={candidate.pretouch_state_band_id} "
            f"PQB={candidate.posttouch_quality_band_id}"
        )
        wr = result["metrics"].get("nogate_win_rate")
        pnl = result["metrics"].get("nogate_realistic_pnl_pct", 0.0)
        print(f"  [{i+1:2d}/{len(DEFAULT_SUBSET)}] {cand_desc}")
        print(f"         trades={result['trade_count']}, win_rate={wr}, realistic_pnl_pct={pnl:.4f}")

        all_results.append(result)

    # 5. Write output JSON
    print(f"\nWriting results to {OUTPUT_JSON}...")

    # Make results JSON-serializable
    def _serialize(obj: Any) -> Any:
        if isinstance(obj, (pd.Timestamp, datetime)):
            return obj.isoformat()
        if isinstance(obj, np.floating):
            return float(obj)
        if isinstance(obj, np.integer):
            return int(obj)
        if isinstance(obj, float) and (np.isnan(obj) or np.isinf(obj)):
            return None
        return obj

    output = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "events_csv": str(EVENTS_CSV),
        "total_events": len(events),
        "exec_params": EXEC_PARAMS,
        "candidates": len(all_results),
        "results": [],
    }

    for r in all_results:
        serialized_metrics = {}
        for k, v in r["metrics"].items():
            serialized_metrics[k] = _serialize(v)
        output["results"].append({
            "candidate": r["candidate"],
            "metrics": serialized_metrics,
            "skipped": r["skipped"],
            "trade_count": r["trade_count"],
        })

    OUTPUT_JSON.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT_JSON.write_text(
        json.dumps(output, indent=2, default=_serialize, ensure_ascii=False),
        encoding="utf-8",
    )

    elapsed = time.time() - t0
    print(f"\nDone in {elapsed:.1f}s. Results written to {OUTPUT_JSON}")


if __name__ == "__main__":
    main()
