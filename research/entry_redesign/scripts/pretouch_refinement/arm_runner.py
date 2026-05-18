"""
arm_runner — 6 个实验 arm × 4 种分类器的训练与评估

复用 pre_breakout_timing.timing_classifier 的训练函数和 ClassifierResult。
"""

from __future__ import annotations

import json
import logging
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

import numpy as np
import pandas as pd

# ---------------------------------------------------------------------------
# Path setup for importing pre_breakout_timing
# ---------------------------------------------------------------------------

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pre_breakout_timing.delay_simulator import DelayResult
from pre_breakout_timing.timing_classifier import (
    ClassifierResult,
    _find_delay_result,
    _compute_calendar_sum_from_delay_results,
    train_decision_tree,
    train_logistic_regression,
    train_random_forest,
    train_rule_based_classifier,
)


# ---------------------------------------------------------------------------
# Arm 定义
# ---------------------------------------------------------------------------


@dataclass
class ArmConfig:
    """实验 arm 配置"""

    name: str  # arm 名称
    feature_set: str  # "original" | "original+enhanced"
    regime_schema: str  # "5-regime" | "3-regime" | "2-regime"
    description: str  # 中文描述


ARM_CONFIGS: list[ArmConfig] = [
    ArmConfig(
        "Baseline", "original", "5-regime", "复用 pre-breakout-timing-classifier 结果"
    ),
    ArmConfig("A1", "original+enhanced", "5-regime", "Path A 单独：增强特征 + 5-regime"),
    ArmConfig("A2", "original", "3-regime", "Path B 单独：原特征 + 三分类"),
    ArmConfig("A3", "original", "2-regime", "Path B 单独：原特征 + 二分类"),
    ArmConfig("A4", "original+enhanced", "3-regime", "Path A+B：增强特征 + 三分类"),
    ArmConfig("A5", "original+enhanced", "2-regime", "Path A+B：增强特征 + 二分类"),
]


# ---------------------------------------------------------------------------
# Arm 结果
# ---------------------------------------------------------------------------


