"""
delay_simulator — 多延迟执行模拟

负责：
- 对每个 V6 gate event 在多个 delay 值（D=0/5/10/15/pullback）下执行 V4 模拟
- 产出 DelayResult 记录各 delay 下的执行结果
- 基于 Delay_PnL_Matrix 为每个 event 分配 Optimal_Delay_Label
- 回调入场（pullback）策略模拟

复用 dynamic_timing/execution_sim.py 的 execute_trade() 函数。
"""

from __future__ import annotations

from dataclasses import dataclass

import pandas as pd

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS, execute_trade


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

DELAY_VALUES: list[int] = [0, 5, 10, 15]  # 固定 delay 候选值（秒）

DELAY_LABELS: dict[int, str] = {
    0: "D0",
    5: "D5",
    10: "D10",
    15: "D15",
}


# ---------------------------------------------------------------------------
# 数据结构
# ---------------------------------------------------------------------------


@dataclass
class DelayResult:
    """单个 event 在单个 delay 下的执行结果。

    Fields
    ------
    event_id : str
        事件唯一标识。
    delay_label : str
        延迟标签："D0", "D5", "D10", "D15", "pullback"。
    delay_seconds : int
        实际 delay 秒数（pullback 时为实际等待秒数）。
    entry_time : pd.Timestamp | None
        入场时间；若未成交则为 None。
    entry_price : float | None
        入场价格（1s bar close）；若未成交则为 None。
    pnl_pct : float | None
        realistic_pnl_pct（含 fee）；若未成交则为 None。
    exit_reason : str | None
        出场原因（InitialSL / BreakevenSL / TrailingSL / MaxHoldExit / NoData）。
    exit_time : pd.Timestamp | None
        出场时间。
    hold_seconds : float | None
        持仓时长（秒）。
    mfe_r : float | None
        最大有利偏移（以 R 为单位）。
    mae_r : float | None
        最大不利偏移（以 R 为单位）。
    traded : bool
        是否成功入场（min_stop 过滤可能导致 False）。
    """

    event_id: str
    delay_label: str
    delay_seconds: int
    entry_time: pd.Timestamp | None
    entry_price: float | None
    pnl_pct: float | None
    exit_reason: str | None
    exit_time: pd.Timestamp | None
    hold_seconds: float | None
    mfe_r: float | None
    mae_r: float | None
    traded: bool


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def simulate_all_delays(
    event: pd.Series,
    second_bars: pd.DataFrame,
    pullback_params: dict,
) -> list[DelayResult]:
    """对单个 event 在所有 delay 值 + pullback 下执行 V4 模拟。

    对 D=0/5/10/15 使用固定延迟入场，对 pullback 使用回调入场策略。
    每个 delay 独立调用 execute_trade() 获取交易结果。

    Parameters
    ----------
    event : pd.Series
        单个 V6 gate event，需含 event_id, touch_time, atr, side 等字段。
    second_bars : pd.DataFrame
        1s bar 数据，index 为 DatetimeIndex，含 high/low/close 列。
    pullback_params : dict
        回调入场参数，需含：
        - pullback_target_atr: float（回调目标，ATR 倍数）
        - pullback_window_seconds: int（最大等待秒数）
        - start_offset_seconds: int（从 touch_time 后多少秒开始监控）

    Returns
    -------
    list[DelayResult]
        5 个 DelayResult：D0, D5, D10, D15, pullback。
    """
    event_id = str(event.get("event_id", ""))
    touch_time = pd.Timestamp(event["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    results: list[DelayResult] = []

    # --- 固定 delay 入场：D=0, 5, 10, 15 ---
    for delay in DELAY_VALUES:
        label = DELAY_LABELS[delay]
        target_time = touch_time + pd.Timedelta(seconds=delay)

        # 找 target_time 后第一个 1s bar
        entry_mask = second_bars.index >= target_time
        if not entry_mask.any():
            # 无可用 bar 数据
            results.append(DelayResult(
                event_id=event_id,
                delay_label=label,
                delay_seconds=delay,
                entry_time=None,
                entry_price=None,
                pnl_pct=None,
                exit_reason="NoData",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            ))
            continue

        entry_idx = second_bars.index[entry_mask][0]
        entry_price = float(second_bars.loc[entry_idx, "close"])
        entry_time = entry_idx

        # 调用 V4 执行模拟
        trade = execute_trade(
            second_bars, event, entry_time, entry_price, params=DEFAULT_EXEC_PARAMS
        )

        if trade is None:
            # min_stop 过滤：止损距离不足
            results.append(DelayResult(
                event_id=event_id,
                delay_label=label,
                delay_seconds=delay,
                entry_time=entry_time,
                entry_price=entry_price,
                pnl_pct=None,
                exit_reason="MinStopFilter",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            ))
        else:
            results.append(DelayResult(
                event_id=event_id,
                delay_label=label,
                delay_seconds=delay,
                entry_time=entry_time,
                entry_price=entry_price,
                pnl_pct=trade["realistic_pnl_pct"],
                exit_reason=trade["exit_reason"],
                exit_time=trade["exit_time"],
                hold_seconds=trade["hold_seconds"],
                mfe_r=trade["mfe_r"],
                mae_r=trade["mae_r"],
                traded=True,
            ))

    # --- Pullback 入场（Task 2.3 实现） ---
    try:
        pb_entry_time, pb_entry_price, pb_triggered = simulate_pullback_entry(
            event,
            second_bars,
            start_offset_seconds=pullback_params.get("start_offset_seconds", 5),
            pullback_target_atr=pullback_params.get("pullback_target_atr", 0.05),
            pullback_window_seconds=pullback_params.get("pullback_window_seconds", 60),
        )
    except NotImplementedError:
        # Task 2.3 尚未实现，返回占位 stub
        results.append(DelayResult(
            event_id=event_id,
            delay_label="pullback",
            delay_seconds=0,
            entry_time=None,
            entry_price=None,
            pnl_pct=None,
            exit_reason="NotImplemented",
            exit_time=None,
            hold_seconds=None,
            mfe_r=None,
            mae_r=None,
            traded=False,
        ))
        return results

    if pb_entry_time is None or pb_entry_price is None:
        # 无数据
        actual_delay = 0
        results.append(DelayResult(
            event_id=event_id,
            delay_label="pullback",
            delay_seconds=actual_delay,
            entry_time=None,
            entry_price=None,
            pnl_pct=None,
            exit_reason="NoData",
            exit_time=None,
            hold_seconds=None,
            mfe_r=None,
            mae_r=None,
            traded=False,
        ))
    else:
        actual_delay = int((pb_entry_time - touch_time).total_seconds())
        trade = execute_trade(
            second_bars, event, pb_entry_time, pb_entry_price, params=DEFAULT_EXEC_PARAMS
        )
        if trade is None:
            results.append(DelayResult(
                event_id=event_id,
                delay_label="pullback",
                delay_seconds=actual_delay,
                entry_time=pb_entry_time,
                entry_price=pb_entry_price,
                pnl_pct=None,
                exit_reason="MinStopFilter",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            ))
        else:
            results.append(DelayResult(
                event_id=event_id,
                delay_label="pullback",
                delay_seconds=actual_delay,
                entry_time=pb_entry_time,
                entry_price=pb_entry_price,
                pnl_pct=trade["realistic_pnl_pct"],
                exit_reason=trade["exit_reason"],
                exit_time=trade["exit_time"],
                hold_seconds=trade["hold_seconds"],
                mfe_r=trade["mfe_r"],
                mae_r=trade["mae_r"],
                traded=True,
            ))

    return results


def simulate_pullback_entry(
    event: pd.Series,
    second_bars: pd.DataFrame,
    start_offset_seconds: int,
    pullback_target_atr: float,
    pullback_window_seconds: int,
) -> tuple[pd.Timestamp | None, float | None, bool]:
    """从 touch_time + start_offset 开始等待回调入场。

    监控 1s bar 价格，等待价格从 breakout 方向回调到目标价位。
    - Long：等待价格下跌 pullback_target_atr × ATR
    - Short：等待价格上涨 pullback_target_atr × ATR

    Parameters
    ----------
    event : pd.Series
        单个 V6 gate event，需含 touch_time, atr, side 等字段。
    second_bars : pd.DataFrame
        1s bar 数据，index 为 DatetimeIndex，含 high/low/close 列。
    start_offset_seconds : int
        从 touch_time 后多少秒开始监控回调（通常为 5s）。
    pullback_target_atr : float
        回调目标（ATR 倍数），例如 0.05 表示回调 0.05 × ATR。
    pullback_window_seconds : int
        最大等待秒数，超时则 fallback 入场。

    Returns
    -------
    tuple[pd.Timestamp | None, float | None, bool]
        (entry_time, entry_price, pullback_triggered)
        - 若回调成功：返回回调时刻的 1s bar close，pullback_triggered=True
        - 若超时：返回超时时刻的 1s bar close（fallback），pullback_triggered=False
        - 若无数据：返回 (None, None, False)
    """
    touch_time = pd.Timestamp(event["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    atr = float(event["atr"])
    side = str(event["side"])

    # 监控起始时间和结束时间
    monitor_start = touch_time + pd.Timedelta(seconds=start_offset_seconds)
    monitor_end = monitor_start + pd.Timedelta(seconds=pullback_window_seconds)

    # 筛选监控窗口内的 1s bars
    window_mask = (second_bars.index >= monitor_start) & (second_bars.index <= monitor_end)
    window_bars = second_bars.loc[window_mask]

    if window_bars.empty:
        return (None, None, False)

    # 参考价格：监控窗口内第一根 1s bar 的 close
    reference_price = float(window_bars.iloc[0]["close"])

    # 计算回调目标价位
    pullback_amount = pullback_target_atr * atr

    if side == "long":
        # Long：等待价格下跌到 reference_price - pullback_amount
        target_price = reference_price - pullback_amount
        # 逐 bar 检查 low 是否触及目标
        for bar_time, bar in window_bars.iterrows():
            if float(bar["low"]) <= target_price:
                # 回调触发：以该 bar 的 close 入场
                return (bar_time, float(bar["close"]), True)
    else:
        # Short：等待价格上涨到 reference_price + pullback_amount
        target_price = reference_price + pullback_amount
        # 逐 bar 检查 high 是否触及目标
        for bar_time, bar in window_bars.iterrows():
            if float(bar["high"]) >= target_price:
                # 回调触发：以该 bar 的 close 入场
                return (bar_time, float(bar["close"]), True)

    # 窗口超时未触发回调：fallback 以最后一根 bar 的 close 入场
    last_bar_time = window_bars.index[-1]
    last_bar_close = float(window_bars.iloc[-1]["close"])
    return (last_bar_time, last_bar_close, False)


def compute_optimal_labels(
    delay_results: list[list[DelayResult]],
    tolerance_bps: float = 5.0,
) -> list[str]:
    """对每个 event 的 delay_results 确定 Optimal_Delay_Label。

    规则：
    1. 选择 pnl_pct 最高的 delay
    2. 若多个 delay 差异 < tolerance_bps，选较短 delay（偏好早入场）
    3. 若所有 delay 均为负，标记为 "skip"（仅分析用，不参与分类器训练）

    Parameters
    ----------
    delay_results : list[list[DelayResult]]
        外层 list 对应每个 event，内层 list 为该 event 的 5 个 DelayResult。
    tolerance_bps : float
        容差阈值（basis points），默认 5.0。
        若最优 delay 与较短 delay 的 pnl_pct 差异 < tolerance_bps/10000，
        则偏好较短 delay。

    Returns
    -------
    list[str]
        每个 event 的 Optimal_Delay_Label：
        "D0", "D5", "D10", "D15", "pullback", 或 "skip"。
    """
    # Delay 排序优先级：越短越优先（用于容差内偏好短 delay）
    _DELAY_ORDER: dict[str, int] = {
        "D0": 0,
        "D5": 1,
        "D10": 2,
        "D15": 3,
        "pullback": 4,
    }

    tolerance = tolerance_bps / 10000.0  # 转换为 pnl_pct 单位

    labels: list[str] = []

    for event_delays in delay_results:
        # 筛选成功入场的 delay（traded=True 且 pnl_pct 不为 None）
        traded_delays = [
            dr for dr in event_delays
            if dr.traded and dr.pnl_pct is not None
        ]

        if not traded_delays:
            # 所有 delay 均未成交 → skip
            labels.append("skip")
            continue

        # 找最高 pnl_pct
        best_pnl = max(dr.pnl_pct for dr in traded_delays)

        if best_pnl < 0:
            # 所有 delay 的 pnl_pct 均为负 → skip
            labels.append("skip")
            continue

        # 找所有在容差范围内的 delay（pnl_pct >= best_pnl - tolerance）
        candidates = [
            dr for dr in traded_delays
            if dr.pnl_pct >= best_pnl - tolerance  # type: ignore[operator]
        ]

        # 在容差内偏好较短 delay（按 _DELAY_ORDER 排序）
        candidates.sort(key=lambda dr: _DELAY_ORDER.get(dr.delay_label, 999))

        # 选择排序后的第一个（最短 delay）
        labels.append(candidates[0].delay_label)

    return labels
