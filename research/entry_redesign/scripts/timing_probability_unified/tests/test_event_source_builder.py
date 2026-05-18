"""Property-based and unit tests for event_source_builder module."""

from __future__ import annotations

import pandas as pd
from hypothesis import given, settings, HealthCheck
from hypothesis import strategies as st

from timing_probability_unified.event_source_builder import (
    EventPoolStats,
    compute_event_pool_stats,
    filter_events_by_bars_cache,
    split_events_by_time,
)


# --- Hypothesis Strategies ---

_symbols = st.sampled_from(["BTCUSDT", "ETHUSDT"])
_sides = st.sampled_from(["long", "short"])
_touch_times = st.datetimes(
    min_value=pd.Timestamp("2020-01-01").to_pydatetime(),
    max_value=pd.Timestamp("2026-12-31").to_pydatetime(),
)


@st.composite
def events_dataframe(draw: st.DrawFn) -> pd.DataFrame:
    """Generate a random events DataFrame with 1..100 rows."""
    n = draw(st.integers(min_value=1, max_value=100))
    symbols = draw(st.lists(_symbols, min_size=n, max_size=n))
    sides = draw(st.lists(_sides, min_size=n, max_size=n))
    touch_times = draw(st.lists(_touch_times, min_size=n, max_size=n))
    return pd.DataFrame(
        {
            "symbol": symbols,
            "side": sides,
            "touch_time": pd.to_datetime(touch_times),
        }
    )


# --- Property Test: Event Pool Statistics Consistency (Property 1) ---
# Feature: timing-probability-unified, Property 1: Event Pool Statistics Consistency


@settings(max_examples=200, suppress_health_check=[HealthCheck.too_slow, HealthCheck.data_too_large])
@given(events=events_dataframe())
def test_event_pool_stats_consistency(events: pd.DataFrame) -> None:
    """Property 1: Event Pool Statistics Consistency.

    For any valid events DataFrame with N rows, the computed EventPoolStats SHALL satisfy:
    - btc_count + eth_count == total_events == N
    - btc_pct + eth_pct == 100.0 (within floating point tolerance)
    - long_count + short_count == total_events
    - earliest_touch_time <= latest_touch_time
    - small_pool_warning == (total_events < 400)

    **Validates: Requirements 1.3, 1.4**
    """
    # Feature: timing-probability-unified, Property 1: Event Pool Statistics Consistency
    stats: EventPoolStats = compute_event_pool_stats(events)
    n = len(events)

    # btc_count + eth_count == total_events == N
    assert stats.total_events == n
    assert stats.btc_count + stats.eth_count == stats.total_events

    # btc_pct + eth_pct == 100.0 (within floating point tolerance)
    assert abs(stats.btc_pct + stats.eth_pct - 100.0) < 1e-9

    # long_count + short_count == total_events
    assert stats.long_count + stats.short_count == stats.total_events

    # earliest_touch_time <= latest_touch_time
    assert stats.earliest_touch_time <= stats.latest_touch_time

    # small_pool_warning == (total_events < 400)
    assert stats.small_pool_warning == (stats.total_events < 400)


# ===========================================================================
# Unit Tests for event_source_builder
# ===========================================================================


class TestComputeEventPoolStatsEmpty:
    """Test compute_event_pool_stats with empty DataFrame."""

    def test_empty_event_pool_returns_zero_total(self) -> None:
        """Empty DataFrame produces stats with total_events=0.

        **Validates: Requirements 1.4**
        """
        events = pd.DataFrame(
            {
                "symbol": pd.Series([], dtype=str),
                "side": pd.Series([], dtype=str),
                "touch_time": pd.Series([], dtype="datetime64[ns, UTC]"),
            }
        )
        stats = compute_event_pool_stats(events)

        assert stats.total_events == 0
        assert stats.btc_count == 0
        assert stats.eth_count == 0
        assert stats.long_count == 0
        assert stats.short_count == 0
        assert stats.small_pool_warning is True

    def test_empty_event_pool_percentages_are_zero(self) -> None:
        """Empty DataFrame produces 0% for all percentage fields."""
        events = pd.DataFrame(
            {
                "symbol": pd.Series([], dtype=str),
                "side": pd.Series([], dtype=str),
                "touch_time": pd.Series([], dtype="datetime64[ns, UTC]"),
            }
        )
        stats = compute_event_pool_stats(events)

        assert stats.btc_pct == 0.0
        assert stats.eth_pct == 0.0
        assert stats.long_pct == 0.0
        assert stats.short_pct == 0.0


