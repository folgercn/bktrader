"""Tests for compute_volume_features() — Task 2.3.

Validates Requirements 1.1, 1.8, 1.9:
- volume_regime_ratio: signal bar volume / 前 20 根 bar volume rolling mean
- volume_regime_percentile: ratio 在所有 events 内的分位数 (0-1)
- volume 不可用时返回全 null 并标记 volume_regime_unavailable=true
"""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from enhanced_features import (
    ENHANCED_FEATURE_GROUPS,
    compute_volume_features,
)


def _make_bars_df(
    n_bars: int = 100,
    start_time: str = "2024-01-15 00:00:00",
    freq: str = "1s",
    volume_values: list | np.ndarray | None = None,
    include_volume: bool = True,
) -> pd.DataFrame:
    """Helper: create a synthetic 1s bars DataFrame."""
    index = pd.date_range(start=start_time, periods=n_bars, freq=freq, tz="UTC")
    data = {
        "open": np.random.uniform(100, 101, n_bars),
        "high": np.random.uniform(101, 102, n_bars),
        "low": np.random.uniform(99, 100, n_bars),
        "close": np.random.uniform(100, 101, n_bars),
    }
    if include_volume:
        if volume_values is not None:
            data["volume"] = volume_values
        else:
            data["volume"] = np.random.uniform(10, 100, n_bars)
    return pd.DataFrame(data, index=index)


class TestComputeVolumeFeaturesBasic:
    """Basic functionality tests."""

    def test_output_columns(self):
        """Output has exactly the expected columns."""
        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:50+00:00"]),
        })
        bars = _make_bars_df(n_bars=100, start_time="2024-01-15 00:00:00")
        bars_cache = {"BTCUSDT_202401": bars}

        result = compute_volume_features(events, bars_cache)

        assert list(result.columns) == ENHANCED_FEATURE_GROUPS["volume_group"]

    def test_index_preserved(self):
        """Output index matches input events index."""
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "touch_time": pd.to_datetime([
                    "2024-01-15 00:00:50+00:00",
                    "2024-01-15 00:00:50+00:00",
                ]),
            },
            index=[42, 99],
        )
        bars = _make_bars_df(n_bars=100, start_time="2024-01-15 00:00:00")
        bars_cache = {
            "BTCUSDT_202401": bars,
            "ETHUSDT_202401": bars.copy(),
        }

        result = compute_volume_features(events, bars_cache)

        assert list(result.index) == [42, 99]

    def test_ratio_calculation_correct(self):
        """volume_regime_ratio = signal_bar_volume / mean(preceding 20 bars volume)."""
        # Create bars with known volumes
        n_bars = 30
        volumes = np.ones(n_bars) * 50.0  # all volumes = 50
        volumes[-1] = 100.0  # signal bar volume = 100
        bars = _make_bars_df(n_bars=n_bars, start_time="2024-01-15 00:00:00",
                             volume_values=volumes)

        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:29+00:00"]),
        })
        bars_cache = {"BTCUSDT_202401": bars}

        result = compute_volume_features(events, bars_cache)

        # ratio = 100 / mean(50 * 20) = 100 / 50 = 2.0
        expected_ratio = 100.0 / 50.0
        assert np.isclose(result["volume_regime_ratio"].iloc[0], expected_ratio)

    def test_percentile_range_0_to_1(self):
        """volume_regime_percentile values are in (0, 1]."""
        n_bars = 100
        bars = _make_bars_df(n_bars=n_bars, start_time="2024-01-15 00:00:00")
        bars_cache = {"BTCUSDT_202401": bars}

        # Multiple events at different times
        events = pd.DataFrame({
            "symbol": ["BTCUSDT"] * 5,
            "touch_time": pd.to_datetime([
                "2024-01-15 00:00:30+00:00",
                "2024-01-15 00:00:40+00:00",
                "2024-01-15 00:00:50+00:00",
                "2024-01-15 00:01:00+00:00",
                "2024-01-15 00:01:10+00:00",
            ]),
        })

        result = compute_volume_features(events, bars_cache)

        valid_pct = result["volume_regime_percentile"].dropna()
        assert (valid_pct > 0).all()
        assert (valid_pct <= 1).all()


