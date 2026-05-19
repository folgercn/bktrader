"""
ablation — 6 类增强特征的逐类消融实验

对每组增强特征执行"单独剔除"消融，量化边际贡献。
"""

from __future__ import annotations

import logging
import sys
from dataclasses import dataclass
from pathlib import Path

import pandas as pd
import numpy as np

from sklearn.ensemble import RandomForestClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler
from sklearn.tree import DecisionTreeClassifier

# ---------------------------------------------------------------------------
# Path setup for importing sibling modules
# ---------------------------------------------------------------------------

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pre_breakout_timing.delay_simulator import DelayResult
from pre_breakout_timing.timing_classifier import (
    _compute_calendar_sum_from_delay_results,
    _find_delay_result,
)
from pretouch_refinement.enhanced_features import ENHANCED_FEATURE_GROUPS

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# 消融结果
# ---------------------------------------------------------------------------


@dataclass
class AblationResult:
    """单组消融结果"""

    group_name: str  # 特征组名称
    features_removed: list[str]  # 被剔除的具体特征
    test_calendar_sum: float  # 剔除后的 test calendar_sum
    loocv_calendar_sum: float  # 剔除后的 LOOCV calendar_sum
    oracle_realization_rate: float  # 剔除后的 Oracle 实现率
    delta_test_cs: float  # 相对全特征基线的变化
    high_value_group: bool  # 剔除后下降 > 0.3%
    negative_value_group: bool  # 剔除后不变或上升
    bootstrap_ci_lower: float  # delta 的 bootstrap 95% CI 下界
    bootstrap_ci_upper: float  # delta 的 bootstrap 95% CI 上界


# ---------------------------------------------------------------------------
# 内部常量
# ---------------------------------------------------------------------------

_INITIAL_BALANCE = 100_000.0
_NOTIONAL_SHARE = 0.26

