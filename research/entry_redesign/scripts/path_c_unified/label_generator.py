"""label_generator — 3-Regime 标签生成模块。

基于扩展池的 Delay_PnL_Matrix 生成 3-regime 标签，执行 60/40 time-split，
并计算标签分布统计（含与原 116 events 的对比）。

复用 pretouch_refinement.regime_labels.generate_3regime_labels() 逻辑，
不重新实现标签分配算法。
"""

from __future__ import annotations

import math
from dataclasses import dataclass

import pandas as pd

from pretouch_refinement.regime_labels import generate_3regime_labels


# ---------------------------------------------------------------------------
# Data Classes
# ---------------------------------------------------------------------------


@dataclass
class LabelStats:
    """标签分布统计。"""

    train_distribution: dict[str, int]  # {"fast": N, "slow": M, "skip": K}
    test_distribution: dict[str, int]
    train_pct: dict[str, float]  # {"fast": 0.35, "slow": 0.40, "skip": 0.25}
    test_pct: dict[str, float]
    label_shift_vs_original: bool  # skip 占比差异 > 10pp
    original_skip_pct: float
    expanded_skip_pct: float


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def generate_labels_and_split(
    delay_pnl_matrix: pd.DataFrame,
    events: pd.DataFrame,
    original_events: pd.DataFrame,
    tolerance_bps: float = 5.0,
    train_ratio: float = 0.6,
) -> tuple[pd.DataFrame, pd.DataFrame, pd.Series, pd.Series, LabelStats]:
    """生成 3-regime 标签并执行 time-split。

    Parameters
    ----------
    delay_pnl_matrix : pd.DataFrame
        扩展池完整 delay PnL 矩阵（N_events × 5 行）。
    events : pd.DataFrame
        扩展事件池（必须包含 event_id 和 touch_time 列）。
    original_events : pd.DataFrame
        原 116 events（用于计算 label_shift_vs_original）。
    tolerance_bps : float
        容差阈值（basis points），默认 5.0。
    train_ratio : float
        训练集比例，默认 0.6。

    Returns
    -------
    tuple[pd.DataFrame, pd.DataFrame, pd.Series, pd.Series, LabelStats]
        - train_events: 训练集 events DataFrame
        - test_events: 测试集 events DataFrame
        - train_labels: 训练集 3-regime 标签 (pd.Series)
        - test_labels: 测试集 3-regime 标签 (pd.Series)
        - label_stats: 标签分布统计
    """
    # --- Step 1: 生成扩展池的 3-regime 标签 ---
    labels = generate_3regime_labels(delay_pnl_matrix, events, tolerance_bps)

    # --- Step 2: 按 touch_time 排序执行 60/40 time-split ---
    events_with_labels = events.copy()
    events_with_labels["touch_time"] = pd.to_datetime(
        events_with_labels["touch_time"], utc=True
    )
    events_with_labels["_label"] = labels.values

    sorted_events = events_with_labels.sort_values("touch_time").reset_index(drop=True)
    split_idx = math.floor(len(sorted_events) * train_ratio)

    train_df = sorted_events.iloc[:split_idx].reset_index(drop=True)
    test_df = sorted_events.iloc[split_idx:].reset_index(drop=True)

    train_labels = pd.Series(
        train_df["_label"].values, index=train_df.index, name="regime_3_label"
    )
    test_labels = pd.Series(
        test_df["_label"].values, index=test_df.index, name="regime_3_label"
    )

    # 移除临时列
    train_events = train_df.drop(columns=["_label"]).reset_index(drop=True)
    test_events = test_df.drop(columns=["_label"]).reset_index(drop=True)

    # --- Step 3: 计算标签分布统计 ---
    train_distribution = _compute_distribution(train_labels)
    test_distribution = _compute_distribution(test_labels)
    train_pct = _compute_pct(train_distribution, len(train_labels))
    test_pct = _compute_pct(test_distribution, len(test_labels))

    # --- Step 4: 计算扩展池 skip 占比 ---
    total_events = len(sorted_events)
    expanded_skip_count = (sorted_events["_label"] == "skip").sum()
    expanded_skip_pct = (
        expanded_skip_count / total_events * 100.0 if total_events > 0 else 0.0
    )

    # --- Step 5: 计算原 116 events 的 skip 占比 ---
    # 从 delay_pnl_matrix 中筛选原 events 的行，重新生成标签
    original_event_ids = set(original_events["event_id"].tolist())
    original_matrix_rows = delay_pnl_matrix[
        delay_pnl_matrix["event_id"].isin(original_event_ids)
    ]

    # 只对原 events 中在 matrix 中存在的 events 生成标签
    original_events_in_matrix = original_events[
        original_events["event_id"].isin(original_matrix_rows["event_id"].unique())
    ].reset_index(drop=True)

    if len(original_events_in_matrix) > 0:
        original_labels = generate_3regime_labels(
            original_matrix_rows, original_events_in_matrix, tolerance_bps
        )
        original_skip_count = (original_labels == "skip").sum()
        original_skip_pct = (
            original_skip_count / len(original_labels) * 100.0
        )
    else:
        original_skip_pct = 0.0

    # --- Step 6: 判断 label_shift_vs_original ---
    label_shift_vs_original = abs(expanded_skip_pct - original_skip_pct) > 10.0

    label_stats = LabelStats(
        train_distribution=train_distribution,
        test_distribution=test_distribution,
        train_pct=train_pct,
        test_pct=test_pct,
        label_shift_vs_original=label_shift_vs_original,
        original_skip_pct=original_skip_pct,
        expanded_skip_pct=expanded_skip_pct,
    )

    return train_events, test_events, train_labels, test_labels, label_stats


# ---------------------------------------------------------------------------
# Internal Helpers
# ---------------------------------------------------------------------------


def _compute_distribution(labels: pd.Series) -> dict[str, int]:
    """计算标签分布 count。"""
    counts = labels.value_counts()
    result: dict[str, int] = {}
    for label in ["fast", "slow", "skip"]:
        result[label] = int(counts.get(label, 0))
    return result


def _compute_pct(distribution: dict[str, int], total: int) -> dict[str, float]:
    """计算标签分布占比（百分比）。"""
    if total == 0:
        return {label: 0.0 for label in distribution}
    return {
        label: round(count / total * 100.0, 2)
        for label, count in distribution.items()
    }