@dataclass
class ArmResult:
    """单个 arm 的完整结果"""

    config: ArmConfig
    classifier_results: list  # list[ClassifierResult]，4 种分类器
    best_classifier: object  # ClassifierResult，LOOCV 最优
    test_calendar_sum: float  # best classifier 的 test 结果
    oracle_realization_rate: float  # Oracle 实现率


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def run_single_arm(
    config: ArmConfig,
    original_features_train: pd.DataFrame,
    original_features_test: pd.DataFrame,
    enhanced_features_train: pd.DataFrame | None,
    enhanced_features_test: pd.DataFrame | None,
    labels: pd.Series,
    delay_results_train: list[list[DelayResult]],
    delay_results_test: list[list[DelayResult]],
    test_events: pd.DataFrame,
    oracle_calendar_sum: float,
) -> ArmResult:
    """运行单个 arm 的 4 种分类器训练与评估。

    Parameters
    ----------
    config : ArmConfig
        arm 配置。
    original_features_train/test : pd.DataFrame
        原始 10 特征（已 impute）。
    enhanced_features_train/test : pd.DataFrame | None
        增强特征（已 impute）；若 config.feature_set == "original" 则为 None。
    labels : pd.Series
        该 arm 对应 regime schema 的训练标签（已排除 skip）。
    delay_results_train/test : list[list[DelayResult]]
        从 delay_pnl_matrix 重建的 DelayResult 列表。
    test_events : pd.DataFrame
        测试集 events。
    oracle_calendar_sum : float
        Oracle 理论上限。

    Returns
    -------
    ArmResult
        包含 4 种分类器结果和最优选择。
    """
    # --- 1. 组装特征矩阵 ---
    if config.feature_set == "original+enhanced":
        if enhanced_features_train is None or enhanced_features_test is None:
            raise ValueError(
                f"Arm '{config.name}' 需要增强特征，但 enhanced_features 为 None。"
            )
        features_train = pd.concat(
            [original_features_train.reset_index(drop=True),
             enhanced_features_train.reset_index(drop=True)],
            axis=1,
        )
        features_test = pd.concat(
            [original_features_test.reset_index(drop=True),
             enhanced_features_test.reset_index(drop=True)],
            axis=1,
        )
    else:
        # "original" — 仅使用原始特征
        features_train = original_features_train.reset_index(drop=True)
        features_test = original_features_test.reset_index(drop=True)

    # --- 2. 过滤 skip 标签（仅 5-regime 和 3-regime 需要过滤 skip）---
    # 对 2-regime，skip 是有效的预测类别（enter/skip），不应过滤
    if config.regime_schema == "2-regime":
        # 2-regime: 保留所有标签（enter + skip 都参与训练）
        train_mask = pd.Series([True] * len(labels))
    else:
        # 5-regime / 3-regime: skip 不参与训练
        train_mask = labels != "skip"
    features_train_filtered = features_train.loc[train_mask].reset_index(drop=True)
    labels_filtered = labels.loc[train_mask].reset_index(drop=True)
    # 对应的 delay_results 也需要过滤
    delay_results_train_filtered = [
        delay_results_train[i]
        for i, keep in enumerate(train_mask)
        if keep
    ]

    # --- 3. 训练 4 种分类器 ---
    classifier_results: list[ClassifierResult] = []

    # 3.1 RuleBased_DT3
    result_dt3 = train_rule_based_classifier(
        features_train_filtered, labels_filtered, delay_results_train_filtered
    )
    classifier_results.append(result_dt3)

    # 3.2 DecisionTree (LOOCV 选 max_depth ∈ {2,3,4,5})
    result_dt = train_decision_tree(
        features_train_filtered, labels_filtered, delay_results_train_filtered,
        max_depths=[2, 3, 4, 5],
    )
    classifier_results.append(result_dt)

    # 3.3 RandomForest (n_estimators=100, LOOCV 选 max_depth ∈ {3,4,5})
    result_rf = train_random_forest(
        features_train_filtered, labels_filtered, delay_results_train_filtered,
        max_depths=[3, 4, 5],
    )
    classifier_results.append(result_rf)

    # 3.4 LogisticRegression (multinomial, solver=lbfgs)
    result_lr = train_logistic_regression(
        features_train_filtered, labels_filtered, delay_results_train_filtered
    )
    classifier_results.append(result_lr)

    # --- 4. 选最优分类器（LOOCV calendar_sum 最高）---
    best_classifier = max(classifier_results, key=lambda r: r.loocv_calendar_sum)

    # --- 5. 评估 best classifier 在 test set 上的表现 ---
    test_calendar_sum = _evaluate_best_on_test(
        best_classifier, features_train_filtered, labels_filtered,
        features_test, delay_results_test, config,
    )
    best_classifier.test_calendar_sum = test_calendar_sum

    # --- 6. 计算 Oracle 实现率 ---
    if oracle_calendar_sum != 0.0:
        oracle_realization_rate = test_calendar_sum / oracle_calendar_sum * 100.0
    else:
        oracle_realization_rate = 0.0

    return ArmResult(
        config=config,
        classifier_results=classifier_results,
        best_classifier=best_classifier,
        test_calendar_sum=test_calendar_sum,
        oracle_realization_rate=oracle_realization_rate,
    )


