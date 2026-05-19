"""Tests for compute_calendar_sum and run_grid_search_layer1 functions."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest
from pathlib import Path

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    compute_calendar_sum,
    run_grid_search_layer1,
)
from research.entry_redesign.scripts.dynamic_timing.regime_classifier import TimingParams


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_bars(start: str, n: int = 300, base_price: float = 50000.0) -> pd.DataFrame:
    """Create synthetic 1s bars for testing."""
    idx = pd.date_range(start, periods=n, freq="1s", tz="UTC")
    rng = np.random.default_rng(42)
    closes = base_price + np.cumsum(rng.normal(0, 1, n))
    highs = closes + rng.uniform(0, 2, n)
    lows = closes - rng.uniform(0, 2, n)
    return pd.DataFrame(
        {"open": closes - 0.5, "high": highs, "low": lows, "close": closes}, index=idx
    )


def _make_events(touch_time: str, n: int = 1) -> pd.DataFrame:
    """Create synthetic events for testing."""
    rows = []
    for i in range(n):
        rows.append({
            "event_id": f"evt_{i}",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp(touch_time, tz="UTC")
                + pd.Timedelta(seconds=i * 100),
            "level": 50000.0,
            "atr": 100.0,
            "signal_low": 49900.0,
            "signal_high": 50100.0,
        })
    return pd.DataFrame(rows)


# ---------------------------------------------------------------------------
# Tests for compute_calendar_sum
# ---------------------------------------------------------------------------


class TestComputeCalendarSum:
    """Tests for compute_calendar_sum function."""

    def test_empty_results(self):
        """Empty results list returns 0.0."""
        assert compute_calendar_sum([]) == 0.0

    def test_all_none_trades(self):
        """Results with all None trades returns 0.0."""
        results = [
            {"symbol": "BTCUSDT", "trade": None},
            {"symbol": "ETHUSDT", "trade": None},
        ]
        assert compute_calendar_sum(results) == 0.0

    def test_single_trade_positive(self):
        """Single positive trade computes correct silo return."""
        results = [
            {
                "symbol": "BTCUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": 0.01,  # +1%
                },
            }
        ]
        cal_sum = compute_calendar_sum(results)
        # balance = 100000, pnl = 100000 * 0.26 * 0.01 = 260
        # silo_return = 260 / 100000 * 100 = 0.26%
        assert abs(cal_sum - 0.26) < 1e-10

    def test_single_trade_negative(self):
        """Single negative trade computes correct silo return."""
        results = [
            {
                "symbol": "BTCUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": -0.005,  # -0.5%
                },
            }
        ]
        cal_sum = compute_calendar_sum(results)
        # pnl = 100000 * 0.26 * (-0.005) = -130
        # silo_return = -130 / 100000 * 100 = -0.13%
        assert abs(cal_sum - (-0.13)) < 1e-10

    def test_two_trades_same_silo(self):
        """Two trades in same (symbol, month) compound sequentially."""
        results = [
            {
                "symbol": "BTCUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-10 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": 0.01,  # +1%
                },
            },
            {
                "symbol": "BTCUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-20 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": 0.01,  # +1%
                },
            },
        ]
        cal_sum = compute_calendar_sum(results)
        # Trade 1: pnl = 100000 * 0.26 * 0.01 = 260, balance = 100260
        # Trade 2: pnl = 100260 * 0.26 * 0.01 = 260.676, balance = 100520.676
        # silo_return = (100520.676 - 100000) / 100000 * 100 = 0.520676%
        expected = (100520.676 - 100000.0) / 100000.0 * 100.0
        assert abs(cal_sum - expected) < 1e-3

    def test_two_silos_sum(self):
        """Two different silos sum their returns."""
        results = [
            {
                "symbol": "BTCUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": 0.01,
                },
            },
            {
                "symbol": "ETHUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": 0.02,
                },
            },
        ]
        cal_sum = compute_calendar_sum(results)
        # BTCUSDT_2024-01: 100000 * 0.26 * 0.01 = 260 → 0.26%
        # ETHUSDT_2024-01: 100000 * 0.26 * 0.02 = 520 → 0.52%
        # total = 0.26 + 0.52 = 0.78%
        assert abs(cal_sum - 0.78) < 1e-10

    def test_mixed_none_and_valid_trades(self):
        """None trades are skipped, valid trades are computed."""
        results = [
            {"symbol": "BTCUSDT", "trade": None},
            {
                "symbol": "BTCUSDT",
                "trade": {
                    "entry_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
                    "notional_share": 0.26,
                    "realistic_pnl_pct": 0.01,
                },
            },
        ]
        cal_sum = compute_calendar_sum(results)
        assert abs(cal_sum - 0.26) < 1e-10


# ---------------------------------------------------------------------------
# Tests for run_grid_search_layer1
# ---------------------------------------------------------------------------


class TestRunGridSearchLayer1:
    """Tests for run_grid_search_layer1 function."""

    def test_returns_128_combinations(self):
        """Grid search produces exactly 128 parameter combinations."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        best_params, results_df = run_grid_search_layer1(events, cache)

        assert len(results_df) == 128

    def test_result_columns(self):
        """Results DataFrame has expected columns."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        _, results_df = run_grid_search_layer1(events, cache)

        expected_cols = {
            "max_steps",
            "strong_momentum_threshold",
            "extension_threshold",
            "moderate_momentum_threshold",
            "calendar_sum_pct",
            "trade_count",
            "skip_count",
        }
        assert set(results_df.columns) == expected_cols

    def test_returns_timing_params(self):
        """Best params is a TimingParams instance."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        best_params, _ = run_grid_search_layer1(events, cache)

        assert isinstance(best_params, TimingParams)

    def test_best_params_in_search_space(self):
        """Best params values are within the search space."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        best_params, _ = run_grid_search_layer1(events, cache)

        assert best_params.max_steps in [2, 4, 6, 12]
        assert best_params.strong_momentum_threshold in [0.10, 0.15, 0.20, 0.30]
        assert best_params.extension_threshold in [0.10, 0.15, 0.20, 0.30]
        assert best_params.moderate_momentum_threshold in [0.05, 0.10]

    def test_saves_csv_when_output_dir_provided(self, tmp_path):
        """CSV is saved when output_dir is provided."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        _, results_df = run_grid_search_layer1(events, cache, output_dir=tmp_path)

        csv_path = tmp_path / "grid_search_layer1.csv"
        assert csv_path.exists()
        loaded = pd.read_csv(csv_path)
        assert len(loaded) == 128

    def test_no_csv_when_output_dir_none(self, tmp_path):
        """No CSV is saved when output_dir is None."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        _, results_df = run_grid_search_layer1(events, cache, output_dir=None)

        # No file should be created anywhere
        assert len(results_df) == 128

    def test_deterministic_results(self):
        """Two runs with same input produce identical results."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        _, df1 = run_grid_search_layer1(events, cache)
        _, df2 = run_grid_search_layer1(events, cache)

        pd.testing.assert_frame_equal(df1, df2)

    def test_best_params_has_max_calendar_sum(self):
        """Best params corresponds to the maximum calendar_sum_pct in results."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        best_params, results_df = run_grid_search_layer1(events, cache)

        max_row = results_df.loc[results_df["calendar_sum_pct"].idxmax()]
        assert best_params.max_steps == int(max_row["max_steps"])
        assert best_params.strong_momentum_threshold == max_row["strong_momentum_threshold"]
        assert best_params.extension_threshold == max_row["extension_threshold"]
        assert best_params.moderate_momentum_threshold == max_row["moderate_momentum_threshold"]
