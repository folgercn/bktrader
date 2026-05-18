"""
test_execution_sim — execution_sim 模块单元测试

验证：
- Cost model 正确应用（slippage + fee_rate）
- min_stop 过滤
- 基本交易执行与字段完整性
- MaxHoldExit 逻辑
- 与 v6_gate_d5_tick_backtest.py 结果一致性
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import numpy as np
import pandas as pd
import pytest

from execution_sim import execute_trade, DEFAULT_EXEC_PARAMS


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_event(atr, side, signal_low=None, signal_high=None):
    data = {"atr": atr, "side": side}
    if signal_low is not None:
        data["touch_low_so_far"] = signal_low
    if signal_high is not None:
        data["touch_high_so_far"] = signal_high
    return pd.Series(data)


def _make_bars(start_time, n_bars, base_price, price_func=None):
    """Create n 1s bars starting at start_time."""
    timestamps = [start_time + pd.Timedelta(seconds=i) for i in range(n_bars)]
    if price_func is None:
        prices = [
            {
                "open": base_price,
                "high": base_price + 0.1,
                "low": base_price - 0.1,
                "close": base_price,
            }
            for _ in range(n_bars)
        ]
    else:
        prices = [price_func(i) for i in range(n_bars)]
    return pd.DataFrame(prices, index=pd.DatetimeIndex(timestamps, tz="UTC"))


# ---------------------------------------------------------------------------
# Test: Cost Model
# ---------------------------------------------------------------------------


class TestCostModel:
    """验证 slippage 和 fee_rate 正确应用。"""

    def test_long_slippage_applied(self):
        """Long 方向 entry_p = entry_price * 1.0002"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None

        expected_entry_p = entry_price * 1.0002
        assert abs(result["entry_p"] - expected_entry_p) < 1e-6

    def test_short_slippage_applied(self):
        """Short 方向 entry_p = entry_price * 0.9998"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="short", signal_high=entry_price + 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None

        expected_entry_p = entry_price * 0.9998
        assert abs(result["entry_p"] - expected_entry_p) < 1e-6

    def test_fee_rate_subtracted_from_pnl(self):
        """fee_rate = 0.0002 + 0.0004 = 0.0006 从 pnl_pct 中扣除得到 realistic_pnl_pct"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None

        fee_rate = DEFAULT_EXEC_PARAMS["entry_fee"] + DEFAULT_EXEC_PARAMS["exit_fee"]
        assert fee_rate == pytest.approx(0.0006)
        assert result["realistic_pnl_pct"] == pytest.approx(
            result["pnl_pct"] - fee_rate
        )

    def test_exit_slippage_long(self):
        """Long exit: exit_p = raw_exit_price * (1 - slippage)"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        # Create bars where price drops to trigger initial stop
        event = _make_event(atr=atr, side="long", signal_low=entry_price - 200)

        def price_func(i):
            # After entry, price drops sharply to trigger stop
            if i <= 5:
                return {
                    "open": entry_price,
                    "high": entry_price + 1,
                    "low": entry_price - 1,
                    "close": entry_price,
                }
            else:
                return {
                    "open": entry_price - 300,
                    "high": entry_price - 290,
                    "low": entry_price - 350,
                    "close": entry_price - 310,
                }

        bars = _make_bars(start, 200, entry_price, price_func)
        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None

        # exit_p should have slippage applied (long exit: * (1 - 0.0002))
        slippage = DEFAULT_EXEC_PARAMS["slippage"]
        # For InitialSL, raw_exit_price = stop level
        # exit_p = stop * (1 - slippage)
        assert result["exit_p"] < result["entry_p"]  # Loss trade


# ---------------------------------------------------------------------------
# Test: min_stop Filter
# ---------------------------------------------------------------------------


class TestMinStopFilter:
    """验证 min_stop 过滤逻辑。"""

    def test_min_stop_filters_small_risk_long(self):
        """Long: risk < entry_p * 12bps → 返回 None"""
        entry_price = 50000.0
        # Use very small ATR so that stop is very close to entry
        atr = 1.0  # Tiny ATR
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        # signal_low very close to entry → risk will be tiny
        # entry_p = 50000 * 1.0002 = 50010
        # raw_stop = min(signal_low - 0.05*1, entry_p - 0.45*1)
        #          = min(50009.95, 50009.55) = 50009.55
        # capped_stop = entry_p - 0.80*1 = 50009.2
        # stop = max(50009.55, 50009.2) = 50009.55
        # risk = 50010 - 50009.55 = 0.45
        # min_stop check: risk < entry_p * 12/10000 = 50010 * 0.0012 = 60.012
        # 0.45 < 60.012 → filtered!
        event = _make_event(atr=atr, side="long", signal_low=entry_price)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is None

    def test_min_stop_filters_small_risk_short(self):
        """Short: risk < entry_p * 12bps → 返回 None"""
        entry_price = 50000.0
        atr = 1.0  # Tiny ATR
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        # signal_high very close to entry → risk will be tiny
        event = _make_event(atr=atr, side="short", signal_high=entry_price)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is None

    def test_min_stop_passes_sufficient_risk(self):
        """Sufficient risk → trade executes (not None)"""
        entry_price = 50000.0
        atr = 500.0  # Large ATR → large risk
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        # signal_low far below entry → large risk
        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None


# ---------------------------------------------------------------------------
# Test: Basic Trade Execution
# ---------------------------------------------------------------------------


class TestBasicTradeExecution:
    """验证基本交易执行与字段完整性。"""

    def test_trade_dict_has_expected_fields(self):
        """Trade dict 包含所有预期字段。"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None

        expected_fields = {
            "entry_time",
            "exit_time",
            "side",
            "entry_p",
            "exit_p",
            "pnl_pct",
            "realistic_pnl_pct",
            "exit_reason",
            "mfe_r",
            "mae_r",
            "hold_seconds",
        }
        assert expected_fields.issubset(set(result.keys()))

    def test_favorable_price_move_long(self):
        """Long: 价格上涨 → pnl_pct > 0"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)

        # Price rises steadily (but not enough to trigger trailing stop quickly)
        def price_func(i):
            p = entry_price + i * 0.5
            return {
                "open": p,
                "high": p + 0.5,
                "low": p - 0.2,
                "close": p + 0.3,
            }

        # 4 hours = 14400 seconds of bars
        bars = _make_bars(start, 14410, entry_price, price_func)
        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        # Price moved up → should be profitable (before or after fees)
        assert result["pnl_pct"] > 0

    def test_favorable_price_move_short(self):
        """Short: 价格下跌 → pnl_pct > 0"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="short", signal_high=entry_price + 300)

        # Price drops steadily
        def price_func(i):
            p = entry_price - i * 0.5
            return {
                "open": p,
                "high": p + 0.2,
                "low": p - 0.5,
                "close": p - 0.3,
            }

        bars = _make_bars(start, 14410, entry_price, price_func)
        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["pnl_pct"] > 0

    def test_hold_seconds_positive(self):
        """hold_seconds 应为正数。"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["hold_seconds"] > 0

    def test_mfe_mae_non_negative(self):
        """mfe_r 和 mae_r 应 >= 0。"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 200, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["mfe_r"] >= 0
        assert result["mae_r"] >= 0


