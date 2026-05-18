"""feature_computer — 从 1s bar cache 计算缺失的 pre-breakout 特征

补全 canonical events CSV 中全 NaN 的 5 个特征：
- signal_atr_percentile: 当前 signal bar ATR 在过去 20 根 1h bar ATR 中的百分位
- prev1_body_atr: 前一根 1h bar 的 body (|close - open|) / ATR
- prev_sma5_gap_atr: (prev1_close - SMA5_close) / ATR
- prev_sma5_slope_atr: (SMA5_close - SMA5_close_1bar_ago) / ATR
- level_to_prev_close_atr: (level - prev1_close) / ATR (方向感知)

所有特征仅使用 touch_time 之前已闭合的 1h bar 数据（Point_In_Time 约束）。
"""

from __future__ import annotations

import logging

import numpy as np
import pandas as pd

logger = logging.getLogger(__name__)


def _resample_1s_to_1h(bars_1s: pd.DataFrame, end_time: pd.Timestamp, n_hours: int = 24) -> pd.DataFrame:
    """从 1s bars 中提取 end_time 之前 n_hours 根已闭合 1h bar。

    Parameters
    ----------
    bars_1s : pd.DataFrame
        1s bar 数据，DatetimeIndex，含 open/high/low/close 列。
    end_time : pd.Timestamp
        截止时间（不含该时刻所在的未闭合 bar）。
    n_hours : int
        需要的 1h bar 数量。

    Returns
    -------
    pd.DataFrame
        1h OHLC bars，按时间升序排列，最多 n_hours 行。
    """
    # 取 end_time 之前的数据
    mask = bars_1s.index < end_time
    pre_bars = bars_1s.loc[mask]

    if pre_bars.empty:
        return pd.DataFrame(columns=["open", "high", "low", "close"])

    # 只取最近 n_hours 小时的数据
    start_cutoff = end_time - pd.Timedelta(hours=n_hours)
    pre_bars = pre_bars.loc[pre_bars.index >= start_cutoff]

    if pre_bars.empty:
        return pd.DataFrame(columns=["open", "high", "low", "close"])

    # Resample to 1h bars
    hourly = pre_bars.resample("1h").agg({
        "open": "first",
        "high": "max",
        "low": "min",
        "close": "last",
    }).dropna(subset=["open"])

    return hourly


def _precompute_hourly_bars(bars_cache: dict[str, pd.DataFrame]) -> dict[str, pd.DataFrame]:
    """一次性将所有月份的 1s bars 预聚合为 1h bars。

    这避免了对每个 event 重复 resample 整个月的 1s 数据。

    Returns
    -------
    dict[str, pd.DataFrame]
        key 与 bars_cache 相同（"{symbol}_{YYYYMM}"），value 为 1h OHLC bars。
    """
    hourly_cache: dict[str, pd.DataFrame] = {}
    for key, bars_1s in bars_cache.items():
        if bars_1s.empty:
            hourly_cache[key] = pd.DataFrame(columns=["open", "high", "low", "close"])
            continue
        hourly = bars_1s.resample("1h").agg({
            "open": "first",
            "high": "max",
            "low": "min",
            "close": "last",
        }).dropna(subset=["open"])
        hourly_cache[key] = hourly
    logger.info("Pre-computed 1h bars for %d months", len(hourly_cache))
    return hourly_cache


def _get_hourly_bars_before(
    hourly_cache: dict[str, pd.DataFrame],
    symbol: str,
    end_time: pd.Timestamp,
    n_hours: int = 24,
) -> pd.DataFrame:
    """从预计算的 1h cache 中获取 end_time 之前的 n_hours 根 bar。"""
    # 当前月 key
    month_key = f"{symbol}_{end_time.strftime('%Y%m')}"
    current_hourly = hourly_cache.get(month_key, pd.DataFrame())

    # 前一个月 key（处理月初事件需要跨月数据）
    prev_month = end_time - pd.Timedelta(days=28)
    prev_key = f"{symbol}_{prev_month.strftime('%Y%m')}"
    prev_hourly = hourly_cache.get(prev_key, pd.DataFrame())

    # 合并
    parts = []
    if not prev_hourly.empty:
        parts.append(prev_hourly)
    if not current_hourly.empty:
        parts.append(current_hourly)

    if not parts:
        return pd.DataFrame(columns=["open", "high", "low", "close"])

    combined = pd.concat(parts).sort_index()
    combined = combined[~combined.index.duplicated(keep='first')]

    # 截取 end_time 之前的 n_hours 根
    before = combined.loc[combined.index < end_time]
    if len(before) > n_hours:
        before = before.iloc[-n_hours:]

    return before