def run_all_arms(
    arm_configs: list[ArmConfig],
    original_features_train: pd.DataFrame,
    original_features_test: pd.DataFrame,
    enhanced_features_train: pd.DataFrame | None,
    enhanced_features_test: pd.DataFrame | None,
    labels_5regime: pd.Series,
    labels_3regime: pd.Series,
    labels_2regime: pd.Series,
    delay_results_train: list[list[DelayResult]],
    delay_results_test: list[list[DelayResult]],
    test_events: pd.DataFrame,
    oracle_calendar_sum: float,
    baseline_legacy_result: ClassifierResult | None = None,
) -> list[ArmResult]:
    """运行全部 6 个 arm。

    Baseline arm 直接复用 pre_breakout_timing_summary.json 的结果，
    不重新训练。A1-A5 调用 run_single_arm()。

    Parameters
    ----------
    arm_configs : list[ArmConfig]
        6 个 arm 配置（ARM_CONFIGS）。
    original_features_train/test : pd.DataFrame
        原始 10 特征（已 impute）。
    enhanced_features_train/test : pd.DataFrame | None
        增强特征（已 impute）。
    labels_5regime : pd.Series
        5-regime 标签（含 skip）。
    labels_3regime : pd.Series
        3-regime 标签（skip/fast/slow）。
    labels_2regime : pd.Series
        2-regime 标签（enter/skip）。
    delay_results_train/test : list[list[DelayResult]]
        从 delay_pnl_matrix 重建的 DelayResult 列表。
    test_events : pd.DataFrame
        测试集 events。
    oracle_calendar_sum : float
        Oracle 理论上限。
    baseline_legacy_result : ClassifierResult | None
        从 pre_breakout_timing_summary.json 构建的 baseline ClassifierResult。
        若为 None，则从默认路径读取 summary.json。

    Returns
    -------
    list[ArmResult]
        6 个 arm 的完整结果。
    """
    logger = logging.getLogger(__name__)
    arm_results: list[ArmResult] = []

    for config in arm_configs:
        if config.name == "Baseline":
            # --- Baseline arm: 直接复用 pre_breakout_timing_summary.json ---
            baseline_result = _build_baseline_arm_result(
                config, oracle_calendar_sum, baseline_legacy_result
            )
            arm_results.append(baseline_result)
            logger.info(
                f"Baseline arm: test_calendar_sum={baseline_result.test_calendar_sum:.4f}%, "
                f"oracle_realization_rate={baseline_result.oracle_realization_rate:.2f}%"
            )
        else:
            # --- A1-A5: 根据 regime_schema 选择标签 ---
            labels = _select_labels_for_arm(
                config, labels_5regime, labels_3regime, labels_2regime
            )

            arm_result = run_single_arm(
                config=config,
                original_features_train=original_features_train,
                original_features_test=original_features_test,
                enhanced_features_train=enhanced_features_train,
                enhanced_features_test=enhanced_features_test,
                labels=labels,
                delay_results_train=delay_results_train,
                delay_results_test=delay_results_test,
                test_events=test_events,
                oracle_calendar_sum=oracle_calendar_sum,
            )
            arm_results.append(arm_result)
            logger.info(
                f"Arm {config.name}: best={arm_result.best_classifier.name}, "
                f"test_calendar_sum={arm_result.test_calendar_sum:.4f}%, "
                f"oracle_realization_rate={arm_result.oracle_realization_rate:.2f}%"
            )

    # --- 选出 best_arm_classifier（test_calendar_sum 最高）---
    best_arm = max(arm_results, key=lambda r: r.test_calendar_sum)
    logger.info(
        f"Best arm: {best_arm.config.name} ({best_arm.best_classifier.name}), "
        f"test_calendar_sum={best_arm.test_calendar_sum:.4f}%"
    )

    # --- 检查 accuracy_calendar_decoupled (Req 4.5) ---
    baseline_legacy_cs = arm_results[0].test_calendar_sum  # Baseline arm
    best_test_accuracy = getattr(best_arm.best_classifier, "test_accuracy", None)
    if (
        best_test_accuracy is not None
        and best_test_accuracy < 0.40
        and best_arm.test_calendar_sum > baseline_legacy_cs
    ):
        logger.warning(
            f"accuracy_calendar_decoupled=true: best arm test_accuracy="
            f"{best_test_accuracy:.2%} < 40%, 但 test_calendar_sum="
            f"{best_arm.test_calendar_sum:.4f}% > baseline {baseline_legacy_cs:.4f}%"
        )

    # --- 产出 arm_comparison.csv（24 行 = 6 arms × 4 classifiers）---
    _generate_arm_comparison_csv(arm_results)

    return arm_results


