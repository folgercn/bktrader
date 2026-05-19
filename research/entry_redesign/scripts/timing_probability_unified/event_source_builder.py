"""事件源构建模块 — 加载 pretouch_small_pullback 事件集合

直接读取 tick-flow worktree 已经产出的 canonical seed events CSV：

- 默认: `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv` (394 events)
  这是 worktree 当前主 lead，对应过滤链：
    1. pretouch_small_pullback seed
    2. RF q50 (probability ranking quantile)
    3. speed_300s_atr >= train q10 (speed gate)
    4. touch30m: pre_touch_seconds <= 1800 (30m 内触发)
    5. eff300le1: eff_300s <= 1.0 (排除过度推进)

- 备选: `pretouch_small_pullback_rf_q50_speed300_ge_q10.csv` (523 events)
  仅 RF q50 + speed gate，不含 touch30m_eff300le1 robust quality 过滤。

这取代了之前的 self-filter 版本（abs(touch_extension_atr) ∈ [0.10, 0.15] 等
硬编码阈值）— 该实现产出的事件池只有 36 events，与 worktree 报告的 394 事件
口径不一致，导致下游 timing classifier / RF 模型样本不足。
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from pathlib import Path
from typing import Tuple

import pandas as pd

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Paths — canonical seed events CSV from tick-flow worktree archive
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parents[4]

# 主候选: robust_quality 版本（394 events）— 与 worktree 当前 lead 对齐
CANONICAL_EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "tick_flow_event_sources"
    / "20260514_pretouch_full_window"
    / "feature_filtered_seed_events"
    / "robust_quality"
    / "pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv"
)

# 备选: 不含 robust_quality 过滤（523 events），用于对照实验
FALLBACK_EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "tick_flow_event_sources"
    / "20260514_pretouch_full_window"
    / "feature_filtered_seed_events"
    / "pretouch_small_pullback_rf_q50_speed300_ge_q10.csv"
)

# 1s bar cache (existing infrastructure)
BARS_CACHE_DIR = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest"
    / "bars_cache"
)


@dataclass
class EventPoolStats:
    """事件池统计信息"""

    total_events: int
    btc_count: int
    eth_count: int
    btc_pct: float
    eth_pct: float
    long_count: int
    short_count: int
    long_pct: float
    short_pct: float
    earliest_touch_time: pd.Timestamp
    latest_touch_time: pd.Timestamp
    small_pool_warning: bool  # total_events < 400


@dataclass
class EventPoolResult:
    """事件源构建结果"""

    events_pool: pd.DataFrame  # 完整事件池
    train_events: pd.DataFrame  # 训练集
    test_events: pd.DataFrame  # 测试集（full-window test）
    forward_events: pd.DataFrame  # forward split (2025-11..2026-04)
    bars_cache: dict[str, pd.DataFrame]  # symbol_month → 1s bars
    stats: EventPoolStats
    skipped_events: pd.DataFrame  # 因数据缺失跳过的事件


# ---------------------------------------------------------------------------
# Internal helpers — pretouch_small_pullback 过滤
# ---------------------------------------------------------------------------


def _is_pretouch_small_pullback(row: pd.Series) -> bool:
    """判断单个 event 是否满足 pretouch_small_pullback 条件。

    条件（与 tick-flow 实验 fast_clean_or_small_pullback 一致）：
    - abs(touch_extension_atr) ∈ [0.10, 0.15]
    - abs(speed_300s_atr) >= 0.20
    - abs(pullback_60s_atr) ∈ [0, 0.05]
    """
    d = abs(float(row.get("touch_extension_atr", 0.0)))
    s = abs(float(row.get("speed_300s_atr", 0.0)))
    p = abs(float(row.get("pullback_60s_atr", 999.0)))

    return (
        _PSB_DISTANCE_MIN <= d <= _PSB_DISTANCE_MAX
        and s >= _PSB_SPEED_MIN
        and 0.0 <= p <= _PSB_PULLBACK_MAX
    )


def _filter_pretouch_small_pullback(events: pd.DataFrame) -> pd.DataFrame:
    """从全量 events 中筛选满足 pretouch_small_pullback 条件的事件。"""
    mask = events.apply(_is_pretouch_small_pullback, axis=1)
    filtered = events[mask].copy()
    logger.info(
        "Pretouch small pullback filter: %d -> %d events",
        len(events),
        len(filtered),
    )
    return filtered


# ---------------------------------------------------------------------------
# Internal helpers — bars cache 加载
# ---------------------------------------------------------------------------


def _find_bars_cache_file(symbol: str, event_time: pd.Timestamp) -> Path | None:
    """在 BARS_CACHE_DIR 中查找覆盖 event_time 的 cache 文件。"""
    candidates = list(BARS_CACHE_DIR.glob(f"{symbol}_*_flow_1s.pkl"))
    for p in candidates:
        parts = p.stem.replace("_flow_1s", "").split("_")
        if len(parts) < 3:
            continue
        try:
            cache_start = pd.Timestamp(parts[1], tz="UTC")
            cache_end = pd.Timestamp(parts[2], tz="UTC")
            if cache_start <= event_time <= cache_end:
                return p
        except Exception:
            continue
    return None


def _load_monthly_bars(symbol: str, month_start: pd.Timestamp) -> pd.DataFrame | None:
    """加载指定 symbol 和月份的 1s bar cache。

    优先尝试精确匹配文件名，否则 fallback 到模糊匹配。
    复用 dynamic_timing.data_layer._load_monthly_bars 逻辑。
    """
    month_end = (month_start + pd.offsets.MonthEnd(0)).replace(
        hour=23, minute=59, second=59
    )
    start_key = month_start.strftime("%Y%m%dT%H%M%S")
    end_key = month_end.strftime("%Y%m%dT%H%M%S")

    exact = BARS_CACHE_DIR / f"{symbol}_{start_key}_{end_key}_flow_1s.pkl"
    if exact.exists():
        return pd.read_pickle(exact)

    cache_path = _find_bars_cache_file(symbol, month_start + pd.Timedelta(days=15))
    if cache_path is not None:
        return pd.read_pickle(cache_path)
    return None


def _load_bars_cache_for_events(
    events: pd.DataFrame,
) -> dict[str, pd.DataFrame]:
    """按 symbol_month key 加载 1s bar pickle cache。

    复用 dynamic_timing.data_layer.load_bars_cache 逻辑。

    Returns
    -------
    dict[str, pd.DataFrame]
        key 为 "{symbol}_{YYYYMM}"，value 为该月的 1s bars DataFrame。
    """
    pairs: set[tuple[str, str]] = set()
    for _, row in events.iterrows():
        symbol = str(row["symbol"])
        tt = pd.Timestamp(row["touch_time"])
        if tt.tzinfo is None:
            tt = tt.tz_localize("UTC")
        month_key = tt.strftime("%Y%m")
        pairs.add((symbol, month_key))

    logger.info("需要加载 %d 个 (symbol, month) 组合的 bars cache", len(pairs))

    cache: dict[str, pd.DataFrame] = {}
    for symbol, month_key in sorted(pairs):
        key = f"{symbol}_{month_key}"
        month_start = pd.Timestamp(
            f"{month_key[:4]}-{month_key[4:]}-01", tz="UTC"
        )
        bars = _load_monthly_bars(symbol, month_start)
        if bars is not None:
            cache[key] = bars
            logger.debug("Loaded %d bars for %s", len(bars), key)
        else:
            logger.warning("No bars cache found for %s, skipping", key)

    logger.info(
        "Bars cache 加载完成: %d loaded, %d missing (共 %d 组合)",
        len(cache),
        len(pairs) - len(cache),
        len(pairs),
    )
    return cache


# ---------------------------------------------------------------------------
# Public helpers — split 与 filter
# ---------------------------------------------------------------------------


def split_events_by_time(
    events: pd.DataFrame,
    forward_start: str,
    train_ratio: float = 0.6,
) -> Tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]:
    """按 touch_time 时间排序后执行 train/test/forward split。

    逻辑：
    1. 按 touch_time 排序
    2. forward split: touch_time >= forward_start 的事件归入 forward set
    3. 剩余事件（full-window）按 train_ratio 做 train/test split
    4. train/test split 也是时间有序的：train 在前，test 在后

    不变量保证：
    - ALL events in train have touch_time <= ALL events in test
    - ALL events in (train ∪ test) have touch_time < forward_start

    Parameters
    ----------
    events : pd.DataFrame
        事件池，需含 touch_time 列。
    forward_start : str
        Forward split 起始日期（ISO 格式，如 "2025-11-01"）。
    train_ratio : float
        Full-window 中 train 的比例，默认 0.6。

    Returns
    -------
    Tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]
        (train_events, test_events, forward_events)
    """
    if events.empty:
        empty = events.iloc[0:0].copy()
        return empty, empty.copy(), empty.copy()

    # Ensure touch_time is datetime
    sorted_events = events.copy()
    sorted_events["touch_time"] = pd.to_datetime(sorted_events["touch_time"], utc=True)
    sorted_events = sorted_events.sort_values("touch_time").reset_index(drop=True)

    # Forward split: events with touch_time >= forward_start
    forward_ts = pd.Timestamp(forward_start, tz="UTC")
    forward_mask = sorted_events["touch_time"] >= forward_ts
    forward_events = sorted_events[forward_mask].reset_index(drop=True)

    # Full-window: events before forward_start
    full_window = sorted_events[~forward_mask].reset_index(drop=True)

    # Train/test split within full-window (time-ordered)
    split_idx = int(len(full_window) * train_ratio)
    train_events = full_window.iloc[:split_idx].reset_index(drop=True)
    test_events = full_window.iloc[split_idx:].reset_index(drop=True)

    return train_events, test_events, forward_events


def compute_event_pool_stats(events: pd.DataFrame) -> EventPoolStats:
    """从事件 DataFrame 计算事件池统计信息。

    Parameters
    ----------
    events : pd.DataFrame
        事件池 DataFrame，需包含 'symbol'、'side'、'touch_time' 列。
        symbol 值为 "BTCUSDT" 或 "ETHUSDT"。
        side 值为 "long" 或 "short"。

    Returns
    -------
    EventPoolStats
        事件池统计信息。
    """
    total_events = len(events)

    if total_events == 0:
        return EventPoolStats(
            total_events=0,
            btc_count=0,
            eth_count=0,
            btc_pct=0.0,
            eth_pct=0.0,
            long_count=0,
            short_count=0,
            long_pct=0.0,
            short_pct=0.0,
            earliest_touch_time=pd.NaT,
            latest_touch_time=pd.NaT,
            small_pool_warning=True,
        )

    btc_count = int((events["symbol"] == "BTCUSDT").sum())
    eth_count = int((events["symbol"] == "ETHUSDT").sum())
    btc_pct = (btc_count / total_events) * 100.0
    eth_pct = (eth_count / total_events) * 100.0

    long_count = int((events["side"] == "long").sum())
    short_count = int((events["side"] == "short").sum())
    long_pct = (long_count / total_events) * 100.0
    short_pct = (short_count / total_events) * 100.0

    earliest_touch_time = events["touch_time"].min()
    latest_touch_time = events["touch_time"].max()

    small_pool_warning = total_events < 400

    return EventPoolStats(
        total_events=total_events,
        btc_count=btc_count,
        eth_count=eth_count,
        btc_pct=btc_pct,
        eth_pct=eth_pct,
        long_count=long_count,
        short_count=short_count,
        long_pct=long_pct,
        short_pct=short_pct,
        earliest_touch_time=earliest_touch_time,
        latest_touch_time=latest_touch_time,
        small_pool_warning=small_pool_warning,
    )


def filter_events_by_bars_cache(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> Tuple[pd.DataFrame, pd.DataFrame]:
    """过滤事件：跳过 bar cache 不可用的 event。

    对每个 event，检查其 symbol_month key 是否在 bars_cache 中存在且非空。
    不可用的 event 记录到 skipped_events。

    Parameters
    ----------
    events : pd.DataFrame
        事件池，需含 symbol 和 touch_time 列。
    bars_cache : dict[str, pd.DataFrame]
        已加载的 bars cache，key 为 "{symbol}_{YYYYMM}"。

    Returns
    -------
    Tuple[pd.DataFrame, pd.DataFrame]
        (valid_events, skipped_events)
    """
    if events.empty:
        return events.copy(), events.iloc[0:0].copy()

    valid_mask = []
    for _, row in events.iterrows():
        symbol = str(row["symbol"])
        tt = pd.Timestamp(row["touch_time"])
        if tt.tzinfo is None:
            tt = tt.tz_localize("UTC")
        month_key = f"{symbol}_{tt.strftime('%Y%m')}"
        # Event is valid if its month key exists in cache and the DataFrame is non-empty
        is_valid = month_key in bars_cache and not bars_cache[month_key].empty
        valid_mask.append(is_valid)

    mask = pd.Series(valid_mask, index=events.index)
    valid_events = events[mask].reset_index(drop=True)
    skipped_events = events[~mask].reset_index(drop=True)

    if len(skipped_events) > 0:
        logger.warning(
            "Skipped %d events due to missing bars cache", len(skipped_events)
        )

    return valid_events, skipped_events


# ---------------------------------------------------------------------------
# Public API — 主入口
# ---------------------------------------------------------------------------


def build_event_pool(
    seed_source: str = "canonical",
    forward_start: str = "2025-11-01",
) -> EventPoolResult:
    """构建 pretouch_small_pullback 事件池。

    默认直接加载 worktree 产出的 canonical seed events CSV（394 events），
    不再从 events_execution_labeled.csv 重新过滤。

    Parameters
    ----------
    seed_source : str
        事件来源：
        - "canonical"（默认）：加载 touch30m_eff300le1 robust quality CSV (394 events)
        - "unfiltered"：加载 RF q50 + speed gate CSV (523 events)，不含 robust quality
    forward_start : str
        Forward split 起始日期（ISO 格式）。

    Returns
    -------
    EventPoolResult
        包含事件池、train/test/forward splits、bars cache、统计信息。

    Raises
    ------
    FileNotFoundError
        如果 canonical events CSV 不存在。
    ValueError
        如果事件池为空。
    """
    logger.info("=" * 60)
    logger.info(
        "build_event_pool() — seed_source=%s, forward_start=%s",
        seed_source,
        forward_start,
    )
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # Step 1: 加载 canonical seed events CSV
    # ------------------------------------------------------------------
    if seed_source == "unfiltered":
        events_csv = FALLBACK_EVENTS_CSV
    else:
        events_csv = CANONICAL_EVENTS_CSV

    if not events_csv.exists():
        raise FileNotFoundError(
            f"Canonical events CSV not found: {events_csv}\n"
            f"请从 bkTrader-research-adverse-fill-20260514 worktree 复制该文件。"
        )

    all_events = pd.read_csv(events_csv)
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)
    logger.info(
        "Canonical events CSV 加载: %d events from %s",
        len(all_events),
        events_csv.name,
    )

    if all_events.empty:
        raise ValueError("事件池为空")

    # 确保 event_id 列存在
    if "event_id" not in all_events.columns:
        all_events["event_id"] = [
            f"psb_{i:04d}" for i in range(len(all_events))
        ]

    # 确保 speed_300s_atr 列存在（用于 speed gate）
    if "speed_300s_atr" not in all_events.columns:
        logger.warning("speed_300s_atr 列不存在，设为 0.0")
        all_events["speed_300s_atr"] = 0.0

    # ------------------------------------------------------------------
    # Step 2: 加载 bars cache
    # ------------------------------------------------------------------
    bars_cache = _load_bars_cache_for_events(all_events)

    # ------------------------------------------------------------------
    # Step 3: 跳过 bar cache 不可用的 events
    # ------------------------------------------------------------------
    events_pool, skipped_events = filter_events_by_bars_cache(
        all_events, bars_cache
    )

    # 为 skipped_events 添加 reason 列
    if not skipped_events.empty:
        skipped_events = skipped_events[
            [c for c in ["event_id", "symbol", "touch_time", "side"] if c in skipped_events.columns]
        ].copy()
        skipped_events["reason"] = "bars_cache_unavailable"

    logger.info(
        "最终事件池: %d events（跳过 %d）",
        len(events_pool),
        len(skipped_events),
    )

    if events_pool.empty:
        raise ValueError("所有事件的 bar cache 均不可用，无法构建事件池")

    # ------------------------------------------------------------------
    # Step 4: 按 touch_time 排序并执行 train/test/forward split
    # ------------------------------------------------------------------
    train_events, test_events, forward_events = split_events_by_time(
        events_pool, forward_start=forward_start, train_ratio=0.6
    )

    # events_pool 也按 touch_time 排序（与 split 一致）
    events_pool = events_pool.copy()
    events_pool["touch_time"] = pd.to_datetime(events_pool["touch_time"], utc=True)
    events_pool = events_pool.sort_values("touch_time").reset_index(drop=True)

    logger.info(
        "Split 完成: train=%d, test=%d, forward=%d",
        len(train_events),
        len(test_events),
        len(forward_events),
    )

    # ------------------------------------------------------------------
    # Step 5: 计算 EventPoolStats
    # ------------------------------------------------------------------
    stats = compute_event_pool_stats(events_pool)

    logger.info("事件池统计:")
    logger.info("  total_events: %d", stats.total_events)
    logger.info(
        "  BTC: %d (%.1f%%), ETH: %d (%.1f%%)",
        stats.btc_count,
        stats.btc_pct,
        stats.eth_count,
        stats.eth_pct,
    )
    logger.info(
        "  Long: %d (%.1f%%), Short: %d (%.1f%%)",
        stats.long_count,
        stats.long_pct,
        stats.short_count,
        stats.short_pct,
    )
    logger.info(
        "  时间范围: %s ~ %s",
        stats.earliest_touch_time,
        stats.latest_touch_time,
    )
    logger.info("  small_pool_warning: %s", stats.small_pool_warning)

    return EventPoolResult(
        events_pool=events_pool,
        train_events=train_events,
        test_events=test_events,
        forward_events=forward_events,
        bars_cache=bars_cache,
        stats=stats,
        skipped_events=skipped_events,
    )