class TestComputeEventPoolStatsSingleEvent:
    """Test compute_event_pool_stats with a single event."""

    def test_single_btc_long_event(self) -> None:
        """Single BTC long event produces correct stats.

        **Validates: Requirements 1.3, 1.4**
        """
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT"],
                "side": ["long"],
                "touch_time": [pd.Timestamp("2025-06-01 12:00:00", tz="UTC")],
            }
        )
        stats = compute_event_pool_stats(events)

        assert stats.total_events == 1
        assert stats.btc_count == 1
        assert stats.eth_count == 0
        assert stats.btc_pct == 100.0
        assert stats.eth_pct == 0.0
        assert stats.long_count == 1
        assert stats.short_count == 0
        assert stats.long_pct == 100.0
        assert stats.short_pct == 0.0
        assert stats.earliest_touch_time == stats.latest_touch_time
        assert stats.small_pool_warning is True  # 1 < 400


class TestBarCacheMissingSkipLogic:
    """Test filter_events_by_bars_cache skip logic.

    **Validates: Requirements 1.6**
    """

    def test_event_skipped_when_bars_cache_empty(self) -> None:
        """When bars_cache has empty DataFrame for event's month, event is skipped."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "side": ["long", "short"],
                "touch_time": [
                    pd.Timestamp("2025-06-15 10:00:00", tz="UTC"),
                    pd.Timestamp("2025-07-15 10:00:00", tz="UTC"),
                ],
            }
        )
        # BTC June has data, ETH July is empty
        bars_cache = {
            "BTCUSDT_202506": pd.DataFrame({"close": [100.0, 101.0]}),
            "ETHUSDT_202507": pd.DataFrame(),  # empty → should skip
        }

        valid, skipped = filter_events_by_bars_cache(events, bars_cache)

        assert len(valid) == 1
        assert valid.iloc[0]["symbol"] == "BTCUSDT"
        assert len(skipped) == 1
        assert skipped.iloc[0]["symbol"] == "ETHUSDT"

    def test_event_skipped_when_bars_cache_key_missing(self) -> None:
        """When bars_cache doesn't have the event's month key, event is skipped."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT"],
                "side": ["long"],
                "touch_time": [pd.Timestamp("2025-03-10 08:00:00", tz="UTC")],
            }
        )
        # No key for BTCUSDT_202503
        bars_cache: dict[str, pd.DataFrame] = {}

        valid, skipped = filter_events_by_bars_cache(events, bars_cache)

        assert len(valid) == 0
        assert len(skipped) == 1

    def test_all_events_valid_when_cache_available(self) -> None:
        """When all events have valid bars cache, none are skipped."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "side": ["long", "short"],
                "touch_time": [
                    pd.Timestamp("2025-06-15 10:00:00", tz="UTC"),
                    pd.Timestamp("2025-06-20 10:00:00", tz="UTC"),
                ],
            }
        )
        bars_cache = {
            "BTCUSDT_202506": pd.DataFrame({"close": [100.0]}),
            "ETHUSDT_202506": pd.DataFrame({"close": [200.0]}),
        }

        valid, skipped = filter_events_by_bars_cache(events, bars_cache)

        assert len(valid) == 2
        assert len(skipped) == 0

    def test_empty_events_returns_empty(self) -> None:
        """Empty events DataFrame returns empty valid and skipped."""
        events = pd.DataFrame(
            {
                "symbol": pd.Series([], dtype=str),
                "side": pd.Series([], dtype=str),
                "touch_time": pd.Series([], dtype="datetime64[ns, UTC]"),
            }
        )
        bars_cache: dict[str, pd.DataFrame] = {}

        valid, skipped = filter_events_by_bars_cache(events, bars_cache)

        assert len(valid) == 0
        assert len(skipped) == 0


class TestForwardSplitBoundary:
    """Test split_events_by_time forward split boundary conditions.

    **Validates: Requirements 1.5**
    """

    def test_no_forward_data_all_before_forward_start(self) -> None:
        """When all events are before forward_start, forward_events is empty."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT", "ETHUSDT", "BTCUSDT"],
                "side": ["long", "short", "long"],
                "touch_time": [
                    pd.Timestamp("2025-01-15", tz="UTC"),
                    pd.Timestamp("2025-05-20", tz="UTC"),
                    pd.Timestamp("2025-10-30", tz="UTC"),
                ],
            }
        )

        train, test, forward = split_events_by_time(
            events, forward_start="2025-11-01"
        )

        assert len(forward) == 0
        assert len(train) + len(test) == 3

    def test_all_forward_data_all_after_forward_start(self) -> None:
        """When all events are after forward_start, train/test are empty."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "side": ["long", "short"],
                "touch_time": [
                    pd.Timestamp("2025-11-15", tz="UTC"),
                    pd.Timestamp("2025-12-20", tz="UTC"),
                ],
            }
        )

        train, test, forward = split_events_by_time(
            events, forward_start="2025-11-01"
        )

        assert len(train) == 0
        assert len(test) == 0
        assert len(forward) == 2

    def test_forward_split_preserves_time_ordering(self) -> None:
        """Train events are before test events, both before forward events."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT"] * 5,
                "side": ["long"] * 5,
                "touch_time": [
                    pd.Timestamp("2025-01-01", tz="UTC"),
                    pd.Timestamp("2025-03-01", tz="UTC"),
                    pd.Timestamp("2025-06-01", tz="UTC"),
                    pd.Timestamp("2025-09-01", tz="UTC"),
                    pd.Timestamp("2025-12-01", tz="UTC"),
                ],
            }
        )

        train, test, forward = split_events_by_time(
            events, forward_start="2025-11-01"
        )

        # forward should have the Dec event
        assert len(forward) == 1
        # full-window has 4 events, split 60/40 → 2 train, 2 test
        assert len(train) == 2
        assert len(test) == 2
        # Time ordering: max(train) <= min(test)
        assert train["touch_time"].max() <= test["touch_time"].min()
        # All train/test before forward_start
        forward_ts = pd.Timestamp("2025-11-01", tz="UTC")
        assert train["touch_time"].max() < forward_ts
        assert test["touch_time"].max() < forward_ts

    def test_empty_events_returns_all_empty(self) -> None:
        """Empty events DataFrame returns three empty DataFrames."""
        events = pd.DataFrame(
            {
                "symbol": pd.Series([], dtype=str),
                "side": pd.Series([], dtype=str),
                "touch_time": pd.Series([], dtype="datetime64[ns, UTC]"),
            }
        )

        train, test, forward = split_events_by_time(
            events, forward_start="2025-11-01"
        )

        assert len(train) == 0
        assert len(test) == 0
        assert len(forward) == 0