def _build_baseline_arm_result(
    config: ArmConfig,
    oracle_calendar_sum: float,
    baseline_legacy_result: ClassifierResult | None = None,
) -> ArmResult:
    """从 pre_breakout_timing_summary.json 构建 Baseline arm 的 ArmResult。

    不重新训练，直接包装已有结果。
    """
    if baseline_legacy_result is not None:
        # 使用外部传入的 ClassifierResult
        summary = None
    else:
        # 从默认路径读取 summary.json
        summary_path = (
            Path(__file__).resolve().parent.parent
            / "output"
            / "pre_breakout_timing"
            / "pre_breakout_timing_summary.json"
        )
        if not summary_path.exists():
            raise FileNotFoundError(
                f"Baseline summary 文件不存在: {summary_path}"
            )
        with open(summary_path, "r", encoding="utf-8") as f:
            summary = json.load(f)

    if summary is not None:
        # 从 JSON 构建 ClassifierResult 列表（4 种分类器）
        classifier_results: list[ClassifierResult] = []
        for clf_data in summary["all_classifiers"]:
            cr = ClassifierResult(
                name=clf_data["name"],
                best_params={},
                train_calendar_sum=clf_data["train_calendar_sum"],
                test_calendar_sum=clf_data["test_calendar_sum"],
                train_accuracy=clf_data["train_accuracy"],
                test_accuracy=clf_data["test_accuracy"],
                loocv_calendar_sum=clf_data["loocv_calendar_sum"],
                confusion_matrix=np.array([]),
                feature_importance={},
                predictions_train=np.array([]),
                predictions_test=np.array([]),
            )
            classifier_results.append(cr)

        # best_classifier 从 summary 中获取
        best_clf_data = summary["best_classifier"]
        best_classifier = ClassifierResult(
            name=best_clf_data["name"],
            best_params=best_clf_data["best_params"],
            train_calendar_sum=best_clf_data["train_calendar_sum"],
            test_calendar_sum=best_clf_data["test_calendar_sum"],
            train_accuracy=best_clf_data["train_accuracy"],
            test_accuracy=best_clf_data["test_accuracy"],
            loocv_calendar_sum=best_clf_data["loocv_calendar_sum"],
            confusion_matrix=np.array([]),
            feature_importance={},
            predictions_train=np.array([]),
            predictions_test=np.array([]),
        )
        test_calendar_sum = best_clf_data["test_calendar_sum"]
    else:
        # 使用外部传入的 baseline_legacy_result
        best_classifier = baseline_legacy_result
        test_calendar_sum = baseline_legacy_result.test_calendar_sum
        # 构建 4 个 classifier_results（仅有 best 有完整数据）
        classifier_results = [baseline_legacy_result]

    # 计算 Oracle 实现率
    if oracle_calendar_sum != 0.0:
        oracle_realization_rate = test_calendar_sum / oracle_calendar_sum * 100.0
    else:
        oracle_realization_rate = 0.0

    return ArmResult(
        config=config,
        classifier_results=classifier_results,
        best_classifier=best_classifier,
        test_calendar_sum=test_calendar_sum,
        oracle_realization_rate=oracle_realization_rate,
    )


def _select_labels_for_arm(
    config: ArmConfig,
    labels_5regime: pd.Series,
    labels_3regime: pd.Series,
    labels_2regime: pd.Series,
) -> pd.Series:
    """根据 arm 的 regime_schema 选择对应标签。"""
    if config.regime_schema == "5-regime":
        return labels_5regime
    elif config.regime_schema == "3-regime":
        return labels_3regime
    elif config.regime_schema == "2-regime":
        return labels_2regime
    else:
        raise ValueError(f"未知的 regime_schema: {config.regime_schema}")


