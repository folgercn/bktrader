"""Speed Gate — speed_300s_atr 质量过滤"""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np
import pandas as pd

from timing_probability_unified.combined_executor import compute_calendar_sum, compute_worst_sm


@dataclass
class SpeedGateResult:
    """Speed gate 分析结果"""

    threshold: float  # train q10 阈值
    gate_pass_rate: float  # 通过率
    gate_on_calendar_sum: float  # gate ON 的 calendar_sum
    gate_off_calendar_sum: float  # gate OFF 的 calendar_sum (baseline)
    gate_on_worst_sm: float
    gate_off_worst_sm: float
    filtered_avg_pnl: float  # 被过滤事件的平均 pnl
    retained_avg_pnl: float  # 保留事件的平均 pnl
    aggressive_gate_warning: bool  # 过滤后 < 70% 事件


def compute_speed_gate(
    events: pd.DataFrame,
    train_events: pd.DataFrame,
    quantile: float = 0.10,
) -> tuple[np.ndarray, float]:
    """计算 speed gate pass/fail 标记。

    Parameters
    ----------
    events : pd.DataFrame
        全部事件，需含 speed_300s_atr 列。
    train_events : pd.DataFrame
        训练集事件（用于计算阈值）。
    quantile : float
        阈值百分位，默认 0.10（q10）。

    Returns
    -------
    tuple[np.ndarray, float]
        - speed_gate_pass: bool array (True=pass)
        - threshold: 计算得到的阈值
    """
    threshold = float(train_events["speed_300s_atr"].quantile(quantile))
    speed_gate_pass = (events["speed_300s_atr"] >= threshold).values
    return speed_gate_pass, threshold


def analyze_speed_gate(
    trades: pd.DataFrame,
    speed_gate_pass: np.ndarray,
    threshold: float,
) -> SpeedGateResult:
    """分析 speed gate 开/关的效果对比。

    Parameters
    ----------
    trades : pd.DataFrame
        unified_trades DataFrame，需含 weighted_pnl, symbol, touch_time,
        speed_gate_pass 列。
    speed_gate_pass : np.ndarray
        每个事件的 speed gate 通过标记 (bool array)。
    threshold : float
        speed gate 阈值（train q10）。

    Returns
    -------
    SpeedGateResult
        包含 gate ON/OFF 对比、pass 率、平均 pnl 对比、warning flag。
    """
    n = len(speed_gate_pass)
    gate_pass_rate = float(np.sum(speed_gate_pass)) / n if n > 0 else 0.0

    # Gate ON: only trades where speed_gate_pass == True
    gate_on_calendar_sum = compute_calendar_sum(trades, gate_filter=True)
    gate_on_worst_sm = compute_worst_sm(trades, gate_filter=True)

    # Gate OFF: all trades (baseline)
    gate_off_calendar_sum = compute_calendar_sum(trades, gate_filter=False)
    gate_off_worst_sm = compute_worst_sm(trades, gate_filter=False)

    # Average pnl of filtered (gate_pass=False) vs retained (gate_pass=True) events
    retained_mask = speed_gate_pass.astype(bool)
    filtered_mask = ~retained_mask

    retained_pnls = trades.loc[retained_mask, "weighted_pnl"]
    filtered_pnls = trades.loc[filtered_mask, "weighted_pnl"]

    retained_avg_pnl = float(retained_pnls.mean()) if len(retained_pnls) > 0 else 0.0
    filtered_avg_pnl = float(filtered_pnls.mean()) if len(filtered_pnls) > 0 else 0.0

    # Warning: aggressive gate if pass rate < 70%
    aggressive_gate_warning = gate_pass_rate < 0.70

    return SpeedGateResult(
        threshold=threshold,
        gate_pass_rate=gate_pass_rate,
        gate_on_calendar_sum=gate_on_calendar_sum,
        gate_off_calendar_sum=gate_off_calendar_sum,
        gate_on_worst_sm=gate_on_worst_sm,
        gate_off_worst_sm=gate_off_worst_sm,
        filtered_avg_pnl=filtered_avg_pnl,
        retained_avg_pnl=retained_avg_pnl,
        aggressive_gate_warning=aggressive_gate_warning,
    )
