"""Unit tests for feature_engine.extract_step_features()."""

import numpy as np
import pandas as pd
import pytest

from feature_engine import StepFeatures, extract_step_features


def _make_bars(timestamps: list[pd.Timestamp], prices: list[dict]) -> pd.DataFrame:
    """Helper: create a 1s bar DataFrame with DatetimeIndex."""
    df = pd.DataFrame(prices, index=pd.DatetimeIndex(timestamps))
    df.index.name = None
    return df


def _make_event(touch_time, level, atr, side) -> pd.Series:
    """Helper: create a minimal event Series."""
    return pd.Series(
        {"touch_time": touch_time, "level": level, "atr": atr, "side": side}
    )


class TestExtractStepFeaturesNone:
    """窗口内无数据返回 None."""

    def test_empty_bars(self):
        bars = pd.DataFrame(columns=["open", "high", "low", "close"])
        bars.index = pd.DatetimeIndex([], tz="UTC")
        event = _make_event(
            pd.Timestamp("2025-01-01 00:00:00", tz="UTC"), 100.0, 1.0, "long"
        )
        result = extract_step_features(bars, event, step_index=1)
        assert result is None

    def test_bars_outside_window(self):
        """Bars exist but all outside the observation window."""
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        # Bars start 10s after window end (step_index=1, window=[0s, 5s])
        timestamps = [touch + pd.Timedelta(seconds=10 + i) for i in range(5)]
        prices = [{"open": 100, "high": 101, "low": 99, "close": 100}] * 5
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch, 100.0, 1.0, "long")
        result = extract_step_features(bars, event, step_index=1)
        assert result is None


class TestExtractStepFeaturesLong:
    """Long side feature calculations."""

    @pytest.fixture
    def setup(self):
        """Create a known scenario for long side."""
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        level = 100.0
        atr = 2.0
        side = "long"

        # 5 bars in window [touch, touch+5s], step_index=1, step_interval=5
        timestamps = [touch + pd.Timedelta(seconds=i) for i in range(5)]
        prices = [
            {"open": 100.0, "high": 100.5, "low": 99.8, "close": 100.2},  # t+0
            {"open": 100.2, "high": 100.8, "low": 100.0, "close": 100.6},  # t+1
            {"open": 100.6, "high": 101.0, "low": 100.4, "close": 100.8},  # t+2
            {"open": 100.8, "high": 101.2, "low": 100.6, "close": 101.0},  # t+3
            {"open": 101.0, "high": 101.5, "low": 100.8, "close": 101.2},  # t+4
        ]
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch, level, atr, side)
        return bars, event, touch, level, atr

    def test_returns_step_features(self, setup):
        bars, event, _, _, _ = setup
        result = extract_step_features(bars, event, step_index=1)
        assert isinstance(result, StepFeatures)
        assert result.step_index == 1

    def test_extension_atr_long(self, setup):
        bars, event, _, level, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # price_at_step_k = 101.2 (last close)
        # extension_atr = (101.2 - 100.0) / 2.0 = 0.6
        assert result is not None
        assert pytest.approx(result.extension_atr, abs=1e-9) == 0.6

    def test_speed_cumulative_atr(self, setup):
        bars, event, _, _, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # price_at_touch = 100.2 (first close), price_at_step_k = 101.2
        # speed = abs(101.2 - 100.2) / 2.0 = 0.5
        assert result is not None
        assert pytest.approx(result.speed_cumulative_atr, abs=1e-9) == 0.5

    def test_max_extension_atr_long(self, setup):
        bars, event, _, level, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # max(high - level) = max(100.5, 100.8, 101.0, 101.2, 101.5) - 100 = 1.5
        # max_extension_atr = 1.5 / 2.0 = 0.75
        assert result is not None
        assert pytest.approx(result.max_extension_atr, abs=1e-9) == 0.75

    def test_pullback_from_max_atr_long(self, setup):
        bars, event, _, _, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # max(high) = 101.5, price_at_step_k = 101.2
        # pullback = (101.5 - 101.2) / 2.0 = 0.15
        assert result is not None
        assert pytest.approx(result.pullback_from_max_atr, abs=1e-9) == 0.15

    def test_flow_imbalance_placeholder(self, setup):
        bars, event, _, _, _ = setup
        result = extract_step_features(bars, event, step_index=1)
        assert result is not None
        assert result.flow_imbalance_cumulative == 0.5
        assert result.flow_imbalance_last_step == 0.5

    def test_dwell_ratio(self, setup):
        bars, event, _, level, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # dwell_threshold = 0.05 * 2.0 = 0.1
        # close values: 100.2, 100.6, 100.8, 101.0, 101.2
        # |close - 100| = 0.2, 0.6, 0.8, 1.0, 1.2 — all > 0.1
        # dwell_ratio = 0 / 5 = 0.0
        assert result is not None
        assert result.dwell_ratio == 0.0

    def test_continuation_ratio_long(self, setup):
        bars, event, _, _, _ = setup
        result = extract_step_features(bars, event, step_index=1)
        # All 5 bars have close > open: 100.2>100, 100.6>100.2, 100.8>100.6, 101.0>100.8, 101.2>101.0
        # continuation_ratio = 5/5 = 1.0
        assert result is not None
        assert result.continuation_ratio == 1.0

    def test_speed_last_step_equals_cumulative_for_step1(self, setup):
        bars, event, _, _, _ = setup
        result = extract_step_features(bars, event, step_index=1)
        # For step_index=1, last_step covers the entire window
        # speed_last_step_atr should equal speed_cumulative_atr
        assert result is not None
        assert pytest.approx(result.speed_last_step_atr, abs=1e-9) == result.speed_cumulative_atr


