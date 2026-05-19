"""classifier_trainer — DT3 训练与评估模块。

职责：
- 在扩展池上训练 RuleBased_DT3（DecisionTree max_depth=3）
- LOOCV calendar_sum 评估
- Test set 3-regime 评估（fast/slow/skip → delay resolution → silo calendar_sum）
- 提取决策规则并与原 A2 规则对比
- 深度对比（max_depth ∈ {2,3,4,5}）

复用：
- pre_breakout_timing.feature_extractor.extract_features() + impute_features()
- pre_breakout_timing.timing_classifier 的 LOOCV 逻辑和 extract_rules_text()
- pretouch_refinement.arm_runner._compute_calendar_sum_silo 的 silo 计算逻辑
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from typing import Any

import numpy as np
import pandas as pd

from pre_breakout_timing.delay_simulator import DelayResult
from pre_breakout_timing.feature_extractor import extract_features, impute_features
from pre_breakout_timing.timing_classifier import extract_rules_text

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants — 与 timing_classifier / arm_runner 保持一致
# ---------------------------------------------------------------------------

_INITIAL_BALANCE = 100_000.0
_NOTIONAL_SHARE = 0.26

# 3-regime delay 分组
_FAST_DELAYS = ["D0", "D5"]
_SLOW_DELAYS = ["D10", "D15", "pullback"]


# ---------------------------------------------------------------------------
# Data Classes
# ---------------------------------------------------------------------------


@dataclass
class TrainingResult:
    """DT3 训练结果。"""

    model: Any  # sklearn DecisionTreeClassifier
    loocv_calendar_sum: float
    train_calendar_sum: float
    test_calendar_sum: float
    accuracy: float
    rule_text: str
    feature_importances: dict[str, float]
    predictions_test: list[str]


@dataclass
class RuleComparisonResult:
    """规则对比结果。"""

    original_rule_text: str
    expanded_rule_text: str
    stability_score: float  # 0.0 - 1.0
    instability_warning: bool  # score < 0.5
    diff_summary: str  # 中文描述差异


@dataclass
class DepthComparisonResult:
    """深度对比结果（验证 DT3 仍为最优）。"""

    results_by_depth: dict[int, float]  # {2: loocv_cs, 3: loocv_cs, 4: ..., 5: ...}
    best_depth: int
    dt3_still_optimal: bool


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def train_and_evaluate(
    train_events: pd.DataFrame,
    test_events: pd.DataFrame,
    train_labels: pd.Series,
    all_delay_results: list[list[DelayResult]],
    train_event_ids: set[str],
    test_event_ids: set[str],
) -> TrainingResult:
    """训练 DT3 并在 test set 上评估。

    Parameters
    ----------
    train_events : pd.DataFrame
        训练集 events（含特征列）。
    test_events : pd.DataFrame
        测试集 events（含特征列）。
    train_labels : pd.Series
        训练集 3-regime 标签（skip/fast/slow）。
    all_delay_results : list[list[DelayResult]]
        所有 events 的 delay results（按 events 顺序排列，包含 train + test）。
    train_event_ids : set[str]
        训练集 event_id 集合。
    test_event_ids : set[str]
        测试集 event_id 集合。

    Returns
    -------
    TrainingResult
        包含模型、评估指标、规则文本等。
    """
    from sklearn.tree import DecisionTreeClassifier

    # --- Step 1: 提取特征 ---
    train_features, used_features, _ = extract_features(train_events)
    test_features, _, _ = extract_features(test_events)
    # 确保 test 使用与 train 相同的特征列
    test_features = test_events[used_features].copy()

    # --- Step 2: 中位数填充 ---
    train_features_imputed, test_features_imputed, _ = impute_features(
        train_features, test_features
    )

    # --- Step 3: 过滤 skip 标签（skip 不参与训练）---
    train_mask = train_labels != "skip"
    train_features_filtered = train_features_imputed.loc[train_mask].reset_index(
        drop=True
    )
    train_labels_filtered = train_labels.loc[train_mask].reset_index(drop=True)

    # 对应的 delay_results 也需要过滤
    # 首先从 all_delay_results 中分离 train 和 test 的 delay_results
    delay_results_train = _extract_delay_results_by_ids(
        all_delay_results, train_event_ids, train_events
    )
    delay_results_test = _extract_delay_results_by_ids(
        all_delay_results, test_event_ids, test_events
    )

    # 过滤 skip 对应的 delay_results
    delay_results_train_filtered = [
        delay_results_train[i] for i, keep in enumerate(train_mask) if keep
    ]

    # --- Step 4: LOOCV calendar_sum ---
    loocv_cs = _loocv_calendar_sum_3regime(
        lambda: DecisionTreeClassifier(max_depth=3, random_state=42),
        train_features_filtered,
        train_labels_filtered,
        delay_results_train_filtered,
    )
    logger.info("LOOCV calendar_sum: %.4f%%", loocv_cs)

    # --- Step 5: 在全量 train（排除 skip）上训练最终模型 ---
    clf = DecisionTreeClassifier(max_depth=3, random_state=42)
    clf.fit(train_features_filtered, train_labels_filtered)

    # --- Step 6: Train calendar_sum ---
    train_predictions = clf.predict(train_features_filtered)
    train_results = _resolve_3regime_predictions(
        train_predictions, delay_results_train_filtered
    )
    train_cs = _compute_calendar_sum_silo(train_results)

    # --- Step 7: Test set 评估 ---
    test_predictions = clf.predict(test_features_imputed)
    predictions_list = test_predictions.tolist()

    # 3-regime 评估：对 test set 每个 event 根据预测选择 delay
    test_results = _resolve_3regime_predictions(test_predictions, delay_results_test)
    test_cs = _compute_calendar_sum_silo(test_results)

    # --- Step 8: Accuracy（在 test set 上，仅对非 skip 预测计算）---
    accuracy = float(np.mean(train_predictions == train_labels_filtered.values))

    # --- Step 9: 特征重要性 ---
    feature_importances: dict[str, float] = dict(
        zip(
            train_features_filtered.columns.tolist(),
            clf.feature_importances_.tolist(),
        )
    )

    # --- Step 10: 提取规则文本 ---
    rule_text = extract_rules_text(clf, train_features_filtered.columns.tolist())

    logger.info(
        "DT3 训练完成: loocv_cs=%.4f%%, train_cs=%.4f%%, test_cs=%.4f%%",
        loocv_cs,
        train_cs,
        test_cs,
    )

    return TrainingResult(
        model=clf,
        loocv_calendar_sum=loocv_cs,
        train_calendar_sum=train_cs,
        test_calendar_sum=test_cs,
        accuracy=accuracy,
        rule_text=rule_text,
        feature_importances=feature_importances,
        predictions_test=predictions_list,
    )


def compare_rules(
    expanded_rule_text: str,
    original_a2_rule_text: str,
) -> RuleComparisonResult:
    """对比扩展池 DT3 规则与原 A2 DT3 规则。

    Rule_Stability_Score 计算：
    - 起始 1.0
    - 对前 2 层的每个分裂节点（最多 3 个：root + 2 children），
      若特征名或分裂方向不同，score 减 0.25
    - rule_instability_warning = score < 0.5

    Parameters
    ----------
    expanded_rule_text : str
        扩展池 DT3 的规则文本。
    original_a2_rule_text : str
        原 A2 DT3 的规则文本。

    Returns
    -------
    RuleComparisonResult
    """
    # 解析两组规则的分裂节点
    expanded_splits = _extract_split_nodes(expanded_rule_text)
    original_splits = _extract_split_nodes(original_a2_rule_text)

    # 前 2 层最多有 1 (root) + 2 (level 1) + 4 (level 2) = 7 个节点
    # 但按设计文档，只比较前 2 层的分裂节点
    # 前 2 层 = root (depth 0) + depth 1 children = 最多 3 个分裂节点
    # 实际上 DT3 的前 2 层最多有 1 + 2 = 3 个内部节点
    max_compare = min(3, max(len(expanded_splits), len(original_splits)))

    score = 1.0
    diff_details: list[str] = []

    for i in range(max_compare):
        exp_node = expanded_splits[i] if i < len(expanded_splits) else None
        orig_node = original_splits[i] if i < len(original_splits) else None

        if exp_node is None and orig_node is None:
            continue

        if exp_node is None or orig_node is None:
            score -= 0.25
            diff_details.append(
                f"节点 {i + 1}: 一侧缺失（扩展={exp_node}, 原始={orig_node}）"
            )
            continue

        # 比较特征名和分裂方向
        exp_feat, exp_op, _ = exp_node
        orig_feat, orig_op, _ = orig_node

        if exp_feat != orig_feat or exp_op != orig_op:
            score -= 0.25
            diff_details.append(
                f"节点 {i + 1}: 扩展=({exp_feat} {exp_op}), "
                f"原始=({orig_feat} {orig_op})"
            )

    # 确保 score 不低于 0
    score = max(0.0, score)

    instability_warning = score < 0.5

    if diff_details:
        diff_summary = "规则差异：" + "；".join(diff_details)
    else:
        diff_summary = "规则完全一致（前 2 层分裂节点相同）"

    if instability_warning:
        diff_summary += "。⚠ 规则不稳定警告：score < 0.5"

    return RuleComparisonResult(
        original_rule_text=original_a2_rule_text,
        expanded_rule_text=expanded_rule_text,
        stability_score=score,
        instability_warning=instability_warning,
        diff_summary=diff_summary,
    )


def compare_depths(
    train_features: pd.DataFrame,
    train_labels: pd.Series,
    delay_results_train: list[list[DelayResult]],
    train_events: pd.DataFrame,
) -> DepthComparisonResult:
    """LOOCV 对比 max_depth ∈ {2,3,4,5}。

    Parameters
    ----------
    train_features : pd.DataFrame
        训练集特征矩阵（已 impute，已排除 skip）。
    train_labels : pd.Series
        训练集标签（已排除 skip）。
    delay_results_train : list[list[DelayResult]]
        训练集 delay results（已排除 skip 对应的 events）。
    train_events : pd.DataFrame
        训练集 events（已排除 skip）。

    Returns
    -------
    DepthComparisonResult
    """
    from sklearn.tree import DecisionTreeClassifier

    depths = [2, 3, 4, 5]
    results_by_depth: dict[int, float] = {}

    for depth in depths:
        loocv_cs = _loocv_calendar_sum_3regime(
            lambda d=depth: DecisionTreeClassifier(max_depth=d, random_state=42),
            train_features,
            train_labels,
            delay_results_train,
        )
        results_by_depth[depth] = loocv_cs
        logger.info("  max_depth=%d: LOOCV calendar_sum=%.4f%%", depth, loocv_cs)

    best_depth = max(results_by_depth, key=results_by_depth.get)  # type: ignore[arg-type]
    dt3_still_optimal = best_depth == 3

    logger.info(
        "深度对比完成: best_depth=%d, dt3_still_optimal=%s",
        best_depth,
        dt3_still_optimal,
    )

    return DepthComparisonResult(
        results_by_depth=results_by_depth,
        best_depth=best_depth,
        dt3_still_optimal=dt3_still_optimal,
    )


# ---------------------------------------------------------------------------
# Internal Helpers — LOOCV (3-regime)
# ---------------------------------------------------------------------------


def _loocv_calendar_sum_3regime(
    classifier_factory,
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
) -> float:
    """内联 LOOCV calendar_sum 计算（3-regime 版本）。

    每次留出一个 event，用剩余 events 训练分类器，
    对留出 event 预测 label 并使用 3-regime delay resolution，
    最终计算 silo-based calendar sum。
    """
    n = len(features)
    loo_results: list[DelayResult] = []

    for i in range(n):
        # 训练集：排除第 i 个 event
        train_mask = np.ones(n, dtype=bool)
        train_mask[i] = False

        X_train = features.iloc[train_mask]
        y_train = labels.iloc[train_mask]
        X_test = features.iloc[[i]]

        # 训练分类器
        clf = classifier_factory()
        clf.fit(X_train, y_train)

        # 预测第 i 个 event 的 label
        predicted_label = clf.predict(X_test)[0]

        # 3-regime delay resolution
        matched_result = _resolve_single_3regime_prediction(
            predicted_label, delay_results[i]
        )
        loo_results.append(matched_result)

    # 计算 silo-based calendar sum
    return _compute_calendar_sum_silo(loo_results)


# ---------------------------------------------------------------------------
# Internal Helpers — 3-Regime Prediction Resolution
# ---------------------------------------------------------------------------


def _resolve_3regime_predictions(
    predictions: np.ndarray,
    delay_results: list[list[DelayResult]],
) -> list[DelayResult]:
    """将 3-regime 预测数组映射到对应的 DelayResult 列表。"""
    results: list[DelayResult] = []
    for i, pred_label in enumerate(predictions):
        matched = _resolve_single_3regime_prediction(pred_label, delay_results[i])
        results.append(matched)
    return results


def _resolve_single_3regime_prediction(
    pred_label: str,
    event_delays: list[DelayResult],
) -> DelayResult:
    """3-regime 预测映射：fast/slow/skip → 对应组内最优 delay。

    - fast → max(D0_pnl, D5_pnl) 对应的 DelayResult
    - slow → max(D10_pnl, D15_pnl, pullback_pnl) 对应的 DelayResult
    - skip → traded=False 占位（pnl=0）
    """
    if pred_label == "skip":
        event_id = event_delays[0].event_id if event_delays else "unknown"
        return DelayResult(
            event_id=event_id,
            delay_label="skip",
            delay_seconds=0,
            entry_time=None,
            entry_price=None,
            pnl_pct=None,
            exit_reason="Skip",
            exit_time=None,
            hold_seconds=None,
            mfe_r=None,
            mae_r=None,
            traded=False,
        )

    if pred_label == "fast":
        target_labels = set(_FAST_DELAYS)
    elif pred_label == "slow":
        target_labels = set(_SLOW_DELAYS)
    else:
        # 未知标签，尝试直接匹配 delay_label
        for dr in event_delays:
            if dr.delay_label == pred_label:
                return dr
        event_id = event_delays[0].event_id if event_delays else "unknown"
        return DelayResult(
            event_id=event_id,
            delay_label=pred_label,
            delay_seconds=0,
            entry_time=None,
            entry_price=None,
            pnl_pct=None,
            exit_reason="NoMatch",
            exit_time=None,
            hold_seconds=None,
            mfe_r=None,
            mae_r=None,
            traded=False,
        )

    # 在目标组内选 pnl 最高的 delay
    best_dr: DelayResult | None = None
    best_pnl = -np.inf

    for dr in event_delays:
        if dr.delay_label in target_labels:
            pnl = dr.pnl_pct if (dr.traded and dr.pnl_pct is not None) else -np.inf
            if pnl > best_pnl:
                best_pnl = pnl
                best_dr = dr

    if best_dr is not None:
        return best_dr

    # 未找到匹配，返回 traded=False 占位
    event_id = event_delays[0].event_id if event_delays else "unknown"
    return DelayResult(
        event_id=event_id,
        delay_label=pred_label,
        delay_seconds=0,
        entry_time=None,
        entry_price=None,
        pnl_pct=None,
        exit_reason="NoMatch",
        exit_time=None,
        hold_seconds=None,
        mfe_r=None,
        mae_r=None,
        traded=False,
    )


# ---------------------------------------------------------------------------
# Internal Helpers — Silo-based Calendar Sum
# ---------------------------------------------------------------------------


def _compute_calendar_sum_silo(results: list[DelayResult]) -> float:
    """计算 silo-based calendar sum (%)。

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。

    从 event_id 推断 symbol（与 timing_classifier._infer_symbol_from_event_id 一致）。
    """
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        symbol = _infer_symbol_from_event_id(r.event_id)
        entry_time = pd.Timestamp(r.entry_time)
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(r)

    total_return_pct = 0.0
    for _silo_key, silo_results in silos.items():
        balance = _INITIAL_BALANCE
        sorted_results = sorted(silo_results, key=lambda r: r.entry_time)
        for r in sorted_results:
            notional = balance * _NOTIONAL_SHARE
            pnl = notional * r.pnl_pct
            balance += pnl
        silo_return = (balance - _INITIAL_BALANCE) / _INITIAL_BALANCE * 100.0
        total_return_pct += silo_return

    return total_return_pct


def _infer_symbol_from_event_id(event_id: str) -> str:
    """从 event_id 推断 symbol。"""
    eid_upper = event_id.upper()
    if "BTC" in eid_upper:
        return "BTCUSDT"
    elif "ETH" in eid_upper:
        return "ETHUSDT"
    return "unknown"


# ---------------------------------------------------------------------------
# Internal Helpers — Delay Results Extraction
# ---------------------------------------------------------------------------


def _extract_delay_results_by_ids(
    all_delay_results: list[list[DelayResult]],
    target_event_ids: set[str],
    target_events: pd.DataFrame,
) -> list[list[DelayResult]]:
    """从 all_delay_results 中按 event_id 提取对应子集。

    返回的列表顺序与 target_events 的行顺序一致。
    """
    # 建立 event_id → index 映射（all_delay_results 的 index）
    id_to_idx: dict[str, int] = {}
    for idx, event_results in enumerate(all_delay_results):
        if event_results:
            eid = event_results[0].event_id
            id_to_idx[eid] = idx

    result: list[list[DelayResult]] = []
    for _, row in target_events.iterrows():
        eid = str(row["event_id"])
        if eid in id_to_idx:
            result.append(all_delay_results[id_to_idx[eid]])
        else:
            # 未找到，返回空占位
            result.append(_make_empty_delay_results(eid))

    return result


def _make_empty_delay_results(event_id: str) -> list[DelayResult]:
    """为缺失的 event 生成 5 个空 DelayResult 占位。"""
    delay_labels = ["D0", "D5", "D10", "D15", "pullback"]
    delay_seconds_map = {"D0": 0, "D5": 5, "D10": 10, "D15": 15, "pullback": 0}
    results: list[DelayResult] = []
    for lbl in delay_labels:
        results.append(
            DelayResult(
                event_id=event_id,
                delay_label=lbl,
                delay_seconds=delay_seconds_map[lbl],
                entry_time=None,
                entry_price=None,
                pnl_pct=None,
                exit_reason="NoData",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            )
        )
    return results


# ---------------------------------------------------------------------------
# Internal Helpers — Rule Parsing for Comparison
# ---------------------------------------------------------------------------


def _extract_split_nodes(rule_text: str) -> list[tuple[str, str, float]]:
    """从规则文本中提取前 2 层的分裂节点信息。

    解析 extract_rules_text() 产出的中文规则格式。
    格式示例：
        规则 1: 若 signal_atr_percentile ≤ 0.50 且 prev1_body_atr ≤ 0.30
          → 立即入场 (D=0s)

    树的前 2 层结构：
    - Layer 0 (root): 1 个分裂节点 → 第一条规则的第 1 个条件
    - Layer 1: 最多 2 个分裂节点 → 左子树第 2 个条件 + 右子树第 2 个条件

    返回按层序排列的 (feature_name, operator, threshold) 列表，最多 3 个。
    """
    # 匹配 "若/且 feature_name ≤/> threshold" 模式
    condition_pattern = r"(?:若|且)\s+(\S+)\s+(≤|>)\s+([\d.]+)"

    # 按规则分组解析
    rule_pattern = r"规则\s+\d+:\s*(.*?)(?=规则\s+\d+:|$)"
    rules = re.findall(rule_pattern, rule_text, re.DOTALL)

    if not rules:
        return []

    # 提取每条规则的条件链
    rule_conditions: list[list[tuple[str, str, float]]] = []
    for rule_body in rules:
        conditions = re.findall(condition_pattern, rule_body)
        parsed = [(f, o, float(t)) for f, o, t in conditions]
        if parsed:
            rule_conditions.append(parsed)

    if not rule_conditions:
        return []

    # Layer 0: root split = 第一条规则的第 1 个条件
    result: list[tuple[str, str, float]] = []
    root_split = rule_conditions[0][0]
    result.append(root_split)

    # Layer 1: 从规则中提取第 2 层的分裂节点
    # 左子树的 layer-1 split: 找到与 root 同方向的规则中的第 2 个条件
    # 右子树的 layer-1 split: 找到与 root 反方向的规则中的第 2 个条件
    root_feat, root_op, root_thresh = root_split
    opposite_op = ">" if root_op == "≤" else "≤"

    left_child_split: tuple[str, str, float] | None = None
    right_child_split: tuple[str, str, float] | None = None

    for conditions in rule_conditions:
        if len(conditions) < 2:
            continue
        first_cond = conditions[0]
        second_cond = conditions[1]

        # 检查第一个条件是否是 root split 的左分支（same direction）
        if (
            first_cond[0] == root_feat
            and first_cond[1] == root_op
            and left_child_split is None
        ):
            left_child_split = second_cond

        # 检查第一个条件是否是 root split 的右分支（opposite direction）
        if (
            first_cond[0] == root_feat
            and first_cond[1] == opposite_op
            and right_child_split is None
        ):
            right_child_split = second_cond

    if left_child_split is not None:
        result.append(left_child_split)
    if right_child_split is not None:
        result.append(right_child_split)

    return result[:3]
