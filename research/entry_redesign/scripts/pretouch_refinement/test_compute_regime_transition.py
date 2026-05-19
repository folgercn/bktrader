"""Tests for compute_regime_transition_features() — Task 2.7.

Validates Requirements 1.1 and 1.2:
- regime_transition_adx_30min: 14-period ADX on 1s→1min aggregated bars
- regime_transition_state: {trending: ADX>25, ranging: ADX≤20, transitional: ADX∈(20,25]}
- Point-In-Time constraint: only uses bars before touch_time
"""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from enhanced_features import (
    compute_regime_transition_features,
    _aggregate_to_1min,
    _compute_adx,
    _get_bars_for_event,
)


def _make_1s_bars(
    start_time: str,
    n_bars: int,
    base_price: float = 100.0,
    trend: float = 0.0,
    volatility: float = 0.1,
    seed: int = 42,
) -> pd.DataFrame:
    """Helper: create synthetic 1s bars with controlled trend and volatility."""
    rng = np.random.default_rng(seed)
    idx = pd.date_range(start=start_time, periods=n_bars, freq="1s", tz="UTC")

    closes = np.zeros(n_bars)
    closes[0] = base_price
    for i in range(1, n_bars):
        closes[i] = closes[i - 1] + trend + rng.normal(0, volatility)

    highs = closes + rng.uniform(0, volatility * 2, n_bars)
    lows = closes - rng.uniform(0, volatility * 2, n_bars)
    opens = closes + rng.normal(0, volatility * 0.5, n_bars)

    return pd.DataFrame(
        {"open": opens, "high": highs, "low": lows, "close": closes},
        index=idx,
    )


def _make_trending_bars(start_time: str, n_bars: int = 1800) -> pd.DataFrame:
    """Create bars with strong trend (should produce high ADX)."""
    return _make_1s_bars(
        start_time, n_bars, base_price=100.0, trend=0.05, volatility=0.02, seed=42
    )


def _make_ranging_bars(start_time: str, n_bars: int = 1800) -> pd.DataFrame:
    """Create bars with no trend (should produce low ADX)."""
    rng = np.random.default_rng(42)
    idx = pd.date_range(start=start_time, periods=n_bars, freq="1s", tz="UTC")

    # Oscillate around base price with mean-reversion
    closes = np.zeros(n_bars)
    closes[0] = 100.0
    for i in range(1, n_bars):
        # Mean-reverting: pull back toward 100
        closes[i] = closes[i - 1] + 0.3 * (100.0 - closes[i - 1]) + rng.normal(0, 0.05)

    highs = closes + rng.uniform(0.01, 0.1, n_bars)
    lows = closes - rng.uniform(0.01, 0.1, n_bars)
    opens = closes + rng.normal(0, 0.02, n_bars)

    return pd.DataFrame(
        {"open": opens, "high": highs, "low": lows, "close": closes},
        index=idx,
    )


