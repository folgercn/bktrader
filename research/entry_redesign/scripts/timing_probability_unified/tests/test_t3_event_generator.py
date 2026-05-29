"""Tests for t3_event_generator module.

Covers:
- T3 long/short 结构条件判定
- 输出 schema 正确性
- prev bars 不足时返回空 DataFrame
"""

import numpy as np
import pandas as pd
import pytest

from timing_probability_unified.t3_event_generator import (
    T3EventConfig,
    generate_t3_events,
)


def _make_1s_bars(
    hourly_ohlc: list[dict],
    bar_seconds: int = 3600,
    low_at_quarter: bool = False,
) -> pd.DataFrame:
    """从 hourly OHLC 构造合成 1s bars。

    确保 resample("1h") 后正确还原原始 OHLC。

    Parameters
    ----------
    low_at_quarter : bool
        If True, place the hourly low at 1/4 mark (for short touch tests).
        Default places high at 1/4 and low at 3/4.
    """
    rows = []
    for i, bar in enumerate(hourly_ohlc):
        o, h, l, c = bar["open"], bar["high"], bar["low"], bar["close"]

        for s in range(bar_seconds):
            frac = s / max(bar_seconds - 1, 1)
            price = o + (c - o) * frac

            s_high = price + 0.001
            s_low = price - 0.001

            if low_at_quarter:
                # Place low at 1/4, high at 3/4
                if s == bar_seconds // 4:
                    s_low = l
                if s == 3 * bar_seconds // 4:
                    s_high = h
            else:
                # Place high at 1/4, low at 3/4
                if s == bar_seconds // 4:
                    s_high = h
                if s == 3 * bar_seconds // 4:
                    s_low = l

            rows.append({
                "open": price,
                "high": max(s_high, price),
                "low": min(s_low, price),
                "close": price,
                "volume": 100.0,
            })

        # Fix first bar open and last bar close
        first_idx = i * bar_seconds
        rows[first_idx]["open"] = o
        rows[first_idx]["close"] = o
        rows[first_idx]["high"] = max(rows[first_idx]["high"], o)
        rows[first_idx]["low"] = min(rows[first_idx]["low"], o)

        last_idx = (i + 1) * bar_seconds - 1
        rows[last_idx]["open"] = c
        rows[last_idx]["close"] = c
        rows[last_idx]["high"] = max(rows[last_idx]["high"], c)
        rows[last_idx]["low"] = min(rows[last_idx]["low"], c)

    idx = pd.date_range(
        "2025-06-01",
        periods=len(rows),
        freq="1s",
        tz="UTC",
    )
    df = pd.DataFrame(rows, index=idx)
    return df


class TestT3LongCondition:
    """T3 long 结构: prev_high_3 > prev_high_2 AND prev_high_3 > prev_high_1 AND prev_high_1 > prev_high_2"""

    def test_detects_t3_long_pattern(self):
        """构造满足 T3 long 条件的 bars，验证能检测到事件。"""
        # prev_high_3 = 110 > prev_high_2 = 100 > prev_high_1 = 105
        # prev_high_1 = 105 > prev_high_2 = 100
        # level = prev_high_3 = 110
        # signal bar high = 112 >= 110
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 100, "low": 94, "close": 98},
            {"open": 98, "high": 105, "low": 93, "close": 102},
            {"open": 102, "high": 112, "low": 99, "close": 108},
        ]

        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        long_events = events[events["side"] == "long"] if not events.empty else pd.DataFrame()
        assert len(long_events) >= 1, f"Expected T3 long event, got {len(long_events)}"

    def test_no_long_when_condition_fails(self):
        """prev_high_1 < prev_high_2 → 不满足 T3 long 条件。"""
        # prev_high_3=110, prev_high_2=107, prev_high_1=105
        # prev_high_1(105) < prev_high_2(107) → fails
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 107, "low": 94, "close": 98},
            {"open": 98, "high": 105, "low": 93, "close": 102},
            {"open": 102, "high": 112, "low": 99, "close": 108},
        ]

        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        long_events = events[events["side"] == "long"] if not events.empty else pd.DataFrame()
        assert len(long_events) == 0, "Should not detect T3 long when condition fails"

    def test_relaxed_long_only_requires_prev3_above_prev2_and_prev1(self):
        """relaxed 口径下 prev_high_2/prev_high_1 谁大不重要。"""
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 107, "low": 94, "close": 98},
            {"open": 98, "high": 105, "low": 93, "close": 102},
            {"open": 102, "high": 112, "low": 99, "close": 108},
        ]

        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(
            symbol="ETHUSDT",
            resample_rule="1h",
            bar_seconds=3600,
            structure_mode="prev3_dominates",
        )
        events = generate_t3_events(bars_1s, config)

        long_events = events[events["side"] == "long"] if not events.empty else pd.DataFrame()
        assert len(long_events) >= 1, f"Expected relaxed T3 long event, got {len(long_events)}"


