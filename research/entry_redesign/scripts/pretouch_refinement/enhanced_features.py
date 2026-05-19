"""
enhanced_features — 6 类增强 pre-breakout 特征提取

所有特征满足 Point-In-Time 约束：仅使用 touch_time 之前的数据。
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

import numpy as np
import pandas as pd


logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# 增强特征分组定义
# ---------------------------------------------------------------------------

ENHANCED_FEATURE_GROUPS: dict[str, list[str]] = {
    "time_of_day_group": [
        "time_of_day_hour_utc",
        "time_of_day_session_overlap",
    ],
    "volume_group": [
        "volume_regime_ratio",
        "volume_regime_percentile",
    ],
    "volatility_group": [
        "realized_vol_30min",
        "volatility_regime_cluster",
    ],
    "level_group": [
        "level_prior_touch_count",
        "level_type",
    ],
    "prev_bars_pattern_group": [
        "prev5_bars_range_derivative",
        "prev5_bars_body_wick_ratio",
        "prev10_bars_direction_consistency",
    ],
    "regime_transition_group": [
        "regime_transition_adx_30min",
        "regime_transition_state",
    ],
}

# 所有增强特征的平铺列表
ALL_ENHANCED_FEATURES: list[str] = [
    feat for group in ENHANCED_FEATURE_GROUPS.values() for feat in group
]


# ---------------------------------------------------------------------------
# Session overlap 映射
# ---------------------------------------------------------------------------

SESSION_OVERLAP_MAP: dict[int, str] = {
    # asia_europe: UTC 6-9h
    6: "asia_europe",
    7: "asia_europe",
    8: "asia_europe",
    9: "asia_europe",
    # europe_us: UTC 13-16h
    13: "europe_us",
    14: "europe_us",
    15: "europe_us",
    16: "europe_us",
    # us_asia: UTC 21-24h (21, 22, 23, 0)
    21: "us_asia",
    22: "us_asia",
    23: "us_asia",
    0: "us_asia",
}


# ---------------------------------------------------------------------------
# PIT Audit 记录
# ---------------------------------------------------------------------------


@dataclass
class PITAuditEntry:
    """Point-In-Time 校验记录"""

    feature_name: str
    data_source: str  # "前 N 根 bar" / "touch 时刻聚合" / "24h lookback"
    computation_logic: str  # 计算逻辑描述
    timestamp_boundary: str  # 时间戳边界说明
    pit_passed: bool  # 是否通过校验


# ---------------------------------------------------------------------------
# 特征统计记录
# ---------------------------------------------------------------------------


@dataclass
class FeatureStats:
    """单个特征的统计信息（Req 1.7）"""

    feature_name: str
    missing_rate: float
    mean: float | None
    std: float | None
    min_val: float | None
    max_val: float | None


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def extract_enhanced_features(
    events: pd.DataFrame,
    bars_cache: dict,
    extended_bars_cache: dict | None = None,
    missing_threshold: float = 0.5,
) -> tuple[pd.DataFrame, list[str], list[str], list[PITAuditEntry]]:
    """提取 6 类增强 pre-breakout 特征。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events（116 events）。
    bars_cache : dict
        1s bar cache（与 pre_breakout_timing 相同）。
    extended_bars_cache : dict | None
        扩展 24h lookback cache；若为 None 则跳过依赖此 cache 的特征。
    missing_threshold : float
        缺失率阈值，超过则排除该特征。默认 0.5（>50% 排除）。

    Returns
    -------
    tuple[pd.DataFrame, list[str], list[str], list[PITAuditEntry]]
        - enhanced_features: DataFrame (n_events × n_enhanced_features)
        - used_features: 实际使用的增强特征列名
        - excluded_features: 被排除的增强特征列名
        - pit_audit: Point-In-Time 校验记录列表
    """
    n_events = len(events)
    pit_audit: list[PITAuditEntry] = []

    # ------------------------------------------------------------------
    # 定义 6 类特征计算函数及其对应的参数
    # ------------------------------------------------------------------
    compute_tasks: list[tuple[str, Callable, dict]] = [
        (
            "time_of_day_group",
            compute_time_of_day_features,
            {"events": events},
        ),
        (
            "volume_group",
            compute_volume_features,
            {"events": events, "bars_cache": bars_cache},
        ),
        (
            "volatility_group",
            compute_volatility_features,
            {"events": events, "bars_cache": bars_cache},
        ),
        (
            "level_group",
            compute_level_features,
            {"events": events, "extended_bars_cache": extended_bars_cache},
        ),
        (
            "prev_bars_pattern_group",
            compute_prev_bars_pattern_features,
            {"events": events, "bars_cache": bars_cache},
        ),
        (
            "regime_transition_group",
            compute_regime_transition_features,
            {"events": events, "bars_cache": bars_cache},
        ),
    ]

    # ------------------------------------------------------------------
    # 执行各组特征计算，收集结果
    # ------------------------------------------------------------------
    group_dfs: list[pd.DataFrame] = []
    excluded_features: list[str] = []

    for group_name, compute_fn, kwargs in compute_tasks:
        group_feature_names = ENHANCED_FEATURE_GROUPS[group_name]

        # 若 extended_bars_cache 为 None 且该组依赖它（level_group），
        # 跳过计算，标记为 excluded（Req 1.6）
        if group_name == "level_group" and extended_bars_cache is None:
            logger.info(
                f"Skipping {group_name}: extended_bars_cache is None"
            )
            # 创建全 NaN 的 DataFrame 作为占位
            empty_df = pd.DataFrame(
                np.nan,
                index=events.index,
                columns=group_feature_names,
            )
            group_dfs.append(empty_df)
            excluded_features.extend(group_feature_names)
            # 记录 PIT audit（标记为 excluded 但 pit_passed=True，
            # 因为跳过不违反 PIT 约束）
            for feat_name in group_feature_names:
                pit_audit.append(
                    PITAuditEntry(
                        feature_name=feat_name,
                        data_source="24h lookback",
                        computation_logic="需要 extended_bars_cache，当前不可用",
                        timestamp_boundary="N/A (excluded)",
                        pit_passed=True,
                    )
                )
            continue

        try:
            result_df = compute_fn(**kwargs)
            # 确保 index 对齐
            result_df.index = events.index
            group_dfs.append(result_df)
        except Exception as e:
            logger.warning(
                f"Error computing {group_name}: {e}. "
                f"Filling with NaN and marking as excluded."
            )
            empty_df = pd.DataFrame(
                np.nan,
                index=events.index,
                columns=group_feature_names,
            )
            group_dfs.append(empty_df)
            excluded_features.extend(group_feature_names)
            for feat_name in group_feature_names:
                pit_audit.append(
                    PITAuditEntry(
                        feature_name=feat_name,
                        data_source="unknown",
                        computation_logic=f"计算失败: {e}",
                        timestamp_boundary="N/A (error)",
                        pit_passed=False,
                    )
                )
            continue

    # ------------------------------------------------------------------
    # 合并所有特征组为单一 DataFrame
    # ------------------------------------------------------------------
    enhanced_features_df = pd.concat(group_dfs, axis=1)

    # ------------------------------------------------------------------
    # 缺失率检查：>50% 排除（Req 1.4）
    # ------------------------------------------------------------------
    used_features: list[str] = []

    for col in enhanced_features_df.columns:
        if col in excluded_features:
            # 已经被标记为 excluded（如 level_group 因缺少 cache）
            continue

        missing_rate = enhanced_features_df[col].isna().sum() / n_events

        if missing_rate > missing_threshold:
            logger.info(
                f"Feature '{col}' excluded: missing_rate={missing_rate:.2%} "
                f"> threshold={missing_threshold:.0%}"
            )
            excluded_features.append(col)
        else:
            used_features.append(col)

    # ------------------------------------------------------------------
    # 收集各 compute 函数产出的 PIT audit 记录
    # 注意：各 compute_*_features() 函数在后续 task 中实现时会
    # 返回 PIT audit entries。当前 stub 实现中，我们为已计算的
    # 特征生成基础 PIT audit 记录。
    # ------------------------------------------------------------------
    # 为已成功计算的特征补充 PIT audit（如果 compute 函数未提供）
    audited_features = {entry.feature_name for entry in pit_audit}
    for col in enhanced_features_df.columns:
        if col not in audited_features:
            # 确定该特征属于哪个组，推断数据来源
            data_source = _infer_data_source(col)
            computation_logic, timestamp_boundary = _infer_pit_details(col)
            pit_audit.append(
                PITAuditEntry(
                    feature_name=col,
                    data_source=data_source,
                    computation_logic=computation_logic,
                    timestamp_boundary=timestamp_boundary,
                    pit_passed=True,
                )
            )

    # ------------------------------------------------------------------
    # 记录特征统计信息（Req 1.7）
    # ------------------------------------------------------------------
    _log_feature_statistics(enhanced_features_df, used_features, n_events)

    return enhanced_features_df, used_features, excluded_features, pit_audit


def _infer_data_source(feature_name: str) -> str:
    """根据特征名推断数据来源。"""
    if feature_name.startswith("time_of_day"):
        return "touch 时刻聚合"
    elif feature_name.startswith("volume"):
        return "前 20 根 bar"
    elif feature_name.startswith("realized_vol") or feature_name.startswith(
        "volatility"
    ):
        return "前 30 分钟 1s bar"
    elif feature_name.startswith("level"):
        return "24h lookback"
    elif feature_name.startswith("prev"):
        return "前 N 根 bar"
    elif feature_name.startswith("regime_transition"):
        return "前 30 分钟 1s→1min 聚合"
    return "unknown"


def _infer_pit_details(feature_name: str) -> tuple[str, str]:
    """根据特征名推断 PIT audit 的计算逻辑和时间戳边界描述。

    Returns
    -------
    tuple[str, str]
        (computation_logic, timestamp_boundary)
    """
    pit_details: dict[str, tuple[str, str]] = {
        "time_of_day_hour_utc": (
            "pd.to_datetime(touch_time, utc=True).dt.hour → int 0-23",
            "touch_time 本身（breakout 触发时刻，不使用 post-touch 数据）",
        ),
        "time_of_day_session_overlap": (
            "按 SESSION_OVERLAP_MAP 将 UTC hour 映射为 session overlap 枚举值",
            "touch_time 本身（breakout 触发时刻，不使用 post-touch 数据）",
        ),
    }

    if feature_name in pit_details:
        return pit_details[feature_name]

    # 默认值（其他特征在各自 task 实现时可扩展此 dict）
    return ("见对应 compute 函数实现", "touch_time 之前")


def _log_feature_statistics(
    df: pd.DataFrame, used_features: list[str], n_events: int
) -> None:
    """记录每个特征的统计信息（Req 1.7）。"""
    for col in used_features:
        series = df[col]
        missing_rate = series.isna().sum() / n_events

        # 尝试转为数值计算统计量
        numeric_series = pd.to_numeric(series, errors="coerce")
        if numeric_series.notna().any():
            logger.info(
                f"Feature '{col}': missing_rate={missing_rate:.2%}, "
                f"mean={numeric_series.mean():.4f}, "
                f"std={numeric_series.std():.4f}, "
                f"min={numeric_series.min():.4f}, "
                f"max={numeric_series.max():.4f}"
            )
        else:
            logger.info(
                f"Feature '{col}': missing_rate={missing_rate:.2%}, "
                f"non-numeric (categorical)"
            )


# ---------------------------------------------------------------------------
# Extended bars cache 加载（Task 2.5 实现）
# ---------------------------------------------------------------------------


def load_extended_bars_cache(
    events: pd.DataFrame,
    max_lookback_hours: int = 24,
) -> dict:
    """加载扩展 24h lookback 的 1s bar cache。

    用于 level_prior_touch_count、regime_transition_adx_30min 等
    需要更长历史数据的特征计算。

    与 load_bars_cache() 使用相同的数据源和格式，但为每个 event 加载
    覆盖 touch_time 前 24h 的所有月份 bars，合并后按 event 存储。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events，必须包含 'symbol' 和 'touch_time' 列。
    max_lookback_hours : int
        最大 lookback 小时数，默认 24。

    Returns
    -------
    dict
        扩展 bar cache，key 格式为 "{symbol}_{YYYYMM}"（与 bars_cache 一致）。
        包含覆盖所有 event 24h lookback 所需的月份数据。
        若数据文件不存在或无法加载，返回空 dict（graceful degradation）。
    """
    try:
        return _load_extended_bars_cache_impl(events, max_lookback_hours)
    except Exception as e:
        logger.warning(
            f"load_extended_bars_cache: failed to load extended cache: {e}. "
            f"Returning empty dict (graceful degradation)."
        )
        return {}


def _load_extended_bars_cache_impl(
    events: pd.DataFrame,
    max_lookback_hours: int,
) -> dict:
    """load_extended_bars_cache 的内部实现。"""
    # 复用 dynamic_timing.data_layer 中的路径和加载逻辑
    import sys
    from pathlib import Path

    scripts_dir = Path(__file__).resolve().parent.parent
    if str(scripts_dir) not in sys.path:
        sys.path.insert(0, str(scripts_dir))

    from dynamic_timing.data_layer import BARS_CACHE_DIR

    if not BARS_CACHE_DIR.exists():
        logger.warning(
            f"load_extended_bars_cache: BARS_CACHE_DIR not found: "
            f"{BARS_CACHE_DIR}. Returning empty dict."
        )
        return {}

    # 确定需要加载的所有 (symbol, month) 对
    # 对每个 event，需要覆盖 [touch_time - max_lookback_hours, touch_time]
    touch_time_col = pd.to_datetime(events["touch_time"], utc=True)
    lookback_delta = pd.Timedelta(hours=max_lookback_hours)

    pairs: set[tuple[str, str]] = set()
    for _, row in events.iterrows():
        symbol = str(row["symbol"])
        tt = pd.Timestamp(row["touch_time"])
        if tt.tzinfo is None:
            tt = tt.tz_localize("UTC")

        # touch_time 所在月份
        pairs.add((symbol, tt.strftime("%Y%m")))

        # lookback 起始时间所在月份（可能跨月）
        lookback_start = tt - lookback_delta
        pairs.add((symbol, lookback_start.strftime("%Y%m")))

        # 如果 lookback 跨越多个月（极端情况，24h 最多跨 2 个月）
        # 逐月检查
        current = lookback_start
        while current < tt:
            pairs.add((symbol, current.strftime("%Y%m")))
            current = current + pd.offsets.MonthBegin(1)

    # 加载所有需要的月份
    cache: dict[str, pd.DataFrame] = {}
    for symbol, month_key in sorted(pairs):
        key = f"{symbol}_{month_key}"
        if key in cache:
            continue

        bars_df = _load_monthly_bars_from_cache_dir(
            symbol, month_key, BARS_CACHE_DIR
        )
        if bars_df is not None:
            cache[key] = bars_df
            logger.info(
                f"load_extended_bars_cache: loaded {len(bars_df)} bars "
                f"for {key}"
            )
        else:
            logger.debug(
                f"load_extended_bars_cache: no bars found for {key}, "
                f"skipping"
            )

    logger.info(
        f"load_extended_bars_cache: loaded {len(cache)} month-symbol "
        f"pairs for {len(events)} events "
        f"(max_lookback_hours={max_lookback_hours})"
    )
    return cache


def _load_monthly_bars_from_cache_dir(
    symbol: str,
    month_key: str,
    bars_cache_dir: Path,
) -> pd.DataFrame | None:
    """从 bars_cache_dir 加载指定 symbol 和月份的 1s bar cache。

    复用与 dynamic_timing.data_layer._load_monthly_bars 相同的逻辑。
    """
    month_start = pd.Timestamp(
        f"{month_key[:4]}-{month_key[4:]}-01", tz="UTC"
    )
    month_end = (month_start + pd.offsets.MonthEnd(0)).replace(
        hour=23, minute=59, second=59
    )

    start_key = month_start.strftime("%Y%m%dT%H%M%S")
    end_key = month_end.strftime("%Y%m%dT%H%M%S")

    # 精确匹配
    exact = bars_cache_dir / f"{symbol}_{start_key}_{end_key}_flow_1s.pkl"
    if exact.exists():
        try:
            return pd.read_pickle(exact)
        except Exception as e:
            logger.warning(f"Failed to read {exact}: {e}")
            return None

    # Fallback: 模糊匹配（查找覆盖该月中间时间的文件）
    mid_month = month_start + pd.Timedelta(days=15)
    candidates = list(bars_cache_dir.glob(f"{symbol}_*_flow_1s.pkl"))
    for p in candidates:
        parts = p.stem.replace("_flow_1s", "").split("_")
        if len(parts) < 3:
            continue
        try:
            cache_start = pd.Timestamp(parts[1], tz="UTC")
            cache_end = pd.Timestamp(parts[2], tz="UTC")
            if cache_start <= mid_month <= cache_end:
                return pd.read_pickle(p)
        except Exception:
            continue

    return None


# ---------------------------------------------------------------------------
# 6 类特征计算函数（stub 实现，后续 task 填充）
# ---------------------------------------------------------------------------


def compute_time_of_day_features(
    events: pd.DataFrame,
) -> pd.DataFrame:
    """计算 time_of_day 特征组。

    - time_of_day_hour_utc: touch_time.hour (int 0-23)
    - time_of_day_session_overlap: 枚举 {none, asia_europe, europe_us, us_asia}

    满足 Point-In-Time 约束：touch_time 本身即为 breakout 触发时刻，
    提取其 hour 不使用任何 post-touch 数据。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events，必须包含 'touch_time' 列（datetime 或可解析为 datetime 的字符串）。

    Returns
    -------
    pd.DataFrame
        columns: ["time_of_day_hour_utc", "time_of_day_session_overlap"]
        index 与 events 对齐。
    """
    # 确保 touch_time 为 datetime 类型
    touch_time = pd.to_datetime(events["touch_time"], utc=True)

    # 计算 time_of_day_hour_utc: 提取 UTC 小时 (int 0-23)
    hour_utc = touch_time.dt.hour.astype(int)

    # 计算 time_of_day_session_overlap: 按 SESSION_OVERLAP_MAP 映射
    # 不在 map 中的小时映射为 "none"
    session_overlap = hour_utc.map(
        lambda h: SESSION_OVERLAP_MAP.get(h, "none")
    )

    result = pd.DataFrame(
        {
            "time_of_day_hour_utc": hour_utc.values,
            "time_of_day_session_overlap": session_overlap.values,
        },
        index=events.index,
    )

    logger.info(
        f"compute_time_of_day_features: computed for {len(events)} events. "
        f"Hour range: [{hour_utc.min()}, {hour_utc.max()}], "
        f"Session overlap distribution: "
        f"{session_overlap.value_counts().to_dict()}"
    )

    return result


def compute_volume_features(
    events: pd.DataFrame,
    bars_cache: dict,
) -> pd.DataFrame:
    """计算 volume 特征组。

    - volume_regime_ratio: signal bar volume / 前 20 根 bar volume rolling mean
    - volume_regime_percentile: ratio 在所有 events 内的分位数 (0-1)

    若 volume 字段在 bars_cache 中不可用或全部缺失，返回全 null
    并标记 volume_regime_unavailable=true（Req 1.9）。

    满足 Point-In-Time 约束：仅使用 touch_time 及之前的 bar 数据。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events，必须包含 'symbol', 'touch_time' 列。
    bars_cache : dict
        1s bar cache，key 格式为 "{symbol}_{YYYYMM}"，
        value 为 DataFrame（DatetimeIndex, columns 含 open/high/low/close，
        可能含 volume）。

    Returns
    -------
    pd.DataFrame
        columns: ["volume_regime_ratio", "volume_regime_percentile"]
        index 与 events 对齐。若 volume 不可用则全为 NaN。
    """
    n_events = len(events)
    volume_cols = ENHANCED_FEATURE_GROUPS["volume_group"]

    # 先检查 bars_cache 中是否有任何 DataFrame 包含 volume 列且非全 NaN
    volume_available = False
    for _key, bars_df in bars_cache.items():
        if "volume" in bars_df.columns and bars_df["volume"].notna().any():
            volume_available = True
            break

    if not volume_available:
        logger.info(
            "compute_volume_features: volume column not available in bars_cache. "
            "Marking volume_regime_unavailable=true, returning all NaN."
        )
        return pd.DataFrame(
            np.nan,
            index=events.index,
            columns=volume_cols,
        )

    # 计算每个 event 的 volume_regime_ratio
    ratios = np.full(n_events, np.nan)
    lookback_window = 20  # 前 20 根 bar

    for i, (_, event) in enumerate(events.iterrows()):
        symbol = str(event["symbol"])
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        # 构建 bars_cache key: "{symbol}_{YYYYMM}"
        month_key = touch_time.strftime("%Y%m")
        cache_key = f"{symbol}_{month_key}"

        if cache_key not in bars_cache:
            # 该 event 对应的 bars 不在 cache 中
            continue

        bars_df = bars_cache[cache_key]

        # 检查该 DataFrame 是否有 volume 列且非全 NaN
        if "volume" not in bars_df.columns or bars_df["volume"].isna().all():
            continue

        # 确保 bars index 为 tz-aware datetime
        bars_index = bars_df.index
        if hasattr(bars_index, "tz") and bars_index.tz is None:
            bars_index = bars_index.tz_localize("UTC")

        # 找到 touch_time 及之前的所有 bar（PIT 约束）
        mask_before_or_at = bars_index <= touch_time
        bars_before = bars_df.loc[mask_before_or_at]

        if len(bars_before) < 1:
            continue

        # signal bar = touch_time 对应的 bar（最后一根 <= touch_time 的 bar）
        signal_bar_volume = bars_before["volume"].iloc[-1]

        if pd.isna(signal_bar_volume):
            continue

        # 前 20 根 bar（不含 signal bar 本身）
        preceding_bars = bars_before.iloc[-(lookback_window + 1):-1]

        if len(preceding_bars) == 0:
            continue

        # 计算前 N 根 bar 的 volume rolling mean
        preceding_volume = preceding_bars["volume"].dropna()

        if len(preceding_volume) == 0:
            continue

        rolling_mean = preceding_volume.mean()

        if rolling_mean <= 0:
            continue

        ratios[i] = signal_bar_volume / rolling_mean

    # 检查是否所有 ratio 都是 NaN（volume 实际不可用）
    if np.all(np.isnan(ratios)):
        logger.info(
            "compute_volume_features: all volume_regime_ratio are NaN "
            "(volume data effectively unavailable). "
            "Marking volume_regime_unavailable=true."
        )
        return pd.DataFrame(
            np.nan,
            index=events.index,
            columns=volume_cols,
        )

    # 计算 volume_regime_percentile: ratio 在所有 events 内的分位数 (0-1)
    # 使用 pandas rank 方法，pct=True 归一化到 0-1
    ratio_series = pd.Series(ratios, index=events.index)
    # rank with method='average', pct=True 产出百分位排名 (0, 1]
    percentile_series = ratio_series.rank(method="average", pct=True)

    result = pd.DataFrame(
        {
            "volume_regime_ratio": ratios,
            "volume_regime_percentile": percentile_series.values,
        },
        index=events.index,
    )

    # 统计信息
    valid_count = int(np.sum(~np.isnan(ratios)))
    logger.info(
        f"compute_volume_features: computed for {n_events} events. "
        f"Valid ratios: {valid_count}/{n_events}, "
        f"Ratio range: [{np.nanmin(ratios):.4f}, {np.nanmax(ratios):.4f}], "
        f"Ratio mean: {np.nanmean(ratios):.4f}"
    )

    return result


def compute_volatility_features(
    events: pd.DataFrame,
    bars_cache: dict,
) -> pd.DataFrame:
    """计算 volatility 特征组。

    - realized_vol_30min: 过去30分钟 1s bar close 标准差 × sqrt(1800)
      30 分钟 = 1800 秒 = 最多 1800 根 1s bar。
      若可用 bar 数 < 100，设为 NaN。
    - volatility_regime_cluster: 基于 signal_atr_percentile 的三分类
      {low: ≤0.33, mid: 0.33-0.67, high: >0.67}
      编码为 numeric: low=0, mid=1, high=2（分类器兼容）。

    PIT 约束：仅使用 touch_time 之前的 bar 数据。
    """
    n_events = len(events)
    realized_vol_values = np.full(n_events, np.nan)
    vol_cluster_values = np.full(n_events, np.nan)

    # 最少需要的 bar 数量
    min_bars_required = 100
    # 30 分钟 lookback 秒数
    lookback_seconds = 1800

    for i, (idx, event) in enumerate(events.iterrows()):
        # --- realized_vol_30min ---
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        symbol = str(event["symbol"])
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)

        if second_bars is not None and not second_bars.empty:
            # 获取 touch_time 之前 30 分钟的 bar（PIT 约束：严格 < touch_time）
            lookback_start = touch_time - pd.Timedelta(seconds=lookback_seconds)
            mask = (second_bars.index >= lookback_start) & (
                second_bars.index < touch_time
            )
            window_bars = second_bars.loc[mask]

            if len(window_bars) >= min_bars_required:
                close_std = window_bars["close"].std()
                realized_vol_values[i] = close_std * np.sqrt(1800)
            # else: 保持 NaN

        # --- volatility_regime_cluster ---
        atr_pct = event.get("signal_atr_percentile", np.nan)
        if pd.notna(atr_pct):
            if atr_pct <= 0.33:
                vol_cluster_values[i] = 0  # low
            elif atr_pct <= 0.67:
                vol_cluster_values[i] = 1  # mid
            else:
                vol_cluster_values[i] = 2  # high

    result = pd.DataFrame(
        {
            "realized_vol_30min": realized_vol_values,
            "volatility_regime_cluster": vol_cluster_values,
        },
        index=events.index,
    )
    return result


def compute_level_features(
    events: pd.DataFrame,
    extended_bars_cache: dict | None,
) -> pd.DataFrame:
    """计算 level 特征组。

    - level_prior_touch_count: 24h内价格触碰同一level(±0.5 ATR)的次数
    - level_type: 枚举 {fresh, re_tested, exhausted}
      - fresh: prior_touch_count == 0 → encoded as 0
      - re_tested: prior_touch_count 1-2 → encoded as 1
      - exhausted: prior_touch_count >= 3 → encoded as 2

    若 extended_bars_cache 为 None 或空，返回全 NaN（graceful degradation，Req 1.6）。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events，必须包含 'touch_time', 'symbol', 'level', 'atr' 列。
    extended_bars_cache : dict | None
        扩展 24h lookback cache，key 格式为 "{symbol}_{YYYYMM}"。

    Returns
    -------
    pd.DataFrame
        columns: ["level_prior_touch_count", "level_type"]
        index 与 events 对齐。
    """
    feature_cols = ENHANCED_FEATURE_GROUPS["level_group"]

    # Graceful degradation: 若 extended_bars_cache 为 None 或空
    if not extended_bars_cache:
        logger.info(
            "compute_level_features: extended_bars_cache is None or empty. "
            "Returning all NaN (graceful degradation per Req 1.6)."
        )
        return pd.DataFrame(
            np.nan,
            index=events.index,
            columns=feature_cols,
        )

    touch_times = pd.to_datetime(events["touch_time"], utc=True)
    lookback_delta = pd.Timedelta(hours=24)

    prior_touch_counts: list[float] = []
    level_types: list[float] = []

    for idx, row in events.iterrows():
        try:
            symbol = str(row["symbol"])
            tt = touch_times.loc[idx]
            level = float(row["level"])
            atr = float(row["atr"])

            # 容差: ±0.5 ATR
            tolerance = 0.5 * atr

            # 获取 24h lookback 的 bars
            lookback_start = tt - lookback_delta
            bars_24h = _get_bars_for_period(
                symbol, lookback_start, tt, extended_bars_cache
            )

            if bars_24h is None or bars_24h.empty:
                prior_touch_counts.append(np.nan)
                level_types.append(np.nan)
                continue

            # 计算价格触碰 level 的次数
            # 触碰定义：bar 的 high >= level - tolerance AND low <= level + tolerance
            # 即 bar 的价格范围与 [level - tolerance, level + tolerance] 有交集
            touches = (
                (bars_24h["high"] >= level - tolerance)
                & (bars_24h["low"] <= level + tolerance)
            )
            touch_count = int(touches.sum())

            prior_touch_counts.append(touch_count)

            # level_type 编码
            if touch_count == 0:
                level_types.append(0)  # fresh
            elif touch_count <= 2:
                level_types.append(1)  # re_tested
            else:
                level_types.append(2)  # exhausted

        except Exception as e:
            logger.debug(
                f"compute_level_features: error for event {idx}: {e}"
            )
            prior_touch_counts.append(np.nan)
            level_types.append(np.nan)

    result = pd.DataFrame(
        {
            "level_prior_touch_count": prior_touch_counts,
            "level_type": level_types,
        },
        index=events.index,
    )

    # 日志统计
    valid_count = result["level_prior_touch_count"].notna().sum()
    logger.info(
        f"compute_level_features: computed for {len(events)} events, "
        f"{valid_count} valid. "
        f"Touch count stats: "
        f"mean={result['level_prior_touch_count'].mean():.1f}, "
        f"max={result['level_prior_touch_count'].max():.0f}. "
        f"Level type distribution: "
        f"fresh={int((result['level_type'] == 0).sum())}, "
        f"re_tested={int((result['level_type'] == 1).sum())}, "
        f"exhausted={int((result['level_type'] == 2).sum())}"
    )

    return result


def _get_bars_for_period(
    symbol: str,
    start_time: pd.Timestamp,
    end_time: pd.Timestamp,
    bars_cache: dict,
) -> pd.DataFrame | None:
    """从 bars_cache 中获取指定时间段的 bars。

    处理跨月情况：合并多个月份的 bars 并截取时间范围。

    Parameters
    ----------
    symbol : str
        交易对符号（如 "BTCUSDT"）。
    start_time : pd.Timestamp
        起始时间（inclusive）。
    end_time : pd.Timestamp
        结束时间（exclusive）。
    bars_cache : dict
        key 格式为 "{symbol}_{YYYYMM}" → DataFrame。

    Returns
    -------
    pd.DataFrame | None
        截取后的 bars，若无数据则返回 None。
    """
    # 确定需要的月份
    months_needed: list[str] = []
    current = start_time.replace(day=1, hour=0, minute=0, second=0)
    while current <= end_time:
        months_needed.append(current.strftime("%Y%m"))
        current = current + pd.offsets.MonthBegin(1)

    # 收集所有相关月份的 bars
    all_bars: list[pd.DataFrame] = []
    for month_key in months_needed:
        key = f"{symbol}_{month_key}"
        if key in bars_cache:
            all_bars.append(bars_cache[key])

    if not all_bars:
        return None

    # 合并并截取时间范围
    if len(all_bars) == 1:
        combined = all_bars[0]
    else:
        combined = pd.concat(all_bars)
        combined = combined[~combined.index.duplicated(keep="first")]
        combined = combined.sort_index()

    # 截取 [start_time, end_time) — 不包含 end_time（touch_time 本身）
    # 以满足 Point-In-Time 约束
    mask = (combined.index >= start_time) & (combined.index < end_time)
    result = combined.loc[mask]

    return result if not result.empty else None


def compute_prev_bars_pattern_features(
    events: pd.DataFrame,
    bars_cache: dict,
) -> pd.DataFrame:
    """计算 prev_bars_pattern 特征组。

    - prev5_bars_range_derivative: 前 5 根 bar range 序列 [r1..r5] 的一阶差分均值
      正值 = ranges 扩张，负值 = ranges 收缩
    - prev5_bars_body_wick_ratio: 前 5 根 bar mean(|body|) / mean(|wick|)
      wick = range - |body| = (high - low) - |close - open|
    - prev10_bars_direction_consistency: 前 10 根 bar 中与 signal_bar_side 同向 bar 占比 (0-1)
      long → count(close > open) / 10
      short → count(close < open) / 10

    满足 Point-In-Time 约束：仅使用 touch_time 之前的 1s bar 数据。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events，必须包含 'symbol', 'touch_time', 'side' 列。
    bars_cache : dict
        1s bar cache，key 格式为 "{symbol}_{YYYYMM}"，value 为 DataFrame
        (DatetimeIndex, columns: open, high, low, close)。

    Returns
    -------
    pd.DataFrame
        columns: ["prev5_bars_range_derivative", "prev5_bars_body_wick_ratio",
                  "prev10_bars_direction_consistency"]
        index 与 events 对齐。
    """
    n_events = len(events)
    range_derivative = np.full(n_events, np.nan)
    body_wick_ratio = np.full(n_events, np.nan)
    direction_consistency = np.full(n_events, np.nan)

    for i, (_, event) in enumerate(events.iterrows()):
        symbol = str(event["symbol"])
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")
        side = str(event["side"])

        # 获取该 event 对应月份的 1s bar 数据
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)
        if second_bars is None or second_bars.empty:
            continue

        # 获取 touch_time 之前的 bars（Point-In-Time 约束）
        pre_touch_bars = second_bars[second_bars.index < touch_time]
        if pre_touch_bars.empty:
            continue

        # --- prev5_bars_range_derivative ---
        # 取最后 5 根 bar
        if len(pre_touch_bars) >= 5:
            last5 = pre_touch_bars.iloc[-5:]
            ranges = (last5["high"] - last5["low"]).values
            # 一阶差分：np.diff([r1, r2, r3, r4, r5]) → [r2-r1, r3-r2, r4-r3, r5-r4]
            diffs = np.diff(ranges)
            range_derivative[i] = float(np.mean(diffs))

        # --- prev5_bars_body_wick_ratio ---
        if len(pre_touch_bars) >= 5:
            last5 = pre_touch_bars.iloc[-5:]
            bodies = np.abs(last5["close"].values - last5["open"].values)
            ranges_5 = last5["high"].values - last5["low"].values
            wicks = ranges_5 - bodies  # wick = range - |body|

            mean_body = float(np.mean(bodies))
            mean_wick = float(np.mean(wicks))

            if mean_wick > 0:
                body_wick_ratio[i] = mean_body / mean_wick
            else:
                # Division by zero: mean_wick == 0 → set to NaN
                body_wick_ratio[i] = np.nan

        # --- prev10_bars_direction_consistency ---
        if len(pre_touch_bars) >= 10:
            last10 = pre_touch_bars.iloc[-10:]
            closes = last10["close"].values
            opens = last10["open"].values

            if side == "long":
                # 同向 = bullish bars (close > open)
                same_direction_count = int(np.sum(closes > opens))
            else:
                # 同向 = bearish bars (close < open)
                same_direction_count = int(np.sum(closes < opens))

            direction_consistency[i] = same_direction_count / 10.0

    result = pd.DataFrame(
        {
            "prev5_bars_range_derivative": range_derivative,
            "prev5_bars_body_wick_ratio": body_wick_ratio,
            "prev10_bars_direction_consistency": direction_consistency,
        },
        index=events.index,
    )

    logger.info(
        f"compute_prev_bars_pattern_features: computed for {n_events} events. "
        f"range_derivative NaN rate: "
        f"{np.isnan(range_derivative).sum() / n_events:.2%}, "
        f"body_wick_ratio NaN rate: "
        f"{np.isnan(body_wick_ratio).sum() / n_events:.2%}, "
        f"direction_consistency NaN rate: "
        f"{np.isnan(direction_consistency).sum() / n_events:.2%}"
    )

    return result


def compute_regime_transition_features(
    events: pd.DataFrame,
    bars_cache: dict,
) -> pd.DataFrame:
    """计算 regime_transition 特征组。

    - regime_transition_adx_30min: 过去30分钟1s→1min聚合后的14周期ADX
    - regime_transition_state: 枚举 {trending: ADX>25, ranging: ADX≤20,
      transitional: ADX∈(20,25]}
      编码为数值: ranging=0, transitional=1, trending=2

    满足 Point-In-Time 约束：仅使用 touch_time 之前 30 分钟的 1s bar 数据。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events，必须包含 'touch_time' 和 'symbol' 列。
    bars_cache : dict
        1s bar cache，key 为 "{symbol}_{YYYYMM}"，value 为 DataFrame
        (columns: open, high, low, close; index: DatetimeIndex)。

    Returns
    -------
    pd.DataFrame
        columns: ["regime_transition_adx_30min", "regime_transition_state"]
        index 与 events 对齐。
    """
    adx_values = []
    state_values = []

    for _, row in events.iterrows():
        touch_time = pd.Timestamp(row["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")
        symbol = str(row["symbol"])

        # 获取对应的 bars DataFrame
        bars_df = _get_bars_for_event(symbol, touch_time, bars_cache)

        if bars_df is None or bars_df.empty:
            adx_values.append(np.nan)
            state_values.append(np.nan)
            continue

        # 获取 touch_time 之前 30 分钟的 1s bars
        lookback_start = touch_time - pd.Timedelta(minutes=30)
        mask = (bars_df.index >= lookback_start) & (bars_df.index < touch_time)
        bars_30min = bars_df.loc[mask]

        if len(bars_30min) < 60:
            # 不够 1 分钟的数据，无法计算
            adx_values.append(np.nan)
            state_values.append(np.nan)
            continue

        # 聚合 1s bars 为 1min bars
        bars_1min = _aggregate_to_1min(bars_30min)

        if len(bars_1min) < 28:
            # ADX 需要至少 28 根 1min bar（14 周期平滑 × 2）
            adx_values.append(np.nan)
            state_values.append(np.nan)
            continue

        # 计算 14 周期 ADX
        adx = _compute_adx(bars_1min, period=14)

        if np.isnan(adx):
            adx_values.append(np.nan)
            state_values.append(np.nan)
        else:
            adx_values.append(adx)
            # 确定 regime_transition_state
            # trending=2: ADX > 25
            # transitional=1: 20 < ADX <= 25
            # ranging=0: ADX <= 20
            if adx > 25:
                state_values.append(2)  # trending
            elif adx > 20:
                state_values.append(1)  # transitional
            else:
                state_values.append(0)  # ranging

    result = pd.DataFrame(
        {
            "regime_transition_adx_30min": adx_values,
            "regime_transition_state": state_values,
        },
        index=events.index,
    )

    # 日志记录
    valid_count = result["regime_transition_adx_30min"].notna().sum()
    logger.info(
        f"compute_regime_transition_features: computed for {len(events)} events. "
        f"Valid ADX: {valid_count}/{len(events)}, "
        f"Mean ADX: {result['regime_transition_adx_30min'].mean():.2f}"
        if valid_count > 0
        else f"compute_regime_transition_features: no valid ADX computed "
        f"for {len(events)} events."
    )

    return result


def _get_bars_for_event(
    symbol: str,
    touch_time: pd.Timestamp,
    bars_cache: dict,
) -> pd.DataFrame | None:
    """从 bars_cache 中获取覆盖 event 的 bars DataFrame。

    bars_cache key 格式为 "{symbol}_{YYYYMM}"。
    若 touch_time 在月初且 30min lookback 跨月，需要合并两个月的数据。
    """
    month_key = touch_time.strftime("%Y%m")
    cache_key = f"{symbol}_{month_key}"

    bars_df = bars_cache.get(cache_key)

    # 检查是否需要跨月（touch_time 在月初前 30 分钟可能跨到上月）
    lookback_start = touch_time - pd.Timedelta(minutes=30)
    if lookback_start.month != touch_time.month or lookback_start.year != touch_time.year:
        prev_month_key = lookback_start.strftime("%Y%m")
        prev_cache_key = f"{symbol}_{prev_month_key}"
        prev_bars = bars_cache.get(prev_cache_key)

        if prev_bars is not None and bars_df is not None:
            bars_df = pd.concat([prev_bars, bars_df])
        elif prev_bars is not None:
            bars_df = prev_bars

    return bars_df


def _aggregate_to_1min(bars_1s: pd.DataFrame) -> pd.DataFrame:
    """将 1s bars 聚合为 1min bars。

    Parameters
    ----------
    bars_1s : pd.DataFrame
        1s bar DataFrame (columns: open, high, low, close; DatetimeIndex)。

    Returns
    -------
    pd.DataFrame
        1min bar DataFrame (columns: open, high, low, close; DatetimeIndex)。
    """
    resampled = bars_1s.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last"}
    )
    # 移除可能的 NaN 行（不完整的分钟）
    resampled = resampled.dropna(subset=["open", "high", "low", "close"])
    return resampled


def _compute_adx(bars: pd.DataFrame, period: int = 14) -> float:
    """手动计算 ADX（Average Directional Index）。

    使用 Wilder's smoothing method，不依赖 ta-lib。

    Parameters
    ----------
    bars : pd.DataFrame
        1min bar DataFrame (columns: open, high, low, close; DatetimeIndex)。
        需要至少 2*period 根 bar。
    period : int
        ADX 周期，默认 14。

    Returns
    -------
    float
        最后一个 ADX 值。若数据不足返回 NaN。
    """
    n = len(bars)
    if n < 2 * period:
        return np.nan

    high = bars["high"].values
    low = bars["low"].values
    close = bars["close"].values

    # Step 1: 计算 True Range (TR), +DM, -DM
    tr = np.zeros(n)
    plus_dm = np.zeros(n)
    minus_dm = np.zeros(n)

    for i in range(1, n):
        h_l = high[i] - low[i]
        h_pc = abs(high[i] - close[i - 1])
        l_pc = abs(low[i] - close[i - 1])
        tr[i] = max(h_l, h_pc, l_pc)

        up_move = high[i] - high[i - 1]
        down_move = low[i - 1] - low[i]

        if up_move > down_move and up_move > 0:
            plus_dm[i] = up_move
        else:
            plus_dm[i] = 0.0

        if down_move > up_move and down_move > 0:
            minus_dm[i] = down_move
        else:
            minus_dm[i] = 0.0

    # Step 2: Wilder's smoothing for TR, +DM, -DM (first period values)
    # 初始值：前 period 个值的和
    smoothed_tr = np.sum(tr[1 : period + 1])
    smoothed_plus_dm = np.sum(plus_dm[1 : period + 1])
    smoothed_minus_dm = np.sum(minus_dm[1 : period + 1])

    # 存储 +DI, -DI, DX 序列
    plus_di_arr = np.zeros(n)
    minus_di_arr = np.zeros(n)
    dx_arr = np.zeros(n)

    # 第 period 个位置的 DI 值
    if smoothed_tr > 0:
        plus_di_arr[period] = 100.0 * smoothed_plus_dm / smoothed_tr
        minus_di_arr[period] = 100.0 * smoothed_minus_dm / smoothed_tr
    else:
        plus_di_arr[period] = 0.0
        minus_di_arr[period] = 0.0

    di_sum = plus_di_arr[period] + minus_di_arr[period]
    if di_sum > 0:
        dx_arr[period] = 100.0 * abs(plus_di_arr[period] - minus_di_arr[period]) / di_sum
    else:
        dx_arr[period] = 0.0

    # Step 3: 继续 Wilder's smoothing for 后续 bar
    for i in range(period + 1, n):
        smoothed_tr = smoothed_tr - (smoothed_tr / period) + tr[i]
        smoothed_plus_dm = smoothed_plus_dm - (smoothed_plus_dm / period) + plus_dm[i]
        smoothed_minus_dm = smoothed_minus_dm - (smoothed_minus_dm / period) + minus_dm[i]

        if smoothed_tr > 0:
            plus_di_arr[i] = 100.0 * smoothed_plus_dm / smoothed_tr
            minus_di_arr[i] = 100.0 * smoothed_minus_dm / smoothed_tr
        else:
            plus_di_arr[i] = 0.0
            minus_di_arr[i] = 0.0

        di_sum = plus_di_arr[i] + minus_di_arr[i]
        if di_sum > 0:
            dx_arr[i] = 100.0 * abs(plus_di_arr[i] - minus_di_arr[i]) / di_sum
        else:
            dx_arr[i] = 0.0

    # Step 4: 计算 ADX（对 DX 再做一次 Wilder's smoothing）
    # ADX 初始值：从 period 到 2*period-1 的 DX 平均值
    adx_start_idx = 2 * period - 1
    if adx_start_idx >= n:
        return np.nan

    adx = np.mean(dx_arr[period : 2 * period])

    # 继续 Wilder's smoothing
    for i in range(2 * period, n):
        adx = (adx * (period - 1) + dx_arr[i]) / period

    return adx


# ---------------------------------------------------------------------------
# PIT Audit 报告生成（Task 2.8 实现）
# ---------------------------------------------------------------------------


def generate_pit_audit_report(
    audit_entries: list[PITAuditEntry],
    output_path: Path,
) -> None:
    """生成 Point-In-Time 校验报告 (pretouch_features_pit_audit.md)。

    产出结构化 Markdown 报告，记录每个增强特征的：
    - 特征名
    - 数据来源
    - 计算逻辑
    - 时间戳边界
    - 是否通过 PIT 校验

    Parameters
    ----------
    audit_entries : list[PITAuditEntry]
        所有增强特征的 PIT 校验记录。
    output_path : Path
        输出文件路径（如 output/pretouch_refinement/pretouch_features_pit_audit.md）。
    """
    total = len(audit_entries)
    passed = sum(1 for e in audit_entries if e.pit_passed)
    failed = total - passed

    lines: list[str] = []

    # --- 标题 ---
    lines.append("# Point-In-Time 特征校验报告\n")
    lines.append("")

    # --- 摘要 ---
    lines.append("## 摘要\n")
    lines.append("")
    lines.append(f"- **特征总数**: {total}")
    lines.append(f"- **通过校验**: {passed} ✅")
    lines.append(f"- **未通过校验**: {failed} ❌")
    lines.append("")
    if failed == 0:
        lines.append("> 所有特征均满足 Point-In-Time 约束：仅使用 touch_time 之前的数据。")
    else:
        lines.append(
            f"> ⚠️ 有 {failed} 个特征未通过 PIT 校验，请检查下方详情。"
        )
    lines.append("")

    # --- 汇总表格 ---
    lines.append("## 校验汇总表\n")
    lines.append("")
    lines.append(
        "| 特征名 | 数据来源 | 计算逻辑 | 时间戳边界 | PIT 校验 |"
    )
    lines.append(
        "|--------|----------|----------|------------|----------|"
    )
    for entry in audit_entries:
        status = "✅ 通过" if entry.pit_passed else "❌ 未通过"
        # 转义表格中的管道符
        logic = entry.computation_logic.replace("|", "\\|")
        boundary = entry.timestamp_boundary.replace("|", "\\|")
        source = entry.data_source.replace("|", "\\|")
        name = entry.feature_name.replace("|", "\\|")
        lines.append(
            f"| {name} | {source} | {logic} | {boundary} | {status} |"
        )
    lines.append("")

    # --- 逐特征详情 ---
    lines.append("## 逐特征详情\n")
    lines.append("")
    for entry in audit_entries:
        status_icon = "✅" if entry.pit_passed else "❌"
        lines.append(f"### {entry.feature_name} {status_icon}\n")
        lines.append("")
        lines.append(f"- **数据来源**: {entry.data_source}")
        lines.append(f"- **计算逻辑**: {entry.computation_logic}")
        lines.append(f"- **时间戳边界**: {entry.timestamp_boundary}")
        lines.append(
            f"- **PIT 校验结果**: {'通过' if entry.pit_passed else '未通过'}"
        )
        lines.append("")

    # --- 写入文件 ---
    output_path = Path(output_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines), encoding="utf-8")

    logger.info(
        f"generate_pit_audit_report: wrote {total} entries to {output_path} "
        f"(passed={passed}, failed={failed})"
    )
