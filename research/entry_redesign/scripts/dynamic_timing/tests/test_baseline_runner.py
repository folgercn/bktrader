"""Tests for run_baseline and run_all_baselines functions."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    run_all_baselines,
    run_baseline,
)
from research.entry_redesign.scripts.dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS


def _make_bars(start: str, n: int = 300, base_price: float = 50000.0) -> pd.DataFrame:
    """Create synthetic 1s bars for testing."""
    idx = pd.date_range(start, periods=n, freq="1s", tz="UTC")
    rng = np.random.default_rng(42)
    closes = base_price + np.cumsum(rng.normal(0, 1, n))
    highs = closes + rng.uniform(0, 2, n)
    lows = closes - rng.uniform(0, 2, n)
    return pd.DataFrame({"open": closes - 0.5, "high": highs, "low": lows, "close": closes}, index=idx)


def _make_events(touch_time: str, n: int = 1) -> pd.DataFrame:
    """Create synthetic events for testing."""
    rows = []
    for i in range(n):
        rows.append({
            "event_id": f"evt_{i}",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp(touch_time, tz="UTC") + pd.Timedelta(seconds=i * 100),
            "level": 50000.0,
            "atr": 100.0,
            "signal_low": 49900.0,
            "signal_high": 50100.0,
        })
    return pd.DataFrame(rows)


class TestRunBaseline:
    """Tests for run_baseline function."""

    def test_basic_delay_0(self):
        """Baseline C (delay=0): entry at first bar >= touch_time."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=0)

        assert len(results) == 1
        r = results[0]
        assert r["entry_decision"] == "baseline"
        assert r["regime"] == "D=0s"
        assert r["entry_time"] is not None
        assert r["entry_price"] is not None
        assert r["entry_delay_seconds"] >= 0

    def test_basic_delay_5(self):
        """Baseline A (delay=5): entry at first bar >= touch_time + 5s."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=5)

        assert len(results) == 1
        r = results[0]
        assert r["entry_decision"] == "baseline"
        assert r["regime"] == "D=5s"
        assert r["entry_time"] is not None
        assert r["entry_delay_seconds"] >= 5.0

    def test_basic_delay_60(self):
        """Baseline B (delay=60): entry at first bar >= touch_time + 60s."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=60)

        assert len(results) == 1
        r = results[0]
        assert r["entry_decision"] == "baseline"
        assert r["regime"] == "D=60s"
        assert r["entry_time"] is not None
        assert r["entry_delay_seconds"] >= 60.0

    def test_no_bars_for_symbol(self):
        """When bars_cache has no data for the symbol, result has None entry."""
        events = _make_events("2024-01-01 00:00:00")
        cache = {}  # empty cache

        results = run_baseline(events, cache, delay_seconds=5)

        assert len(results) == 1
        r = results[0]
        assert r["entry_time"] is None
        assert r["entry_price"] is None
        assert r["trade"] is None

    def test_no_bars_after_target_time(self):
        """When no bars exist after target_time, result has None entry."""
        # Bars end before touch_time + 60s
        bars = _make_bars("2024-01-01 00:00:00", n=30)  # only 30s of data
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=60)

        assert len(results) == 1
        r = results[0]
        assert r["entry_time"] is None
        assert r["entry_price"] is None
        assert r["trade"] is None

    def test_entry_price_is_close(self):
        """Entry price must be the close of the first bar at/after target time."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:05")  # touch at +5s
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=0)

        r = results[0]
        # Entry should be at the bar at 00:00:05 (index 5)
        expected_price = float(bars.iloc[5]["close"])
        assert r["entry_price"] == expected_price

    def test_uses_custom_exec_params(self):
        """Custom exec_params are passed through to execute_trade."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        # Use default params - should produce a result
        results_default = run_baseline(events, cache, delay_seconds=5)
        # Use custom params with very large min_stop_bps to force None trade
        custom_params = {**DEFAULT_EXEC_PARAMS, "min_stop_bps": 99999.0}
        results_custom = run_baseline(events, cache, delay_seconds=5, exec_params=custom_params)

        # Default should have a trade (or None due to min_stop), custom should have None trade
        assert results_custom[0]["trade"] is None

    def test_multiple_events(self):
        """Multiple events are processed correctly."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=3)
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=5)

        assert len(results) == 3
        for r in results:
            assert r["entry_decision"] == "baseline"

    def test_result_structure(self):
        """Result dict has all expected keys."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=5)

        expected_keys = {
            "event_id", "symbol", "side", "touch_time",
            "entry_decision", "regime", "decision_path",
            "entry_time", "entry_price", "entry_delay_seconds", "trade",
        }
        assert set(results[0].keys()) == expected_keys

    def test_decision_path_is_empty(self):
        """Baseline has no decision path (no step-by-step decisions)."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        results = run_baseline(events, cache, delay_seconds=5)

        assert results[0]["decision_path"] == []


class TestRunAllBaselines:
    """Tests for run_all_baselines function."""

    def test_returns_three_baselines(self):
        """run_all_baselines returns dict with baseline_a, baseline_b, baseline_c."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        result = run_all_baselines(events, cache)

        assert set(result.keys()) == {"baseline_a", "baseline_b", "baseline_c"}

    def test_baseline_delays_correct(self):
        """Each baseline uses the correct delay."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        result = run_all_baselines(events, cache)

        # Baseline A = D=5s
        assert result["baseline_a"][0]["regime"] == "D=5s"
        assert result["baseline_a"][0]["entry_delay_seconds"] >= 5.0
        # Baseline B = D=60s
        assert result["baseline_b"][0]["regime"] == "D=60s"
        assert result["baseline_b"][0]["entry_delay_seconds"] >= 60.0
        # Baseline C = D=0s
        assert result["baseline_c"][0]["regime"] == "D=0s"
        assert result["baseline_c"][0]["entry_delay_seconds"] >= 0.0

    def test_passes_custom_exec_params(self):
        """Custom exec_params are forwarded to all baselines."""
        bars = _make_bars("2024-01-01 00:00:00")
        events = _make_events("2024-01-01 00:00:00")
        cache = {"BTCUSDT_202401": bars}

        # Very large min_stop_bps forces all trades to None
        custom_params = {**DEFAULT_EXEC_PARAMS, "min_stop_bps": 99999.0}
        result = run_all_baselines(events, cache, exec_params=custom_params)

        for key in ("baseline_a", "baseline_b", "baseline_c"):
            assert result[key][0]["trade"] is None