class TestT3ShortCondition:
    """T3 short 结构: prev_low_3 < prev_low_2 AND prev_low_3 < prev_low_1 AND prev_low_1 < prev_low_2"""

    def test_detects_t3_short_pattern(self):
        """构造满足 T3 short 条件的 bars，验证能检测到事件。"""
        # prev_low_3 = 88 < prev_low_2 = 92 AND < prev_low_1 = 90
        # prev_low_1 = 90 < prev_low_2 = 92
        # level = prev_low_3 = 88
        # Signal bar low = 86 <= 88
        # Use low_at_quarter=True so the touch happens early (within max_pre_touch)
        hourly = [
            {"open": 95, "high": 100, "low": 88, "close": 93},
            {"open": 93, "high": 98, "low": 92, "close": 94},
            {"open": 94, "high": 97, "low": 90, "close": 91},
            {"open": 91, "high": 95, "low": 86, "close": 89},
        ]

        bars_1s = _make_1s_bars(hourly, low_at_quarter=True)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        short_events = events[events["side"] == "short"] if not events.empty else pd.DataFrame()
        assert len(short_events) >= 1, f"Expected T3 short event, got {len(short_events)}"

    def test_no_short_when_level_not_touched(self):
        """Signal bar low > level → 不触发。"""
        hourly = [
            {"open": 95, "high": 100, "low": 88, "close": 93},
            {"open": 93, "high": 98, "low": 92, "close": 94},
            {"open": 94, "high": 97, "low": 90, "close": 91},
            {"open": 91, "high": 95, "low": 89, "close": 92},  # low=89 > level=88
        ]

        bars_1s = _make_1s_bars(hourly, low_at_quarter=True)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        short_events = events[events["side"] == "short"] if not events.empty else pd.DataFrame()
        assert len(short_events) == 0, "Should not trigger when signal bar doesn't touch level"

    def test_relaxed_short_only_requires_prev3_below_prev2_and_prev1(self):
        """relaxed 口径下 prev_low_2/prev_low_1 谁小不重要。"""
        hourly = [
            {"open": 95, "high": 100, "low": 88, "close": 93},
            {"open": 93, "high": 98, "low": 90, "close": 94},
            {"open": 94, "high": 97, "low": 92, "close": 91},
            {"open": 91, "high": 95, "low": 86, "close": 89},
        ]

        bars_1s = _make_1s_bars(hourly, low_at_quarter=True)
        config = T3EventConfig(
            symbol="ETHUSDT",
            resample_rule="1h",
            bar_seconds=3600,
            structure_mode="prev3_dominates",
        )
        events = generate_t3_events(bars_1s, config)

        short_events = events[events["side"] == "short"] if not events.empty else pd.DataFrame()
        assert len(short_events) >= 1, f"Expected relaxed T3 short event, got {len(short_events)}"


class TestOutputSchema:
    """验证输出 DataFrame 的 schema 正确性。"""

    def test_output_contains_required_columns(self):
        """Output should have all required columns matching T2 canonical schema."""
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 100, "low": 94, "close": 98},
            {"open": 98, "high": 105, "low": 93, "close": 102},
            {"open": 102, "high": 112, "low": 99, "close": 108},
        ]
        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        if events.empty:
            pytest.skip("No events generated from synthetic data")

        required_cols = [
            "event_id", "symbol", "side", "touch_time", "touch_price",
            "level", "atr", "shape",
        ]
        for col in required_cols:
            assert col in events.columns, f"Missing required column: {col}"

    def test_shape_column_is_t3_swing(self):
        """All events should have shape='t3_swing'."""
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 100, "low": 94, "close": 98},
            {"open": 98, "high": 105, "low": 93, "close": 102},
            {"open": 102, "high": 112, "low": 99, "close": 108},
        ]
        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        if events.empty:
            pytest.skip("No events generated from synthetic data")

        assert (events["shape"] == "t3_swing").all()

    def test_event_id_has_t3_prefix(self):
        """Event IDs should contain 't3' prefix."""
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 100, "low": 94, "close": 98},
            {"open": 98, "high": 105, "low": 93, "close": 102},
            {"open": 102, "high": 112, "low": 99, "close": 108},
        ]
        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        if events.empty:
            pytest.skip("No events generated from synthetic data")

        for eid in events["event_id"]:
            assert "_t3_" in eid, f"Event ID should contain '_t3_': {eid}"


class TestInsufficientBars:
    """prev bars 不足时返回空 DataFrame。"""

    def test_fewer_than_4_bars_returns_empty(self):
        """With less than 4 hours of data, should return empty DataFrame."""
        hourly = [
            {"open": 100, "high": 110, "low": 95, "close": 105},
            {"open": 97, "high": 108, "low": 94, "close": 98},
            {"open": 98, "high": 106, "low": 93, "close": 102},
        ]
        bars_1s = _make_1s_bars(hourly)
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        assert events.empty, "Should return empty DataFrame with insufficient bars"

    def test_empty_input_returns_empty(self):
        """Empty 1s bars input should return empty DataFrame."""
        bars_1s = pd.DataFrame(
            columns=["open", "high", "low", "close", "volume"],
            index=pd.DatetimeIndex([], tz="UTC"),
        )
        config = T3EventConfig(symbol="ETHUSDT", resample_rule="1h", bar_seconds=3600)
        events = generate_t3_events(bars_1s, config)

        assert events.empty