def compute_missing_features(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> pd.DataFrame:
    """为事件池计算 5 个缺失特征。

    Parameters
    ----------
    events : pd.DataFrame
        事件池，需含 signal_start, touch_time, atr, level, side, symbol 列。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache，key 为 "{symbol}_{YYYYMM}"。

    Returns
    -------
    pd.DataFrame
        与 events 同长度的 DataFrame，含 5 个计算后的特征列。
        无法计算的值为 NaN。
    """
    n = len(events)
    signal_atr_percentile = np.full(n, np.nan)
    prev1_body_atr = np.full(n, np.nan)
    prev_sma5_gap_atr = np.full(n, np.nan)
    prev_sma5_slope_atr = np.full(n, np.nan)
    level_to_prev_close_atr = np.full(n, np.nan)

    # 预计算 1h bars（一次性，避免重复 resample）
    hourly_cache = _precompute_hourly_bars(bars_cache)

    computed = 0
    failed = 0

    for idx in range(n):
        row = events.iloc[idx]
        try:
            symbol = str(row["symbol"])
            atr = float(row["atr"])
            level = float(row["level"])
            side = str(row["side"])

            # signal_start 是当前 signal bar 的开始时间
            signal_start = pd.Timestamp(row["signal_start"])
            if signal_start.tzinfo is None:
                signal_start = signal_start.tz_localize("UTC")

            if atr <= 0 or np.isnan(atr):
                failed += 1
                continue

            # 从预计算的 1h cache 获取 signal_start 之前的 24 根 bar
            hourly_bars = _get_hourly_bars_before(
                hourly_cache, symbol, signal_start, n_hours=24
            )

            if len(hourly_bars) < 2:
                failed += 1
                continue

            # --- prev1 bar = 最后一根已闭合 1h bar ---
            prev1 = hourly_bars.iloc[-1]
            prev1_open = float(prev1["open"])
            prev1_close = float(prev1["close"])

            # prev1_body_atr = |close - open| / ATR
            prev1_body_atr[idx] = abs(prev1_close - prev1_open) / atr

            # level_to_prev_close_atr = (level - prev1_close) / ATR (方向感知)
            diff = level - prev1_close
            if side == "short":
                diff = -diff
            level_to_prev_close_atr[idx] = diff / atr

            # --- SMA5 计算（需要至少 5 根 1h bar） ---
            if len(hourly_bars) >= 5:
                closes = hourly_bars["close"].values
                sma5_current = float(np.mean(closes[-5:]))

                # prev_sma5_gap_atr = (prev1_close - SMA5) / ATR
                prev_sma5_gap_atr[idx] = (prev1_close - sma5_current) / atr

                # prev_sma5_slope_atr = (SMA5_current - SMA5_prev) / ATR
                if len(hourly_bars) >= 6:
                    sma5_prev = float(np.mean(closes[-6:-1]))
                    prev_sma5_slope_atr[idx] = (sma5_current - sma5_prev) / atr

            # --- signal_atr_percentile ---
            if len(hourly_bars) >= 5:
                ranges = (hourly_bars["high"] - hourly_bars["low"]).values
                percentile = float(np.sum(ranges < atr)) / len(ranges)
                signal_atr_percentile[idx] = percentile

            computed += 1

        except Exception as e:
            failed += 1
            if failed <= 5:
                logger.debug("Feature computation failed for event %d: %s", idx, e)

    logger.info(
        "Feature computation: %d computed, %d failed out of %d events",
        computed, failed, n,
    )

    result = pd.DataFrame({
        "signal_atr_percentile": signal_atr_percentile,
        "prev1_body_atr": prev1_body_atr,
        "prev_sma5_gap_atr": prev_sma5_gap_atr,
        "prev_sma5_slope_atr": prev_sma5_slope_atr,
        "level_to_prev_close_atr": level_to_prev_close_atr,
    }, index=events.index)

    return result


def fill_missing_features(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> pd.DataFrame:
    """补全事件池中全 NaN 的 5 个特征列。

    如果特征列已有非 NaN 值则不覆盖。

    Parameters
    ----------
    events : pd.DataFrame
        事件池 DataFrame。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache。

    Returns
    -------
    pd.DataFrame
        补全特征后的事件池（原地修改的副本）。
    """
    target_features = [
        "signal_atr_percentile",
        "prev1_body_atr",
        "prev_sma5_gap_atr",
        "prev_sma5_slope_atr",
        "level_to_prev_close_atr",
    ]

    # 检查哪些特征需要补全
    needs_fill = []
    for feat in target_features:
        if feat not in events.columns:
            needs_fill.append(feat)
        elif events[feat].isna().all():
            needs_fill.append(feat)

    if not needs_fill:
        logger.info("所有 5 个特征已有值，无需补全")
        return events

    logger.info("需要补全 %d 个特征: %s", len(needs_fill), needs_fill)

    # 计算缺失特征
    computed = compute_missing_features(events, bars_cache)

    # 填充到 events
    result = events.copy()
    for feat in needs_fill:
        if feat in computed.columns:
            result[feat] = computed[feat].values

    # 报告填充结果
    for feat in needs_fill:
        na_count = result[feat].isna().sum()
        total = len(result)
        logger.info("  %s: %d/%d filled (%.1f%% NaN remaining)",
                    feat, total - na_count, total, na_count / total * 100)

    return result
