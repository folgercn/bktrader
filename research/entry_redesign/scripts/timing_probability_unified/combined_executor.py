"""Combined Executor — Timing × Probability → Position"""

from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np
import pandas as pd

from timing_probability_unified.timing_classifier import (
    DelayResult,
    get_selected_delay_pnl,
)


@dataclass
class CombinedPositionConfig:
    """组合仓位配置"""

    base_notional_share: float = 0.30
    sensitivity_shares: list[float] = field(default_factory=lambda: [0.25, 0.30, 0.35, 0.40])


@dataclass
class SensitivityRow:
    """敏感性分析单行"""

    base_share: float
    calendar_sum: float
    worst_sm: float
    trade_count: int
    avg_pnl_per_trade: float


def compute_combined_positions(
    events: pd.DataFrame,
    timing_predictions: np.ndarray,
    sizing_multipliers: np.ndarray,
    delay_results: list[list[DelayResult]],
    speed_gate_pass: np.ndarray,
    config: CombinedPositionConfig | None = None,
) -> pd.DataFrame:
    """计算组合仓位并产出 unified_trades ledger。

    逻辑：
    - IF timing = skip → position = 0, delay_pnl_pct = 0, weighted_pnl = 0
    - ELSE → position = base_share × multiplier
    - fast → 使用 get_selected_delay_pnl 获取 D0/D5 中最优结果
    - slow → 使用 get_selected_delay_pnl 获取 D10/D15/pullback 中最优结果
    - weighted_pnl = delay_pnl_pct × position_size

    Parameters
    ----------
    events : pd.DataFrame
        事件池 DataFrame，需含 event_id, symbol, side, touch_time,
        speed_300s_atr 列。
    timing_predictions : np.ndarray
        每个事件的 timing 预测 ("skip", "fast", "slow")。
    sizing_multipliers : np.ndarray
        每个事件的 sizing 乘数 (0..2)。
    delay_results : list[list[DelayResult]]
        每个事件的 5 种 delay 模拟结果。
    speed_gate_pass : np.ndarray
        每个事件的 speed gate 通过标记 (bool)。
    config : CombinedPositionConfig | None
        组合仓位配置。若为 None 则使用默认配置。

    Returns
    -------
    pd.DataFrame
        unified_trades，列包含：event_id, symbol, side, touch_time,
        timing_prediction, selected_delay, rf_probability,
        sizing_multiplier, position_size, delay_pnl_pct,
        weighted_pnl, speed_300s_atr, speed_gate_pass
    """
    if config is None:
        config = CombinedPositionConfig()

    n = len(events)
    base_share = config.base_notional_share

    # Pre-allocate output arrays
    selected_delays: list[str] = []
    position_sizes = np.zeros(n, dtype=np.float64)
    delay_pnl_pcts = np.zeros(n, dtype=np.float64)
    weighted_pnls = np.zeros(n, dtype=np.float64)

    for i in range(n):
        prediction = timing_predictions[i]

        if prediction == "skip":
            selected_delays.append("none")
            position_sizes[i] = 0.0
            delay_pnl_pcts[i] = 0.0
            weighted_pnls[i] = 0.0
        else:
            # Get the best delay and its PnL for this prediction
            delay_label, pnl = get_selected_delay_pnl(prediction, delay_results[i])
            selected_delays.append(delay_label)

            # Position = base_share × multiplier
            position_sizes[i] = base_share * sizing_multipliers[i]
            delay_pnl_pcts[i] = pnl
            weighted_pnls[i] = pnl * position_sizes[i]

    # Build the unified_trades DataFrame
    unified_trades = pd.DataFrame(
        {
            "event_id": events["event_id"].values,
            "symbol": events["symbol"].values,
            "side": events["side"].values,
            "touch_time": events["touch_time"].values,
            "timing_prediction": timing_predictions,
            "selected_delay": selected_delays,
            "rf_probability": sizing_multipliers / 2.0,  # reverse: multiplier = p * 2
            "sizing_multiplier": sizing_multipliers,
            "position_size": position_sizes,
            "delay_pnl_pct": delay_pnl_pcts,
            "weighted_pnl": weighted_pnls,
            "speed_300s_atr": events["speed_300s_atr"].values,
            "speed_gate_pass": speed_gate_pass,
        }
    )

    return unified_trades


