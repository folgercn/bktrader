"""
feature_extractor — Pre-Breakout 特征提取

负责：
- 定义 PRE_BREAKOUT_FEATURES（核心特征，始终期望可用）
- 定义 OPTIONAL_FEATURES（可能缺失率高的可选特征）
- 从 events DataFrame 提取特征矩阵，排除缺失率过高的特征
- 中位数填充（统计量仅从 train set 计算）
- 产出特征描述性统计和各 regime 下的分布对比

所有特征均来自 events_execution_labeled.csv 已有列，
在 breakout 触发时刻（signal bar 时间）已完全确定（Point_In_Time 约束）。
"""

from __future__ import annotations

import pandas as pd


# ---------------------------------------------------------------------------
# 核心特征列表（始终期望可用）
# ---------------------------------------------------------------------------

PRE_BREAKOUT_FEATURES: list[str] = [
    # ATR 与波动率
    "signal_atr_percentile",
    "roundtrip_cost_atr",
    # 前一根 bar 结构
    "prev1_body_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    # 均线与趋势
    "prev_sma5_gap_atr",
    "prev_sma5_slope_atr",
    # Level 强度
    "level_to_prev_close_atr",
    "level_to_signal_open_atr",
    "touch_extension_atr",
]

# ---------------------------------------------------------------------------
# 可选特征列表（可能缺失率高）
# ---------------------------------------------------------------------------

OPTIONAL_FEATURES: list[str] = [
    # 状态序列特征（PRE-TOUCH: 使用 touch_time 前 60s 的状态序列）
    "state_frac_0",
    "state_frac_1",
    "state_frac_2",
    "state_frac_3",
    "state_entropy",
]

# ---------------------------------------------------------------------------
# 违反 Point_In_Time 约束的特征（POST-TOUCH 数据，禁止用于分类器）
# ---------------------------------------------------------------------------
# dwell_*_pass 特征使用 touch_time 之后的 1s bar 数据（touch_pos 到
# touch_pos + seconds），属于 post-breakout 数据泄露。
# 详见 verify_point_in_time.py 的源码审计。
EXCLUDED_POST_TOUCH_FEATURES: list[str] = [
    "dwell_5s_pass",
    "dwell_15s_pass",
    "dwell_30s_pass",
    "dwell_60s_pass",
]


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def extract_features(
    events: pd.DataFrame,
    missing_threshold: float = 0.5,
) -> tuple[pd.DataFrame, list[str], list[str]]:
    """从 events DataFrame 提取 pre-breakout 特征矩阵。

    合并 PRE_BREAKOUT_FEATURES 和 OPTIONAL_FEATURES，检查每个特征在
    events 中的可用性和缺失率。缺失率超过 missing_threshold 的特征
    将被排除。

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate events DataFrame，需含特征列。
    missing_threshold : float
        缺失率阈值，默认 0.5（50%）。超过此阈值的特征将被排除。

    Returns
    -------
    tuple[pd.DataFrame, list[str], list[str]]
        - features: DataFrame (n_events × n_features)，仅含通过筛选的特征列
        - used_features: 实际使用的特征列名列表
        - excluded_features: 因缺失率过高或不存在被排除的特征列名列表
    """
    candidates = PRE_BREAKOUT_FEATURES + OPTIONAL_FEATURES
    n_rows = len(events)

    used_features: list[str] = []
    excluded_features: list[str] = []

    for col in candidates:
        if col not in events.columns:
            excluded_features.append(col)
            continue

        missing_rate = events[col].isna().sum() / n_rows if n_rows > 0 else 0.0
        if missing_rate > missing_threshold:
            excluded_features.append(col)
        else:
            used_features.append(col)

    features = events[used_features].copy()
    return features, used_features, excluded_features