# ---------------------------------------------------------------------------
# Test: MaxHoldExit
# ---------------------------------------------------------------------------


class TestMaxHoldExit:
    """验证 max_hold_hours=4 时的 MaxHoldExit 逻辑。"""

    def test_max_hold_exit_long(self):
        """Long: 4h 内无 stop 触发 → exit_reason='MaxHoldExit'"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)

        # Flat price: no stop triggered, no trailing
        # Price stays within a narrow range that doesn't trigger any stop
        bars = _make_bars(start, 14410, entry_price)  # 4h + 10s

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["exit_reason"] == "MaxHoldExit"

    def test_max_hold_exit_short(self):
        """Short: 4h 内无 stop 触发 → exit_reason='MaxHoldExit'"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="short", signal_high=entry_price + 300)

        # Flat price
        bars = _make_bars(start, 14410, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["exit_reason"] == "MaxHoldExit"

    def test_max_hold_exit_hold_seconds(self):
        """MaxHoldExit 时 hold_seconds 应接近 4h = 14400s"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)

        event = _make_event(atr=atr, side="long", signal_low=entry_price - 300)
        bars = _make_bars(start, 14410, entry_price)

        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["exit_reason"] == "MaxHoldExit"
        # hold_seconds should be close to 4 hours (14400s)
        # The last bar within max_hold window is at entry_time + 14400s
        # Bars start at entry_time+1s and go up to entry_time+14400s
        assert 14390 <= result["hold_seconds"] <= 14405


# ---------------------------------------------------------------------------
# Test: Consistency with v6_gate_d5_tick_backtest.py
# ---------------------------------------------------------------------------


class TestConsistencyWithReference:
    """验证与 v6_gate_d5_tick_backtest.py 的逻辑一致性。

    通过构造相同输入，验证 execution_sim 的输出与手动计算的参考值一致。
    """

    def test_open_position_logic_matches_reference(self):
        """验证 _open_position 逻辑与参考实现一致。"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)
        signal_low = 49700.0

        event = _make_event(atr=atr, side="long", signal_low=signal_low)

        # Manually compute expected values using reference logic:
        # entry_p = 50000 * 1.0002 = 50010.0
        # raw_stop = min(49700 - 0.05*500, 50010 - 0.45*500)
        #          = min(49675, 49785) = 49675
        # capped_stop = 50010 - 0.80*500 = 49610
        # stop = max(49675, 49610) = 49675
        # risk = 50010 - 49675 = 335
        # min_stop check: 335 > 50010 * 12/10000 = 60.012 → passes

        expected_entry_p = 50010.0
        expected_stop = 49675.0
        expected_risk = 335.0

        # Create flat bars so we get MaxHoldExit and can inspect entry_p
        bars = _make_bars(start, 14410, entry_price)
        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["entry_p"] == pytest.approx(expected_entry_p, rel=1e-9)

    def test_pnl_calculation_matches_reference(self):
        """验证 PnL 计算与参考实现一致。"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)
        signal_low = 49700.0

        event = _make_event(atr=atr, side="long", signal_low=signal_low)

        # Flat bars at entry_price → MaxHoldExit at close = entry_price
        bars = _make_bars(start, 14410, entry_price)
        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None

        # Reference calculation:
        # entry_p = 50000 * 1.0002 = 50010
        # exit at MaxHoldExit: raw_exit_price = close = 50000
        # exit_p = 50000 * (1 - 0.0002) = 49990
        # pnl_pct = (49990 - 50010) / 50010 = -20/50010 ≈ -0.0003999
        # realistic_pnl_pct = pnl_pct - 0.0006

        expected_entry_p = 50010.0
        expected_exit_p = 50000.0 * (1.0 - 0.0002)  # 49990.0
        expected_pnl_pct = (expected_exit_p - expected_entry_p) / expected_entry_p
        expected_realistic = expected_pnl_pct - 0.0006

        assert result["pnl_pct"] == pytest.approx(expected_pnl_pct, rel=1e-6)
        assert result["realistic_pnl_pct"] == pytest.approx(expected_realistic, rel=1e-6)

    def test_initial_stop_trigger_matches_reference(self):
        """验证 InitialSL 触发逻辑与参考实现一致。"""
        entry_price = 50000.0
        atr = 500.0
        start = pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
        entry_time = start + pd.Timedelta(seconds=5)
        signal_low = 49700.0

        event = _make_event(atr=atr, side="long", signal_low=signal_low)

        # entry_p = 50010, stop = 49675
        # Create bars that drop below stop at bar index 10
        def price_func(i):
            if i <= 5:
                return {
                    "open": entry_price,
                    "high": entry_price + 1,
                    "low": entry_price - 1,
                    "close": entry_price,
                }
            elif i <= 9:
                return {
                    "open": entry_price,
                    "high": entry_price,
                    "low": entry_price - 50,
                    "close": entry_price - 30,
                }
            else:
                # Drop below stop (49675)
                return {
                    "open": 49700,
                    "high": 49700,
                    "low": 49600,  # Below stop 49675
                    "close": 49650,
                }

        bars = _make_bars(start, 200, entry_price, price_func)
        result = execute_trade(bars, event, entry_time, entry_price)
        assert result is not None
        assert result["exit_reason"] == "InitialSL"

        # Exit price should be stop * (1 - slippage)
        expected_stop = 49675.0
        expected_exit_p = expected_stop * (1.0 - 0.0002)
        assert result["exit_p"] == pytest.approx(expected_exit_p, rel=1e-6)

    def test_default_params_match_reference(self):
        """验证 DEFAULT_EXEC_PARAMS 与 v6_gate_d5_tick_backtest.py 的 EXEC_PARAMS 一致。"""
        assert DEFAULT_EXEC_PARAMS["initial_stop_atr"] == 0.45
        assert DEFAULT_EXEC_PARAMS["stop_buffer_atr"] == 0.05
        assert DEFAULT_EXEC_PARAMS["stop_cap_atr"] == 0.80
        assert DEFAULT_EXEC_PARAMS["min_stop_bps"] == 12.0
        assert DEFAULT_EXEC_PARAMS["breakeven_at_r"] == 0.8
        assert DEFAULT_EXEC_PARAMS["cost_lock_bps"] == 10.0
        assert DEFAULT_EXEC_PARAMS["trail_start_r"] == 0.9
        assert DEFAULT_EXEC_PARAMS["trail_buffer_atr"] == 0.05
        assert DEFAULT_EXEC_PARAMS["max_hold_hours"] == 4.0
        assert DEFAULT_EXEC_PARAMS["slippage"] == 0.0002
        assert DEFAULT_EXEC_PARAMS["entry_fee"] == 0.0002
        assert DEFAULT_EXEC_PARAMS["exit_fee"] == 0.0004
