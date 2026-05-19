"""回调入场策略优化模块。

对被分类为 wait_pullback regime 的 event，通过 grid search 确定最优回调参数组合。
优化目标：train set 上 pullback-labeled events 的 calendar_sum。
"""

from __future__ import annotations

from dataclasses import dataclass
from itertools import product

import pandas as pd

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS, execute_trade
from pre_breakout_timing.delay_simulator import DelayResult, simulate_pullback_entry


# ---------------------------------------------------------------------------
# 常量（与 timing_classifier.py 保持一致）
# ---------------------------------------------------------------------------

_INITIAL_BALANCE = 100_000.0
_NOTIONAL_SHARE = 0.26


@dataclass
class PullbackConfig:
    """回调入场参数配置。

    Attributes:
        pullback_target_atr: 回调目标幅度（ATR 倍数）。价格需回调此幅度才触发入场。
        pullback_window_seconds: 最大等待窗口（秒）。超时则 fallback 入场。
        start_offset_seconds: 从 touch_time 后多少秒开始监控回调。
    """

    pullback_target_atr: float = 0.05
    pullback_window_seconds: int = 60
    start_offset_seconds: int = 5


# Grid search 候选参数空间
PULLBACK_GRID: dict[str, list] = {
    "pullback_target_atr": [0.03, 0.05, 0.08, 0.10],
    "pullback_window_seconds": [30, 60, 120],
}


def _infer_symbol_from_event_id(event_id: str) -> str:
    """从 event_id 推断 symbol。"""
    eid_upper = event_id.upper()
    if "BTC" in eid_upper:
        return "BTCUSDT"
    elif "ETH" in eid_upper:
        return "ETHUSDT"
    return "unknown"


def _compute_calendar_sum(results: list[DelayResult]) -> float:
    """从 DelayResult 列表计算 silo-based calendar sum (%)。

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    """
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        symbol = _infer_symbol_from_event_id(r.event_id)
        entry_time = pd.Timestamp(r.entry_time)
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(r)

    total_return_pct = 0.0
    for _silo_key, silo_results in silos.items():
        balance = _INITIAL_BALANCE
        sorted_results = sorted(silo_results, key=lambda r: r.entry_time)
        for r in sorted_results:
            notional = balance * _NOTIONAL_SHARE
            pnl = notional * r.pnl_pct
            balance += pnl
        silo_return = (balance - _INITIAL_BALANCE) / _INITIAL_BALANCE * 100.0
        total_return_pct += silo_return

    return total_return_pct


def optimize_pullback_params(
    pullback_events: pd.DataFrame,
    bars_cache: dict,
    grid: dict = PULLBACK_GRID,
) -> PullbackConfig:
    """在 train set 的 pullback-labeled events 上 grid search 最优回调参数。

    对 grid 中所有 (pullback_target_atr, pullback_window_seconds) 组合，
    逐一模拟回调入场并计算 calendar_sum，选择 calendar_sum 最优的参数组合。

    Args:
        pullback_events: train set 中 Optimal_Delay_Label == "pullback" 的 events DataFrame。
        bars_cache: symbol → 1s bar DataFrame 的缓存字典（key 格式 "{SYMBOL}_{YYYYMM}"）。
        grid: 参数搜索空间，默认为 PULLBACK_GRID。

    Returns:
        PullbackConfig: calendar_sum 最优的参数组合。
    """
    # 若无 pullback events，返回默认配置
    if pullback_events.empty:
        return PullbackConfig()

    start_offset_seconds = 5  # 固定，不参与 grid search

    best_calendar_sum = float("-inf")
    best_target_atr = PullbackConfig.pullback_target_atr
    best_window = PullbackConfig.pullback_window_seconds

    target_atr_values = grid["pullback_target_atr"]
    window_values = grid["pullback_window_seconds"]

    for target_atr, window_sec in product(target_atr_values, window_values):
        # 对当前参数组合，模拟所有 pullback events
        combo_results: list[DelayResult] = []

        for _, event in pullback_events.iterrows():
            event_id = str(event.get("event_id", ""))
            symbol = str(event["symbol"])
            touch_time = pd.Timestamp(event["touch_time"])
            if touch_time.tzinfo is None:
                touch_time = touch_time.tz_localize("UTC")

            # 获取该 event 对应月份的 1s bar 数据
            month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
            second_bars = bars_cache.get(month_key)

            if second_bars is None or second_bars.empty:
                # 无数据，跳过
                combo_results.append(DelayResult(
                    event_id=event_id,
                    delay_label="pullback",
                    delay_seconds=0,
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

            # 模拟回调入场
            pb_entry_time, pb_entry_price, pb_triggered = simulate_pullback_entry(
                event,
                second_bars,
                start_offset_seconds=start_offset_seconds,
                pullback_target_atr=target_atr,
                pullback_window_seconds=window_sec,
            )

            if pb_entry_time is None or pb_entry_price is None:
                combo_results.append(DelayResult(
                    event_id=event_id,
                    delay_label="pullback",
                    delay_seconds=0,
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

            actual_delay = int((pb_entry_time - touch_time).total_seconds())

            # 执行交易
            trade = execute_trade(
                second_bars, event, pb_entry_time, pb_entry_price,
                params=DEFAULT_EXEC_PARAMS,
            )

            if trade is None:
                combo_results.append(DelayResult(
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
                combo_results.append(DelayResult(
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

        # 计算当前参数组合的 calendar_sum
        calendar_sum = _compute_calendar_sum(combo_results)

        if calendar_sum > best_calendar_sum:
            best_calendar_sum = calendar_sum
            best_target_atr = target_atr
            best_window = window_sec

    return PullbackConfig(
        pullback_target_atr=best_target_atr,
        pullback_window_seconds=best_window,
        start_offset_seconds=start_offset_seconds,
    )
