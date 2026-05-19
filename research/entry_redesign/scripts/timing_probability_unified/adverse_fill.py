"""adverse_fill — Adverse next-second fill + extra slippage 压力测试

实现 worktree `adverse_next_second_slippage_stress_decision.md` 中的成交语义：

- `same_close_xslip0bps`: 用 entry_time 那一秒的 close 价（最乐观，spec baseline）
- `next_close_xslipNbps`: 用 entry_time + 1s 的 close 价 + N bps 滑点
- `next_adverse_xslipNbps`: 用 entry_time + 1s 这根 1s bar 的不利价
  - long 用 next bar 的 high
  - short 用 next bar 的 low
  - 再加 N bps 滑点

这不是交易所 bid/ask 回放，是更严格的 1s OHLC adverse proxy。

用法：
    pipeline 的 Step 2（simulate_all_delays）后，对 delay_results 中每个
    DelayResult 重新模拟 trade，使用 adverse fill 替换 entry_price。
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

import numpy as np
import pandas as pd

logger = logging.getLogger(__name__)


@dataclass
class FillScenario:
    """成交场景配置"""

    name: str  # "same_close_xslip0bps", "next_adverse_xslip3bps" 等
    use_next_bar: bool  # True → 用 entry_time 后下一根 1s bar
    use_adverse: bool  # True → 用不利价 (long 取 high, short 取 low)；False → 用 close
    extra_slippage_bps: float  # 额外滑点 (basis points)


# 标准成交场景集
STANDARD_FILL_SCENARIOS: list[FillScenario] = [
    FillScenario("same_close_xslip0bps", use_next_bar=False, use_adverse=False, extra_slippage_bps=0),
    FillScenario("next_close_xslip0bps", use_next_bar=True, use_adverse=False, extra_slippage_bps=0),
    FillScenario("next_adverse_xslip0bps", use_next_bar=True, use_adverse=True, extra_slippage_bps=0),
    FillScenario("next_adverse_xslip1bps", use_next_bar=True, use_adverse=True, extra_slippage_bps=1),
    FillScenario("next_adverse_xslip3bps", use_next_bar=True, use_adverse=True, extra_slippage_bps=3),
    FillScenario("next_adverse_xslip5bps", use_next_bar=True, use_adverse=True, extra_slippage_bps=5),
    FillScenario("next_adverse_xslip7bps", use_next_bar=True, use_adverse=True, extra_slippage_bps=7),
    FillScenario("next_adverse_xslip10bps", use_next_bar=True, use_adverse=True, extra_slippage_bps=10),
]


def adjust_entry_price(
    second_bars: pd.DataFrame,
    original_entry_time: pd.Timestamp,
    original_entry_price: float,
    side: str,
    scenario: FillScenario,
) -> tuple[pd.Timestamp, float] | None:
    """根据 fill scenario 调整入场价格。

    Parameters
    ----------
    second_bars : pd.DataFrame
        1s bar 数据，DatetimeIndex，含 open/high/low/close 列。
    original_entry_time : pd.Timestamp
        原始入场时间（即 simulate_all_delays 选定的 delay 时刻）。
    original_entry_price : float
        原始入场价格（同时刻的 close）。
    side : str
        "long" / "short"。
    scenario : FillScenario
        成交场景配置。

    Returns
    -------
    tuple[pd.Timestamp, float] | None
        调整后的 (entry_time, entry_price)，若数据不足返回 None。
    """
    # Step 1: 决定用哪根 bar
    if scenario.use_next_bar:
        # 取 original_entry_time 后第一根 1s bar
        next_mask = second_bars.index > original_entry_time
        if not next_mask.any():
            return None
        target_time = second_bars.index[next_mask][0]
        target_bar = second_bars.loc[target_time]
    else:
        target_time = original_entry_time
        if original_entry_time not in second_bars.index:
            return None
        target_bar = second_bars.loc[original_entry_time]

    # Step 2: 决定用哪个价格（adverse 或 close）
    if scenario.use_adverse:
        if side == "long":
            base_price = float(target_bar["high"])  # long 用不利端 high
        else:
            base_price = float(target_bar["low"])   # short 用不利端 low
    else:
        base_price = float(target_bar["close"])

    # Step 3: 加额外滑点（每边 N bps，向不利方向偏移）
    if scenario.extra_slippage_bps > 0:
        slip_pct = scenario.extra_slippage_bps / 10000.0
        if side == "long":
            base_price = base_price * (1.0 + slip_pct)
        else:
            base_price = base_price * (1.0 - slip_pct)

    return (target_time, base_price)


def reprice_delay_results(
    delay_results: list,
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    scenario: FillScenario,
) -> list:
    """为已有的 delay_results 应用新的 fill scenario，重新计算 pnl_pct。

    使用方式：先调用 simulate_all_delays 得到原始 results（baseline same_close 口径），
    然后用本函数重新评估每个 trade 的 pnl_pct（基于新的 entry_price）。

    本函数采用近似策略：用 (new_entry_price - original_entry_price) / atr 计算
    额外的不利偏离，从原 pnl_pct 中扣除。这是一个 first-order approximation，
    适用于 1bps~10bps 量级。

    Parameters
    ----------
    delay_results : list[list[DelayResult]]
        原始 delay results（来自 simulate_all_delays）。
    events : pd.DataFrame
        对应的 events DataFrame。
    bars_cache : dict
        1s bar cache。
    scenario : FillScenario
        要应用的 fill scenario。

    Returns
    -------
    list[list[DelayResult]]
        新的 delay_results，pnl_pct 已根据 scenario 调整。
    """
    from copy import deepcopy
    from dataclasses import replace

    new_results: list = []

    for evt_idx, event_delays in enumerate(delay_results):
        if evt_idx >= len(events):
            new_results.append(event_delays)
            continue

        event = events.iloc[evt_idx]
        symbol = str(event["symbol"])
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")
        side = str(event["side"])

        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        bars = bars_cache.get(month_key)
        if bars is None or bars.empty:
            new_results.append(event_delays)
            continue

        new_event_delays = []
        for dr in event_delays:
            if not dr.traded or dr.entry_price is None or dr.pnl_pct is None:
                new_event_delays.append(dr)
                continue

            # 计算 fill scenario 下的新 entry price
            adj = adjust_entry_price(
                bars,
                dr.entry_time,
                dr.entry_price,
                side,
                scenario,
            )
            if adj is None:
                new_event_delays.append(dr)
                continue

            new_entry_time, new_entry_price = adj
            # 用价格差异计算 pnl 调整 (first-order approximation)
            # 假设：原 trade 的 exit price 不变，只改 entry price
            # long: pnl 减少 (new_entry - old_entry) / old_entry
            # short: pnl 减少 (old_entry - new_entry) / old_entry
            old_entry = dr.entry_price
            if side == "long":
                pnl_delta = -(new_entry_price - old_entry) / old_entry
            else:
                pnl_delta = -(old_entry - new_entry_price) / old_entry

            new_pnl = dr.pnl_pct + pnl_delta
            new_event_delays.append(replace(dr, pnl_pct=new_pnl))

        new_results.append(new_event_delays)

    return new_results


def evaluate_fill_scenarios(
    delay_results: list,
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    timing_predictions: np.ndarray,
    sizing_multipliers: np.ndarray,
    speed_gate_pass: np.ndarray,
    base_share: float = 0.30,
    scenarios: list[FillScenario] | None = None,
) -> pd.DataFrame:
    """对每个 fill scenario 计算 calendar_sum / worst_sm，产出对照表。

    Parameters
    ----------
    delay_results : list[list[DelayResult]]
        原始 delay results。
    events : pd.DataFrame
        事件池。
    bars_cache : dict
        1s bar cache。
    timing_predictions : np.ndarray
        每个事件的 timing 预测。
    sizing_multipliers : np.ndarray
        每个事件的 sizing 乘数。
    speed_gate_pass : np.ndarray
        每个事件的 speed gate 通过标记。
    base_share : float
        base notional share。
    scenarios : list[FillScenario] | None
        要评估的 scenario 列表，None 时使用 STANDARD_FILL_SCENARIOS。

    Returns
    -------
    pd.DataFrame
        每行一个 scenario，列含 scenario_name, calendar_sum_gate_on,
        calendar_sum_gate_off, worst_sm_gate_on, neg_sm_count, btc_cs, eth_cs。
    """
    from timing_probability_unified.combined_executor import (
        CombinedPositionConfig,
        compute_calendar_sum,
        compute_combined_positions,
        compute_worst_sm,
    )

    if scenarios is None:
        scenarios = STANDARD_FILL_SCENARIOS

    config = CombinedPositionConfig(base_notional_share=base_share)

    rows = []
    for scenario in scenarios:
        # 重新定价
        repriced = reprice_delay_results(delay_results, events, bars_cache, scenario)

        # 计算 unified_trades
        trades = compute_combined_positions(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=repriced,
            speed_gate_pass=speed_gate_pass,
            config=config,
        )

        cs_on = compute_calendar_sum(trades, gate_filter=True)
        cs_off = compute_calendar_sum(trades, gate_filter=False)
        ws_on = compute_worst_sm(trades, gate_filter=True)

        # Per-symbol breakdown (gate ON)
        gate_on_trades = trades[trades["speed_gate_pass"] == True]  # noqa: E712
        btc_cs = float(gate_on_trades[gate_on_trades["symbol"] == "BTCUSDT"]["weighted_pnl"].sum())
        eth_cs = float(gate_on_trades[gate_on_trades["symbol"] == "ETHUSDT"]["weighted_pnl"].sum())

        # Negative monthly silos count
        if not gate_on_trades.empty:
            df = gate_on_trades.copy()
            df["year_month"] = pd.to_datetime(df["touch_time"]).dt.to_period("M")
            monthly = df.groupby(["symbol", "year_month"])["weighted_pnl"].sum()
            neg_sm = int((monthly < 0).sum())
        else:
            neg_sm = 0

        rows.append({
            "scenario": scenario.name,
            "calendar_sum_gate_on": cs_on,
            "calendar_sum_gate_off": cs_off,
            "worst_sm_gate_on": ws_on,
            "neg_sm_count": neg_sm,
            "btc_cs_gate_on": btc_cs,
            "eth_cs_gate_on": eth_cs,
            "trade_count_gate_on": int(len(gate_on_trades)),
        })

    return pd.DataFrame(rows)