def _generate_arm_comparison_csv(arm_results: list[ArmResult]) -> None:
    """产出 arm_comparison.csv（24 行 = 6 arms × 4 classifiers）。

    Columns: arm_name, feature_set, regime_schema, classifier_name,
             loocv_calendar_sum, train_calendar_sum, test_calendar_sum, accuracy
    """
    output_dir = (
        Path(__file__).resolve().parent.parent / "output" / "pretouch_refinement"
    )
    output_dir.mkdir(parents=True, exist_ok=True)

    rows: list[dict] = []
    for arm_result in arm_results:
        config = arm_result.config
        for cr in arm_result.classifier_results:
            rows.append({
                "arm_name": config.name,
                "feature_set": config.feature_set,
                "regime_schema": config.regime_schema,
                "classifier_name": cr.name,
                "loocv_calendar_sum": cr.loocv_calendar_sum,
                "train_calendar_sum": cr.train_calendar_sum,
                "test_calendar_sum": cr.test_calendar_sum,
                "accuracy": cr.test_accuracy,
            })

    df = pd.DataFrame(rows)
    csv_path = output_dir / "arm_comparison.csv"
    df.to_csv(csv_path, index=False)
    logging.getLogger(__name__).info(
        f"arm_comparison.csv 已生成: {csv_path} ({len(df)} 行)"
    )


def _evaluate_best_on_test(
    best_result: ClassifierResult,
    features_train: pd.DataFrame,
    labels_train: pd.Series,
    features_test: pd.DataFrame,
    delay_results_test: list[list[DelayResult]],
    config: ArmConfig,
) -> float:
    """重新训练 best classifier 并在 test set 上评估 calendar_sum。

    使用 best_result.best_params 重建分类器，在全量 train 上 fit，
    然后对 test set 预测并计算 calendar_sum。

    对不同 regime_schema 的预测结果映射到 delay_results 的逻辑：
    - 5-regime: 预测的 label 直接对应 delay_label (D0/D5/D10/D15/pullback)
    - 3-regime: fast → 使用该 event 在 {D0, D5} 中 pnl 最高的 delay
                slow → 使用该 event 在 {D10, D15, pullback} 中 pnl 最高的 delay
                skip → 不入场 (pnl=0)
    - 2-regime: enter → 使用 Best_Global_Delay 对应的 delay
                skip → 不入场 (pnl=0)
    """
    from sklearn.ensemble import RandomForestClassifier
    from sklearn.linear_model import LogisticRegression
    from sklearn.pipeline import Pipeline
    from sklearn.preprocessing import StandardScaler
    from sklearn.tree import DecisionTreeClassifier

    # 重建分类器
    name = best_result.name
    params = best_result.best_params

    if name == "RuleBased_DT3":
        clf = DecisionTreeClassifier(max_depth=3, random_state=42)
        clf.fit(features_train, labels_train)
        predictions = clf.predict(features_test)
    elif name == "DecisionTree":
        clf = DecisionTreeClassifier(max_depth=params["max_depth"], random_state=42)
        clf.fit(features_train, labels_train)
        predictions = clf.predict(features_test)
    elif name == "RandomForest":
        clf = RandomForestClassifier(
            n_estimators=params.get("n_estimators", 100),
            max_depth=params["max_depth"],
            random_state=42,
        )
        clf.fit(features_train, labels_train)
        predictions = clf.predict(features_test)
    elif name == "LogisticRegression":
        pipeline = Pipeline([
            ("scaler", StandardScaler()),
            ("clf", LogisticRegression(
                multi_class="multinomial",
                solver="lbfgs",
                random_state=42,
                max_iter=1000,
            )),
        ])
        pipeline.fit(features_train, labels_train)
        predictions = pipeline.predict(features_test)
    else:
        raise ValueError(f"未知分类器名称: {name}")

    # 将预测映射到 DelayResult 并计算 calendar_sum
    test_results: list[DelayResult] = []
    regime_schema = config.regime_schema

    for i, pred_label in enumerate(predictions):
        event_delays = delay_results_test[i]

        if regime_schema == "3-regime":
            matched = _resolve_3regime_prediction(pred_label, event_delays)
        elif regime_schema == "2-regime":
            matched = _resolve_2regime_prediction(pred_label, event_delays, config)
        else:
            # 5-regime: 直接使用 _find_delay_result
            matched = _find_delay_result(event_delays, pred_label)

        test_results.append(matched)

    return _compute_calendar_sum_from_delay_results(test_results)