# 3-regime delay 分组
_FAST_DELAYS = ["D0", "D5"]
_SLOW_DELAYS = ["D10", "D15", "pullback"]


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def run_ablation(
    full_features_train: pd.DataFrame,
    full_features_test: pd.DataFrame,
    labels: pd.Series,
    delay_results_train: list,
    delay_results_test: list,
    test_events: pd.DataFrame,
    oracle_calendar_sum: float,
    baseline_test_cs: float,
    best_classifier_name: str = "LogisticRegression",
    best_classifier_params: dict | None = None,
    regime_schema: str = "5-regime",
    best_global_delay: str | None = None,
    n_bootstrap: int = 1000,
    random_state: int = 42,
) -> list[AblationResult]:
    """对 6 类增强特征执行逐类消融。

    基线：全部增强特征 + 原特征 + 最优 arm 的 regime schema。
    消融 i：剔除第 i 组增强特征，其他不变，重新训练最优分类器。

    Parameters
    ----------
    full_features_train : pd.DataFrame
        包含原特征 + 全部增强特征的完整训练特征矩阵。
    full_features_test : pd.DataFrame
        包含原特征 + 全部增强特征的完整测试特征矩阵。
    labels : pd.Series
        最优 arm 对应的训练标签（含 skip；skip 在训练时排除）。
    delay_results_train : list
        训练集 DelayResult 列表（list[list[DelayResult]]）。
    delay_results_test : list
        测试集 DelayResult 列表（list[list[DelayResult]]）。
    test_events : pd.DataFrame
        测试集 events。
    oracle_calendar_sum : float
        Oracle 理论上限。
    baseline_test_cs : float
        全特征基线的 test calendar_sum。
    best_classifier_name : str
        最优分类器名称（"RuleBased_DT3" / "DecisionTree" / "RandomForest" /
        "LogisticRegression"）。
    best_classifier_params : dict | None
        最优分类器超参数（如 {"max_depth": 3}）。
    regime_schema : str
        最优 arm 的 regime schema（"5-regime" / "3-regime" / "2-regime"）。
    best_global_delay : str | None
        仅 2-regime 需要，Best_Global_Delay 标签（如 "D5"）。
    n_bootstrap : int
        Bootstrap 重采样次数，默认 1000。
    random_state : int
        随机种子，默认 42。

    Returns
    -------
    list[AblationResult]
        6 组消融结果。
    """
    if best_classifier_params is None:
        best_classifier_params = {}

    # --- 过滤 skip 标签（skip 不参与训练）---
    train_mask = labels != "skip"
    features_train_filtered = full_features_train.loc[train_mask].reset_index(
        drop=True
    )
    labels_filtered = labels.loc[train_mask].reset_index(drop=True)
    delay_results_train_filtered = [
        delay_results_train[i] for i, keep in enumerate(train_mask) if keep
    ]

    # 测试集特征（不过滤 skip，test set 全部评估）
    features_test_reset = full_features_test.reset_index(drop=True)

    results: list[AblationResult] = []

    for group_name, group_features in ENHANCED_FEATURE_GROUPS.items():
        # 确定该组中实际存在于特征矩阵中的列
        features_to_remove = [
            f for f in group_features if f in features_train_filtered.columns
        ]

        if not features_to_remove:
            logger.info(
                f"Ablation: group '{group_name}' has no features in the "
                f"feature matrix. Skipping."
            )
            # 该组特征不在矩阵中（可能已被排除），delta=0
            results.append(
                AblationResult(
                    group_name=group_name,
                    features_removed=group_features,
                    test_calendar_sum=baseline_test_cs,
                    loocv_calendar_sum=0.0,
                    oracle_realization_rate=(
                        baseline_test_cs / oracle_calendar_sum * 100.0
                        if oracle_calendar_sum != 0
                        else 0.0
                    ),
                    delta_test_cs=0.0,
                    high_value_group=False,
                    negative_value_group=True,  # 不变 → negative_value
                    bootstrap_ci_lower=0.0,
                    bootstrap_ci_upper=0.0,
                )
            )
            continue

        # --- 剔除该组特征 ---
        ablated_train = features_train_filtered.drop(
            columns=features_to_remove
        )
        ablated_test = features_test_reset.drop(columns=features_to_remove)

        # --- 重新训练最优分类器 ---
        model = _build_classifier(
            best_classifier_name, best_classifier_params
        )
        model.fit(ablated_train, labels_filtered)

        # --- 在 test set 上预测并计算 calendar_sum ---
        predictions = model.predict(ablated_test)
        ablated_test_cs = _compute_test_calendar_sum(
            predictions,
            delay_results_test,
            test_events,
            regime_schema,
            best_global_delay,
        )

        # --- 计算 LOOCV calendar_sum（简化版：使用 train set LOO）---
        loocv_cs = _compute_loocv_calendar_sum(
            ablated_train,
            labels_filtered,
            delay_results_train_filtered,
            best_classifier_name,
            best_classifier_params,
        )

        # --- 计算 delta ---
        delta_test_cs = ablated_test_cs - baseline_test_cs

        # --- Bootstrap 95% CI for delta ---
        ci_lower, ci_upper = _bootstrap_delta_ci(
            model=model,
            full_features_test=features_test_reset,
            ablated_features_test=ablated_test,
            delay_results_test=delay_results_test,
            test_events=test_events,
            regime_schema=regime_schema,
            best_global_delay=best_global_delay,
            baseline_test_cs=baseline_test_cs,
            best_classifier_name=best_classifier_name,
            best_classifier_params=best_classifier_params,
            ablated_train=ablated_train,
            labels_filtered=labels_filtered,
            n_bootstrap=n_bootstrap,
            random_state=random_state,
        )

        # --- Oracle 实现率 ---
        if oracle_calendar_sum != 0.0:
            oracle_rate = ablated_test_cs / oracle_calendar_sum * 100.0
        else:
            oracle_rate = 0.0

        # --- 标注 ---
        high_value = delta_test_cs < -0.3
        negative_value = delta_test_cs >= 0.0

        result = AblationResult(
            group_name=group_name,
            features_removed=features_to_remove,
            test_calendar_sum=ablated_test_cs,
            loocv_calendar_sum=loocv_cs,
            oracle_realization_rate=oracle_rate,
            delta_test_cs=delta_test_cs,
            high_value_group=high_value,
            negative_value_group=negative_value,
            bootstrap_ci_lower=ci_lower,
            bootstrap_ci_upper=ci_upper,
        )
        results.append(result)

        logger.info(
            f"Ablation '{group_name}': removed {features_to_remove}, "
            f"test_cs={ablated_test_cs:.4f}%, delta={delta_test_cs:.4f}%, "
            f"CI=[{ci_lower:.4f}, {ci_upper:.4f}], "
            f"high_value={high_value}, negative_value={negative_value}"
        )

    return results