def compute_calendar_sum(
    trades: pd.DataFrame,
    gate_filter: bool = False,
) -> float:
    """Silo-based calendar_sum 计算。

    每个 (symbol, month) 独立从 100k 开始。
    若 gate_filter=True，仅使用 speed_gate_pass=True 的 trades。

    逻辑：
    1. 按 gate_filter 过滤 trades
    2. 按 (symbol, year-month) 分组
    3. 每个 silo 内 sum(weighted_pnl) 即为该 silo 的收益百分比
    4. calendar_sum = 所有 silo 收益之和

    Parameters
    ----------
    trades : pd.DataFrame
        unified_trades DataFrame，需含 symbol, touch_time, weighted_pnl,
        speed_gate_pass 列。
    gate_filter : bool
        若为 True，仅使用 speed_gate_pass=True 的 trades。

    Returns
    -------
    float
        Silo-based calendar_sum。若无 trades 则返回 0.0。
    """
    if trades.empty:
        return 0.0

    df = trades.copy()

    if gate_filter:
        df = df[df["speed_gate_pass"] == True]  # noqa: E712

    if df.empty:
        return 0.0

    # Extract year-month from touch_time
    df = df.assign(year_month=pd.to_datetime(df["touch_time"]).dt.to_period("M"))

    # Group by (symbol, year_month) and sum weighted_pnl per silo
    silo_returns = df.groupby(["symbol", "year_month"])["weighted_pnl"].sum()

    # calendar_sum = sum of all silo returns
    return float(silo_returns.sum())


def compute_worst_sm(
    trades: pd.DataFrame,
    gate_filter: bool = False,
) -> float:
    """计算 worst single month（所有单月中最差的月度 weighted_pnl 之和）。

    逻辑：
    1. 按 gate_filter 过滤 trades
    2. 按 year-month 分组（跨所有 symbols）
    3. 每个月 sum(weighted_pnl)
    4. worst_sm = 所有月度和中的最小值
    5. 若无 trades，返回 0.0

    Parameters
    ----------
    trades : pd.DataFrame
        unified_trades DataFrame，需含 touch_time, weighted_pnl,
        speed_gate_pass 列。
    gate_filter : bool
        若为 True，仅使用 speed_gate_pass=True 的 trades。

    Returns
    -------
    float
        Worst single month 的 weighted_pnl 之和。若无 trades 则返回 0.0。
    """
    if trades.empty:
        return 0.0

    df = trades.copy()

    if gate_filter:
        df = df[df["speed_gate_pass"] == True]  # noqa: E712

    if df.empty:
        return 0.0

    # Extract year-month from touch_time
    df = df.assign(year_month=pd.to_datetime(df["touch_time"]).dt.to_period("M"))

    # Group by year_month (across all symbols) and sum weighted_pnl
    monthly_sums = df.groupby("year_month")["weighted_pnl"].sum()

    # worst_sm = minimum of all monthly sums
    return float(monthly_sums.min())


def run_sensitivity_analysis(
    events: pd.DataFrame,
    timing_predictions: np.ndarray,
    sizing_multipliers: np.ndarray,
    delay_results: list[list[DelayResult]],
    speed_gate_pass: np.ndarray,
    shares: list[float] | None = None,
) -> list[SensitivityRow]:
    """Base share 敏感性测试。

    对不同 base_share 值分别计算组合仓位，评估 calendar_sum、worst_sm、
    trade_count 和 avg_pnl_per_trade，产出 SensitivityRow 列表。

    Parameters
    ----------
    events : pd.DataFrame
        事件池 DataFrame。
    timing_predictions : np.ndarray
        每个事件的 timing 预测。
    sizing_multipliers : np.ndarray
        每个事件的 sizing 乘数。
    delay_results : list[list[DelayResult]]
        每个事件的 delay 模拟结果。
    speed_gate_pass : np.ndarray
        每个事件的 speed gate 通过标记。
    shares : list[float] | None
        要测试的 base_share 列表。若为 None，使用 [0.25, 0.30, 0.35, 0.40]。

    Returns
    -------
    list[SensitivityRow]
        每个 base_share 对应一行敏感性分析结果。
    """
    if shares is None:
        shares = [0.25, 0.30, 0.35, 0.40]

    rows: list[SensitivityRow] = []

    for base_share in shares:
        config = CombinedPositionConfig(base_notional_share=base_share)

        # Compute combined positions with this base_share
        trades = compute_combined_positions(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
            config=config,
        )

        # Compute metrics with gate_filter=True
        calendar_sum = compute_calendar_sum(trades, gate_filter=True)
        worst_sm = compute_worst_sm(trades, gate_filter=True)

        # Count trades: timing != "skip" AND speed_gate_pass == True
        active_trades = trades[
            (trades["timing_prediction"] != "skip") & (trades["speed_gate_pass"] == True)  # noqa: E712
        ]
        trade_count = len(active_trades)

        # Compute avg_pnl_per_trade
        if trade_count > 0:
            avg_pnl_per_trade = float(active_trades["weighted_pnl"].sum() / trade_count)
        else:
            avg_pnl_per_trade = 0.0

        rows.append(
            SensitivityRow(
                base_share=base_share,
                calendar_sum=calendar_sum,
                worst_sm=worst_sm,
                trade_count=trade_count,
                avg_pnl_per_trade=avg_pnl_per_trade,
            )
        )

    return rows
