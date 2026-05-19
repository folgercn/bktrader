"""
timing_classifier — 分类器训练与评估

负责：
- 定义 TimingRegime enum（入场延迟策略类别）
- 定义 ClassifierResult dataclass（分类器评估结果）
- 提供 LOOCV 评估函数（以 calendar_sum 为选择标准）
- 训练多种分类器（规则型、DecisionTree、RandomForest、LogisticRegression）
- 从 DecisionTree 提取人类可读的中文规则描述

所有分类器训练仅在 train set 上进行，使用 LOOCV calendar_sum 选择最优超参数。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum

import numpy as np
import pandas as pd

from pre_breakout_timing.delay_simulator import DelayResult


# ---------------------------------------------------------------------------
# TimingRegime 枚举
# ---------------------------------------------------------------------------


class TimingRegime(str, Enum):
    """入场延迟策略类别"""

    IMMEDIATE = "immediate"  # D=0s
    STANDARD = "standard"  # D=5s
    DELAYED = "delayed"  # D=10s
    EXTENDED = "extended"  # D=15s
    WAIT_PULLBACK = "pullback"  # 等待回调


# ---------------------------------------------------------------------------
# 辅助映射
# ---------------------------------------------------------------------------

REGIME_TO_DELAY: dict[str, int] = {
    "immediate": 0,
    "standard": 5,
    "delayed": 10,
    "extended": 15,
    "pullback": -1,  # pullback 使用特殊逻辑，-1 表示非固定 delay
}

LABEL_TO_REGIME: dict[str, str] = {
    "D0": "immediate",
    "D5": "standard",
    "D10": "delayed",
    "D15": "extended",
    "pullback": "pullback",
}


# ---------------------------------------------------------------------------
# ClassifierResult dataclass
# ---------------------------------------------------------------------------


@dataclass
class ClassifierResult:
    """分类器评估结果"""

    name: str  # 分类器名称
    best_params: dict  # 最优超参数
    train_calendar_sum: float  # train set calendar sum
    test_calendar_sum: float  # test set calendar sum
    train_accuracy: float  # train set accuracy
    test_accuracy: float  # test set accuracy
    loocv_calendar_sum: float  # LOOCV calendar sum
    confusion_matrix: np.ndarray  # 混淆矩阵
    feature_importance: dict[str, float]  # 特征重要性
    predictions_train: np.ndarray  # train 预测
    predictions_test: np.ndarray  # test 预测


# ---------------------------------------------------------------------------
# 函数接口
# ---------------------------------------------------------------------------


def loocv_calendar_sum(
    classifier_factory,
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
    events: pd.DataFrame,
    bars_cache: dict,
) -> float:
    """
    Leave-One-Out CV 评估函数，以 calendar_sum 为选择标准。

    每次留出一个 event，用剩余 events 训练分类器，
    对留出 event 预测 regime 并执行对应 delay，累计 calendar sum。

    Parameters
    ----------
    classifier_factory : callable
        无参数调用返回一个新的分类器实例（需有 fit/predict 接口）。
    features : pd.DataFrame
        训练特征矩阵 (n_events × n_features)。
    labels : pd.Series
        训练标签（Optimal_Delay_Label）。
    delay_results : list[list[DelayResult]]
        每个 event 在各 delay 下的执行结果。
    events : pd.DataFrame
        原始 events DataFrame（含 symbol、touch_time 等）。
    bars_cache : dict
        1s bar 缓存。

    Returns
    -------
    float
        LOOCV calendar sum（百分比）。
    """
    n = len(features)

    # Regime 到 delay_label 的映射（反向 LABEL_TO_REGIME）
    regime_to_delay_label: dict[str, str] = {v: k for k, v in LABEL_TO_REGIME.items()}

    # 收集每个 LOO 预测对应的 DelayResult
    loo_results: list[DelayResult] = []

    for i in range(n):
        # --- 训练集：排除第 i 个 event ---
        train_mask = np.ones(n, dtype=bool)
        train_mask[i] = False

        X_train = features.iloc[train_mask]
        y_train = labels.iloc[train_mask]

        X_test = features.iloc[[i]]

        # --- 训练分类器 ---
        clf = classifier_factory()
        clf.fit(X_train, y_train)

        # --- 预测第 i 个 event 的 regime ---
        predicted_label = clf.predict(X_test)[0]

        # --- 将预测的 label 映射到 delay_label ---
        # predicted_label 可能是 Optimal_Delay_Label 格式（"D0", "D5", ...）
        # 或者是 regime 格式（"immediate", "standard", ...）
        # 需要找到对应的 delay_label
        if predicted_label in regime_to_delay_label:
            # 预测的是 regime 名称 → 转为 delay_label
            target_delay_label = regime_to_delay_label[predicted_label]
        else:
            # 预测的直接是 delay_label（"D0", "D5", "D10", "D15", "pullback"）
            target_delay_label = predicted_label

        # --- 从 delay_results[i] 中查找对应的 DelayResult ---
        event_delay_results = delay_results[i]
        matched_result = None
        for dr in event_delay_results:
            if dr.delay_label == target_delay_label:
                matched_result = dr
                break

        if matched_result is None:
            # 未找到匹配的 delay result，创建一个 traded=False 的占位
            event_id = str(events.iloc[i].get("event_id", f"event_{i}"))
            matched_result = DelayResult(
                event_id=event_id,
                delay_label=target_delay_label,
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

        loo_results.append(matched_result)

    # --- 计算 silo-based calendar sum ---
    return _compute_calendar_sum_from_results(loo_results, events)


# ---------------------------------------------------------------------------
# 内部辅助函数：silo-based calendar sum 计算
# ---------------------------------------------------------------------------

_INITIAL_BALANCE = 100_000.0
_NOTIONAL_SHARE = 0.26


def _compute_calendar_sum_from_results(
    results: list[DelayResult],
    events: pd.DataFrame,
) -> float:
    """计算 silo-based calendar sum (%).

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。

    逻辑与 run_delay_simulation.compute_calendar_sum_from_results 完全一致。
    """
    # Build event_id -> symbol mapping
    event_symbol_map: dict[str, str] = {}
    for _, row in events.iterrows():
        eid = str(row.get("event_id", ""))
        event_symbol_map[eid] = str(row["symbol"])

    # Group trades by (symbol, month)
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        symbol = event_symbol_map.get(r.event_id, "unknown")
        entry_time = pd.Timestamp(r.entry_time)
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(r)

    total_return_pct = 0.0
    for _silo_key, silo_results in silos.items():
        balance = _INITIAL_BALANCE
        # Sort by entry_time
        sorted_results = sorted(silo_results, key=lambda r: r.entry_time)
        for r in sorted_results:
            notional = balance * _NOTIONAL_SHARE
            # realistic_pnl_pct already includes fees
            pnl = notional * r.pnl_pct
            balance += pnl
        silo_return = (balance - _INITIAL_BALANCE) / _INITIAL_BALANCE * 100.0
        total_return_pct += silo_return

    return total_return_pct


def _loocv_calendar_sum_inline(
    classifier_factory,
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
) -> float:
    """内联 LOOCV calendar_sum 计算（不依赖 events/bars_cache）。

    每次留出一个 event，用剩余 events 训练分类器，
    对留出 event 预测 label 并从 delay_results 查找对应结果，
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

        # 从 delay_results[i] 中查找对应的 DelayResult
        matched_result = _find_delay_result(delay_results[i], predicted_label)
        loo_results.append(matched_result)

    # 计算 calendar sum
    return _compute_calendar_sum_from_delay_results(loo_results)