def generate_ablation_report(
    results: list[AblationResult],
    output_dir: str,
    baseline_test_cs: float = 0.0,
) -> None:
    """生成 ablation_results.csv 和 ablation_report.md。

    Parameters
    ----------
    results : list[AblationResult]
        6 组消融结果（来自 run_ablation()）。
    output_dir : str
        输出目录路径。
    baseline_test_cs : float
        全特征基线的 test calendar_sum（用于报告中的基线引用）。
    """
    output_path = Path(output_dir)
    output_path.mkdir(parents=True, exist_ok=True)

    # ------------------------------------------------------------------
    # 1. 产出 ablation_results.csv
    # ------------------------------------------------------------------
    rows = []
    for r in results:
        rows.append(
            {
                "group_name": r.group_name,
                "features_removed": ", ".join(r.features_removed),
                "test_calendar_sum": round(r.test_calendar_sum, 4),
                "loocv_calendar_sum": round(r.loocv_calendar_sum, 4),
                "oracle_realization_rate": round(r.oracle_realization_rate, 4),
                "delta_test_cs": round(r.delta_test_cs, 4),
                "high_value_group": r.high_value_group,
                "negative_value_group": r.negative_value_group,
                "bootstrap_ci_lower": round(r.bootstrap_ci_lower, 4),
                "bootstrap_ci_upper": round(r.bootstrap_ci_upper, 4),
            }
        )
    df = pd.DataFrame(rows)
    csv_path = output_path / "ablation_results.csv"
    df.to_csv(csv_path, index=False)
    logger.info(f"Ablation CSV saved to {csv_path}")

    # ------------------------------------------------------------------
    # 2. 产出 ablation_report.md（中文消融分析报告）
    # ------------------------------------------------------------------
    high_value_groups = [r for r in results if r.high_value_group]
    negative_value_groups = [r for r in results if r.negative_value_group]

    lines: list[str] = []

    # --- 标题 ---
    lines.append("# 消融实验分析报告\n")

    # --- 摘要 ---
    lines.append("## 摘要\n")
    lines.append(f"- **基线 test calendar_sum**: {baseline_test_cs:.4f}%")
    lines.append(f"- **高价值特征组数量**: {len(high_value_groups)}")
    lines.append(f"- **负价值特征组数量**: {len(negative_value_groups)}")
    lines.append("")

    # --- 消融结果表 ---
    lines.append("## 消融结果总表\n")
    lines.append(
        "| 特征组 | 剔除后 test_cs (%) | delta (%) | "
        "LOOCV_cs (%) | Oracle 实现率 (%) | Bootstrap 95% CI | "
        "高价值 | 负价值 |"
    )
    lines.append(
        "|--------|-------------------|-----------|"
        "-------------|-------------------|"
        "------------------|--------|--------|"
    )
    for r in results:
        ci_str = f"[{r.bootstrap_ci_lower:.4f}, {r.bootstrap_ci_upper:.4f}]"
        hv_mark = "✓" if r.high_value_group else ""
        nv_mark = "✓" if r.negative_value_group else ""
        lines.append(
            f"| {r.group_name} | {r.test_calendar_sum:.4f} | "
            f"{r.delta_test_cs:.4f} | {r.loocv_calendar_sum:.4f} | "
            f"{r.oracle_realization_rate:.4f} | {ci_str} | "
            f"{hv_mark} | {nv_mark} |"
        )
    lines.append("")

    # --- 高价值特征组 ---
    lines.append("## 高价值特征组（剔除后下降 > 0.3%）\n")
    if high_value_groups:
        for r in high_value_groups:
            significant = _is_significant(r)
            sig_label = "显著" if significant else "不显著"
            lines.append(
                f"- **{r.group_name}**: 剔除后 delta = {r.delta_test_cs:.4f}%，"
                f"CI = [{r.bootstrap_ci_lower:.4f}, {r.bootstrap_ci_upper:.4f}]，"
                f"统计{sig_label}"
            )
        lines.append("")
        lines.append("这些特征组对模型表现有显著正向贡献，建议在 Path C 中保留。\n")
    else:
        lines.append("无高价值特征组（所有特征组剔除后下降均 ≤ 0.3%）。\n")

    # --- 负价值特征组 ---
    lines.append("## 负价值特征组（剔除后不变或上升）\n")
    if negative_value_groups:
        for r in negative_value_groups:
            significant = _is_significant(r)
            sig_label = "显著" if significant else "不显著"
            lines.append(
                f"- **{r.group_name}**: 剔除后 delta = {r.delta_test_cs:.4f}%，"
                f"CI = [{r.bootstrap_ci_lower:.4f}, {r.bootstrap_ci_upper:.4f}]，"
                f"统计{sig_label}"
            )
        lines.append("")
        lines.append(
            "这些特征组对模型表现无正向贡献或有负面影响，"
            "建议在 Path C 中排除以降低过拟合风险。\n"
        )
    else:
        lines.append("无负价值特征组（所有特征组剔除后均有下降）。\n")

    # --- 显著性评估 ---
    lines.append("## 显著性评估\n")
    lines.append(
        "以下对每组消融的 bootstrap 95% CI 进行显著性判断"
        "（CI 不包含 0 则认为该组贡献显著）：\n"
    )
    for r in results:
        significant = _is_significant(r)
        if significant:
            sig_text = (
                f"**显著** — CI [{r.bootstrap_ci_lower:.4f}, "
                f"{r.bootstrap_ci_upper:.4f}] 不包含 0"
            )
        else:
            sig_text = (
                f"不显著 — CI [{r.bootstrap_ci_lower:.4f}, "
                f"{r.bootstrap_ci_upper:.4f}] 包含 0"
            )
        lines.append(f"- **{r.group_name}**: {sig_text}")
    lines.append("")

    # --- 建议 ---
    lines.append("## Path C 特征保留建议\n")
    keep_groups = [
        r.group_name for r in results if not r.negative_value_group
    ]
    exclude_groups = [
        r.group_name for r in results if r.negative_value_group
    ]

    if keep_groups:
        lines.append("**建议保留的特征组**：\n")
        for g in keep_groups:
            lines.append(f"- {g}")
        lines.append("")

    if exclude_groups:
        lines.append("**建议排除的特征组**：\n")
        for g in exclude_groups:
            lines.append(f"- {g}")
        lines.append("")

    if not keep_groups and not exclude_groups:
        lines.append("无明确建议（结果需进一步分析）。\n")

    report_content = "\n".join(lines)
    report_path = output_path / "ablation_report.md"
    report_path.write_text(report_content, encoding="utf-8")
    logger.info(f"Ablation report saved to {report_path}")


