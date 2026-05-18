"""
pullback_monitor 单元测试

验证：
- 回调触发入场（long / short）
- 超时放弃
- Early abandon 逻辑（long / short）
- price_improvement_bps 计算

Validates: Requirements 3.2, 3.3, 3.4
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pandas as pd

from pullback_monitor import wait_for_pullback, PullbackResult
from regime_classifier import TimingParams


# ============================================================
# Helpers
# ============================================================


def _make_bars(timestamps, prices):
    """创建 1s bar DataFrame。prices 为 dict 列表或 close 列表。

    若 prices 为 float 列表，则 open=high=low=close=price。
    若 prices 为 dict 列表，则需含 open/high/low/close。
    """
    if isinstance(prices[0], (int, float)):
        data = {
            "open": prices,
            "high": prices,
            "low": prices,
            "close": prices,
        }
    else:
        data = {
            "open": [p["open"] for p in prices],
            "high": [p["high"] for p in prices],
            "low": [p["low"] for p in prices],
            "close": [p["close"] for p in prices],
        }
    df = pd.DataFrame(data, index=pd.DatetimeIndex(timestamps))
    return df


def _make_event(atr, side):
    """创建 event Series，含 atr 和 side 字段。"""
    return pd.Series({"atr": atr, "side": side})


# ============================================================
# 测试参数说明
# ============================================================
# decision_price=100.0, atr=2.0
# pullback_target_atr=0.05 → target for long = 100 - 0.05*2 = 99.9
#                           → target for short = 100 + 0.05*2 = 100.1
# abandon_extension_atr=0.30 → abandon for long = 100 + 0.30*2 = 100.6
#                             → abandon for short = 100 - 0.30*2 = 99.4
# decision_window_seconds=60


# ============================================================
# 1. 回调触发入场测试
# ============================================================


class TestPullbackTriggered:
    """验证价格回调到目标时正确触发入场。"""

    def test_long_pullback_triggered(self):
        """Long: 价格回落到 target (99.9) 以下 → triggered=True, 以 bar close 入场。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="long")

        # 构造 bars：前几根在 target 之上，第 5 根 low 触及 target
        timestamps = [
            decision_time + pd.Timedelta(seconds=i) for i in range(1, 11)
        ]
        prices = [
            {"open": 100.0, "high": 100.1, "low": 100.0, "close": 100.0},  # s1
            {"open": 100.0, "high": 100.0, "low": 99.95, "close": 99.95},  # s2
            {"open": 99.95, "high": 99.95, "low": 99.93, "close": 99.93},  # s3
            {"open": 99.93, "high": 99.93, "low": 99.92, "close": 99.92},  # s4
            {"open": 99.92, "high": 99.92, "low": 99.88, "close": 99.89},  # s5: low=99.88 <= 99.9 → trigger
            {"open": 99.89, "high": 99.90, "low": 99.85, "close": 99.87},  # s6
            {"open": 99.87, "high": 99.88, "low": 99.86, "close": 99.88},  # s7
            {"open": 99.88, "high": 99.90, "low": 99.87, "close": 99.89},  # s8
            {"open": 99.89, "high": 99.91, "low": 99.88, "close": 99.90},  # s9
            {"open": 99.90, "high": 99.92, "low": 99.89, "close": 99.91},  # s10
        ]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is True
        assert result.reason == "triggered"
        assert result.entry_time == timestamps[4]  # 第 5 根 bar
        assert result.entry_price == 99.89  # bar close
        assert result.wait_seconds == 5.0
        # price_improvement_bps: (100.0 - 99.89) / 100.0 * 10000 = 11.0 bps
        assert result.price_improvement_bps > 0
        assert abs(result.price_improvement_bps - 11.0) < 0.01

    def test_short_pullback_triggered(self):
        """Short: 价格回升到 target (100.1) 以上 → triggered=True, 以 bar close 入场。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="short")

        # Short: target = 100 + 0.05*2 = 100.1
        timestamps = [
            decision_time + pd.Timedelta(seconds=i) for i in range(1, 8)
        ]
        prices = [
            {"open": 100.0, "high": 100.02, "low": 99.98, "close": 100.01},  # s1
            {"open": 100.01, "high": 100.05, "low": 100.00, "close": 100.04},  # s2
            {"open": 100.04, "high": 100.08, "low": 100.03, "close": 100.07},  # s3
            {"open": 100.07, "high": 100.12, "low": 100.06, "close": 100.11},  # s4: high=100.12 >= 100.1 → trigger
            {"open": 100.11, "high": 100.15, "low": 100.10, "close": 100.13},  # s5
            {"open": 100.13, "high": 100.14, "low": 100.11, "close": 100.12},  # s6
            {"open": 100.12, "high": 100.13, "low": 100.10, "close": 100.11},  # s7
        ]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is True
        assert result.reason == "triggered"
        assert result.entry_time == timestamps[3]  # 第 4 根 bar
        assert result.entry_price == 100.11  # bar close
        assert result.wait_seconds == 4.0
        # price_improvement_bps: (100.11 - 100.0) / 100.0 * 10000 = 11.0 bps
        assert result.price_improvement_bps > 0
        assert abs(result.price_improvement_bps - 11.0) < 0.01


# ============================================================
# 2. 超时放弃测试
# ============================================================


class TestTimeout:
    """验证超时后正确放弃入场。"""

    def test_timeout_long(self):
        """Long: 价格始终在 target 之上，窗口结束 → triggered=False, reason='timeout'。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="long")

        # target = 99.9, abandon = 100.6
        # 所有 bar 的 low 都 > 99.9，且 high < 100.6
        timestamps = [
            decision_time + pd.Timedelta(seconds=i) for i in range(1, 61)
        ]
        # 价格在 99.95~100.05 之间波动，始终不触及 99.9
        prices = [
            {"open": 100.0, "high": 100.05, "low": 99.95, "close": 100.0}
            for _ in range(60)
        ]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is False
        assert result.reason == "timeout"
        assert result.entry_time is None
        assert result.entry_price is None
        assert result.wait_seconds == 60.0
        assert result.price_improvement_bps == 0.0

    def test_timeout_no_bars_in_window(self):
        """窗口内无 bar 数据 → triggered=False, reason='timeout'。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="long")

        # 所有 bar 都在 decision_time 之前
        timestamps = [
            decision_time - pd.Timedelta(seconds=i) for i in range(1, 11)
        ]
        prices = [100.0] * 10
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is False
        assert result.reason == "timeout"
        assert result.wait_seconds == params.decision_window_seconds


# ============================================================
# 3. Early Abandon 测试
# ============================================================


class TestEarlyAbandon:
    """验证顺方向延伸超过阈值时提前放弃。"""

    def test_early_abandon_long(self):
        """Long: 价格上涨到 abandon (100.6) → triggered=False, reason='early_abandon'。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="long")

        # abandon = 100 + 0.30*2 = 100.6
        timestamps = [
            decision_time + pd.Timedelta(seconds=i) for i in range(1, 6)
        ]
        prices = [
            {"open": 100.0, "high": 100.1, "low": 99.95, "close": 100.05},  # s1
            {"open": 100.05, "high": 100.2, "low": 100.0, "close": 100.15},  # s2
            {"open": 100.15, "high": 100.4, "low": 100.1, "close": 100.35},  # s3
            {"open": 100.35, "high": 100.65, "low": 100.3, "close": 100.55},  # s4: high=100.65 >= 100.6 → abandon
            {"open": 100.55, "high": 100.7, "low": 100.5, "close": 100.6},  # s5
        ]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is False
        assert result.reason == "early_abandon"
        assert result.entry_time is None
        assert result.entry_price is None
        assert result.wait_seconds == 4.0
        assert result.price_improvement_bps == 0.0

    def test_early_abandon_short(self):
        """Short: 价格下跌到 abandon (99.4) → triggered=False, reason='early_abandon'。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="short")

        # Short: abandon = 100 - 0.30*2 = 99.4
        timestamps = [
            decision_time + pd.Timedelta(seconds=i) for i in range(1, 6)
        ]
        prices = [
            {"open": 100.0, "high": 100.02, "low": 99.9, "close": 99.92},  # s1
            {"open": 99.92, "high": 99.95, "low": 99.8, "close": 99.82},  # s2
            {"open": 99.82, "high": 99.85, "low": 99.6, "close": 99.65},  # s3
            {"open": 99.65, "high": 99.68, "low": 99.35, "close": 99.45},  # s4: low=99.35 <= 99.4 → abandon
            {"open": 99.45, "high": 99.50, "low": 99.30, "close": 99.35},  # s5
        ]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is False
        assert result.reason == "early_abandon"
        assert result.entry_time is None
        assert result.entry_price is None
        assert result.wait_seconds == 4.0
        assert result.price_improvement_bps == 0.0


# ============================================================
# 4. price_improvement_bps 计算测试
# ============================================================


class TestPriceImprovementBps:
    """验证 price_improvement_bps 计算公式正确。"""

    def test_long_improvement_calculation(self):
        """Long: improvement = (decision_price - entry_price) / decision_price * 10000"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="long")

        # target = 99.9; 第 1 根 bar 直接触发，close=99.80
        timestamps = [decision_time + pd.Timedelta(seconds=1)]
        prices = [{"open": 100.0, "high": 100.0, "low": 99.80, "close": 99.80}]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is True
        # improvement = (100.0 - 99.80) / 100.0 * 10000 = 20.0 bps
        expected_bps = (100.0 - 99.80) / 100.0 * 10000
        assert abs(result.price_improvement_bps - expected_bps) < 0.001

    def test_short_improvement_calculation(self):
        """Short: improvement = (entry_price - decision_price) / decision_price * 10000"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="short")

        # target = 100.1; 第 1 根 bar 直接触发，close=100.20
        timestamps = [decision_time + pd.Timedelta(seconds=1)]
        prices = [{"open": 100.0, "high": 100.20, "low": 100.0, "close": 100.20}]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is True
        # improvement = (100.20 - 100.0) / 100.0 * 10000 = 20.0 bps
        expected_bps = (100.20 - 100.0) / 100.0 * 10000
        assert abs(result.price_improvement_bps - expected_bps) < 0.001

    def test_long_negative_improvement_when_entry_above_decision(self):
        """Long: 若 entry_price > decision_price（close 高于决策价），improvement 为负。"""
        decision_time = pd.Timestamp("2024-01-01 10:00:00")
        decision_price = 100.0
        atr = 2.0
        # 使用较大的 pullback_target_atr 使 target 更容易触发
        params = TimingParams(
            pullback_target_atr=0.05,
            decision_window_seconds=60,
            abandon_extension_atr=0.30,
        )
        event = _make_event(atr=atr, side="long")

        # target = 99.9; bar low 触及 99.9 但 close 回到 100.05（高于 decision_price）
        timestamps = [decision_time + pd.Timedelta(seconds=1)]
        prices = [{"open": 100.0, "high": 100.1, "low": 99.88, "close": 100.05}]
        bars = _make_bars(timestamps, prices)

        result = wait_for_pullback(bars, decision_time, decision_price, event, params)

        assert result.triggered is True
        # improvement = (100.0 - 100.05) / 100.0 * 10000 = -5.0 bps
        expected_bps = (100.0 - 100.05) / 100.0 * 10000
        assert expected_bps < 0
        assert abs(result.price_improvement_bps - expected_bps) < 0.001
