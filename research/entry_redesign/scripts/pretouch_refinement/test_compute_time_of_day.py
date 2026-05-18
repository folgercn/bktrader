"""Tests for compute_time_of_day_features() — Task 2.2.

Validates Requirements 1.1 and 1.2:
- time_of_day_hour_utc: touch_time.hour (int 0-23)
- time_of_day_session_overlap: SESSION_OVERLAP_MAP mapping with "none" default
- Point-In-Time constraint: only uses touch_time itself
"""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from enhanced_features import (
    SESSION_OVERLAP_MAP,
    compute_time_of_day_features,
)


class TestComputeTimeOfDayFeatures:
    """Tests for compute_time_of_day_features()."""

    def test_basic_hour_extraction(self):
        """Hour is correctly extracted from datetime touch_time."""
        events = pd.DataFrame({
            "touch_time": pd.to_datetime([
                "2024-01-15 03:30:00+00:00",
                "2024-01-15 14:45:00+00:00",
                "2024-01-15 22:10:00+00:00",
            ]),
        })

        result = compute_time_of_day_features(events)

        assert list(result["time_of_day_hour_utc"]) == [3, 14, 22]

    def test_hour_range_0_to_23(self):
        """All hours 0-23 are correctly extracted."""
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in range(24)]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        assert list(result["time_of_day_hour_utc"]) == list(range(24))

    def test_session_overlap_asia_europe(self):
        """Hours 6-9 map to 'asia_europe'."""
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in [6, 7, 8, 9]]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        assert all(result["time_of_day_session_overlap"] == "asia_europe")

    def test_session_overlap_europe_us(self):
        """Hours 13-16 map to 'europe_us'."""
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in [13, 14, 15, 16]]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        assert all(result["time_of_day_session_overlap"] == "europe_us")

    def test_session_overlap_us_asia(self):
        """Hours 21-23 and 0 map to 'us_asia'."""
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in [21, 22, 23, 0]]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        assert all(result["time_of_day_session_overlap"] == "us_asia")

    def test_session_overlap_none(self):
        """Hours not in SESSION_OVERLAP_MAP map to 'none'."""
        # Hours not in any session: 1-5, 10-12, 17-20
        none_hours = [1, 2, 3, 4, 5, 10, 11, 12, 17, 18, 19, 20]
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in none_hours]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        assert all(result["time_of_day_session_overlap"] == "none")

    def test_output_columns(self):
        """Output has exactly the expected columns."""
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00"]),
        })

        result = compute_time_of_day_features(events)

        assert list(result.columns) == [
            "time_of_day_hour_utc",
            "time_of_day_session_overlap",
        ]

    def test_index_preserved(self):
        """Output index matches input events index."""
        events = pd.DataFrame(
            {"touch_time": pd.to_datetime(["2024-01-15 10:00:00+00:00", "2024-01-15 14:00:00+00:00"])},
            index=[42, 99],
        )

        result = compute_time_of_day_features(events)

        assert list(result.index) == [42, 99]

    def test_string_touch_time_handled(self):
        """String touch_time values are correctly parsed."""
        events = pd.DataFrame({
            "touch_time": ["2024-01-15 08:30:00", "2024-02-20 15:45:00"],
        })

        result = compute_time_of_day_features(events)

        assert list(result["time_of_day_hour_utc"]) == [8, 15]
        assert result["time_of_day_session_overlap"].iloc[0] == "asia_europe"
        assert result["time_of_day_session_overlap"].iloc[1] == "europe_us"

    def test_hour_utc_is_int_type(self):
        """time_of_day_hour_utc values are integers."""
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(["2024-01-15 10:30:45+00:00"]),
        })

        result = compute_time_of_day_features(events)

        assert result["time_of_day_hour_utc"].dtype in [np.int64, np.int32, int]

    def test_session_overlap_completeness(self):
        """All 24 hours produce a valid session overlap value."""
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in range(24)]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        valid_values = {"none", "asia_europe", "europe_us", "us_asia"}
        assert set(result["time_of_day_session_overlap"].unique()).issubset(valid_values)

    def test_session_overlap_map_consistency(self):
        """Output is consistent with SESSION_OVERLAP_MAP constant."""
        times = [f"2024-01-15 {h:02d}:00:00+00:00" for h in range(24)]
        events = pd.DataFrame({
            "touch_time": pd.to_datetime(times),
        })

        result = compute_time_of_day_features(events)

        for i, h in enumerate(range(24)):
            expected = SESSION_OVERLAP_MAP.get(h, "none")
            assert result["time_of_day_session_overlap"].iloc[i] == expected
