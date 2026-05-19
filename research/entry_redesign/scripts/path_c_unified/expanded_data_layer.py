"""
expanded_data_layer — 扩展数据加载层

基于 gate_explorer.py 确认的最优策略（维度 C：直接使用 events_execution_labeled.csv
按 bars_cache 覆盖时间范围筛选），加载扩展事件池和对应的 1s bar cache。

保证：
- 返回的 events 包含原 116 events 作为子集
- 复用 pre_breakout_timing.data_layer 的 load_bars_cache 逻辑
- touch_time 已 UTC 标准化
"""

from __future__ import annotations

import logging
from pathlib import Path

import pandas as pd

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Paths (与 gate_explorer.py 一致)
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parents[4]

EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "2025m03_2026apr_original_t2_delay60"
    / "events_execution_labeled.csv"
)

BARS_CACHE_DIR = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest"
    / "bars_cache"
)

# bars_cache 覆盖时间范围
_CACHE_START = pd.Timestamp("2025-06-01", tz="UTC")
_CACHE_END = pd.Timestamp("2026-04-30 23:59:59", tz="UTC")


# ---------------------------------------------------------------------------
# Internal helpers (复用 dynamic_timing.data_layer 的 bars cache 加载逻辑)
# ---------------------------------------------------------------------------
def _find_bars_cache(symbol: str, event_time: pd.Timestamp) -> Path | None:
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

    cache_path = _find_bars_cache(symbol, month_start + pd.Timedelta(days=15))
    if cache_path is not None:
        return pd.read_pickle(cache_path)
    return None


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------
def load_expanded_events(
    original_events: pd.DataFrame,
) -> pd.DataFrame:
    """根据选定策略（维度 C）加载扩展事件池。

    策略：直接使用 events_execution_labeled.csv（original_t2_delay60），
    按 bars_cache 覆盖时间范围（2025-06-01 至 2026-04-30）筛选。

    保证：
    - 返回的 events 包含原 116 events 作为子集
    - events 含 events_execution_labeled.csv 的所有列
    - touch_time 已 UTC 标准化

    Parameters
    ----------
    original_events : 原 116 events（从 load_v6_gate_events() 获取）

    Returns
    -------
    pd.DataFrame : 扩展事件池
    """
    if not EVENTS_CSV.exists():
        raise FileNotFoundError(
            f"events_execution_labeled.csv not found: {EVENTS_CSV}"
        )

    # 加载全量 events
    all_events = pd.read_csv(EVENTS_CSV)
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)
    logger.info("全量 events CSV 加载: %d events", len(all_events))

    # 按 bars_cache 覆盖时间范围筛选
    mask = (all_events["touch_time"] >= _CACHE_START) & (
        all_events["touch_time"] <= _CACHE_END
    )
    filtered = all_events[mask].copy()
    logger.info(
        "bars_cache 时间范围筛选: %d -> %d events",
        len(all_events),
        len(filtered),
    )

    # 确保原 116 events 包含在内（子集不变量）
    original_events = original_events.copy()
    original_events["touch_time"] = pd.to_datetime(
        original_events["touch_time"], utc=True
    )
    original_ids = set(original_events["event_id"])
    filtered_ids = set(filtered["event_id"])
    missing_ids = original_ids - filtered_ids

    if missing_ids:
        logger.warning(
            "扩展池缺少 %d 个原始 events，补充中...", len(missing_ids)
        )
        missing_events = original_events[
            original_events["event_id"].isin(missing_ids)
        ].copy()
        filtered = pd.concat([filtered, missing_events], ignore_index=True)
        filtered = filtered.drop_duplicates(subset=["event_id"])

    # 验证子集不变量
    final_ids = set(filtered["event_id"])
    assert original_ids.issubset(final_ids), (
        f"子集不变量违反: {len(original_ids - final_ids)} 个原始 events 缺失"
    )

    logger.info(
        "扩展事件池加载完成: %d events (原 %d events 全部包含)",
        len(filtered),
        len(original_ids),
    )

    return filtered.reset_index(drop=True)


def load_expanded_bars_cache(
    events: pd.DataFrame,
) -> dict[str, pd.DataFrame]:
    """为扩展事件池加载 1s bar cache。

    复用 pre_breakout_timing.data_layer.load_bars_cache 逻辑：
    - 确定所有 (symbol, month) 对
    - 按 "{symbol}_{YYYYMM}" key 加载对应月份的 1s bar pickle cache
    - 跳过无法加载的月份（记录 warning）

    Parameters
    ----------
    events : 扩展事件池 DataFrame（需含 symbol, touch_time 列）

    Returns
    -------
    dict[str, pd.DataFrame] : key 为 "{symbol}_{YYYYMM}"，value 为该月的 1s bars DataFrame
    """
    # 确定所有需要的 (symbol, month) 对
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
    loaded_count = 0
    missing_count = 0

    for symbol, month_key in sorted(pairs):
        key = f"{symbol}_{month_key}"
        month_start = pd.Timestamp(
            f"{month_key[:4]}-{month_key[4:]}-01", tz="UTC"
        )
        bars = _load_monthly_bars(symbol, month_start)
        if bars is not None:
            cache[key] = bars
            loaded_count += 1
            logger.debug("Loaded %d bars for %s", len(bars), key)
        else:
            missing_count += 1
            logger.warning("No bars cache found for %s, skipping", key)

    logger.info(
        "Bars cache 加载完成: %d loaded, %d missing (共 %d 组合)",
        loaded_count,
        missing_count,
        len(pairs),
    )

    return cache
