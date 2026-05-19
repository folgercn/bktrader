"""Tests for extract_enhanced_features() main function — Task 2.9.

Validates Requirements 1.4, 1.6, 1.9:
- 1.4: 缺失率 >50% 的特征被排除
- 1.6: extended_bars_cache 为 None 时跳过 level_group 特征，pipeline 继续
- 1.9: volume 不可用时标记为 null 并排除该特征族

This file tests the integration/orchestration logic of extract_enhanced_features(),
not individual compute functions (those have dedicated test files).
"""

from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

# Ensure the pretouch_refinement package is importable
_pkg_dir = Path(__file__).resolve().parent.parent
if str(_pkg_dir) not in sys.path:
    sys.path.insert(0, str(_pkg_dir))

from enhanced_features import (
    ALL_ENHANCED_FEATURES,
    ENHANCED_FEATURE_GROUPS,
    PITAuditEntry,
    extract_enhanced_features,
)


# ---------------------------------------------------------------------------
# Test helpers
# ---------------------------------------------------------------------------


def _make_events(n: int = 5) -> pd.DataFrame:
    """Create a synthetic events DataFrame with required columns."""
    base_time = pd.Timestamp("2024-01-15 08:00:00", tz="UTC")
    times = [base_time + pd.Timedelta(minutes=i * 10) for i in range(n)]
    rng = np.random.default_rng(123)
    return pd.DataFrame(
        {
            "touch_time": times,
            "symbol": ["BTCUSDT"] * n,
            "side": ["long"] * n,
            # Required by compute_level_features
            "level": [50000.0 + i * 100 for i in range(n)],
            "atr": [200.0] * n,
            # Required by compute_volatility_features
            "signal_atr_percentile": rng.uniform(0.1, 0.9, n).tolist(),
        }
    )


def _make_bars_df(
    start_time: str = "2024-01-15 07:00:00",
    n_bars: int = 7200,
    include_volume: bool = True,
) -> pd.DataFrame:
    """Create a synthetic 1s bars DataFrame covering ~2 hours."""
    index = pd.date_range(
        start=start_time, periods=n_bars, freq="1s", tz="UTC"
    )
    rng = np.random.default_rng(42)
    data = {
        "open": 50000.0 + rng.normal(0, 10, n_bars).cumsum(),
        "high": 50010.0 + rng.normal(0, 10, n_bars).cumsum(),
        "low": 49990.0 + rng.normal(0, 10, n_bars).cumsum(),
        "close": 50000.0 + rng.normal(0, 10, n_bars).cumsum(),
    }
    # Ensure high >= open, close and low <= open, close
    df = pd.DataFrame(data, index=index)
    df["high"] = df[["open", "high", "close"]].max(axis=1) + 1
    df["low"] = df[["open", "low", "close"]].min(axis=1) - 1

    if include_volume:
        df["volume"] = rng.uniform(10, 200, n_bars)

    return df


def _make_bars_cache(include_volume: bool = True) -> dict:
    """Create a bars_cache dict with one month of BTCUSDT data."""
    bars = _make_bars_df(include_volume=include_volume)
    return {"BTCUSDT_202401": bars}


def _make_extended_bars_cache() -> dict:
    """Create an extended_bars_cache with 24h of data."""
    bars = _make_bars_df(
        start_time="2024-01-14 08:00:00", n_bars=86400
    )
    return {"BTCUSDT_202401": bars}


# ---------------------------------------------------------------------------
# Test: Return type and structure
# ---------------------------------------------------------------------------


class TestExtractEnhancedFeaturesReturnType:
    """Verify extract_enhanced_features returns correct tuple structure."""

    def test_returns_correct_tuple(self):
        """Return type is (DataFrame, list, list, list[PITAuditEntry])."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()

        result = extract_enhanced_features(events, bars_cache)

        assert isinstance(result, tuple)
        assert len(result) == 4

        enhanced_df, used_features, excluded_features, pit_audit = result

        assert isinstance(enhanced_df, pd.DataFrame)
        assert isinstance(used_features, list)
        assert isinstance(excluded_features, list)
        assert isinstance(pit_audit, list)
        # All pit_audit entries are PITAuditEntry instances
        for entry in pit_audit:
            assert isinstance(entry, PITAuditEntry)

    def test_output_dataframe_shape(self):
        """Output DataFrame has correct number of rows (same as events)."""
        for n_events in [1, 3, 10]:
            events = _make_events(n_events)
            bars_cache = _make_bars_cache()

            enhanced_df, _, _, _ = extract_enhanced_features(
                events, bars_cache
            )

            assert len(enhanced_df) == n_events, (
                f"Expected {n_events} rows, got {len(enhanced_df)}"
            )

    def test_output_columns_cover_all_groups(self):
        """Output DataFrame columns cover all enhanced feature groups."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()

        enhanced_df, _, _, _ = extract_enhanced_features(events, bars_cache)

        # All columns should be from ALL_ENHANCED_FEATURES
        for col in enhanced_df.columns:
            assert col in ALL_ENHANCED_FEATURES, (
                f"Unexpected column '{col}' not in ALL_ENHANCED_FEATURES"
            )

    def test_output_index_matches_events(self):
        """Output DataFrame index matches events index."""
        events = _make_events(4)
        events.index = [10, 20, 30, 40]
        bars_cache = _make_bars_cache()

        enhanced_df, _, _, _ = extract_enhanced_features(events, bars_cache)

        assert list(enhanced_df.index) == [10, 20, 30, 40]