class TestExtractStepFeaturesShort:
    """Short side feature calculations."""

    @pytest.fixture
    def setup(self):
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        level = 100.0
        atr = 2.0
        side = "short"

        # Price drops below level (short breakout)
        timestamps = [touch + pd.Timedelta(seconds=i) for i in range(5)]
        prices = [
            {"open": 100.0, "high": 100.2, "low": 99.5, "close": 99.8},  # t+0
            {"open": 99.8, "high": 100.0, "low": 99.2, "close": 99.4},  # t+1
            {"open": 99.4, "high": 99.6, "low": 99.0, "close": 99.2},  # t+2
            {"open": 99.2, "high": 99.4, "low": 98.8, "close": 99.0},  # t+3
            {"open": 99.0, "high": 99.2, "low": 98.5, "close": 98.8},  # t+4
        ]
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch, level, atr, side)
        return bars, event, touch, level, atr

    def test_extension_atr_short(self, setup):
        bars, event, _, level, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # price_at_step_k = 98.8
        # extension_atr = (100.0 - 98.8) / 2.0 = 0.6
        assert result is not None
        assert pytest.approx(result.extension_atr, abs=1e-9) == 0.6

    def test_max_extension_atr_short(self, setup):
        bars, event, _, level, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # max(level - low) = max(100-99.5, 100-99.2, 100-99.0, 100-98.8, 100-98.5)
        # = max(0.5, 0.8, 1.0, 1.2, 1.5) = 1.5
        # max_extension_atr = 1.5 / 2.0 = 0.75
        assert result is not None
        assert pytest.approx(result.max_extension_atr, abs=1e-9) == 0.75

    def test_pullback_from_max_atr_short(self, setup):
        bars, event, _, _, atr = setup
        result = extract_step_features(bars, event, step_index=1)
        # min(low) = 98.5, price_at_step_k = 98.8
        # pullback = (98.8 - 98.5) / 2.0 = 0.15
        assert result is not None
        assert pytest.approx(result.pullback_from_max_atr, abs=1e-9) == 0.15

    def test_continuation_ratio_short(self, setup):
        bars, event, _, _, _ = setup
        result = extract_step_features(bars, event, step_index=1)
        # All 5 bars have close < open: 99.8<100, 99.4<99.8, 99.2<99.4, 99.0<99.2, 98.8<99.0
        # continuation_ratio = 5/5 = 1.0
        assert result is not None
        assert result.continuation_ratio == 1.0


