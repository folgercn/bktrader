"""RF Probability Model — 概率模型训练与 Sizing Overlay"""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import roc_auc_score


@dataclass
class RFProbabilityResult:
    """RF 概率模型训练结果"""

    train_auc: float
    test_auc: float
    feature_importance_top5: list[tuple[str, float]]
    prob_mean: float
    prob_median: float
    prob_std: float
    rf_no_signal_warning: bool  # test AUC < 0.50
    model: object  # 训练好的 RF 分类器
    train_probabilities: np.ndarray
    test_probabilities: np.ndarray


def generate_rf_binary_labels(
    timing_predictions: pd.Series,
    delay_pnls: pd.Series,
) -> pd.Series:
    """Generate binary labels for RF training.

    二分类标签逻辑：
    - timing=skip → 0
    - timing=fast/slow AND delay_pnl > 0 → 1
    - timing=fast/slow AND delay_pnl <= 0 → 0

    Parameters
    ----------
    timing_predictions : pd.Series
        Timing 分类器预测结果，值域 {"skip", "fast", "slow"}。
    delay_pnls : pd.Series
        对应 delay 的 PnL 值（与 timing_predictions 同长度）。

    Returns
    -------
    pd.Series
        二分类标签（0 或 1），dtype=int。
    """
    labels = pd.Series(0, index=timing_predictions.index, dtype=int)
    # timing=fast/slow 且 pnl > 0 → 1
    non_skip_mask = timing_predictions.isin(["fast", "slow"])
    positive_mask = delay_pnls > 0
    labels[non_skip_mask & positive_mask] = 1
    return labels


def train_rf_probability(
    train_features: pd.DataFrame,
    train_labels: pd.Series,  # binary: 0=skip/negative, 1=positive
    test_features: pd.DataFrame,
    test_labels: pd.Series,
    n_estimators: int = 200,
    random_state: int = 42,
) -> RFProbabilityResult:
    """训练 RF 概率模型。

    使用 Original_10_Features 训练 RandomForestClassifier，输出每个事件的
    成功概率 p_success ∈ [0, 1]。

    Parameters
    ----------
    train_features : pd.DataFrame
        训练集特征矩阵（列为 Original_10_Features）。
    train_labels : pd.Series
        训练集二分类标签（0=skip/negative, 1=positive）。
    test_features : pd.DataFrame
        测试集特征矩阵。
    test_labels : pd.Series
        测试集二分类标签。
    n_estimators : int
        RF 树的数量，默认 200。
    random_state : int
        随机种子，默认 42。

    Returns
    -------
    RFProbabilityResult
        包含 AUC、feature importance、概率分布统计等。
    """
    # 1. Train RandomForestClassifier
    rf = RandomForestClassifier(
        n_estimators=n_estimators,
        random_state=random_state,
    )
    rf.fit(train_features, train_labels)

    # 2. Compute probabilities (class 1 probability)
    # Handle edge case: if only one class in training data, predict_proba
    # returns a single column. In that case, assign uniform probability.
    if len(rf.classes_) < 2:
        train_probabilities = np.full(len(train_features), 0.5)
        test_probabilities = np.full(len(test_features), 0.5)
    else:
        # Find the index of class 1 in rf.classes_
        class_1_idx = list(rf.classes_).index(1)
        train_probabilities = rf.predict_proba(train_features)[:, class_1_idx]
        test_probabilities = rf.predict_proba(test_features)[:, class_1_idx]

    # 3. Compute AUC — handle edge case where only one class exists
    train_unique = np.unique(train_labels)
    test_unique = np.unique(test_labels)

    if len(train_unique) < 2:
        train_auc = 0.5
    else:
        train_auc = roc_auc_score(train_labels, train_probabilities)

    if len(test_unique) < 2:
        test_auc = 0.5
    else:
        test_auc = roc_auc_score(test_labels, test_probabilities)

    # 4. Feature importance top-5
    importances = rf.feature_importances_
    feature_names = list(train_features.columns)
    importance_pairs = list(zip(feature_names, importances))
    importance_pairs.sort(key=lambda x: x[1], reverse=True)
    feature_importance_top5 = importance_pairs[:5]

    # 5. Probability distribution stats on test probabilities
    prob_mean = float(np.mean(test_probabilities))
    prob_median = float(np.median(test_probabilities))
    prob_std = float(np.std(test_probabilities))

    # 6. Warning flag
    rf_no_signal_warning = test_auc < 0.50

    return RFProbabilityResult(
        train_auc=train_auc,
        test_auc=test_auc,
        feature_importance_top5=feature_importance_top5,
        prob_mean=prob_mean,
        prob_median=prob_median,
        prob_std=prob_std,
        rf_no_signal_warning=rf_no_signal_warning,
        model=rf,
        train_probabilities=train_probabilities,
        test_probabilities=test_probabilities,
    )


def compute_sizing_multiplier(
    probabilities: np.ndarray,
) -> np.ndarray:
    """将概率映射为仓位乘数。

    multiplier = clip(p_success × 2, 0, 2)
    - p=0.0 → 0× (不入场)
    - p=0.5 → 1× (base)
    - p=1.0 → 2× (最大)

    Parameters
    ----------
    probabilities : np.ndarray
        概率数组，值域 [0, 1]。

    Returns
    -------
    np.ndarray
        仓位乘数数组，值域 [0, 2]。
    """
    return np.clip(probabilities * 2, 0, 2)