def _find_delay_result(
    event_delay_results: list[DelayResult],
    predicted_label: str,
) -> DelayResult:
    """从 event 的 delay_results 中查找与 predicted_label 匹配的结果。

    predicted_label 可能是 delay_label 格式（"D0", "D5", ...）
    或 regime 格式（"immediate", "standard", ...）。
    """
    # 如果是 regime 格式，转为 delay_label
    regime_to_delay_label: dict[str, str] = {v: k for k, v in LABEL_TO_REGIME.items()}
    if predicted_label in regime_to_delay_label:
        target_label = regime_to_delay_label[predicted_label]
    else:
        target_label = predicted_label

    for dr in event_delay_results:
        if dr.delay_label == target_label:
            return dr

    # 未找到匹配，返回一个 traded=False 的占位
    event_id = event_delay_results[0].event_id if event_delay_results else "unknown"
    return DelayResult(
        event_id=event_id,
        delay_label=target_label,
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


def _collect_results_from_predictions(
    predictions: np.ndarray,
    delay_results: list[list[DelayResult]],
) -> list[DelayResult]:
    """根据预测标签从 delay_results 中收集对应的 DelayResult。"""
    results: list[DelayResult] = []
    for i, pred_label in enumerate(predictions):
        matched = _find_delay_result(delay_results[i], pred_label)
        results.append(matched)
    return results


def _compute_calendar_sum_from_delay_results(
    results: list[DelayResult],
) -> float:
    """从 DelayResult 列表计算 silo-based calendar sum (%)。

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。

    注意：此函数从 event_id 中推断 symbol（event_id 格式含 symbol 信息），
    若无法推断则使用 "unknown" 作为 symbol。
    """
    # Group trades by (symbol, month)
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        # 从 event_id 推断 symbol（格式通常为 "BTCUSDT_..." 或 "ETHUSDT_..."）
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
    """从 event_id 推断 symbol。

    event_id 格式通常为 "BTCUSDT_..." 或 "ETHUSDT_..."。
    若无法推断则返回 "unknown"。
    """
    eid_upper = event_id.upper()
    if "BTC" in eid_upper:
        return "BTCUSDT"
    elif "ETH" in eid_upper:
        return "ETHUSDT"
    return "unknown"


def train_rule_based_classifier(
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
) -> ClassifierResult:
    """
    训练规则型分类器（max_depth <= 3 的 DecisionTree）。

    使用浅层决策树作为规则型分类器，产出人类可读的 IF-THEN 规则。
    max_depth 限制为 3，确保规则简洁可解释。

    Parameters
    ----------
    features : pd.DataFrame
        训练特征矩阵。
    labels : pd.Series
        训练标签（Optimal_Delay_Label）。
    delay_results : list[list[DelayResult]]
        每个 event 在各 delay 下的执行结果。

    Returns
    -------
    ClassifierResult
        包含训练结果、混淆矩阵、特征重要性等。
    """
    from sklearn.metrics import confusion_matrix as sk_confusion_matrix
    from sklearn.tree import DecisionTreeClassifier

    max_depth = 3

    # --- 1. LOOCV 评估 ---
    loocv_cs = _loocv_calendar_sum_inline(
        lambda: DecisionTreeClassifier(max_depth=max_depth, random_state=42),
        features,
        labels,
        delay_results,
    )

    # --- 2. 在全部训练数据上 fit 最终模型 ---
    clf = DecisionTreeClassifier(max_depth=max_depth, random_state=42)
    clf.fit(features, labels)

    # --- 3. 训练集预测与 accuracy ---
    predictions_train = clf.predict(features)
    train_accuracy = float(np.mean(predictions_train == labels.values))

    # --- 4. 特征重要性 ---
    feature_importance: dict[str, float] = dict(
        zip(features.columns.tolist(), clf.feature_importances_.tolist())
    )

    # --- 5. 混淆矩阵 ---
    cm = sk_confusion_matrix(labels, predictions_train)

    # --- 6. 计算 train_calendar_sum ---
    # 使用全量 fit 模型在 train 上的预测，查找对应 delay_results，计算 calendar_sum
    train_results = _collect_results_from_predictions(predictions_train, delay_results)
    train_calendar_sum = _compute_calendar_sum_from_delay_results(train_results)

    # --- 7. 返回 ClassifierResult ---
    return ClassifierResult(
        name="RuleBased_DT3",
        best_params={"max_depth": max_depth},
        train_calendar_sum=train_calendar_sum,
        test_calendar_sum=0.0,  # 由 runner 后续填充
        train_accuracy=train_accuracy,
        test_accuracy=0.0,  # 由 runner 后续填充
        loocv_calendar_sum=loocv_cs,
        confusion_matrix=cm,
        feature_importance=feature_importance,
        predictions_train=predictions_train,
        predictions_test=np.array([]),  # 由 runner 后续填充
    )


def train_decision_tree(
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
    max_depths: list[int] = [2, 3, 4, 5],
) -> ClassifierResult:
    """
    训练 DecisionTreeClassifier，LOOCV 选 max_depth。

    对 max_depths 中的每个值训练一个 DecisionTree，
    使用 LOOCV calendar_sum 选择最优 max_depth。

    Parameters
    ----------
    features : pd.DataFrame
        训练特征矩阵。
    labels : pd.Series
        训练标签（Optimal_Delay_Label）。
    delay_results : list[list[DelayResult]]
        每个 event 在各 delay 下的执行结果。
    max_depths : list[int]
        候选 max_depth 值列表。

    Returns
    -------
    ClassifierResult
        使用最优 max_depth 训练的分类器结果。
    """
    from sklearn.metrics import confusion_matrix as sk_confusion_matrix
    from sklearn.tree import DecisionTreeClassifier

    # --- 1. 对每个 max_depth 计算 LOOCV calendar_sum，选最优 ---
    best_depth = max_depths[0]
    best_loocv_cs = -np.inf

    for depth in max_depths:
        loocv_cs = _loocv_calendar_sum_inline(
            lambda d=depth: DecisionTreeClassifier(max_depth=d, random_state=42),
            features,
            labels,
            delay_results,
        )
        if loocv_cs > best_loocv_cs:
            best_loocv_cs = loocv_cs
            best_depth = depth

    # --- 2. 在全部训练数据上 fit 最终模型（使用最优 max_depth）---
    clf = DecisionTreeClassifier(max_depth=best_depth, random_state=42)
    clf.fit(features, labels)

    # --- 3. 训练集预测与 accuracy ---
    predictions_train = clf.predict(features)
    train_accuracy = float(np.mean(predictions_train == labels.values))

    # --- 4. 特征重要性 ---
    feature_importance: dict[str, float] = dict(
        zip(features.columns.tolist(), clf.feature_importances_.tolist())
    )

    # --- 5. 混淆矩阵 ---
    cm = sk_confusion_matrix(labels, predictions_train)

    # --- 6. 计算 train_calendar_sum ---
    train_results = _collect_results_from_predictions(predictions_train, delay_results)
    train_calendar_sum = _compute_calendar_sum_from_delay_results(train_results)

    # --- 7. 返回 ClassifierResult ---
    return ClassifierResult(
        name="DecisionTree",
        best_params={"max_depth": best_depth},
        train_calendar_sum=train_calendar_sum,
        test_calendar_sum=0.0,  # 由 runner 后续填充
        train_accuracy=train_accuracy,
        test_accuracy=0.0,  # 由 runner 后续填充
        loocv_calendar_sum=best_loocv_cs,
        confusion_matrix=cm,
        feature_importance=feature_importance,
        predictions_train=predictions_train,
        predictions_test=np.array([]),  # 由 runner 后续填充
    )


def train_random_forest(
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
    max_depths: list[int] = [3, 4, 5],
) -> ClassifierResult:
    """
    训练 RandomForestClassifier，LOOCV 选 max_depth。

    使用 n_estimators=100，random_state=42，
    对 max_depths 中的每个值训练一个 RandomForest，
    使用 LOOCV calendar_sum 选择最优 max_depth。

    Parameters
    ----------
    features : pd.DataFrame
        训练特征矩阵。
    labels : pd.Series
        训练标签（Optimal_Delay_Label）。
    delay_results : list[list[DelayResult]]
        每个 event 在各 delay 下的执行结果。
    max_depths : list[int]
        候选 max_depth 值列表。

    Returns
    -------
    ClassifierResult
        使用最优 max_depth 训练的分类器结果。
    """
    from sklearn.ensemble import RandomForestClassifier
    from sklearn.metrics import confusion_matrix as sk_confusion_matrix

    # --- 1. 对每个 max_depth 计算 LOOCV calendar_sum，选最优 ---
    best_depth = max_depths[0]
    best_loocv_cs = -np.inf

    for depth in max_depths:
        loocv_cs = _loocv_calendar_sum_inline(
            lambda d=depth: RandomForestClassifier(
                n_estimators=100, max_depth=d, random_state=42
            ),
            features,
            labels,
            delay_results,
        )
        if loocv_cs > best_loocv_cs:
            best_loocv_cs = loocv_cs
            best_depth = depth

    # --- 2. 在全部训练数据上 fit 最终模型（使用最优 max_depth）---
    clf = RandomForestClassifier(
        n_estimators=100, max_depth=best_depth, random_state=42
    )
    clf.fit(features, labels)

    # --- 3. 训练集预测与 accuracy ---
    predictions_train = clf.predict(features)
    train_accuracy = float(np.mean(predictions_train == labels.values))

    # --- 4. 特征重要性 ---
    feature_importance: dict[str, float] = dict(
        zip(features.columns.tolist(), clf.feature_importances_.tolist())
    )

    # --- 5. 混淆矩阵 ---
    cm = sk_confusion_matrix(labels, predictions_train)

    # --- 6. 计算 train_calendar_sum ---
    train_results = _collect_results_from_predictions(predictions_train, delay_results)
    train_calendar_sum = _compute_calendar_sum_from_delay_results(train_results)

    # --- 7. 返回 ClassifierResult ---
    return ClassifierResult(
        name="RandomForest",
        best_params={"max_depth": best_depth, "n_estimators": 100},
        train_calendar_sum=train_calendar_sum,
        test_calendar_sum=0.0,  # 由 runner 后续填充
        train_accuracy=train_accuracy,
        test_accuracy=0.0,  # 由 runner 后续填充
        loocv_calendar_sum=best_loocv_cs,
        confusion_matrix=cm,
        feature_importance=feature_importance,
        predictions_train=predictions_train,
        predictions_test=np.array([]),  # 由 runner 后续填充
    )


def train_logistic_regression(
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
) -> ClassifierResult:
    """
    训练 LogisticRegression (multinomial)。

    使用 multi_class='multinomial'，solver='lbfgs'，random_state=42，max_iter=1000。
    特征标准化后训练（Pipeline: StandardScaler + LogisticRegression），
    系数绝对值的跨类别均值用于 feature_importance。

    Parameters
    ----------
    features : pd.DataFrame
        训练特征矩阵。
    labels : pd.Series
        训练标签（Optimal_Delay_Label）。
    delay_results : list[list[DelayResult]]
        每个 event 在各 delay 下的执行结果。

    Returns
    -------
    ClassifierResult
        训练结果，feature_importance 基于系数绝对值排序。
    """
    from sklearn.linear_model import LogisticRegression
    from sklearn.metrics import confusion_matrix as sk_confusion_matrix
    from sklearn.pipeline import Pipeline
    from sklearn.preprocessing import StandardScaler

    # --- 1. LOOCV 评估（Pipeline 确保标准化在 LOO 循环内）---
    def _make_pipeline():
        return Pipeline([
            ("scaler", StandardScaler()),
            ("clf", LogisticRegression(
                multi_class="multinomial",
                solver="lbfgs",
                random_state=42,
                max_iter=1000,
            )),
        ])

    loocv_cs = _loocv_calendar_sum_inline(
        _make_pipeline,
        features,
        labels,
        delay_results,
    )

    # --- 2. 在全部训练数据上 fit 最终模型（Pipeline: StandardScaler + LR）---
    final_pipeline = _make_pipeline()
    final_pipeline.fit(features, labels)

    # --- 3. 训练集预测与 accuracy ---
    predictions_train = final_pipeline.predict(features)
    train_accuracy = float(np.mean(predictions_train == labels.values))

    # --- 4. 特征重要性：各类别系数绝对值的均值 ---
    lr_model = final_pipeline.named_steps["clf"]
    # lr_model.coef_ shape: (n_classes, n_features)
    mean_abs_coef = np.mean(np.abs(lr_model.coef_), axis=0)
    feature_importance: dict[str, float] = dict(
        zip(features.columns.tolist(), mean_abs_coef.tolist())
    )

    # --- 5. 混淆矩阵 ---
    cm = sk_confusion_matrix(labels, predictions_train)

    # --- 6. 计算 train_calendar_sum ---
    train_results = _collect_results_from_predictions(predictions_train, delay_results)
    train_calendar_sum = _compute_calendar_sum_from_delay_results(train_results)

    # --- 7. 返回 ClassifierResult ---
    return ClassifierResult(
        name="LogisticRegression",
        best_params={"solver": "lbfgs", "multi_class": "multinomial"},
        train_calendar_sum=train_calendar_sum,
        test_calendar_sum=0.0,  # 由 runner 后续填充
        train_accuracy=train_accuracy,
        test_accuracy=0.0,  # 由 runner 后续填充
        loocv_calendar_sum=loocv_cs,
        confusion_matrix=cm,
        feature_importance=feature_importance,
        predictions_train=predictions_train,
        predictions_test=np.array([]),  # 由 runner 后续填充
    )


def extract_rules_text(tree_model, feature_names: list[str]) -> str:
    """
    从 DecisionTree 提取人类可读的中文 IF-THEN 规则描述。

    遍历决策树的每条路径，将分裂条件翻译为中文规则，
    格式为 IF-THEN 规则链，便于后续 live 实现。

    Parameters
    ----------
    tree_model : sklearn.tree.DecisionTreeClassifier
        已训练的决策树模型。
    feature_names : list[str]
        特征名称列表，与训练时的列顺序一致。

    Returns
    -------
    str
        中文 IF-THEN 规则描述文本。
    """
    from sklearn.tree._tree import TREE_LEAF, TREE_UNDEFINED  # noqa: F401

    tree = tree_model.tree_
    classes = tree_model.classes_

    # Regime 中文名称映射
    regime_display: dict[str, str] = {
        "D0": "立即入场 (D=0s)",
        "D5": "标准延迟 (D=5s)",
        "D10": "延迟入场 (D=10s)",
        "D15": "扩展延迟 (D=15s)",
        "pullback": "等待回调",
    }

    # --- 收集所有从根到叶的路径 ---
    paths: list[tuple[list[tuple[str, str, float]], int, np.ndarray]] = []
    # 每条路径: (conditions, leaf_node_id, leaf_value)
    # conditions: list of (feature_name, operator, threshold)

    def _traverse(node_id: int, conditions: list[tuple[str, str, float]]):
        """递归遍历树，收集每条根到叶的路径。"""
        left_child = tree.children_left[node_id]
        right_child = tree.children_right[node_id]

        # 叶节点：children_left == TREE_LEAF (-1)
        if left_child == TREE_LEAF:
            paths.append((list(conditions), node_id, tree.value[node_id]))
            return

        # 内部节点：获取分裂特征和阈值
        feat_idx = tree.feature[node_id]
        threshold = tree.threshold[node_id]
        feat_name = feature_names[feat_idx]

        # 左子树：feature <= threshold
        conditions.append((feat_name, "≤", threshold))
        _traverse(left_child, conditions)
        conditions.pop()

        # 右子树：feature > threshold
        conditions.append((feat_name, ">", threshold))
        _traverse(right_child, conditions)
        conditions.pop()

    _traverse(0, [])

    # --- 格式化为中文规则 ---
    rules_lines: list[str] = []

    for rule_idx, (conditions, node_id, leaf_value) in enumerate(paths, start=1):
        # 确定预测类别
        class_proportions = leaf_value[0]  # shape: (n_classes,) — 可能是比例或计数
        total_samples = int(tree.n_node_samples[node_id])
        predicted_class_idx = int(np.argmax(class_proportions))
        predicted_class = str(classes[predicted_class_idx])
        # 置信度：该类别的比例
        proportion_sum = class_proportions.sum()
        if proportion_sum > 0:
            confidence = class_proportions[predicted_class_idx] / proportion_sum * 100.0
        else:
            confidence = 0.0

        # 获取中文 regime 名称
        regime_name = regime_display.get(predicted_class, predicted_class)

        # 构建条件文本
        if not conditions:
            # 根节点即为叶（depth=0 的退化情况）
            condition_text = "（无条件）"
        else:
            parts: list[str] = []
            for i, (feat_name, operator, threshold) in enumerate(conditions):
                if i == 0:
                    parts.append(f"若 {feat_name} {operator} {threshold:.2f}")
                else:
                    parts.append(f"且 {feat_name} {operator} {threshold:.2f}")
            condition_text = " ".join(parts)

        # 格式化规则
        rules_lines.append(f"规则 {rule_idx}: {condition_text}")
        rules_lines.append(f"  → {regime_name}")
        rules_lines.append(
            f"  [训练样本: {total_samples} 个, 置信度: {confidence:.1f}%]"
        )
        rules_lines.append("")  # 空行分隔

    return "\n".join(rules_lines)
