"""multi_timeframe_builder — 多时间框架 pretouch 事件生成器

从 1s bar cache 重新构建 30min/4h 的 signal bar 和 pretouch 事件。

流程：
1. 从 1s bars resample 成目标时间框架 bars (30min / 4h)
2. 计算 breakout level (prev_high_2 / prev_low_2)
3. 检测 pretouch 事件 (touch_extension_atr, speed_300s_atr 等)
4. 应用质量过滤 (touch_Nm, eff_300s <= 1.0)
5. 输出 events CSV 供 pipeline 使用
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd

logger = logging.getLogger(__name__)

PROJECT_ROOT = Path(__file__).resolve().parents[4]
BARS_CACHE_DIR = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest"
    / "bars_cache"
)

OUTPUT_DIR = PROJECT_ROOT / "research" / "entry_redesign" / "scripts" / "output" / "multi_timeframe"


@dataclass
class TimeframeConfig:
    """时间框架配置"""
    name: str              # "30min" / "1h" / "4h"
    resample_rule: str     # "30min" / "1h" / "4h"
    bar_seconds: int       # 1800 / 3600 / 14400
    max_pre_touch_seconds: float  # 触发必须在 bar 开始后多少秒内
    max_hold_hours: float  # 最大持仓时间
    atr_lookback: int      # ATR 计算回看 bar 数
    min_speed_300s_atr: float  # speed gate 阈值（会从 train set 重新计算 q10）


# 预定义配置
TIMEFRAME_CONFIGS = {
    "30min": TimeframeConfig(
        name="30min",
        resample_rule="30min",
        bar_seconds=1800,
        max_pre_touch_seconds=900,   # 30min bar 的前 15 分钟
        max_hold_hours=1.0,          # 缩短 hold
        atr_lookback=14,
        min_speed_300s_atr=0.0,      # 从 train set 重新计算
    ),
    "1h": TimeframeConfig(
        name="1h",
        resample_rule="1h",
        bar_seconds=3600,
        max_pre_touch_seconds=1800,  # 1h bar 的前 30 分钟
        max_hold_hours=2.0,
        atr_lookback=14,
        min_speed_300s_atr=0.0,
    ),
    "4h": TimeframeConfig(
        name="4h",
        resample_rule="4h",
        bar_seconds=14400,
        max_pre_touch_seconds=7200,  # 4h bar 的前 2 小时
        max_hold_hours=8.0,          # 扩大 hold
        atr_lookback=14,
        min_speed_300s_atr=0.0,
    ),
}


def load_all_1s_bars(symbol: str) -> pd.DataFrame:
    """加载指定 symbol 的所有月份 1s bar 数据并合并。"""
    pattern = f"{symbol}_*_flow_1s.pkl"
    files = sorted(BARS_CACHE_DIR.glob(pattern))
    if not files:
        raise FileNotFoundError(f"No 1s bar files found for {symbol} in {BARS_CACHE_DIR}")

    dfs = []
    for f in files:
        df = pd.read_pickle(f)
        dfs.append(df)
        logger.info("Loaded %s: %d bars", f.name, len(df))

    combined = pd.concat(dfs).sort_index()
    combined = combined[~combined.index.duplicated(keep='first')]
    logger.info("Combined %s: %d bars, %s to %s", symbol, len(combined),
                combined.index.min(), combined.index.max())
    return combined


def resample_to_timeframe(bars_1s: pd.DataFrame, rule: str) -> pd.DataFrame:
    """将 1s bars resample 到目标时间框架。"""
    resampled = bars_1s.resample(rule).agg({
        "open": "first",
        "high": "max",
        "low": "min",
        "close": "last",
        "volume": "sum",
    }).dropna(subset=["open"])
    logger.info("Resampled to %s: %d bars", rule, len(resampled))
    return resampled


def compute_atr(bars: pd.DataFrame, lookback: int = 14) -> pd.Series:
    """计算 ATR（简单 range 均值）。"""
    ranges = bars["high"] - bars["low"]
    return ranges.rolling(lookback, min_periods=1).mean()


def detect_pretouch_events(
    bars_tf: pd.DataFrame,
    bars_1s: pd.DataFrame,
    config: TimeframeConfig,
    symbol: str,
) -> pd.DataFrame:
    """检测指定时间框架下的 pretouch 事件（向量化优化版）。

    优化策略：
    1. 预先将 1s bars 的 index 转为 numpy int64 数组
    2. 用 np.searchsorted 做 O(log N) 的时间范围查找
    3. 批量处理所有 signal bars
    """
    atr_series = compute_atr(bars_tf, config.atr_lookback)
    events = []

    bar_times = bars_tf.index.tolist()
    n_bars = len(bar_times)

    if n_bars < 3:
        return pd.DataFrame()

    # 预处理：将 1s bars index 转为 int64 纳秒数组用于 searchsorted
    bars_1s_times = bars_1s.index.astype(np.int64)
    bars_1s_high = bars_1s["high"].values
    bars_1s_low = bars_1s["low"].values
    bars_1s_close = bars_1s["close"].values
    bars_1s_open = bars_1s["open"].values

    bar_seconds_td = pd.Timedelta(seconds=config.bar_seconds)
    speed_window_td = pd.Timedelta(seconds=300)

    processed = 0
    for i in range(2, n_bars):
        signal_bar_start = bar_times[i]
        signal_bar_end = signal_bar_start + bar_seconds_td

        # prev bars
        prev_bar_1_high = bars_tf.iloc[i - 1]["high"]
        prev_bar_1_low = bars_tf.iloc[i - 1]["low"]
        prev_bar_1_open = bars_tf.iloc[i - 1]["open"]
        prev_bar_1_close = bars_tf.iloc[i - 1]["close"]

        prev_bar_2_high = bars_tf.iloc[i - 2]["high"]
        prev_bar_2_low = bars_tf.iloc[i - 2]["low"]

        # Breakout levels
        long_level = prev_bar_2_high
        short_level = prev_bar_2_low

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

        # 取 signal bar 内的 1s 数据切片
        sig_high = bars_1s_high[idx_start:idx_end]
        sig_low = bars_1s_low[idx_start:idx_end]
        sig_close = bars_1s_close[idx_start:idx_end]

        # Long touch: first 1s bar where high >= long_level
        if prev_bar_1_high < long_level:
            long_touch_indices = np.where(sig_high >= long_level)[0]
            if len(long_touch_indices) > 0:
                touch_local_idx = long_touch_indices[0]
                touch_abs_idx = idx_start + touch_local_idx
                touch_time = bars_1s.index[touch_abs_idx]
                touch_price = float(sig_high[touch_local_idx])

                event = _build_event_fast(
                    symbol=symbol, side="long",
                    touch_time=touch_time, touch_price=touch_price,
                    level=long_level, atr=atr,
                    signal_bar_start=signal_bar_start,
                    prev_bar_1_open=prev_bar_1_open, prev_bar_1_high=prev_bar_1_high,
                    prev_bar_1_low=prev_bar_1_low, prev_bar_1_close=prev_bar_1_close,
                    bars_1s_times=bars_1s_times, bars_1s_high=bars_1s_high,
                    bars_1s_low=bars_1s_low, bars_1s_close=bars_1s_close,
                    bars_1s_open=bars_1s_open, bars_1s_index=bars_1s.index,
                    config=config,
                )
                if event is not None:
                    events.append(event)

        # Short touch: first 1s bar where low <= short_level
        if prev_bar_1_low > short_level:
            short_touch_indices = np.where(sig_low <= short_level)[0]
            if len(short_touch_indices) > 0:
                touch_local_idx = short_touch_indices[0]
                touch_abs_idx = idx_start + touch_local_idx
                touch_time = bars_1s.index[touch_abs_idx]
                touch_price = float(sig_low[touch_local_idx])

                event = _build_event_fast(
                    symbol=symbol, side="short",
                    touch_time=touch_time, touch_price=touch_price,
                    level=short_level, atr=atr,
                    signal_bar_start=signal_bar_start,
                    prev_bar_1_open=prev_bar_1_open, prev_bar_1_high=prev_bar_1_high,
                    prev_bar_1_low=prev_bar_1_low, prev_bar_1_close=prev_bar_1_close,
                    bars_1s_times=bars_1s_times, bars_1s_high=bars_1s_high,
                    bars_1s_low=bars_1s_low, bars_1s_close=bars_1s_close,
                    bars_1s_open=bars_1s_open, bars_1s_index=bars_1s.index,
                    config=config,
                )
                if event is not None:
                    events.append(event)

        processed += 1
        if processed % 1000 == 0:
            logger.info("  Processed %d/%d bars, %d events so far", processed, n_bars - 2, len(events))

    if not events:
        return pd.DataFrame()

    df = pd.DataFrame(events)
    logger.info("Detected %d raw pretouch events for %s %s", len(df), symbol, config.name)
    return df


def _build_event_fast(
    symbol: str, side: str,
    touch_time: pd.Timestamp, touch_price: float,
    level: float, atr: float,
    signal_bar_start: pd.Timestamp,
    prev_bar_1_open: float, prev_bar_1_high: float,
    prev_bar_1_low: float, prev_bar_1_close: float,
    bars_1s_times: np.ndarray, bars_1s_high: np.ndarray,
    bars_1s_low: np.ndarray, bars_1s_close: np.ndarray,
    bars_1s_open: np.ndarray, bars_1s_index,
    config: TimeframeConfig,
) -> dict | None:
    """构建单个事件（向量化版本，避免 DataFrame 操作）。"""

    pre_touch_seconds = (touch_time - signal_bar_start).total_seconds()
    if pre_touch_seconds > config.max_pre_touch_seconds:
        return None

    # touch_extension_atr
    if side == "long":
        touch_ext_atr = (touch_price - level) / atr
    else:
        touch_ext_atr = (level - touch_price) / atr

    # speed_300s: 用 searchsorted 找 300s 窗口
    window_start_ns = (touch_time - pd.Timedelta(seconds=300)).value
    touch_ns = touch_time.value
    ws_idx = np.searchsorted(bars_1s_times, window_start_ns, side='left')
    we_idx = np.searchsorted(bars_1s_times, touch_ns, side='right')

    if we_idx - ws_idx < 10:
        return None

    speed_close = bars_1s_close[ws_idx:we_idx]
    speed_high = bars_1s_high[ws_idx:we_idx]
    speed_low = bars_1s_low[ws_idx:we_idx]

    first_price = float(speed_close[0])
    last_price = float(speed_close[-1])
    speed_300s_atr = (last_price - first_price) / atr

    # eff_300s
    high_300 = float(speed_high.max())
    low_300 = float(speed_low.min())
    total_range = high_300 - low_300
    net_move = abs(last_price - first_price)
    eff_300s = net_move / total_range if total_range > 0 else 0.0

    if eff_300s > 1.0:
        return None

    # roundtrip_cost_atr (simplified)
    roundtrip_cost_atr = 0.10  # placeholder

    # prev1 features
    prev1_range = prev_bar_1_high - prev_bar_1_low
    prev1_body_atr = abs(prev_bar_1_close - prev_bar_1_open) / atr
    prev1_range_atr = prev1_range / atr
    prev1_close_pos = (prev_bar_1_close - prev_bar_1_low) / prev1_range if prev1_range > 0 else 0.5
    if side == "short":
        prev1_close_pos = 1.0 - prev1_close_pos

    # signal_open
    sig_start_idx = np.searchsorted(bars_1s_times, signal_bar_start.value, side='left')
    signal_open = float(bars_1s_open[sig_start_idx]) if sig_start_idx < len(bars_1s_open) else touch_price
    level_to_signal_open_atr = (level - signal_open) / atr

    event_id = f"{symbol}_{config.name}_{touch_time.strftime('%Y%m%d_%H%M%S')}_{side}"

    return {
        "event_id": event_id,
        "symbol": symbol,
        "side": side,
        "touch_time": touch_time,
        "touch_price": touch_price,
        "level": level,
        "atr": atr,
        "touch_extension_atr": touch_ext_atr,
        "speed_300s_atr": speed_300s_atr,
        "eff_300s": eff_300s,
        "pre_touch_seconds": pre_touch_seconds,
        "roundtrip_cost_atr": roundtrip_cost_atr,
        "signal_start": signal_bar_start,
        "signal_end": signal_bar_start + pd.Timedelta(seconds=config.bar_seconds),
        "signal_open": signal_open,
        "signal_high": 0.0,
        "signal_low": 0.0,
        "signal_close": 0.0,
        "prev1_body_atr": prev1_body_atr,
        "prev1_range_atr": prev1_range_atr,
        "prev1_close_pos_side": prev1_close_pos,
        "level_to_signal_open_atr": level_to_signal_open_atr,
        "timeframe": config.name,
        "max_hold_hours": config.max_hold_hours,
    }


def build_multi_timeframe_events(
    symbol: str = "ETHUSDT",
    timeframes: list[str] | None = None,
) -> dict[str, pd.DataFrame]:
    """为指定 symbol 构建多时间框架的 pretouch 事件池。

    Returns
    -------
    dict[str, pd.DataFrame]
        key 为时间框架名称，value 为事件 DataFrame。
    """
    if timeframes is None:
        timeframes = ["30min", "1h", "4h"]

    # Load all 1s bars
    bars_1s = load_all_1s_bars(symbol)

    results = {}
    for tf_name in timeframes:
        config = TIMEFRAME_CONFIGS[tf_name]
        logger.info("=" * 60)
        logger.info("Building %s events for %s", tf_name, symbol)
        logger.info("=" * 60)

        # Resample to target timeframe
        bars_tf = resample_to_timeframe(bars_1s, config.resample_rule)

        # Detect pretouch events
        events = detect_pretouch_events(bars_tf, bars_1s, config, symbol)

        if events.empty:
            logger.warning("No events detected for %s %s", symbol, tf_name)
            results[tf_name] = events
            continue

        # Save to CSV
        OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
        out_path = OUTPUT_DIR / f"pretouch_events_{symbol}_{tf_name}.csv"
        events.to_csv(out_path, index=False)
        logger.info("Saved %d events to %s", len(events), out_path)

        results[tf_name] = events

    return results


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")

    results = build_multi_timeframe_events(symbol="ETHUSDT", timeframes=["30min", "1h", "4h"])

    print("\n" + "=" * 60)
    print("Multi-Timeframe Event Summary")
    print("=" * 60)
    for tf, df in results.items():
        if df.empty:
            print(f"  {tf}: 0 events")
        else:
            print(f"  {tf}: {len(df)} events, {df['touch_time'].min()} to {df['touch_time'].max()}")
            print(f"       long={len(df[df['side']=='long'])}, short={len(df[df['side']=='short'])}")
            print(f"       speed_300s_atr: mean={df['speed_300s_atr'].abs().mean():.4f}, min={df['speed_300s_atr'].abs().min():.4f}")