def _resolve_3regime_prediction(
    pred_label: str,
    event_delays: list[DelayResult],
) -> DelayResult:
    """3-regime 预测映射：fast/slow/skip → 对应组内最优 delay。

    - fast → max(D0_pnl, D5_pnl) 对应的 DelayResult
    - slow → max(D10_pnl, D15_pnl, pullback_pnl) 对应的 DelayResult
    - skip → traded=False 占位
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

    fast_labels = {"D0", "D5"}
    slow_labels = {"D10", "D15", "pullback"}

    if pred_label == "fast":
        target_labels = fast_labels
    elif pred_label == "slow":
        target_labels = slow_labels
    else:
        # 未知标签，尝试直接匹配
        return _find_delay_result(event_delays, pred_label)

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


def _resolve_2regime_prediction(
    pred_label: str,
    event_delays: list[DelayResult],
    config: ArmConfig,
) -> DelayResult:
    """2-regime 预测映射：enter/skip → Best_Global_Delay 或不入场。

    - enter → 使用 Best_Global_Delay 对应的 DelayResult
    - skip → traded=False 占位

    注意：Best_Global_Delay 存储在 config 中（通过外部传入），
    若 config 没有该属性，默认使用 "D0"。
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

    # enter → 使用 Best_Global_Delay
    best_global_delay = getattr(config, "best_global_delay", "D0")
    return _find_delay_result(event_delays, best_global_delay)


def rebuild_delay_results_from_matrix(
    matrix: pd.DataFrame,
    events: pd.DataFrame,
) -> list[list[DelayResult]]:
    """从 delay_pnl_matrix.csv 重建 DelayResult 列表。

    将 CSV 行转换为 DelayResult 对象，按 event 分组，
    保持与 pre_breakout_timing_runner 中 all_delay_results 相同的结构。

    Parameters
    ----------
    matrix : pd.DataFrame
        delay_pnl_matrix（580行 = 116 events × 5 delays），
        列包含 event_id, delay_label, delay_seconds, entry_time, entry_price,
        pnl_pct, exit_reason, exit_time, hold_seconds, mfe_r, mae_r, traded。
    events : pd.DataFrame
        116 events（用于确定顺序）。

    Returns
    -------
    list[list[DelayResult]]
        外层 list 对应每个 event（116个），按 events 顺序排列，
        内层 list 为该 event 的 5 个 DelayResult（D0, D5, D10, D15, pullback）。

    Raises
    ------
    ValueError
        当重建后的 DelayResult 总数与 matrix 行数不一致时抛出。
    """
    # 按 event_id 分组 matrix 行
    grouped = matrix.groupby("event_id", sort=False)

    # 确定 delay 标签的排序顺序（与 simulate_all_delays 产出一致）
    delay_order = ["D0", "D5", "D10", "D15", "pullback"]

    all_delay_results: list[list[DelayResult]] = []

    for _, event_row in events.iterrows():
        event_id = str(event_row["event_id"])

        if event_id not in grouped.groups:
            raise ValueError(
                f"event_id '{event_id}' 在 matrix 中不存在。"
            )

        event_rows = grouped.get_group(event_id)

        # 按 delay_order 排序，确保内层 list 顺序为 D0, D5, D10, D15, pullback
        event_delay_results: list[DelayResult] = []
        for delay_label in delay_order:
            row_mask = event_rows["delay_label"] == delay_label
            if not row_mask.any():
                raise ValueError(
                    f"event_id '{event_id}' 缺少 delay_label '{delay_label}'。"
                )
            row = event_rows.loc[row_mask].iloc[0]

            # 解析时间戳字段（可能为 NaN/NaT）
            entry_time = _parse_timestamp(row.get("entry_time"))
            exit_time = _parse_timestamp(row.get("exit_time"))

            # 解析可选数值字段（NaN → None）
            entry_price = _parse_optional_float(row.get("entry_price"))
            pnl_pct = _parse_optional_float(row.get("pnl_pct"))
            hold_seconds = _parse_optional_float(row.get("hold_seconds"))
            mfe_r = _parse_optional_float(row.get("mfe_r"))
            mae_r = _parse_optional_float(row.get("mae_r"))

            # 解析 exit_reason（NaN → None）
            exit_reason = row.get("exit_reason")
            if pd.isna(exit_reason):
                exit_reason = None
            else:
                exit_reason = str(exit_reason)

            # traded 字段
            traded = bool(row["traded"])

            # delay_seconds
            delay_seconds = int(row["delay_seconds"])

            dr = DelayResult(
                event_id=event_id,
                delay_label=str(row["delay_label"]),
                delay_seconds=delay_seconds,
                entry_time=entry_time,
                entry_price=entry_price,
                pnl_pct=pnl_pct,
                exit_reason=exit_reason,
                exit_time=exit_time,
                hold_seconds=hold_seconds,
                mfe_r=mfe_r,
                mae_r=mae_r,
                traded=traded,
            )
            event_delay_results.append(dr)

        all_delay_results.append(event_delay_results)

    # 验证重建后的 DelayResult 总数与 matrix 行数一致
    total_rebuilt = sum(len(inner) for inner in all_delay_results)
    if total_rebuilt != len(matrix):
        raise ValueError(
            f"重建后的 DelayResult 总数 ({total_rebuilt}) 与 matrix 行数 ({len(matrix)}) 不一致。"
        )

    return all_delay_results


