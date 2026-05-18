"""
execution_sim — V4 执行模拟

负责：
- 封装 V4 Execution Model 的交易模拟逻辑
- 参数：initial_stop_atr=0.45, breakeven_at_r=0.8, trail_start_r=0.9,
  trail_buffer_atr=0.05, max_hold_hours=4
- Cost model：slippage=2bps/side, entry_fee=2bps, exit_fee=4bps
- 复用 v6_gate_d5_tick_backtest.py 中 open_position + simulate_position 逻辑
"""

from __future__ import annotations

import numpy as np
import pandas as pd


# ---------------------------------------------------------------------------
# V4 Execution Model 默认参数
# ---------------------------------------------------------------------------

DEFAULT_EXEC_PARAMS: dict = {
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.80,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "trail_start_r": 0.9,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 4.0,
    "slippage": 0.0002,       # 2 bps / side
    "entry_fee": 0.0002,      # 2 bps
    "exit_fee": 0.0004,       # 4 bps
}


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def execute_trade(
    second_bars: pd.DataFrame,
    event: pd.Series,
    entry_time: pd.Timestamp,
    entry_price: float,
    balance: float = 100000.0,
    notional_share: float = 0.26,
    params: dict | None = None,
) -> dict | None:
    """执行单笔交易，返回 trade dict 或 None（min_stop 过滤）。

    Wraps _open_position + _simulate_position logic from v6_gate_d5_tick_backtest.py.

    Parameters
    ----------
    second_bars : pd.DataFrame
        1s bar 数据，index 为 DatetimeIndex，含 high/low/close 列。
    event : pd.Series
        事件信息，需含 atr, side, touch_low_so_far/signal_low, touch_high_so_far/signal_high。
    entry_time : pd.Timestamp
        入场时间。
    entry_price : float
        入场价格（真实 tick 价格）。
    balance : float
        账户余额，默认 100000。
    notional_share : float
        仓位占比，默认 0.26。
    params : dict | None
        V4 execution params，None 时使用 DEFAULT_EXEC_PARAMS。

    Returns
    -------
    dict | None
        交易结果 dict，若 min_stop 过滤则返回 None。
    """
    if params is None:
        params = DEFAULT_EXEC_PARAMS

    pos = _open_position(event, entry_price, entry_time, balance, notional_share, params)
    if pos is None:
        return None

    return _simulate_position(pos, second_bars, params)


# ---------------------------------------------------------------------------
# 内部辅助函数
# ---------------------------------------------------------------------------


def _open_position(
    event: pd.Series,
    entry_price: float,
    entry_time: pd.Timestamp,
    balance: float,
    notional_share: float,
    params: dict,
) -> dict | None:
    """建仓逻辑：计算止损、风险，min_stop 过滤。"""
    atr = float(event["atr"])
    side = str(event["side"])
    slippage = params["slippage"]

    # 滑点调整入场价
    if side == "long":
        entry_p = entry_price * (1.0 + slippage)
    else:
        entry_p = entry_price * (1.0 - slippage)

    # 信号期间的极值（用于 raw stop 计算）
    sig_low = float(event.get("touch_low_so_far", event.get("signal_low", 0)))
    sig_high = float(event.get("touch_high_so_far", event.get("signal_high", 0)))

    # 止损计算
    if side == "long":
        raw_stop = min(
            sig_low - params["stop_buffer_atr"] * atr,
            entry_p - params["initial_stop_atr"] * atr,
        )
        capped_stop = entry_p - params["stop_cap_atr"] * atr
        stop = max(raw_stop, capped_stop)
        risk = entry_p - stop
    else:
        raw_stop = max(
            sig_high + params["stop_buffer_atr"] * atr,
            entry_p + params["initial_stop_atr"] * atr,
        )
        capped_stop = entry_p + params["stop_cap_atr"] * atr
        stop = min(raw_stop, capped_stop)
        risk = stop - entry_p

    # min_stop 过滤
    if risk <= 0 or risk < entry_p * params["min_stop_bps"] / 10000.0:
        return None

    return {
        "side": side,
        "entry_time": entry_time,
        "entry_p": entry_p,
        "entry_raw": entry_price,
        "sl": stop,
        "risk": risk,
        "atr": atr,
        "notional": balance * notional_share,
        "notional_share": notional_share,
        "protected": False,
        "trailing_active": False,
        "hwm": entry_p,
        "lwm": entry_p,
        "mfe_r": 0.0,
        "mae_r": 0.0,
    }


