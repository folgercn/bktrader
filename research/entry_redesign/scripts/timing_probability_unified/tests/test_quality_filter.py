"""Tests for quality_filter module.

Covers:
- speed threshold 计算正确性
- pre_touch_seconds 过滤
- 空 train set 时的 fallback
"""

import numpy as np
import pandas as pd
import pytest

from timing_probability_unified.quality_filter import (
    QualityFilterConfig,
    apply_t3_quality_filter,
    get_t2_filter_info,
)


def _make_events(speed_values: list[float], pre_touch_values: list[float]) -> pd.DataFrame:
    """Helper to create synthetic events with given speed and pre_touch values."""
    n = len(speed_values)
    return pd.DataFrame({
        "event_id": [f"evt_{i:03d}" for i in range(n)],
        "symbol": ["ETHUSDT"] * n,
        "side": ["long"] * n,
        "touch_time": pd.date_range("2025-06-01", periods=n, freq="1h", tz="UTC"),
        "speed_300s_atr": speed_values,
        "pre_touch_seconds": pre_touch_values,
    })


class TestSpeedThresholdCalculation:
    """验证 speed threshold 从 train set 正确计算。"""

    def test_speed_threshold_is_q25_of_train_abs(self):
        """Threshold should be q25 of abs(speed_300s_atr) from train_events."""
        # train speeds abs: [0.1, 0.2, 0.3, 0.4, 0.5]
        # q25 = 0.2 (numpy default linear interpolation)
        train = _make_events(
            speed_values=[0.1, -0.2, 0.3, -0.4, 0.5],
            pre_touch_values=[100] * 5,
        )
        events = _make_events(
            speed_values=[0.15, 0.25, 0.05, 0.35],
            pre_touch_values=[100] * 4,
        )

        config = QualityFilterConfig(t3_speed_quantile=0.25, t3_pre_touch_max=900.0)
        filtered, params = apply_t3_quality_filter(events, train, config)

        expected_threshold = np.quantile([0.1, 0.2, 0.3, 0.4, 0.5], 0.25)
        assert abs(params["t3_speed_threshold"] - expected_threshold) < 1e-9

    def test_events_below_threshold_are_removed(self):
        """Events with abs(speed) < threshold should be filtered out."""
        # train: abs speeds = [1.0, 2.0, 3.0, 4.0] → q25 = 1.75
        train = _make_events(
            speed_values=[1.0, 2.0, 3.0, 4.0],
            pre_touch_values=[100] * 4,
        )
        # events: abs speeds = [0.5, 1.8, 2.5, 1.0]
        # after filter (>= 1.75): only [1.8, 2.5] pass
        events = _make_events(
            speed_values=[0.5, 1.8, 2.5, 1.0],
            pre_touch_values=[100] * 4,
        )

        config = QualityFilterConfig(t3_speed_quantile=0.25, t3_pre_touch_max=900.0)
        filtered, params = apply_t3_quality_filter(events, train, config)

        assert len(filtered) == 2
        assert params["n_after_speed"] == 2

    def test_negative_speeds_use_absolute_value(self):
        """Negative speed values should be evaluated by their absolute value."""
        train = _make_events(
            speed_values=[1.0, 2.0, 3.0, 4.0],
            pre_touch_values=[100] * 4,
        )
        # Event with speed=-2.0 → abs=2.0 should pass if threshold <= 2.0
        events = _make_events(
            speed_values=[-2.0],
            pre_touch_values=[100],
        )

        config = QualityFilterConfig(t3_speed_quantile=0.25, t3_pre_touch_max=900.0)
        filtered, _ = apply_t3_quality_filter(events, train, config)

        # threshold = q25 of [1,2,3,4] = 1.75; abs(-2.0)=2.0 >= 1.75
        assert len(filtered) == 1


class TestPreTouchFilter:
    """验证 pre_touch_seconds 过滤。"""

    def test_events_exceeding_pre_touch_max_removed(self):
        """Events with pre_touch_seconds > max should be filtered out."""
        train = _make_events(
            speed_values=[1.0, 2.0],
            pre_touch_values=[100, 200],
        )
        # All pass speed, but pre_touch varies
        events = _make_events(
            speed_values=[5.0, 5.0, 5.0],
            pre_touch_values=[800, 901, 500],
        )

        config = QualityFilterConfig(t3_speed_quantile=0.0, t3_pre_touch_max=900.0)
        filtered, params = apply_t3_quality_filter(events, train, config)

        # pre_touch: 800 <= 900 ✓, 901 > 900 ✗, 500 <= 900 ✓
        assert len(filtered) == 2
        assert params["n_after_pre_touch"] == 2

    def test_exact_boundary_passes(self):
        """Event with pre_touch_seconds == max should pass."""
        train = _make_events(speed_values=[1.0], pre_touch_values=[100])
        events = _make_events(speed_values=[5.0], pre_touch_values=[900.0])

        config = QualityFilterConfig(t3_speed_quantile=0.0, t3_pre_touch_max=900.0)
        filtered, _ = apply_t3_quality_filter(events, train, config)

        assert len(filtered) == 1


class TestEmptyTrainFallback:
    """空 train set 时应使用 0.0 作为 fallback threshold。"""

    def test_empty_train_uses_zero_threshold(self):
        """When train_events is empty, speed threshold falls back to 0.0."""
        train = pd.DataFrame(columns=["speed_300s_atr", "pre_touch_seconds"])
        events = _make_events(
            speed_values=[0.01, 0.5, 1.0],
            pre_touch_values=[100, 200, 300],
        )

        config = QualityFilterConfig(t3_speed_quantile=0.25, t3_pre_touch_max=900.0)
        filtered, params = apply_t3_quality_filter(events, train, config)

        assert params["t3_speed_threshold"] == 0.0
        # All events should pass speed filter since threshold = 0
        assert len(filtered) == 3

    def test_empty_events_returns_empty(self):
        """When events is empty, returns empty DataFrame."""
        train = _make_events(speed_values=[1.0, 2.0], pre_touch_values=[100, 200])
        events = pd.DataFrame(columns=["speed_300s_atr", "pre_touch_seconds"])

        config = QualityFilterConfig()
        filtered, params = apply_t3_quality_filter(events, train, config)

        assert filtered.empty
        assert params["n_final"] == 0


class TestParamsSnapshot:
    """验证返回的 params_snapshot 内容完整。"""

    def test_params_snapshot_keys(self):
        """Params snapshot should contain all expected keys."""
        train = _make_events(speed_values=[1.0, 2.0], pre_touch_values=[100, 200])
        events = _make_events(speed_values=[1.5], pre_touch_values=[100])

        config = QualityFilterConfig()
        _, params = apply_t3_quality_filter(events, train, config)

        expected_keys = {
            "t3_speed_quantile", "t3_speed_threshold", "t3_pre_touch_max",
            "n_train_events", "n_before", "n_after_speed", "n_after_pre_touch", "n_final",
        }
        assert expected_keys.issubset(set(params.keys()))


class TestT2FilterInfo:
    """验证 get_t2_filter_info 返回结构。"""

    def test_returns_dict_with_expected_keys(self):
        info = get_t2_filter_info()
        assert "source" in info
        assert "filter_chain" in info
        assert "parameters" in info
        assert isinstance(info["filter_chain"], list)