class TestExtractStepFeaturesMultiStep:
    """Multi-step scenarios (step_index > 1)."""

    def test_step2_uses_wider_window(self):
        """step_index=2 should use [touch, touch+10s] window."""
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        level = 100.0
        atr = 1.0

        # 10 bars covering [touch, touch+9s]
        timestamps = [touch + pd.Timedelta(seconds=i) for i in range(10)]
        prices = [
            {"open": 100.0, "high": 100.1, "low": 99.9, "close": 100.05}
        ] * 10
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch, level, atr, "long")

        result = extract_step_features(bars, event, step_index=2)
        assert result is not None
        assert result.step_index == 2

    def test_speed_last_step_different_from_cumulative(self):
        """For step_index=2, speed_last_step should only use last 5s."""
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        level = 100.0
        atr = 1.0

        # First 5s: price goes from 100 to 101
        # Last 5s: price stays flat at 101
        timestamps = [touch + pd.Timedelta(seconds=i) for i in range(10)]
        prices = [
            {"open": 100.0, "high": 100.2, "low": 99.9, "close": 100.2},  # t+0
            {"open": 100.2, "high": 100.4, "low": 100.1, "close": 100.4},  # t+1
            {"open": 100.4, "high": 100.6, "low": 100.3, "close": 100.6},  # t+2
            {"open": 100.6, "high": 100.8, "low": 100.5, "close": 100.8},  # t+3
            {"open": 100.8, "high": 101.0, "low": 100.7, "close": 101.0},  # t+4
            {"open": 101.0, "high": 101.0, "low": 101.0, "close": 101.0},  # t+5
            {"open": 101.0, "high": 101.0, "low": 101.0, "close": 101.0},  # t+6
            {"open": 101.0, "high": 101.0, "low": 101.0, "close": 101.0},  # t+7
            {"open": 101.0, "high": 101.0, "low": 101.0, "close": 101.0},  # t+8
            {"open": 101.0, "high": 101.0, "low": 101.0, "close": 101.0},  # t+9
        ]
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch, level, atr, "long")

        result = extract_step_features(bars, event, step_index=2)
        assert result is not None
        # speed_cumulative = abs(101.0 - 100.2) / 1.0 = 0.8
        assert pytest.approx(result.speed_cumulative_atr, abs=1e-9) == 0.8
        # speed_last_step: last 5s is [t+5, t+9], first close=101.0, last close=101.0
        # speed_last_step = abs(101.0 - 101.0) / 1.0 = 0.0
        assert pytest.approx(result.speed_last_step_atr, abs=1e-9) == 0.0


class TestPointInTimeConstraint:
    """Verify Point_In_Time constraint: no future data leakage."""

    def test_does_not_use_future_bars(self):
        """Bars after window_end should not affect features."""
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        level = 100.0
        atr = 1.0

        # Window for step_index=1, step_interval=5 is [touch, touch+5s]
        # Bars at t+0..t+5 are IN window (6 bars), t+6..t+9 are AFTER window
        timestamps = [touch + pd.Timedelta(seconds=i) for i in range(10)]
        prices_in_window = [
            {"open": 100.0, "high": 100.5, "low": 99.8, "close": 100.2},
        ] * 6  # t+0 through t+5 (inclusive)
        # Future bars with extreme prices that should NOT affect features
        prices_future = [
            {"open": 100.0, "high": 200.0, "low": 50.0, "close": 150.0},
        ] * 4  # t+6 through t+9
        bars = _make_bars(timestamps, prices_in_window + prices_future)
        event = _make_event(touch, level, atr, "long")

        result = extract_step_features(bars, event, step_index=1)
        assert result is not None
        # max_extension should only reflect in-window data
        # max(high - level) in window = (100.5 - 100) = 0.5
        assert pytest.approx(result.max_extension_atr, abs=1e-9) == 0.5


class TestDwellRatio:
    """Dwell ratio edge cases."""

    def test_all_bars_near_level(self):
        """All bars close within level ± 0.05*ATR → dwell_ratio = 1.0."""
        touch = pd.Timestamp("2025-01-01 00:00:00", tz="UTC")
        level = 100.0
        atr = 2.0  # threshold = 0.05 * 2.0 = 0.1

        timestamps = [touch + pd.Timedelta(seconds=i) for i in range(5)]
        # All closes within 100 ± 0.1
        prices = [
            {"open": 100.0, "high": 100.05, "low": 99.95, "close": 100.05},
            {"open": 100.0, "high": 100.05, "low": 99.95, "close": 99.95},
            {"open": 100.0, "high": 100.05, "low": 99.95, "close": 100.0},
            {"open": 100.0, "high": 100.05, "low": 99.95, "close": 100.08},
            {"open": 100.0, "high": 100.05, "low": 99.95, "close": 99.92},
        ]
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch, level, atr, "long")

        result = extract_step_features(bars, event, step_index=1)
        assert result is not None
        assert result.dwell_ratio == 1.0


class TestTimezoneHandling:
    """Timezone-aware timestamp handling."""

    def test_naive_touch_time_gets_localized(self):
        """Event with naive touch_time should still work."""
        touch_naive = pd.Timestamp("2025-01-01 00:00:00")  # no tz
        level = 100.0
        atr = 1.0

        timestamps = [
            pd.Timestamp("2025-01-01 00:00:00", tz="UTC") + pd.Timedelta(seconds=i)
            for i in range(5)
        ]
        prices = [{"open": 100, "high": 101, "low": 99, "close": 100.5}] * 5
        bars = _make_bars(timestamps, prices)
        event = _make_event(touch_naive, level, atr, "long")

        result = extract_step_features(bars, event, step_index=1)
        assert result is not None
        assert result.step_index == 1
