"""
data_layer — 数据加载层

负责：
- 从 V6 lifecycle ledger 提取 candidate_001 gate 选中的 116 unique events
- 匹配回 events_execution_labeled.csv 获取完整 event 信息
- 按 symbol_month key 加载 1s bar pickle cache
- 按 touch_time 排序后 60/40 time split（train/test）
"""

from __future__ import annotations

import logging
from pathlib import Path

import pandas as pd

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Paths — dynamic_timing 比 v6_gate_d5_tick_backtest.py 多一层目录
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parents[4]

V6_LEDGER_BASE = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_2025m06_2026apr_combo_baseline_short_speed"
    / "union_lifecycle_reentry_window_candidate_001_calendar_holdout"
    / "power0_fixed_1p30"
)

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


# ---------------------------------------------------------------------------
# V6 gate event extraction (复用 v6_gate_d5_tick_backtest.py 逻辑)
# ---------------------------------------------------------------------------
def _extract_v6_gate_entries() -> pd.DataFrame:
    """从 V6 lifecycle ledger 提取 candidate_001 gate 选中的 entry events。"""
    entries: list[dict] = []
    for execute_dir in sorted(V6_LEDGER_BASE.glob("execute_*")):
        for symbol_dir in sorted(execute_dir.iterdir()):
            if not symbol_dir.is_dir():
                continue
            ledger_path = symbol_dir / "lifecycle_ledger.csv"
            if not ledger_path.exists():
                continue
            symbol = symbol_dir.name
            df = pd.read_csv(ledger_path, parse_dates=["time"])
            entry_rows = df[df["type"] != "EXIT"].copy()
            for _, row in entry_rows.iterrows():
                entries.append(
                    {
                        "symbol": symbol,
                        "touch_time": pd.Timestamp(row["time"]),
                        "side": "short" if "SHORT" in str(row["type"]) else "long",
                        "entry_reason": str(row["reason"]),
                    }
                )
    result = pd.DataFrame(entries)
    if not result.empty:
        result["touch_time"] = pd.to_datetime(result["touch_time"], utc=True)
    return result


def _match_events(
    gate_entries: pd.DataFrame, all_events: pd.DataFrame
) -> pd.DataFrame:
    """将 gate entries 匹配回 events_execution_labeled.csv 获取完整 event 信息。

    匹配条件：symbol + side + touch_time 差异 <= 2 秒。
    """
    all_events = all_events.copy()
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)

    matched: list[pd.Series] = []
    for _, entry in gate_entries.iterrows():
        symbol = entry["symbol"]
        touch = entry["touch_time"]
        side = entry["side"]

        candidates = all_events[
            (all_events["symbol"] == symbol)
            & (all_events["side"] == side)
            & ((all_events["touch_time"] - touch).abs() <= pd.Timedelta(seconds=2))
        ]
        if not candidates.empty:
            idx = (candidates["touch_time"] - touch).abs().idxmin()
            matched.append(candidates.loc[idx])

    if not matched:
        return pd.DataFrame()
    return pd.DataFrame(matched).drop_duplicates(subset=["event_id"])


# ---------------------------------------------------------------------------
# 1s bar cache loading (复用 v6_gate_d5_tick_backtest.py 逻辑)
# ---------------------------------------------------------------------------
def _find_bars_cache(symbol: str, event_time: pd.Timestamp) -> Path | None:
    """在 BARS_CACHE_DIR 中查找覆盖 event_time 的 cache 文件."""
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
    """加载指定 symbol 和月份的 1s bar cache.

    优先尝试精确匹配文件名，否则 fallback 到模糊匹配。
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
def load_v6_gate_events() -> pd.DataFrame:
    """提取 V6 gate 选中的 ~116 unique events，匹配回 events_execution_labeled.csv。

    流程：
    1. 从 V6 lifecycle ledger 提取 candidate_001 gate 选中的 entry events
    2. 过滤 Zero-Initial-Reentry 类型
    3. 加载 events_execution_labeled.csv
    4. 匹配 gate entries 到完整 event 信息

    Returns:
        pd.DataFrame: 匹配后的 events，包含 events_execution_labeled.csv 的所有列
    """
    # Step 1: 从 V6 ledger 提取 gate entries
    gate_entries = _extract_v6_gate_entries()

    # Step 2: 只保留 Zero-Initial-Reentry（初始入场）
    initial_entries = gate_entries[
        gate_entries["entry_reason"] == "Zero-Initial-Reentry"
    ].copy()

    # Step 3: 加载完整 events CSV
    all_events = pd.read_csv(EVENTS_CSV)
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)

    # Step 4: 匹配
    matched = _match_events(initial_entries, all_events)

    return matched


def load_bars_cache(events: pd.DataFrame) -> dict[str, pd.DataFrame]:
    """按 symbol_month key 加载 1s bar pickle cache.

    Returns dict keyed by "{symbol}_{YYYYMM}" → DataFrame of 1s bars for that month.
    Only loads months that have events in them.
    """
    # Determine unique (symbol, month) pairs from events' touch_time
    pairs: set[tuple[str, str]] = set()
    for _, row in events.iterrows():
        symbol = str(row["symbol"])
        tt = pd.Timestamp(row["touch_time"])
        if tt.tzinfo is None:
            tt = tt.tz_localize("UTC")
        month_key = tt.strftime("%Y%m")
        pairs.add((symbol, month_key))

    cache: dict[str, pd.DataFrame] = {}
    for symbol, month_key in sorted(pairs):
        key = f"{symbol}_{month_key}"
        month_start = pd.Timestamp(f"{month_key[:4]}-{month_key[4:]}-01", tz="UTC")
        bars = _load_monthly_bars(symbol, month_start)
        if bars is not None:
            cache[key] = bars
            logger.info("Loaded %d bars for %s", len(bars), key)
        else:
            logger.warning("No bars cache found for %s, skipping", key)

    return cache


def time_split_events(
    events: pd.DataFrame, train_ratio: float = 0.6
) -> tuple[pd.DataFrame, pd.DataFrame]:
    """按 touch_time 排序后 60/40 split.

    Args:
        events: DataFrame with touch_time column
        train_ratio: fraction for train set (default 0.6)

    Returns:
        (train_events, test_events) tuple of DataFrames
    """
    sorted_events = events.sort_values("touch_time").reset_index(drop=True)
    split_idx = int(len(sorted_events) * train_ratio)
    train = sorted_events.iloc[:split_idx].reset_index(drop=True)
    test = sorted_events.iloc[split_idx:].reset_index(drop=True)
    return train, test