class TestSmallPoolWarning:
    """Test small_pool_warning threshold logic.

    **Validates: Requirements 1.4**
    """

    def test_below_400_triggers_warning(self) -> None:
        """total_events < 400 triggers small_pool_warning=True."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT"] * 399,
                "side": ["long"] * 399,
                "touch_time": pd.date_range(
                    "2025-01-01", periods=399, freq="h", tz="UTC"
                ),
            }
        )
        stats = compute_event_pool_stats(events)
        assert stats.small_pool_warning is True

    def test_exactly_400_no_warning(self) -> None:
        """total_events == 400 does NOT trigger small_pool_warning."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT"] * 400,
                "side": ["long"] * 400,
                "touch_time": pd.date_range(
                    "2025-01-01", periods=400, freq="h", tz="UTC"
                ),
            }
        )
        stats = compute_event_pool_stats(events)
        assert stats.small_pool_warning is False

    def test_above_400_no_warning(self) -> None:
        """total_events > 400 does NOT trigger small_pool_warning."""
        events = pd.DataFrame(
            {
                "symbol": ["ETHUSDT"] * 500,
                "side": ["short"] * 500,
                "touch_time": pd.date_range(
                    "2025-01-01", periods=500, freq="h", tz="UTC"
                ),
            }
        )
        stats = compute_event_pool_stats(events)
        assert stats.small_pool_warning is False
