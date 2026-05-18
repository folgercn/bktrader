"""
Unit tests for data_layer module.

Validates:
- load_v6_gate_events() returns ~116 unique events with correct schema
- time_split_events() produces correct 60/40 split with time ordering
- load_bars_cache() loads non-empty DataFrames without None values

Requirements: 5.1, 6.6
"""

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

# ---------------------------------------------------------------------------
# Path setup — allow importing from parent directory
# ---------------------------------------------------------------------------
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from data_layer import (  # noqa: E402
    BARS_CACHE_DIR,
    EVENTS_CSV,
    V6_LEDGER_BASE,
    load_bars_cache,
    load_v6_gate_events,
    time_split_events,
)

# ---------------------------------------------------------------------------
# Skip marker for tests that require real data files
# ---------------------------------------------------------------------------
DATA_AVAILABLE = EVENTS_CSV.exists() and V6_LEDGER_BASE.exists()
skip_no_data = pytest.mark.skipif(
    not DATA_AVAILABLE,
    reason="Real data files not available (EVENTS_CSV or V6_LEDGER_BASE missing)",
)


# ===========================================================================
# Tests for load_v6_gate_events()
# ===========================================================================
class TestLoadV6GateEvents:
    """验证 116 events 加载正确."""

    @skip_no_data
    def test_returns_non_empty_dataframe(self):
        """load_v6_gate_events should return a non-empty DataFrame."""
        events = load_v6_gate_events()
        assert isinstance(events, pd.DataFrame)
        assert len(events) > 0

    @skip_no_data
    def test_event_count_approximately_116(self):
        """Should have ~116 unique events (tolerance: 100-130)."""
        events = load_v6_gate_events()
        unique_count = events["event_id"].nunique()
        assert 100 <= unique_count <= 130, (
            f"Expected ~116 unique events, got {unique_count}"
        )

    @skip_no_data
    def test_required_columns_exist(self):
        """DataFrame must contain required columns: event_id, symbol, side, touch_time, atr, level."""
        events = load_v6_gate_events()
        required_cols = {"event_id", "symbol", "side", "touch_time", "atr", "level"}
        missing = required_cols - set(events.columns)
        assert not missing, f"Missing required columns: {missing}"

    @skip_no_data
    def test_symbols_are_valid(self):
        """Symbols should only be BTCUSDT or ETHUSDT."""
        events = load_v6_gate_events()
        valid_symbols = {"BTCUSDT", "ETHUSDT"}
        actual_symbols = set(events["symbol"].unique())
        assert actual_symbols.issubset(valid_symbols), (
            f"Unexpected symbols: {actual_symbols - valid_symbols}"
        )

    @skip_no_data
    def test_sides_are_valid(self):
        """Sides should only be 'long' or 'short'."""
        events = load_v6_gate_events()
        valid_sides = {"long", "short"}
        actual_sides = set(events["side"].unique())
        assert actual_sides.issubset(valid_sides), (
            f"Unexpected sides: {actual_sides - valid_sides}"
        )


# ===========================================================================
# Tests for time_split_events()
# ===========================================================================
class TestTimeSplitEvents:
    """验证 time split 比例正确（60/40）."""

    def _make_mock_events(self, n: int = 100) -> pd.DataFrame:
        """Create a mock DataFrame with n events and random touch_times."""
        rng = np.random.default_rng(42)
        base_time = pd.Timestamp("2025-01-01", tz="UTC")
        # Generate sorted random offsets to simulate realistic touch_times
        offsets = np.sort(rng.integers(0, 365 * 24 * 3600, size=n))
        touch_times = [base_time + pd.Timedelta(seconds=int(o)) for o in offsets]
        return pd.DataFrame(
            {
                "event_id": range(n),
                "symbol": rng.choice(["BTCUSDT", "ETHUSDT"], size=n),
                "side": rng.choice(["long", "short"], size=n),
                "touch_time": touch_times,
                "atr": rng.uniform(50, 200, size=n),
                "level": rng.uniform(30000, 100000, size=n),
            }
        )

    def test_split_sizes_60_40(self):
        """60/40 split of 100 events should produce 60 train and 40 test."""
        events = self._make_mock_events(100)
        train, test = time_split_events(events, train_ratio=0.6)
        assert len(train) == 60
        assert len(test) == 40

    def test_train_events_before_test_events(self):
        """All train events must have touch_time <= min test touch_time."""
        events = self._make_mock_events(100)
        train, test = time_split_events(events, train_ratio=0.6)
        max_train_time = train["touch_time"].max()
        min_test_time = test["touch_time"].min()
        assert max_train_time <= min_test_time, (
            f"Train max time {max_train_time} > Test min time {min_test_time}"
        )

    def test_indices_are_reset(self):
        """Both train and test DataFrames should have reset (0-based) indices."""
        events = self._make_mock_events(100)
        train, test = time_split_events(events, train_ratio=0.6)
        assert list(train.index) == list(range(len(train)))
        assert list(test.index) == list(range(len(test)))

    def test_custom_ratio(self):
        """Custom train_ratio should produce correct split sizes."""
        events = self._make_mock_events(50)
        train, test = time_split_events(events, train_ratio=0.8)
        assert len(train) == 40
        assert len(test) == 10

    def test_no_data_loss(self):
        """Total events after split should equal original count."""
        events = self._make_mock_events(100)
        train, test = time_split_events(events, train_ratio=0.6)
        assert len(train) + len(test) == 100


# ===========================================================================
# Tests for load_bars_cache()
# ===========================================================================
class TestLoadBarsCache:
    """验证 bars cache 加载无空值."""

    @skip_no_data
    def test_returns_non_empty_dict(self):
        """load_bars_cache should return a non-empty dict."""
        events = load_v6_gate_events()
        cache = load_bars_cache(events)
        assert isinstance(cache, dict)
        assert len(cache) > 0

    @skip_no_data
    def test_all_values_are_dataframes(self):
        """All values in the cache dict must be DataFrames."""
        events = load_v6_gate_events()
        cache = load_bars_cache(events)
        for key, df in cache.items():
            assert isinstance(df, pd.DataFrame), (
                f"Value for key '{key}' is {type(df)}, expected DataFrame"
            )

    @skip_no_data
    def test_no_none_values(self):
        """Cache dict must not contain None values."""
        events = load_v6_gate_events()
        cache = load_bars_cache(events)
        for key, val in cache.items():
            assert val is not None, f"Cache key '{key}' has None value"

    @skip_no_data
    def test_bars_have_ohlc_columns(self):
        """Each cached DataFrame should have OHLC-like columns."""
        events = load_v6_gate_events()
        cache = load_bars_cache(events)
        # At minimum we expect open, high, low, close (case-insensitive check)
        expected_cols = {"open", "high", "low", "close"}
        for key, df in cache.items():
            cols_lower = {c.lower() for c in df.columns}
            missing = expected_cols - cols_lower
            assert not missing, (
                f"Key '{key}' missing OHLC columns: {missing}. "
                f"Available: {list(df.columns)}"
            )

    @skip_no_data
    def test_bars_are_non_empty(self):
        """Each cached DataFrame should be non-empty."""
        events = load_v6_gate_events()
        cache = load_bars_cache(events)
        for key, df in cache.items():
            assert len(df) > 0, f"Cache key '{key}' has empty DataFrame"