def impute_features(
    train_features: pd.DataFrame,
    test_features: pd.DataFrame,
) -> tuple[pd.DataFrame, pd.DataFrame, dict]:
    """中位数填充：统计量仅从 train set 计算，应用于 train 和 test set。

    对每个特征列，计算 train set 的中位数，用该中位数填充 train 和
    test set 中的缺失值。

    Parameters
    ----------
    train_features : pd.DataFrame
        训练集特征矩阵。
    test_features : pd.DataFrame
        测试集特征矩阵。

    Returns
    -------
    tuple[pd.DataFrame, pd.DataFrame, dict]
        - imputed_train: 填充后的训练集特征矩阵
        - imputed_test: 填充后的测试集特征矩阵
        - imputation_stats: dict，每个特征的填充统计信息，格式为
          {feature_name: {"median": float, "train_missing_count": int,
                          "test_missing_count": int, "train_missing_rate": float,
                          "test_missing_rate": float}}
    """
    # 不修改输入 DataFrame，在副本上操作
    imputed_train = train_features.copy()
    imputed_test = test_features.copy()

    n_train = len(train_features)
    n_test = len(test_features)

    imputation_stats: dict = {}

    for col in train_features.columns:
        train_missing_count = int(train_features[col].isna().sum())
        test_missing_count = int(test_features[col].isna().sum())
        train_missing_rate = train_missing_count / n_train if n_train > 0 else 0.0
        test_missing_rate = test_missing_count / n_test if n_test > 0 else 0.0

        # 中位数仅从 train set 计算
        median_val = float(train_features[col].median())

        # 用 train 中位数填充 train 和 test 的缺失值
        imputed_train[col] = imputed_train[col].fillna(median_val)
        imputed_test[col] = imputed_test[col].fillna(median_val)

        imputation_stats[col] = {
            "median": median_val,
            "train_missing_count": train_missing_count,
            "test_missing_count": test_missing_count,
            "train_missing_rate": train_missing_rate,
            "test_missing_rate": test_missing_rate,
        }

    return imputed_train, imputed_test, imputation_stats


def feature_statistics(
    features: pd.DataFrame,
    labels: pd.Series,
) -> pd.DataFrame:
    """产出特征描述性统计 + 各 regime 下的分布对比。

    对每个特征计算全局统计（mean, std, min, max, missing_rate），
    以及按 labels（Timing_Regime）分组后各组的 mean 和 std。

    Parameters
    ----------
    features : pd.DataFrame
        特征矩阵 (n_events × n_features)。
    labels : pd.Series
        每个 event 的 Optimal_Delay_Label（如 "D0", "D5", "D10", "D15",
        "pullback", "skip"）。

    Returns
    -------
    pd.DataFrame
        统计结果 DataFrame，index 为特征名，columns 包含：
        - mean, std, min, max, missing_rate（全局统计）
        - {regime}_mean, {regime}_std（各 regime 下的统计）
    """
    n_rows = len(features)
    feature_names = list(features.columns)

    # --- 全局描述性统计 ---
    stats: dict[str, dict] = {}
    for col in feature_names:
        series = features[col]
        missing_count = int(series.isna().sum())
        missing_rate = missing_count / n_rows if n_rows > 0 else 0.0
        stats[col] = {
            "mean": series.mean(),    # NaN if all missing
            "std": series.std(),      # NaN if all missing or single non-NaN
            "min": series.min(),
            "max": series.max(),
            "missing_rate": missing_rate,
        }

    # --- 各 regime 下的分组统计 ---
    unique_regimes = sorted(labels.unique())

    for regime in unique_regimes:
        mask = labels == regime
        regime_features = features.loc[mask]
        for col in feature_names:
            series = regime_features[col]
            # mean/std 对全 NaN 列或单样本自然返回 NaN
            stats[col][f"{regime}_mean"] = series.mean()
            stats[col][f"{regime}_std"] = series.std()

    # --- 组装 DataFrame ---
    result = pd.DataFrame.from_dict(stats, orient="index")
    result.index.name = "feature"
    return result