def _is_significant(result: AblationResult) -> bool:
    """判断消融结果是否统计显著（bootstrap CI 不包含 0）。"""
    return not (result.bootstrap_ci_lower <= 0.0 <= result.bootstrap_ci_upper)


# ---------------------------------------------------------------------------
# 内部辅助函数
# ---------------------------------------------------------------------------


def _build_classifier(name: str, params: dict):
    """根据分类器名称和参数构建 sklearn 分类器实例。"""
    if name == "RuleBased_DT3":
        return DecisionTreeClassifier(max_depth=3, random_state=42)
    elif name == "DecisionTree":
        max_depth = params.get("max_depth", 3)
        return DecisionTreeClassifier(max_depth=max_depth, random_state=42)
    elif name == "RandomForest":
        return RandomForestClassifier(
            n_estimators=params.get("n_estimators", 100),
            max_depth=params.get("max_depth", 4),
            random_state=42,
        )
    elif name == "LogisticRegression":
        return Pipeline(
            [
                ("scaler", StandardScaler()),
                (
                    "clf",
                    LogisticRegression(
                        multi_class="multinomial",
                        solver="lbfgs",
                        random_state=42,
                        max_iter=1000,
                    ),
                ),
            ]
        )
    else:
        raise ValueError(f"未知分类器名称: {name}")


def _compute_test_calendar_sum(
    predictions: np.ndarray,
    delay_results_test: list[list[DelayResult]],
    test_events: pd.DataFrame,
    regime_schema: str,
    best_global_delay: str | None,
) -> float:
    """根据预测标签和 regime_schema 计算 test set 的 silo-based calendar_sum。

    逻辑与 arm_runner._evaluate_best_on_test 一致。
    """
    selected_results: list[DelayResult] = []

    for i, pred_label in enumerate(predictions):
        event_delays = delay_results_test[i]

        if pred_label == "skip":
            continue

        if regime_schema == "5-regime":
            matched = _find_delay_result_by_label(event_delays, pred_label)
            if matched is not None:
                selected_results.append(matched)

        elif regime_schema == "2-regime":
            if pred_label == "enter":
                if best_global_delay is None:
                    raise ValueError(
                        "2-regime 需要提供 best_global_delay 参数。"
                    )
                matched = _find_delay_result_by_label(
                    event_delays, best_global_delay
                )
                if matched is not None:
                    selected_results.append(matched)

        elif regime_schema == "3-regime":
            if pred_label == "fast":
                matched = _find_best_in_group(event_delays, _FAST_DELAYS)
                if matched is not None:
                    selected_results.append(matched)
            elif pred_label == "slow":
                matched = _find_best_in_group(event_delays, _SLOW_DELAYS)
                if matched is not None:
                    selected_results.append(matched)

        else:
            raise ValueError(f"未知的 regime_schema: {regime_schema}")

    return _compute_calendar_sum_silo(selected_results, test_events)


