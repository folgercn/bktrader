"""
pullback_monitor — 回调等待模块

负责：
- 对 Over-Extended regime 的 event 实现基于 tick 的回调入场
- 从 decision_time 开始逐 bar 监控价格，等待回调到目标价位
- 超时放弃（不兜底入场）
- Early abandon：顺方向进一步延伸超过阈值则提前放弃
- 记录 pullback 统计（triggered, wait_seconds, price_improvement_bps 等）
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

import pandas as pd

if TYPE_CHECKING:
    from .regime_classifier import TimingParams


@dataclass
class PullbackResult:
    """回调等待的结果"""

    triggered: bool  # 是否成功回调入场
    entry_time: pd.Timestamp | None  # 入场时间
    entry_price: float | None  # 入场价格
    wait_seconds: float  # 实际等待时间
    reason: str  # "triggered" / "timeout" / "early_abandon"
    price_improvement_bps: float  # 相对决策时刻价格的改善


def wait_for_pullback(
    second_bars: pd.DataFrame,
    decision_time: pd.Timestamp,
    decision_price: float,
    event: pd.Series,
    params: "TimingParams",
) -> PullbackResult:
    """
    从 decision_time 开始监控价格，等待回调到目标。

    - 目标价格：decision_price ∓ pullback_target_atr * ATR
      - Long: 等价格回落（target < decision_price）
      - Short: 等价格回升（target > decision_price）
    - 超时：decision_window_seconds 后放弃（不兜底入场）
    - Early abandon：顺方向延伸超过 abandon_extension_atr * ATR 则提前放弃
    - 成功触发时以触发时刻的 1s bar close 价格入场
    - 计算 price_improvement_bps（相对决策时刻价格的改善）

    Parameters
    ----------
    second_bars : pd.DataFrame
        1s bar 数据，index 为 Timestamp，含 open/high/low/close 列
    decision_time : pd.Timestamp
        pullback 决策时刻（即 Over-Extended regime 判定时刻）
    decision_price : float
        决策时刻的价格（用于计算目标和改善）
    event : pd.Series
        event 信息，需含 'atr' 和 'side' 字段
    params : TimingParams
        包含 pullback_target_atr, decision_window_seconds, abandon_extension_atr

    Returns
    -------
    PullbackResult
        回调等待结果
    """
    atr = event["atr"]
    side = event["side"]

    # 计算目标价格和放弃价格
    if side == "long":
        # Long: 等价格回落到 target（低于 decision_price）
        target = decision_price - params.pullback_target_atr * atr
        # Long: 价格继续上涨超过 abandon 则放弃
        abandon = decision_price + params.abandon_extension_atr * atr
    else:
        # Short: 等价格回升到 target（高于 decision_price）
        target = decision_price + params.pullback_target_atr * atr
        # Short: 价格继续下跌超过 abandon 则放弃
        abandon = decision_price - params.abandon_extension_atr * atr

    # 计算窗口结束时间
    window_end = decision_time + pd.Timedelta(seconds=params.decision_window_seconds)

    # 过滤窗口内的 bars：(decision_time, window_end]
    mask = (second_bars.index > decision_time) & (second_bars.index <= window_end)
    window_bars = second_bars.loc[mask]

    # 若窗口内无 bar 数据，直接超时
    if window_bars.empty:
        return PullbackResult(
            triggered=False,
            entry_time=None,
            entry_price=None,
            wait_seconds=params.decision_window_seconds,
            reason="timeout",
            price_improvement_bps=0.0,
        )

    # 逐 bar 监控
    for bar_time, bar in window_bars.iterrows():
        if side == "long":
            # Long 方向：检查是否回调到目标（low <= target）
            if bar["low"] <= target:
                # 触发！以该 bar 的 close 价格入场
                entry_price = bar["close"]
                wait_seconds = (bar_time - decision_time).total_seconds()
                # price_improvement_bps: 正值 = 更好的入场价格
                # Long: 入场价格低于决策价格 → 正改善
                improvement_bps = (
                    (decision_price - entry_price) / decision_price * 10000
                )
                return PullbackResult(
                    triggered=True,
                    entry_time=bar_time,
                    entry_price=entry_price,
                    wait_seconds=wait_seconds,
                    reason="triggered",
                    price_improvement_bps=improvement_bps,
                )
            # Long 方向：检查是否顺方向延伸过多（high >= abandon）
            if bar["high"] >= abandon:
                wait_seconds = (bar_time - decision_time).total_seconds()
                return PullbackResult(
                    triggered=False,
                    entry_time=None,
                    entry_price=None,
                    wait_seconds=wait_seconds,
                    reason="early_abandon",
                    price_improvement_bps=0.0,
                )
        else:
            # Short 方向：检查是否回调到目标（high >= target）
            if bar["high"] >= target:
                # 触发！以该 bar 的 close 价格入场
                entry_price = bar["close"]
                wait_seconds = (bar_time - decision_time).total_seconds()
                # price_improvement_bps: 正值 = 更好的入场价格
                # Short: 入场价格高于决策价格 → 正改善
                improvement_bps = (
                    (entry_price - decision_price) / decision_price * 10000
                )
                return PullbackResult(
                    triggered=True,
                    entry_time=bar_time,
                    entry_price=entry_price,
                    wait_seconds=wait_seconds,
                    reason="triggered",
                    price_improvement_bps=improvement_bps,
                )
            # Short 方向：检查是否顺方向延伸过多（low <= abandon）
            if bar["low"] <= abandon:
                wait_seconds = (bar_time - decision_time).total_seconds()
                return PullbackResult(
                    triggered=False,
                    entry_time=None,
                    entry_price=None,
                    wait_seconds=wait_seconds,
                    reason="early_abandon",
                    price_improvement_bps=0.0,
                )

    # 窗口内所有 bar 都未触发 → 超时
    last_bar_time = window_bars.index[-1]
    wait_seconds = (last_bar_time - decision_time).total_seconds()
    return PullbackResult(
        triggered=False,
        entry_time=None,
        entry_price=None,
        wait_seconds=wait_seconds,
        reason="timeout",
        price_improvement_bps=0.0,
    )