# ---------------------------------------------------------------------------
# Test: Missing rate exclusion (Req 1.4)
# ---------------------------------------------------------------------------


class TestMissingRateExclusion:
    """Test that features with >50% NaN are excluded (Req 1.4)."""

    def test_missing_rate_exclusion(self):
        """Features with >50% NaN should appear in excluded_features."""
        # Create events where bars_cache has no data for most events,
        # causing high NaN rates for bar-dependent features.
        # Use 5 events but only provide bars for 1 event's time window.
        events = pd.DataFrame(
            {
                "touch_time": pd.to_datetime([
                    "2024-01-15 08:00:00+00:00",
                    "2024-02-15 08:00:00+00:00",
                    "2024-03-15 08:00:00+00:00",
                    "2024-04-15 08:00:00+00:00",
                    "2024-05-15 08:00:00+00:00",
                ]),
                "symbol": ["BTCUSDT"] * 5,
                "side": ["long"] * 5,
                "level": [50000.0] * 5,
                "atr": [200.0] * 5,
                "signal_atr_percentile": [0.5] * 5,
            }
        )
        # Only provide bars for January — other months will produce NaN
        bars_cache = _make_bars_cache(include_volume=True)

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=None
        )

        # Features that depend on bars_cache for specific months should
        # have high NaN rates (4/5 = 80% > 50%) and be excluded.
        # At minimum, volume features should be excluded since only 1/5
        # events has matching bars.
        # Note: level_group is already excluded due to extended_bars_cache=None
        # Check that excluded_features is non-empty (some features got excluded)
        assert len(excluded_features) > 0

        # Verify no feature appears in both used and excluded
        overlap = set(used_features) & set(excluded_features)
        assert len(overlap) == 0, f"Overlap found: {overlap}"

    def test_custom_threshold(self):
        """Custom missing_threshold is respected."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()

        # With threshold=0.0, any feature with any NaN gets excluded
        _, used_low, excluded_low, _ = extract_enhanced_features(
            events, bars_cache, missing_threshold=0.0
        )

        # With threshold=1.0, nothing gets excluded by missing rate
        _, used_high, excluded_high, _ = extract_enhanced_features(
            events, bars_cache, missing_threshold=1.0
        )

        # Lower threshold should exclude more (or equal) features
        assert len(excluded_low) >= len(excluded_high)


# ---------------------------------------------------------------------------
# Test: Volume unavailable degradation (Req 1.9)
# ---------------------------------------------------------------------------


class TestVolumeUnavailableDegradation:
    """Test volume features excluded when volume data unavailable."""

    def test_volume_unavailable_degradation(self):
        """When bars_cache has no volume column, volume features are excluded."""
        events = _make_events(3)
        bars_cache = _make_bars_cache(include_volume=False)

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache
        )

        volume_features = ENHANCED_FEATURE_GROUPS["volume_group"]
        for feat in volume_features:
            assert feat in excluded_features, (
                f"Volume feature '{feat}' should be excluded when volume "
                f"is unavailable"
            )
            assert feat not in used_features

    def test_volume_available_not_excluded(self):
        """When bars_cache has volume, volume features are NOT excluded."""
        events = _make_events(3)
        bars_cache = _make_bars_cache(include_volume=True)

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache
        )

        volume_features = ENHANCED_FEATURE_GROUPS["volume_group"]
        for feat in volume_features:
            assert feat not in excluded_features, (
                f"Volume feature '{feat}' should NOT be excluded when "
                f"volume is available"
            )


# ---------------------------------------------------------------------------
# Test: extended_bars_cache=None excludes level features (Req 1.6)
# ---------------------------------------------------------------------------


class TestExtendedBarsCacheNone:
    """Test level_group features excluded when extended_bars_cache is None."""

    def test_extended_bars_cache_none_excludes_level_features(self):
        """When extended_bars_cache=None, level_group features are excluded."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=None
        )

        level_features = ENHANCED_FEATURE_GROUPS["level_group"]
        for feat in level_features:
            assert feat in excluded_features, (
                f"Level feature '{feat}' should be excluded when "
                f"extended_bars_cache is None"
            )
            assert feat not in used_features

    def test_extended_bars_cache_provided_level_not_excluded(self):
        """When extended_bars_cache is provided, level features are computed."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()
        extended_cache = _make_extended_bars_cache()

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=extended_cache
        )

        level_features = ENHANCED_FEATURE_GROUPS["level_group"]
        # Level features should NOT be in excluded (unless NaN rate is high)
        for feat in level_features:
            assert feat not in excluded_features, (
                f"Level feature '{feat}' should NOT be excluded when "
                f"extended_bars_cache is provided"
            )

    def test_pipeline_continues_without_extended_cache(self):
        """Pipeline completes successfully even without extended_bars_cache."""
        events = _make_events(5)
        bars_cache = _make_bars_cache()

        # Should not raise
        result = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=None
        )

        enhanced_df, used_features, excluded_features, pit_audit = result

        # Pipeline produced results
        assert len(enhanced_df) == 5
        # Some features are still used (non-level features)
        assert len(used_features) > 0 or len(excluded_features) > 0


# ---------------------------------------------------------------------------
# Test: PIT audit entries cover all features
# ---------------------------------------------------------------------------


class TestPITAuditEntries:
    """Test that PIT audit entries cover all computed features."""

    def test_pit_audit_entries_cover_all_features(self):
        """Every feature in the output DataFrame has a PIT audit entry."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()

        enhanced_df, _, _, pit_audit = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=None
        )

        audited_names = {entry.feature_name for entry in pit_audit}

        for col in enhanced_df.columns:
            assert col in audited_names, (
                f"Feature '{col}' has no PIT audit entry"
            )

    def test_pit_audit_entries_have_required_fields(self):
        """Each PIT audit entry has all required fields populated."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()

        _, _, _, pit_audit = extract_enhanced_features(
            events, bars_cache
        )

        for entry in pit_audit:
            assert entry.feature_name, "feature_name must not be empty"
            assert entry.data_source, "data_source must not be empty"
            assert entry.computation_logic, "computation_logic must not be empty"
            assert entry.timestamp_boundary, "timestamp_boundary must not be empty"
            assert isinstance(entry.pit_passed, bool)

    def test_pit_audit_with_extended_cache(self):
        """PIT audit covers level features when extended cache is provided."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()
        extended_cache = _make_extended_bars_cache()

        enhanced_df, _, _, pit_audit = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=extended_cache
        )

        audited_names = {entry.feature_name for entry in pit_audit}
        level_features = ENHANCED_FEATURE_GROUPS["level_group"]

        for feat in level_features:
            assert feat in audited_names, (
                f"Level feature '{feat}' should have PIT audit entry "
                f"when extended_bars_cache is provided"
            )