def _find_delay_result_by_label(
    event_delay_results: list[DelayResult],
    target_label: str,
) -> DelayResult | None:
    """从 event 的 delay_results 中查找指定 delay_label 的结果。"""
    from pre_breakout_timing.timing_classifier import LABEL_TO_REGIME

    regime_to_delay_label: dict[str, str] = {
        v: k for k, v in LABEL_TO_REGIME.items()
    }
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
    """从 event 的 delay_results 中找到指定 delay 组内 pnl 最优的 DelayResult。"""
    best_dr: DelayResult | None = None
    best_pnl: float = -np.inf

    for dr in event_delay_results:
        if dr.delay_label not in group_delays:
            continue
        pnl = (
            dr.pnl_pct if (dr.traded and dr.pnl_pct is not None) else None
        )
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
        silo_return = (
            (balance - _INITIAL_BALANCE) / _INITIAL_BALANCE * 100.0
        )
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


def _compute_loocv_calendar_sum(
    features_train: pd.DataFrame,
    labels_train: pd.Series,
    delay_results_train: list[list[DelayResult]],
    classifier_name: str,
    classifier_params: dict,
) -> float:
    """计算 Leave-One-Out Cross-Validation calendar_sum。

    对每个训练样本，用其余样本训练分类器，预测该样本的标签，
    收集所有 LOO 预测对应的 DelayResult，最终计算 calendar_sum。
    """
    n = len(features_train)
    loo_results: list[DelayResult] = []

    for i in range(n):
        # 排除第 i 个样本
        train_idx = list(range(n))
        train_idx.remove(i)

        X_train = features_train.iloc[train_idx]
        y_train = labels_train.iloc[train_idx]

        # 训练分类器
        model = _build_classifier(classifier_name, classifier_params)
        model.fit(X_train, y_train)

        # 预测第 i 个样本
        X_test_i = features_train.iloc[[i]]
        pred = model.predict(X_test_i)[0]

        # 获取对应的 DelayResult
        event_delays = delay_results_train[i]
        if pred == "skip":
            continue

        # 对 LOO，使用 5-regime 逻辑（直接匹配 delay_label）
        matched = _find_delay_result(event_delays, pred)
        if matched is not None:
            loo_results.append(matched)

    return _compute_calendar_sum_from_delay_results(loo_results)


def _bootstrap_delta_ci(
    model,
    full_features_test: pd.DataFrame,
    ablated_features_test: pd.DataFrame,
    delay_results_test: list[list[DelayResult]],
    test_events: pd.DataFrame,
    regime_schema: str,
    best_global_delay: str | None,
    baseline_test_cs: float,
    best_classifier_name: str,
    best_classifier_params: dict,
    ablated_train: pd.DataFrame,
    labels_filtered: pd.Series,
    n_bootstrap: int,
    random_state: int,
) -> tuple[float, float]:
    """计算 delta_test_cs 的 bootstrap 95% CI。

    对 test set 进行 n_bootstrap 次有放回重采样，每次：
    1. 用已训练好的 ablated model 对重采样的 test set 预测
    2. 计算 ablated_cs（重采样）
    3. delta_i = ablated_cs_i - baseline_test_cs

    取 5th 和 95th percentile 作为 95% CI bounds。

    注意：baseline_test_cs 作为固定参考点，CI 反映的是消融后模型
    在不同 test set 重采样下表现的不确定性。
    """
    rng = np.random.RandomState(random_state)
    n_test = len(ablated_features_test)

    deltas: list[float] = []

    for _ in range(n_bootstrap):
        # 重采样 test set indices（有放回）
        indices = rng.choice(n_test, size=n_test, replace=True)

        # 获取重采样的特征和 delay_results
        resampled_features = ablated_features_test.iloc[indices].reset_index(
            drop=True
        )
        resampled_delay_results = [delay_results_test[i] for i in indices]
        resampled_events = test_events.iloc[indices].reset_index(drop=True)

        # 用 ablated model 预测
        ablated_preds = model.predict(resampled_features)
        ablated_cs = _compute_test_calendar_sum(
            ablated_preds,
            resampled_delay_results,
            resampled_events,
            regime_schema,
            best_global_delay,
        )

        # delta = ablated_cs（重采样）- baseline_test_cs（固定）
        delta_i = ablated_cs - baseline_test_cs
        deltas.append(delta_i)

    deltas_arr = np.array(deltas)
    ci_lower = float(np.percentile(deltas_arr, 5.0))
    ci_upper = float(np.percentile(deltas_arr, 95.0))

    return ci_lower, ci_upper