class TestComputeVolumeFeaturesVolumeUnavailable:
    """Tests for volume unavailable scenarios (Req 1.9)."""

    def test_no_volume_column_returns_all_nan(self):
        """When bars have no volume column, return all NaN."""
        bars = _make_bars_df(n_bars=100, include_volume=False)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:50+00:00"]),
        })

        result = compute_volume_features(events, bars_cache)

        assert result["volume_regime_ratio"].isna().all()
        assert result["volume_regime_percentile"].isna().all()

    def test_all_nan_volume_returns_all_nan(self):
        """When volume column is all NaN, return all NaN."""
        n_bars = 100
        volumes = np.full(n_bars, np.nan)
        bars = _make_bars_df(n_bars=n_bars, volume_values=volumes)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:50+00:00"]),
        })

        result = compute_volume_features(events, bars_cache)

        assert result["volume_regime_ratio"].isna().all()
        assert result["volume_regime_percentile"].isna().all()

    def test_empty_bars_cache_returns_all_nan(self):
        """When bars_cache is empty, return all NaN."""
        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:50+00:00"]),
        })

        result = compute_volume_features(events, {})

        assert result["volume_regime_ratio"].isna().all()
        assert result["volume_regime_percentile"].isna().all()


class TestComputeVolumeFeaturesEdgeCases:
    """Edge case tests."""

    def test_event_not_in_cache_returns_nan_for_that_event(self):
        """When event's symbol/month not in cache, that event gets NaN."""
        bars = _make_bars_df(n_bars=100, start_time="2024-01-15 00:00:00")
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "symbol": ["BTCUSDT", "ETHUSDT"],
            "touch_time": pd.to_datetime([
                "2024-01-15 00:00:50+00:00",
                "2024-01-15 00:00:50+00:00",
            ]),
        })

        result = compute_volume_features(events, bars_cache)

        # BTCUSDT should have a valid ratio
        assert not pd.isna(result["volume_regime_ratio"].iloc[0])
        # ETHUSDT not in cache → NaN
        assert pd.isna(result["volume_regime_ratio"].iloc[1])

    def test_insufficient_preceding_bars(self):
        """When fewer than 20 preceding bars, still computes with available bars."""
        # Only 5 bars before signal bar
        n_bars = 6
        volumes = np.array([10.0, 20.0, 30.0, 40.0, 50.0, 200.0])
        bars = _make_bars_df(n_bars=n_bars, start_time="2024-01-15 00:00:00",
                             volume_values=volumes)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:05+00:00"]),
        })

        result = compute_volume_features(events, bars_cache)

        # signal bar = index 5 (volume=200), preceding = index 0-4 (mean=30)
        expected_ratio = 200.0 / 30.0
        assert np.isclose(result["volume_regime_ratio"].iloc[0], expected_ratio)

    def test_single_event(self):
        """Single event produces percentile of 1.0."""
        n_bars = 50
        volumes = np.ones(n_bars) * 50.0
        volumes[-1] = 100.0
        bars = _make_bars_df(n_bars=n_bars, start_time="2024-01-15 00:00:00",
                             volume_values=volumes)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:49+00:00"]),
        })

        result = compute_volume_features(events, bars_cache)

        # Single valid event → percentile = 1.0
        assert result["volume_regime_percentile"].iloc[0] == 1.0

    def test_zero_volume_rolling_mean_returns_nan(self):
        """When rolling mean is 0, ratio is NaN (avoid division by zero)."""
        n_bars = 30
        volumes = np.zeros(n_bars)
        volumes[-1] = 100.0  # signal bar has volume
        bars = _make_bars_df(n_bars=n_bars, start_time="2024-01-15 00:00:00",
                             volume_values=volumes)
        bars_cache = {"BTCUSDT_202401": bars}

        events = pd.DataFrame({
            "symbol": ["BTCUSDT"],
            "touch_time": pd.to_datetime(["2024-01-15 00:00:29+00:00"]),
        })

        result = compute_volume_features(events, bars_cache)

        # rolling_mean = 0 → skip (NaN)
        assert pd.isna(result["volume_regime_ratio"].iloc[0])