def _simulate_position(pos: dict, second_bars: pd.DataFrame, params: dict) -> dict:
    """模拟持仓过程：breakeven、trailing stop、max hold exit。"""
    entry_time = pos["entry_time"]
    max_hold = pd.Timedelta(hours=params["max_hold_hours"])
    end_time = entry_time + max_hold

    mask = (second_bars.index > entry_time) & (second_bars.index <= end_time)
    bars = second_bars.loc[mask]

    if bars.empty:
        return _make_exit(pos, pos["entry_raw"], entry_time, "NoData", params)

    entry_p = pos["entry_p"]
    risk = pos["risk"]

    highs = bars["high"].values.astype(np.float64)
    lows = bars["low"].values.astype(np.float64)
    closes = bars["close"].values.astype(np.float64)
    timestamps = bars.index

    for i in range(len(bars)):
        h = highs[i]
        l = lows[i]

        # 更新 HWM / LWM
        if pos["side"] == "long":
            pos["hwm"] = max(pos["hwm"], h)
            favorable = max(0.0, pos["hwm"] - entry_p)
            adverse = max(0.0, entry_p - l)
        else:
            pos["lwm"] = min(pos["lwm"], l)
            favorable = max(0.0, entry_p - pos["lwm"])
            adverse = max(0.0, h - entry_p)

        pos["mfe_r"] = max(pos["mfe_r"], favorable / risk)
        pos["mae_r"] = max(pos["mae_r"], adverse / risk)

        # Breakeven 保护
        if favorable / risk >= params["breakeven_at_r"]:
            if pos["side"] == "long":
                be_sl = entry_p * (1.0 + params["cost_lock_bps"] / 10000.0)
                if be_sl > pos["sl"]:
                    pos["sl"] = be_sl
                    pos["protected"] = True
            else:
                be_sl = entry_p * (1.0 - params["cost_lock_bps"] / 10000.0)
                if be_sl < pos["sl"]:
                    pos["sl"] = be_sl
                    pos["protected"] = True

        # Trailing stop
        if pos["mfe_r"] >= params["trail_start_r"]:
            atr = pos["atr"]
            if pos["side"] == "long":
                trail = pos["hwm"] - params["trail_buffer_atr"] * atr
                if trail > pos["sl"]:
                    pos["sl"] = trail
                    pos["trailing_active"] = True
            else:
                trail = pos["lwm"] + params["trail_buffer_atr"] * atr
                if trail < pos["sl"]:
                    pos["sl"] = trail
                    pos["trailing_active"] = True

        # Stop 触发检查
        if pos["side"] == "long":
            if l <= pos["sl"]:
                reason = (
                    "TrailingSL"
                    if pos["trailing_active"]
                    else ("BreakevenSL" if pos["protected"] else "InitialSL")
                )
                return _make_exit(pos, pos["sl"], timestamps[i], reason, params)
        else:
            if h >= pos["sl"]:
                reason = (
                    "TrailingSL"
                    if pos["trailing_active"]
                    else ("BreakevenSL" if pos["protected"] else "InitialSL")
                )
                return _make_exit(pos, pos["sl"], timestamps[i], reason, params)

    # Max hold exit
    return _make_exit(pos, closes[-1], timestamps[-1], "MaxHoldExit", params)


def _make_exit(
    pos: dict,
    raw_exit_price: float,
    exit_time,
    reason: str,
    params: dict,
) -> dict:
    """计算出场价格（含滑点）和 PnL。"""
    slippage = params["slippage"]

    if pos["side"] == "long":
        exit_p = raw_exit_price * (1.0 - slippage)
        pnl_pct = (exit_p - pos["entry_p"]) / pos["entry_p"]
    else:
        exit_p = raw_exit_price * (1.0 + slippage)
        pnl_pct = (pos["entry_p"] - exit_p) / pos["entry_p"]

    fee_rate = params["entry_fee"] + params["exit_fee"]

    return {
        "entry_time": pos["entry_time"],
        "exit_time": exit_time,
        "side": pos["side"],
        "entry_p": pos["entry_p"],
        "exit_p": exit_p,
        "notional_share": pos["notional_share"],
        "pnl_pct": pnl_pct,
        "realistic_pnl_pct": pnl_pct - fee_rate,
        "exit_reason": reason,
        "mfe_r": pos["mfe_r"],
        "mae_r": pos["mae_r"],
        "hold_seconds": (
            pd.Timestamp(exit_time) - pd.Timestamp(pos["entry_time"])
        ).total_seconds(),
    }