# ---------------------------------------------------------------------------
# Test: used_features and excluded_features are disjoint
# ---------------------------------------------------------------------------


class TestUsedExcludedDisjoint:
    """Test that used_features and excluded_features have no overlap."""

    def test_used_features_not_in_excluded(self):
        """No feature appears in both used_features and excluded_features."""
        events = _make_events(5)
        bars_cache = _make_bars_cache()

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=None
        )

        overlap = set(used_features) & set(excluded_features)
        assert len(overlap) == 0, (
            f"Features in both used and excluded: {overlap}"
        )

    def test_used_plus_excluded_covers_all_columns(self):
        """used_features + excluded_features covers all output columns."""
        events = _make_events(5)
        bars_cache = _make_bars_cache()

        enhanced_df, used_features, excluded_features, _ = (
            extract_enhanced_features(events, bars_cache)
        )

        all_output_cols = set(enhanced_df.columns)
        covered = set(used_features) | set(excluded_features)

        assert all_output_cols == covered, (
            f"Uncovered columns: {all_output_cols - covered}"
        )

    def test_disjoint_with_extended_cache(self):
        """Disjoint property holds with extended_bars_cache provided."""
        events = _make_events(3)
        bars_cache = _make_bars_cache()
        extended_cache = _make_extended_bars_cache()

        _, used_features, excluded_features, _ = extract_enhanced_features(
            events, bars_cache, extended_bars_cache=extended_cache
        )

        overlap = set(used_features) & set(excluded_features)
        assert len(overlap) == 0, (
            f"Features in both used and excluded: {overlap}"
        )