# ---------------------------------------------------------------------------
# 内部辅助函数
# ---------------------------------------------------------------------------


def _parse_timestamp(value) -> pd.Timestamp | None:
    """将 CSV 中的时间戳值解析为 pd.Timestamp 或 None。"""
    if value is None or (isinstance(value, float) and np.isnan(value)):
        return None
    if pd.isna(value):
        return None
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        ts = ts.tz_localize("UTC")
    return ts


def _parse_optional_float(value) -> float | None:
    """将 CSV 中的可选数值解析为 float 或 None。"""
    if value is None:
        return None
    if isinstance(value, float) and np.isnan(value):
        return None
    if pd.isna(value):
        return None
    return float(value)


# ---------------------------------------------------------------------------
# 常量（与 timing_classifier 保持一致）
# ---------------------------------------------------------------------------

_INITIAL_BALANCE = 100_000.0
_NOTIONAL_SHARE = 0.26

# 3-regime delay 分组
_FAST_DELAYS = ["D0", "D5"]
_SLOW_DELAYS = ["D10", "D15", "pullback"]


# ---------------------------------------------------------------------------
# predict_and_evaluate_test
# ---------------------------------------------------------------------------


def predict_and_evaluate_test(
    model,
    test_features: pd.DataFrame,
    delay_results_test: list[list[DelayResult]],
    test_events: pd.DataFrame,
    regime_schema: str,
    best_global_delay: str | None = None,
) -> tuple[float, np.ndarray]:
    """对 test set 预测并计算 silo-based calendar_sum。

    根据 regime_schema 不同，预测标签到 DelayResult 的映射逻辑不同：

    - 5-regime: 预测为具体 delay label (D0/D5/D10/D15/pullback)，
      直接使用对应的 DelayResult；"skip" → pnl=0。
    - 2-regime: "enter" → 使用 best_global_delay 对应的 DelayResult；
      "skip" → pnl=0。
    - 3-regime: "fast" → 使用该 event 在 {D0, D5} 中 pnl 最优的 delay 的
      DelayResult；"slow" → 使用该 event 在 {D10, D15, pullback} 中 pnl
      最优的 delay 的 DelayResult；"skip" → pnl=0。

    Parameters
    ----------
    model : fitted classifier
        已在 train set 上训练好的分类器（支持 .predict()）。
    test_features : pd.DataFrame
        测试集特征矩阵（已 impute）。
    delay_results_test : list[list[DelayResult]]
        测试集每个 event 的 5 个 DelayResult（D0, D5, D10, D15, pullback）。
    test_events : pd.DataFrame
        测试集 events（用于推断 symbol 和 entry_time 以计算 silo key）。
    regime_schema : str
        "5-regime" | "3-regime" | "2-regime"。
    best_global_delay : str | None
        仅 2-regime 需要，Best_Global_Delay 标签（如 "D5"）。

    Returns
    -------
    tuple[float, np.ndarray]
        - test_calendar_sum: silo-based calendar sum (%)
        - predictions: 预测标签数组
    """
    # 1. 获取预测
    predictions = model.predict(test_features)

    # 2. 根据 regime_schema 将预测映射到 DelayResult
    selected_results: list[DelayResult] = []

    for i, pred_label in enumerate(predictions):
        event_delay_results = delay_results_test[i]

        if pred_label == "skip":
            # skip → 不入场，不加入 results（等效 pnl=0）
            continue

        if regime_schema == "5-regime":
            # 5-regime: 预测为具体 delay label，直接查找
            matched = _find_delay_result_by_label(event_delay_results, pred_label)
            if matched is not None:
                selected_results.append(matched)

        elif regime_schema == "2-regime":
            # 2-regime: "enter" → 使用 best_global_delay 对应的 DelayResult
            if pred_label == "enter":
                if best_global_delay is None:
                    raise ValueError(
                        "2-regime 需要提供 best_global_delay 参数。"
                    )
                matched = _find_delay_result_by_label(
                    event_delay_results, best_global_delay
                )
                if matched is not None:
                    selected_results.append(matched)

        elif regime_schema == "3-regime":
            # 3-regime: "fast"/"slow" → 使用对应组内最优 delay 的 DelayResult
            if pred_label == "fast":
                matched = _find_best_in_group(event_delay_results, _FAST_DELAYS)
                if matched is not None:
                    selected_results.append(matched)
            elif pred_label == "slow":
                matched = _find_best_in_group(event_delay_results, _SLOW_DELAYS)
                if matched is not None:
                    selected_results.append(matched)

        else:
            raise ValueError(f"未知的 regime_schema: {regime_schema}")

    # 3. 计算 silo-based calendar_sum
    test_calendar_sum = _compute_calendar_sum_silo(selected_results, test_events)

    return test_calendar_sum, np.array(predictions)


