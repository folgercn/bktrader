"""
feature_engine — 特征提取引擎

负责：
- 从 [touch_time, touch_time + step_index * step_interval] 内的 1s bars 提取
  10 维 Tick_Feature_Vector（extension_atr, speed_cumulative_atr, max_extension_atr,
  pullback_from_max_atr, flow_imbalance_cumulative, flow_imbalance_last_step,
  dwell_ratio, speed_last_step_atr, continuation_ratio, step_index）
- 保证 Point_In_Time 约束：仅使用当前 step 结束时刻及之前的数据
"""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np
import pandas as pd


@dataclass
class StepFeatures:
    """单个 Decision_Step 的特征向量（10 维）"""

    step_index: int  # 当前步数 k (1-based)
    extension_atr: float  # (price_now - level) / ATR
    speed_cumulative_atr: float  # abs(price_now - price_at_touch) / ATR
    max_extension_atr: float  # 累计窗口内最大延伸
    pullback_from_max_atr: float  # 从最高点回撤
    flow_imbalance_cumulative: float  # 累计 flow (若无 volume 数据则为 0.5)
    flow_imbalance_last_step: float  # 最近 5s flow
    dwell_ratio: float  # 价格在 level±0.05ATR 内的 bar 占比
    speed_last_step_atr: float  # 最近 5s 价格位移 / ATR
    continuation_ratio: float  # 顺方向 bar 占比


def extract_step_features(
    second_bars: pd.DataFrame,
    event: pd.Series,
    step_index: int,
    step_interval: int = 5,
) -> StepFeatures | None:
    """从 [touch_time, touch_time + step_index * step_interval] 内的 1s bars 提取特征。

    Point_In_Time 约束：只使用 <= touch_time + step_index * step_interval 的数据。
    若窗口内无数据返回 None。

    Args:
        second_bars: 1s bar DataFrame，DatetimeIndex (UTC) + columns: open, high, low, close
        event: 单个 event Series，需包含 touch_time, level, atr, side
        step_index: 当前步数 k (1-based)
        step_interval: 步进间隔秒数（默认 5）

    Returns:
        StepFeatures 或 None（窗口内无数据时）
    """
    # --- 从 event 提取关键字段 ---
    touch_time = pd.Timestamp(event["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")

    level = float(event["level"])
    atr = float(event["atr"])
    side = str(event["side"])  # "long" or "short"

    # --- 计算观察窗口 [touch_time, touch_time + step_index * step_interval] ---
    window_end = touch_time + pd.Timedelta(seconds=step_index * step_interval)

    # Point_In_Time 约束：只使用 <= window_end 的数据，且 >= touch_time
    mask = (second_bars.index >= touch_time) & (second_bars.index <= window_end)
    bars = second_bars.loc[mask]

    # 窗口内无数据返回 None
    if bars.empty:
        return None

    # --- 基础价格 ---
    price_at_step_k = float(bars["close"].iloc[-1])
    price_at_touch = float(bars["close"].iloc[0])

    # --- extension_atr ---
    # long: (price_at_step_k - level) / atr
    # short: (level - price_at_step_k) / atr
    if side == "long":
        extension_atr = (price_at_step_k - level) / atr
    else:
        extension_atr = (level - price_at_step_k) / atr

    # --- speed_cumulative_atr ---
    speed_cumulative_atr = abs(price_at_step_k - price_at_touch) / atr

    # --- max_extension_atr ---
    # long: max(bars['high'] - level) / atr
    # short: max(level - bars['low']) / atr
    if side == "long":
        max_extension_atr = float((bars["high"] - level).max()) / atr
    else:
        max_extension_atr = float((level - bars["low"]).max()) / atr

    # --- pullback_from_max_atr ---
    # long: (max(bars['high']) - price_at_step_k) / atr
    # short: (price_at_step_k - min(bars['low'])) / atr
    if side == "long":
        pullback_from_max_atr = (float(bars["high"].max()) - price_at_step_k) / atr
    else:
        pullback_from_max_atr = (price_at_step_k - float(bars["low"].min())) / atr

    # --- flow_imbalance_cumulative / flow_imbalance_last_step ---
    # 暂用 0.5 占位（1s bar 无 volume 分解）
    flow_imbalance_cumulative = 0.5
    flow_imbalance_last_step = 0.5

    # --- dwell_ratio ---
    # 价格在 level ± 0.05 * ATR 内的 bar 占比
    dwell_threshold = 0.05 * atr
    dwell_count = int(((bars["close"] - level).abs() <= dwell_threshold).sum())
    dwell_ratio = dwell_count / len(bars)

    # --- speed_last_step_atr ---
    # 最近 step_interval 秒的价格位移 / ATR
    last_step_start = window_end - pd.Timedelta(seconds=step_interval)
    last_step_mask = (bars.index >= last_step_start) & (bars.index <= window_end)
    last_step_bars = bars.loc[last_step_mask]

    if len(last_step_bars) >= 2:
        last_step_first_close = float(last_step_bars["close"].iloc[0])
        last_step_last_close = float(last_step_bars["close"].iloc[-1])
        speed_last_step_atr = abs(last_step_last_close - last_step_first_close) / atr
    elif len(last_step_bars) == 1:
        # 只有一根 bar，位移为 0
        speed_last_step_atr = 0.0
    else:
        # 最近 step 无数据，使用累计 speed 作为 fallback
        speed_last_step_atr = speed_cumulative_atr

    # step_index == 1 时，last_step 就是整个窗口，speed_last_step_atr == speed_cumulative_atr
    # 上面的逻辑已自然满足这一点

    # --- continuation_ratio ---
    # long: count(close > open) / total bars
    # short: count(close < open) / total bars
    if side == "long":
        continuation_count = int((bars["close"] > bars["open"]).sum())
    else:
        continuation_count = int((bars["close"] < bars["open"]).sum())
    continuation_ratio = continuation_count / len(bars)

    return StepFeatures(
        step_index=step_index,
        extension_atr=extension_atr,
        speed_cumulative_atr=speed_cumulative_atr,
        max_extension_atr=max_extension_atr,
        pullback_from_max_atr=pullback_from_max_atr,
        flow_imbalance_cumulative=flow_imbalance_cumulative,
        flow_imbalance_last_step=flow_imbalance_last_step,
        dwell_ratio=dwell_ratio,
        speed_last_step_atr=speed_last_step_atr,
        continuation_ratio=continuation_ratio,
    )
