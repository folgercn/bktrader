"""Tests for union_strategy_runner module.

Covers:
- 互斥性验证（构造合法和非法数据）
- T2/T3 full-size concat tagging
- 样本不足跳过逻辑
"""

import pandas as pd
import pytest

from timing_probability_unified.union_strategy_runner import (
    UnionStrategyConfig,
    validate_mutual_exclusion,
    _active_trade_count,
    _config_to_dict,
    _compute_monthly_attribution,
    _next_month_start,
    _slice_trades_to_window,
    _tag_pool,
    MIN_TRAIN_EVENTS,
)


def _make_events_with_touch_time(
    touch_times: list[str],
    sides: list[str],
) -> pd.DataFrame:
    """Helper to create events with specific touch_time and side."""
    n = len(touch_times)
    return pd.DataFrame({
        "event_id": [f"evt_{i:03d}" for i in range(n)],
        "symbol": ["ETHUSDT"] * n,
        "side": sides,
        "touch_time": pd.to_datetime(touch_times, utc=True),
    })


class TestValidateMutualExclusion:
    """互斥性验证：同 signal_start + 同 side 不应有 T2/T3 共存。"""

    def test_no_conflict_passes(self):
        """Non-overlapping events should pass validation."""
        t2_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:15:00", "2025-06-01 14:30:00"],
            sides=["long", "short"],
        )
        t3_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 11:20:00", "2025-06-01 15:45:00"],
            sides=["long", "short"],
        )

        result = validate_mutual_exclusion(t2_events, t3_events)
        assert result is True

    def test_same_hour_different_side_passes(self):
        """Same hour but different sides should pass."""
        t2_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:15:00"],
            sides=["long"],
        )
        t3_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:45:00"],  # same hour
            sides=["short"],  # different side
        )

        result = validate_mutual_exclusion(t2_events, t3_events)
        assert result is True

    def test_same_hour_same_side_raises(self):
        """Same hour AND same side should raise ValueError."""
        t2_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:15:00"],
            sides=["long"],
        )
        t3_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:45:00"],  # same hour (floors to 10:00)
            sides=["long"],  # same side
        )

        with pytest.raises(ValueError, match="互斥性验证失败"):
            validate_mutual_exclusion(t2_events, t3_events)

    def test_empty_t2_always_passes(self):
        """Empty T2 events should always pass."""
        t2_events = pd.DataFrame(columns=["event_id", "symbol", "side", "touch_time"])
        t3_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:15:00"],
            sides=["long"],
        )

        result = validate_mutual_exclusion(t2_events, t3_events)
        assert result is True

    def test_empty_t3_always_passes(self):
        """Empty T3 events should always pass."""
        t2_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:15:00"],
            sides=["long"],
        )
        t3_events = pd.DataFrame(columns=["event_id", "symbol", "side", "touch_time"])

        result = validate_mutual_exclusion(t2_events, t3_events)
        assert result is True

    def test_multiple_conflicts_reported(self):
        """Multiple conflicts should all be reported in error message."""
        t2_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:15:00", "2025-06-01 14:20:00"],
            sides=["long", "short"],
        )
        t3_events = _make_events_with_touch_time(
            touch_times=["2025-06-01 10:45:00", "2025-06-01 14:50:00"],
            sides=["long", "short"],  # Both conflict
        )

        with pytest.raises(ValueError, match="2 个冲突"):
            validate_mutual_exclusion(t2_events, t3_events)


class TestTagPool:
    """Pool tagging must not change final exposure or PnL."""

    def test_tags_without_scaling_position_or_weighted_pnl(self):
        trades = pd.DataFrame({
            "position_size": [0.8, 0.4],
            "weighted_pnl": [0.08, -0.02],
            "timing_prediction": ["fast", "slow"],
            "speed_gate_pass": [True, True],
        })

        tagged = _tag_pool(trades, "T3")

        assert list(tagged["pool"]) == ["T3", "T3"]
        assert tagged["position_size"].tolist() == pytest.approx([0.8, 0.4])
        assert tagged["weighted_pnl"].tolist() == pytest.approx([0.08, -0.02])

    def test_empty_frame_keeps_pool_column(self):
        trades = pd.DataFrame(columns=["position_size", "weighted_pnl"])

        tagged = _tag_pool(trades, "T2")

        assert tagged.empty
        assert "pool" in tagged.columns


class TestActiveTradeCount:
    """Trade count should reflect actual active entries."""

    def test_trade_count_respects_timing_and_speed_gates(self):
        trades = pd.DataFrame({
            "timing_prediction": ["fast", "skip", "slow", "fast"],
            "speed_gate_pass": [True, True, False, True],
        })

        assert _active_trade_count(trades) == 2


class TestRollingWindowSlicing:
    """Rolling reports should use one forward month per window."""

    def test_slice_trades_to_window_is_start_inclusive_end_exclusive(self):
        trades = pd.DataFrame({
            "touch_time": pd.to_datetime([
                "2026-02-01 00:00:00",
                "2026-02-28 23:59:59",
                "2026-03-01 00:00:00",
            ], utc=True),
            "weighted_pnl": [0.1, 0.2, 0.3],
            "position_size": [0.8, 0.8, 0.8],
            "timing_prediction": ["fast", "fast", "fast"],
            "speed_gate_pass": [True, True, True],
            "symbol": ["ETHUSDT", "ETHUSDT", "ETHUSDT"],
        })

        sliced = _slice_trades_to_window(
            trades,
            pd.Timestamp("2026-02-01", tz="UTC"),
            pd.Timestamp("2026-03-01", tz="UTC"),
        )

        assert len(sliced) == 2
        assert sliced["weighted_pnl"].sum() == pytest.approx(0.3)

    def test_next_month_start(self):
        assert _next_month_start("2026-04-01") == pd.Timestamp("2026-05-01", tz="UTC")


class TestMonthlyAttribution:
    """Monthly attribution should report active trade counts."""

    def test_trade_counts_exclude_skip_rows(self):
        trades = pd.DataFrame({
            "touch_time": pd.to_datetime([
                "2026-02-01 00:00:00",
                "2026-02-02 00:00:00",
                "2026-02-03 00:00:00",
                "2026-02-04 00:00:00",
            ], utc=True),
            "pool": ["T2", "T2", "T3", "T3"],
            "weighted_pnl": [0.10, 0.0, 0.03, 0.0],
            "timing_prediction": ["fast", "skip", "slow", "skip"],
            "speed_gate_pass": [True, True, True, True],
        })

        attr = _compute_monthly_attribution(trades, [])

        assert attr == [{
            "month": "2026-02",
            "t2_pnl": 0.1,
            "t3_pnl": 0.03,
            "union_pnl": 0.13,
            "t2_trades": 1,
            "t3_trades": 1,
        }]


class TestMinTrainEventsConstant:
    """验证 MIN_TRAIN_EVENTS 常量存在且合理。"""

    def test_min_train_events_is_30(self):
        """MIN_TRAIN_EVENTS should be 30 as per design."""
        assert MIN_TRAIN_EVENTS == 30


class TestConfigSerialization:
    """Config reports should expose T3 quality gate params."""

    def test_config_to_dict_includes_t3_pre_touch_max(self):
        config = UnionStrategyConfig(t3_pre_touch_max=600.0)

        assert _config_to_dict(config)["t3_pre_touch_max"] == 600.0