class TestComputeRegimeTransitionFeatures:
    """Tests for compute_regime_transition_features()."""

    def test_output_columns(self):
        """Output has exactly the expected columns."""
        bars = _make_1s_bars("2024-01-15 09:30:00", 1800)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        assert list(result.columns) == [
            "regime_transition_adx_30min",
            "regime_transition_state",
        ]

    def test_index_preserved(self):
        """Output index matches input events index."""
        bars = _make_1s_bars("2024-01-15 09:30:00", 1800)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame(
            {
                "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
                "symbol": ["BTCUSDT"],
            },
            index=[42],
        )

        result = compute_regime_transition_features(events, bars_cache)

        assert list(result.index) == [42]

    def test_nan_when_no_bars_available(self):
        """Returns NaN when bars_cache has no data for the event."""
        bars_cache = {}  # empty cache

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        assert np.isnan(result["regime_transition_adx_30min"].iloc[0])
        assert np.isnan(result["regime_transition_state"].iloc[0])

    def test_nan_when_insufficient_bars(self):
        """Returns NaN when not enough bars for ADX calculation."""
        # Only 60 bars (1 minute) - not enough for 28 1-min bars
        bars = _make_1s_bars("2024-01-15 09:59:00", 60)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        assert np.isnan(result["regime_transition_adx_30min"].iloc[0])
        assert np.isnan(result["regime_transition_state"].iloc[0])

    def test_adx_value_is_numeric(self):
        """ADX value is a valid float when sufficient data is available."""
        bars = _make_1s_bars("2024-01-15 09:30:00", 1800)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        adx = result["regime_transition_adx_30min"].iloc[0]
        assert not np.isnan(adx)
        assert 0 <= adx <= 100

    def test_trending_market_high_adx(self):
        """Strong trending market should produce ADX > 25 (trending state)."""
        bars = _make_trending_bars("2024-01-15 09:30:00")
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        adx = result["regime_transition_adx_30min"].iloc[0]
        state = result["regime_transition_state"].iloc[0]
        # Strong trend should give high ADX
        assert adx > 25, f"Expected ADX > 25 for trending market, got {adx}"
        assert state == 2  # trending

    def test_ranging_market_low_adx(self):
        """Ranging market should produce ADX ≤ 20 (ranging state)."""
        bars = _make_ranging_bars("2024-01-15 09:30:00")
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        adx = result["regime_transition_adx_30min"].iloc[0]
        state = result["regime_transition_state"].iloc[0]
        # Ranging market should give low ADX
        assert adx <= 20, f"Expected ADX ≤ 20 for ranging market, got {adx}"
        assert state == 0  # ranging

    def test_state_encoding(self):
        """State encoding: ranging=0, transitional=1, trending=2."""
        # We test the encoding logic directly by checking boundary conditions
        bars = _make_1s_bars("2024-01-15 09:30:00", 1800)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        state = result["regime_transition_state"].iloc[0]
        adx = result["regime_transition_adx_30min"].iloc[0]

        # Verify state is consistent with ADX value
        if adx > 25:
            assert state == 2
        elif adx > 20:
            assert state == 1
        else:
            assert state == 0

    def test_multiple_events(self):
        """Handles multiple events correctly."""
        bars_trending = _make_trending_bars("2024-01-15 09:30:00")
        bars_ranging = _make_ranging_bars("2024-02-15 09:30:00")
        bars_cache = {
            "BTCUSDT_202401": bars_trending,
            "BTCUSDT_202402": bars_ranging,
        }

        events = pd.DataFrame({
            "touch_time": pd.to_datetime([
                "2024-01-15 10:00:00+00:00",
                "2024-02-15 10:00:00+00:00",
            ]),
            "symbol": ["BTCUSDT", "BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        assert len(result) == 2
        # First event (trending) should have higher ADX than second (ranging)
        assert result["regime_transition_adx_30min"].iloc[0] > result["regime_transition_adx_30min"].iloc[1]

    def test_point_in_time_constraint(self):
        """Only bars BEFORE touch_time are used (PIT constraint)."""
        # Create bars that span before and after touch_time
        bars = _make_1s_bars("2024-01-15 09:00:00", 7200)  # 2 hours of data
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
            "symbol": ["BTCUSDT"],
        })

        result = compute_regime_transition_features(events, bars_cache)

        # Should produce a valid result (not NaN) since there's enough pre-touch data
        assert not np.isnan(result["regime_transition_adx_30min"].iloc[0])


class TestAggregateTo1Min:
    """Tests for _aggregate_to_1min helper."""

    def test_basic_aggregation(self):
        """60 1s bars aggregate to 1 1min bar."""
        idx = pd.date_range("2024-01-15 10:00:00", periods=60, freq="1s", tz="UTC")
        bars = pd.DataFrame({
            "open": [100.0] * 60,
            "high": [101.0] * 60,
            "low": [99.0] * 60,
            "close": [100.5] * 60,
        }, index=idx)

        result = _aggregate_to_1min(bars)

        assert len(result) == 1
        assert result["open"].iloc[0] == 100.0
        assert result["high"].iloc[0] == 101.0
        assert result["low"].iloc[0] == 99.0
        assert result["close"].iloc[0] == 100.5

    def test_ohlc_aggregation_logic(self):
        """OHLC aggregation: open=first, high=max, low=min, close=last."""
        idx = pd.date_range("2024-01-15 10:00:00", periods=60, freq="1s", tz="UTC")
        opens = [100.0 + i * 0.01 for i in range(60)]
        highs = [102.0 + i * 0.01 for i in range(60)]
        lows = [98.0 - i * 0.01 for i in range(60)]
        closes = [100.5 + i * 0.01 for i in range(60)]

        bars = pd.DataFrame(
            {"open": opens, "high": highs, "low": lows, "close": closes},
            index=idx,
        )

        result = _aggregate_to_1min(bars)

        assert result["open"].iloc[0] == opens[0]  # first
        assert result["high"].iloc[0] == max(highs)  # max
        assert result["low"].iloc[0] == min(lows)  # min
        assert result["close"].iloc[0] == closes[-1]  # last

    def test_multiple_minutes(self):
        """1800 1s bars produce 30 1min bars."""
        idx = pd.date_range("2024-01-15 10:00:00", periods=1800, freq="1s", tz="UTC")
        bars = pd.DataFrame({
            "open": np.ones(1800) * 100,
            "high": np.ones(1800) * 101,
            "low": np.ones(1800) * 99,
            "close": np.ones(1800) * 100.5,
        }, index=idx)

        result = _aggregate_to_1min(bars)

        assert len(result) == 30


class TestComputeADX:
    """Tests for _compute_adx helper."""

    def test_returns_nan_for_insufficient_data(self):
        """Returns NaN when fewer than 2*period bars."""
        idx = pd.date_range("2024-01-15 10:00:00", periods=20, freq="1min", tz="UTC")
        bars = pd.DataFrame({
            "open": np.ones(20) * 100,
            "high": np.ones(20) * 101,
            "low": np.ones(20) * 99,
            "close": np.ones(20) * 100,
        }, index=idx)

        result = _compute_adx(bars, period=14)

        assert np.isnan(result)

    def test_adx_in_valid_range(self):
        """ADX value is between 0 and 100."""
        rng = np.random.default_rng(42)
        n = 30
        idx = pd.date_range("2024-01-15 10:00:00", periods=n, freq="1min", tz="UTC")
        closes = 100 + np.cumsum(rng.normal(0, 0.5, n))
        highs = closes + rng.uniform(0.1, 0.5, n)
        lows = closes - rng.uniform(0.1, 0.5, n)
        opens = closes + rng.normal(0, 0.1, n)

        bars = pd.DataFrame(
            {"open": opens, "high": highs, "low": lows, "close": closes},
            index=idx,
        )

        result = _compute_adx(bars, period=14)

        assert not np.isnan(result)
        assert 0 <= result <= 100

    def test_strong_trend_high_adx(self):
        """Monotonically increasing prices should produce high ADX."""
        n = 30
        idx = pd.date_range("2024-01-15 10:00:00", periods=n, freq="1min", tz="UTC")
        # Strong uptrend
        closes = np.linspace(100, 110, n)
        highs = closes + 0.2
        lows = closes - 0.1
        opens = closes - 0.05

        bars = pd.DataFrame(
            {"open": opens, "high": highs, "low": lows, "close": closes},
            index=idx,
        )

        result = _compute_adx(bars, period=14)

        assert result > 25, f"Expected ADX > 25 for strong trend, got {result}"

    def test_flat_market_low_adx(self):
        """Flat/ranging market should produce low ADX."""
        n = 30
        idx = pd.date_range("2024-01-15 10:00:00", periods=n, freq="1min", tz="UTC")
        rng = np.random.default_rng(123)
        # Oscillating prices with no trend
        closes = 100 + np.sin(np.linspace(0, 6 * np.pi, n)) * 0.5
        highs = closes + rng.uniform(0.1, 0.3, n)
        lows = closes - rng.uniform(0.1, 0.3, n)
        opens = closes + rng.normal(0, 0.05, n)

        bars = pd.DataFrame(
            {"open": opens, "high": highs, "low": lows, "close": closes},
            index=idx,
        )

        result = _compute_adx(bars, period=14)

        assert result < 25, f"Expected ADX < 25 for flat market, got {result}"
