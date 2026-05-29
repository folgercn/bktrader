"""t3_event_generator — T3 swing breakout 事件生成器

从 1s bar cache resample 1h bars 并检测 T3 breakout touch 事件。

T3 结构定义：
- T3 long: prev_high_3 > prev_high_2 AND prev_high_3 > prev_high_1 AND prev_high_1 > prev_high_2
  level = prev_high_3
  trigger: current bar 1s high >= level

- T3 short: prev_low_3 < prev_low_2 AND prev_low_3 < prev_low_1 AND prev_low_1 < prev_low_2
  level = prev_low_3
  trigger: current bar 1s low <= level

输出 schema 与 T2 canonical events 兼容，添加 shape="t3_swing" 列。
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

import numpy as np
import pandas as pd

from .multi_timeframe_builder import (
    _build_event_fast,
    compute_atr,
    load_all_1s_bars,
    resample_to_timeframe,
    TimeframeConfig,
)

logger = logging.getLogger(__name__)


@dataclass
class T3EventConfig:
    """T3 事件生成配置"""
    symbol: str = "ETHUSDT"
    resample_rule: str = "1h"
    bar_seconds: int = 3600
    max_pre_touch_seconds: float = 1800  # 默认 30min
    atr_lookback: int = 14
    structure_mode: str = "strict_current"


def generate_t3_events(
    bars_1s: pd.DataFrame,
    config: T3EventConfig,
) -> pd.DataFrame:
    """从 1s bars resample 1h 并检测 T3 breakout touch 事件。

    T3 long 条件:
      prev_high_3 > prev_high_2
      prev_high_3 > prev_high_1
      prev_high_1 > prev_high_2
      level = prev_high_3
      trigger: current bar 1s high >= level

    T3 short 条件:
      prev_low_3 < prev_low_2
      prev_low_3 < prev_low_1
      prev_low_1 < prev_low_2
      level = prev_low_3
      trigger: current bar 1s low <= level

    Parameters
    ----------
    bars_1s : pd.DataFrame
        1s bar 数据，需包含 open/high/low/close/volume 列，index 为 DatetimeIndex。
    config : T3EventConfig
        T3 事件生成配置。

    Returns
    -------
    pd.DataFrame
        T3 事件 DataFrame，schema 与 T2 canonical events 兼容，
        额外包含 shape="t3_swing" 列。
    """
    # Resample 1s → 目标时间框架
    bars_tf = resample_to_timeframe(bars_1s, config.resample_rule)
    atr_series = compute_atr(bars_tf, config.atr_lookback)

    bar_times = bars_tf.index.tolist()
    n_bars = len(bar_times)

    if n_bars < 4:
        logger.warning("Not enough bars for T3 detection (need >= 4, got %d)", n_bars)
        return pd.DataFrame()

    # 预处理 1s bars numpy 数组用于 searchsorted
    bars_1s_times = bars_1s.index.astype(np.int64)
    bars_1s_high = bars_1s["high"].values
    bars_1s_low = bars_1s["low"].values
    bars_1s_close = bars_1s["close"].values
    bars_1s_open = bars_1s["open"].values

    bar_seconds_td = pd.Timedelta(seconds=config.bar_seconds)

    # 构建 TimeframeConfig 给 _build_event_fast 复用
    tf_config = TimeframeConfig(
        name="1h",
        resample_rule=config.resample_rule,
        bar_seconds=config.bar_seconds,
        max_pre_touch_seconds=config.max_pre_touch_seconds,
        max_hold_hours=2.0,
        atr_lookback=config.atr_lookback,
        min_speed_300s_atr=0.0,
    )

    events = []
    processed = 0

    # 从 index=3 开始遍历，需要 3 根 prev bars
    for i in range(3, n_bars):
        signal_bar_start = bar_times[i]
        signal_bar_end = signal_bar_start + bar_seconds_td

        # prev bars (i-1=prev1, i-2=prev2, i-3=prev3)
        prev_high_1 = bars_tf.iloc[i - 1]["high"]
        prev_low_1 = bars_tf.iloc[i - 1]["low"]
        prev_open_1 = bars_tf.iloc[i - 1]["open"]
        prev_close_1 = bars_tf.iloc[i - 1]["close"]

        prev_high_2 = bars_tf.iloc[i - 2]["high"]
        prev_low_2 = bars_tf.iloc[i - 2]["low"]

        prev_high_3 = bars_tf.iloc[i - 3]["high"]
        prev_low_3 = bars_tf.iloc[i - 3]["low"]

        atr = atr_series.iloc[i]
        if atr <= 0 or np.isnan(atr):
            continue

        # 用 searchsorted 快速定位 signal bar 内的 1s bars 范围
        start_ns = signal_bar_start.value
        end_ns = signal_bar_end.value
        idx_start = np.searchsorted(bars_1s_times, start_ns, side='left')
        idx_end = np.searchsorted(bars_1s_times, end_ns, side='left')

        if idx_start >= idx_end:
            continue

        sig_high = bars_1s_high[idx_start:idx_end]
        sig_low = bars_1s_low[idx_start:idx_end]

        # T3 long 条件判定
        if config.structure_mode == "prev3_dominates":
            t3_long_ready = prev_high_3 > prev_high_2 and prev_high_3 > prev_high_1
        else:
            t3_long_ready = (
                prev_high_3 > prev_high_2
                and prev_high_3 > prev_high_1
                and prev_high_1 > prev_high_2
            )

        if t3_long_ready:
            long_level = prev_high_3
            long_touch_indices = np.where(sig_high >= long_level)[0]
            if len(long_touch_indices) > 0:
                touch_local_idx = long_touch_indices[0]
                touch_abs_idx = idx_start + touch_local_idx
                touch_time = bars_1s.index[touch_abs_idx]
                touch_price = float(sig_high[touch_local_idx])

                event = _build_event_fast(
                    symbol=config.symbol, side="long",
                    touch_time=touch_time, touch_price=touch_price,
                    level=long_level, atr=atr,
                    signal_bar_start=signal_bar_start,
                    prev_bar_1_open=prev_open_1, prev_bar_1_high=prev_high_1,
                    prev_bar_1_low=prev_low_1, prev_bar_1_close=prev_close_1,
                    bars_1s_times=bars_1s_times, bars_1s_high=bars_1s_high,
                    bars_1s_low=bars_1s_low, bars_1s_close=bars_1s_close,
                    bars_1s_open=bars_1s_open, bars_1s_index=bars_1s.index,
                    config=tf_config,
                )
                if event is not None:
                    events.append(event)

        # T3 short 条件判定
        if config.structure_mode == "prev3_dominates":
            t3_short_ready = prev_low_3 < prev_low_2 and prev_low_3 < prev_low_1
        else:
            t3_short_ready = (
                prev_low_3 < prev_low_2
                and prev_low_3 < prev_low_1
                and prev_low_1 < prev_low_2
            )

        if t3_short_ready:
            short_level = prev_low_3
            short_touch_indices = np.where(sig_low <= short_level)[0]
            if len(short_touch_indices) > 0:
                touch_local_idx = short_touch_indices[0]
                touch_abs_idx = idx_start + touch_local_idx
                touch_time = bars_1s.index[touch_abs_idx]
                touch_price = float(sig_low[touch_local_idx])

                event = _build_event_fast(
                    symbol=config.symbol, side="short",
                    touch_time=touch_time, touch_price=touch_price,
                    level=short_level, atr=atr,
                    signal_bar_start=signal_bar_start,
                    prev_bar_1_open=prev_open_1, prev_bar_1_high=prev_high_1,
                    prev_bar_1_low=prev_low_1, prev_bar_1_close=prev_close_1,
                    bars_1s_times=bars_1s_times, bars_1s_high=bars_1s_high,
                    bars_1s_low=bars_1s_low, bars_1s_close=bars_1s_close,
                    bars_1s_open=bars_1s_open, bars_1s_index=bars_1s.index,
                    config=tf_config,
                )
                if event is not None:
                    events.append(event)

        processed += 1
        if processed % 1000 == 0:
            logger.info("  T3 processed %d/%d bars, %d events so far",
                        processed, n_bars - 3, len(events))

    if not events:
        logger.warning("No T3 events detected for %s", config.symbol)
        return pd.DataFrame()

    df = pd.DataFrame(events)

    # 添加 shape 列标记为 t3_swing
    df["shape"] = "t3_swing"

    # 覆盖 event_id 使用 t3 前缀以区分
    df["event_id"] = df.apply(
        lambda row: f"{row['symbol']}_t3_{row['touch_time'].strftime('%Y%m%d_%H%M%S')}_{row['side']}",
        axis=1,
    )

    logger.info("Generated %d T3 events for %s (long=%d, short=%d)",
                len(df), config.symbol,
                len(df[df["side"] == "long"]),
                len(df[df["side"] == "short"]))

    return df


if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    config = T3EventConfig(symbol="ETHUSDT")
    bars_1s = load_all_1s_bars(config.symbol)
    events = generate_t3_events(bars_1s, config)

    print(f"\nT3 Event Summary: {len(events)} events")
    if not events.empty:
        print(f"  Long: {len(events[events['side'] == 'long'])}")
        print(f"  Short: {len(events[events['side'] == 'short'])}")
        print(f"  Time range: {events['touch_time'].min()} to {events['touch_time'].max()}")
        print(f"  Shape: {events['shape'].unique()}")
        print(f"\nColumns: {list(events.columns)}")