def _find_delay_result_by_label(
    event_delay_results: list[DelayResult],
    target_label: str,
) -> DelayResult | None:
    """从 event 的 delay_results 中查找指定 delay_label 的结果。

    支持 delay_label 格式（D0, D5, ...）和 regime 格式（immediate, standard, ...）。
    """
    from pre_breakout_timing.timing_classifier import LABEL_TO_REGIME

    # 如果是 regime 格式，转为 delay_label
    regime_to_delay_label: dict[str, str] = {v: k for k, v in LABEL_TO_REGIME.items()}
    if target_label in regime_to_delay_label:
        target_label = regime_to_delay_label[target_label]

    for dr in event_delay_results:
        if dr.delay_label == target_label:
            return dr

    return None


def _find_best_in_group(
    event_delay_results: list[DelayResult],
    group_delays: list[str],
) -> DelayResult | None:
    """从 event 的 delay_results 中找到指定 delay 组内 pnl 最优的 DelayResult。

    选择 pnl_pct 最大的 DelayResult。若所有候选均无有效 pnl（None 或未交易），
    返回 None。
    """
    best_dr: DelayResult | None = None
    best_pnl: float = -np.inf

    for dr in event_delay_results:
        if dr.delay_label not in group_delays:
            continue
        # 获取有效 pnl
        pnl = dr.pnl_pct if (dr.traded and dr.pnl_pct is not None) else None
        if pnl is not None and pnl > best_pnl:
            best_pnl = pnl
            best_dr = dr

    return best_dr


def _compute_calendar_sum_silo(
    results: list[DelayResult],
    events: pd.DataFrame,
) -> float:
    """计算 silo-based calendar sum (%)。

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。

    从 event_id 推断 symbol（与 timing_classifier._infer_symbol_from_event_id 一致）。
    """
    # Group trades by (symbol, month)
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
