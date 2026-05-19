"""
report_generator — 报告生成模块

负责：
- Go/No-Go 判定逻辑 (determine_go_nogo)
- 产出所有输出文件：
  - pre_breakout_timing_report.md（中文主报告）
  - pre_breakout_timing_summary.json（结构化 summary）
  - pre_breakout_timing_attribution.csv（per-event 归因表）
  - pre_breakout_timing_trades.csv（trade ledger）
  - classifier_rules.md（规则型分类器的中文规则）
  - feature_importance.csv（特征重要性排序）
"""

from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path

import numpy as np
import pandas as pd

from pre_breakout_timing.delay_simulator import DelayResult
from pre_breakout_timing.pullback_strategy import PullbackConfig
from pre_breakout_timing.timing_classifier import (
    ClassifierResult,
    extract_rules_text,
)


# ---------------------------------------------------------------------------
# Go/No-Go 判定
# ---------------------------------------------------------------------------


def determine_go_nogo(
    classifier_calendar_sum: float,
    baseline_b_calendar_sum: float,
    overfitting_flag: bool,
    loocv_degradation_flag: bool,
) -> str:
    """Go/No-Go 判定。

    基于 classifier test set calendar_sum 与 Baseline B (D=5s) 的对比，
    以及过拟合/LOOCV 退化标志，给出最终建议。

    判定规则：
    - **Go**：classifier > baseline_b + 0.5%（绝对值）且无 overfitting_flag
    - **Conditional Go**：classifier > baseline_b 但改善 < 0.5%，
      或存在 loocv_degradation_flag
    - **No-Go**：classifier <= baseline_b 或 overfitting_flag=true

    Parameters
    ----------
    classifier_calendar_sum : float
        分类器在 test set 上的 calendar_sum (%)。
    baseline_b_calendar_sum : float
        Baseline B (D=5s) 在 test set 上的 calendar_sum (%)。
    overfitting_flag : bool
        是否存在过拟合标志（train→test 下降 > 50%）。
    loocv_degradation_flag : bool
        是否存在 LOOCV 退化标志（LOOCV 与 train full-fit 差异 > 30%）。

    Returns
    -------
    str
        "Go", "Conditional Go", 或 "No-Go"。
    """
    improvement = classifier_calendar_sum - baseline_b_calendar_sum

    # No-Go 条件：classifier <= baseline_b 或 overfitting_flag
    if overfitting_flag:
        return "No-Go"
    if improvement <= 0:
        return "No-Go"

    # Go 条件：improvement > 0.5% 且无 overfitting
    if improvement > 0.5 and not loocv_degradation_flag:
        return "Go"

    # Conditional Go：improvement > 0 但 < 0.5%，或有 loocv_degradation
    return "Conditional Go"


# ---------------------------------------------------------------------------
# 主报告生成
# ---------------------------------------------------------------------------


def generate_report(
    classifier_results: list[ClassifierResult],
    best_classifier: ClassifierResult,
    baselines: dict[str, dict],
    oracle_calendar_sum: float,
    bootstrap_ci: dict,
    symbol_results: dict,
    overfitting_check: dict,
    regime_stability: dict,
    feature_stats: pd.DataFrame | None,
    used_features: list[str],
    excluded_features: list[str],
    events: pd.DataFrame,
    test_events: pd.DataFrame,
    test_results: list[DelayResult],
    test_predictions: np.ndarray,
    optimal_labels: list[str],
    all_delay_results: list[list[DelayResult]],
    best_pullback_config: PullbackConfig,
    output_dir: Path,
    baseline_b_test_calendar_sum: float | None = None,
) -> None:
    """产出所有输出文件。

    生成以下文件到 output_dir：
    - pre_breakout_timing_report.md（中文主报告）
    - pre_breakout_timing_summary.json（结构化 summary）
    - pre_breakout_timing_attribution.csv（per-event 归因表）
    - pre_breakout_timing_trades.csv（trade ledger）
    - classifier_rules.md（规则型分类器的中文规则）
    - feature_importance.csv（特征重要性排序）

    Parameters
    ----------
    classifier_results : list[ClassifierResult]
        所有分类器的评估结果。
    best_classifier : ClassifierResult
        最优分类器。
    baselines : dict[str, dict]
        各 delay baseline 的统计（calendar_sum, trade_count, win_rate, avg_pnl）。
    oracle_calendar_sum : float
        Oracle baseline 的 calendar_sum (%)。
    bootstrap_ci : dict
        Bootstrap 置信区间结果。
    symbol_results : dict
        分 symbol 验证结果。
    overfitting_check : dict
        过拟合检查结果。
    regime_stability : dict
        Regime 稳定性分析结果。
    feature_stats : pd.DataFrame | None
        特征描述性统计 DataFrame。
    used_features : list[str]
        实际使用的特征列名。
    excluded_features : list[str]
        被排除的特征列名。
    events : pd.DataFrame
        全部 events DataFrame。
    test_events : pd.DataFrame
        Test set events DataFrame。
    test_results : list[DelayResult]
        Test set 使用分类器预测后的执行结果。
    test_predictions : np.ndarray
        Test set 的分类器预测标签。
    optimal_labels : list[str]
        每个 event 的 Optimal_Delay_Label。
    all_delay_results : list[list[DelayResult]]
        每个 event 在各 delay 下的执行结果。
    best_pullback_config : PullbackConfig
        最优回调参数配置。
    output_dir : Path
        输出目录。
    """
    output_dir.mkdir(parents=True, exist_ok=True)

    # --- 计算 Go/No-Go ---
    # Use test-set Baseline B for fair comparison (same set as classifier)
    if baseline_b_test_calendar_sum is not None:
        baseline_b_for_go_nogo = baseline_b_test_calendar_sum
    else:
        baseline_b_for_go_nogo = baselines["D5"]["calendar_sum"]
    overfitting_flag = overfitting_check["overfitting_flag"]
    loocv_degradation_flag = overfitting_check["loocv_degradation_flag"]

    go_nogo = determine_go_nogo(
        classifier_calendar_sum=best_classifier.test_calendar_sum,
        baseline_b_calendar_sum=baseline_b_for_go_nogo,
        overfitting_flag=overfitting_flag,
        loocv_degradation_flag=loocv_degradation_flag,
    )

    # --- 1. 生成 pre_breakout_timing_trades.csv ---
    _generate_trades_csv(test_results, test_events, test_predictions, output_dir)

    # --- 2. 生成 pre_breakout_timing_attribution.csv ---
    _generate_attribution_csv(
        events, test_events, test_results, test_predictions,
        optimal_labels, all_delay_results, baselines, used_features, output_dir,
    )

    # --- 3. 生成 feature_importance.csv ---
    _generate_feature_importance_csv(best_classifier, output_dir)

    # --- 4. 生成 classifier_rules.md ---
    _generate_classifier_rules_md(classifier_results, used_features, output_dir)

    # --- 5. 生成 pre_breakout_timing_summary.json ---
    _generate_summary_json(
        classifier_results, best_classifier, baselines, oracle_calendar_sum,
        bootstrap_ci, symbol_results, overfitting_check, regime_stability,
        go_nogo, best_pullback_config, used_features, excluded_features,
        events, output_dir,
        baseline_b_test_calendar_sum=baseline_b_for_go_nogo,
    )

    # --- 6. 生成 pre_breakout_timing_report.md ---
    _generate_report_md(
        classifier_results, best_classifier, baselines, oracle_calendar_sum,
        bootstrap_ci, symbol_results, overfitting_check, regime_stability,
        feature_stats, used_features, excluded_features, go_nogo,
        best_pullback_config, events, optimal_labels, output_dir,
        baseline_b_test_calendar_sum=baseline_b_for_go_nogo,
    )

    print(f"\n  报告生成完成。输出目录: {output_dir}")
    print(f"    - pre_breakout_timing_report.md")
    print(f"    - pre_breakout_timing_summary.json")
    print(f"    - pre_breakout_timing_attribution.csv")
    print(f"    - pre_breakout_timing_trades.csv")
    print(f"    - classifier_rules.md")
    print(f"    - feature_importance.csv")


# ---------------------------------------------------------------------------
# 内部辅助函数
# ---------------------------------------------------------------------------


def _generate_trades_csv(
    test_results: list[DelayResult],
    test_events: pd.DataFrame,
    test_predictions: np.ndarray,
    output_dir: Path,
) -> None:
    """生成 pre_breakout_timing_trades.csv — trade ledger。"""
    rows: list[dict] = []
    for i, (test_idx, event_row) in enumerate(test_events.iterrows()):
        dr = test_results[i]
        rows.append({
            "event_id": dr.event_id,
            "symbol": event_row["symbol"],
            "side": event_row["side"],
            "touch_time": event_row["touch_time"],
            "predicted_regime": test_predictions[i],
            "delay_seconds": dr.delay_seconds,
            "entry_time": dr.entry_time,
            "entry_price": dr.entry_price,
            "exit_time": dr.exit_time,
            "exit_reason": dr.exit_reason,
            "pnl_pct": dr.pnl_pct,
            "hold_seconds": dr.hold_seconds,
            "mfe_r": dr.mfe_r,
            "mae_r": dr.mae_r,
            "traded": dr.traded,
        })

    df = pd.DataFrame(rows)
    path = output_dir / "pre_breakout_timing_trades.csv"
    df.to_csv(path, index=False)


def _generate_attribution_csv(
    events: pd.DataFrame,
    test_events: pd.DataFrame,
    test_results: list[DelayResult],
    test_predictions: np.ndarray,
    optimal_labels: list[str],
    all_delay_results: list[list[DelayResult]],
    baselines: dict[str, dict],
    used_features: list[str],
    output_dir: Path,
) -> None:
    """生成 pre_breakout_timing_attribution.csv — per-event 归因表。"""
    # Build event index → position mapping
    index_to_position = {idx: pos for pos, idx in enumerate(events.index)}

    rows: list[dict] = []
    for i, (test_idx, event_row) in enumerate(test_events.iterrows()):
        dr = test_results[i]
        pos = index_to_position[test_idx]
        optimal_label = optimal_labels[pos]

        # Find baseline B (D=5s) result for this event
        baseline_b_result = None
        for delay_r in all_delay_results[pos]:
            if delay_r.delay_label == "D5":
                baseline_b_result = delay_r
                break

        baseline_b_pnl = (
            baseline_b_result.pnl_pct
            if baseline_b_result and baseline_b_result.traded
            else None
        )
        baseline_b_entry_price = (
            baseline_b_result.entry_price
            if baseline_b_result and baseline_b_result.traded
            else None
        )

        delta_pnl = None
        if dr.pnl_pct is not None and baseline_b_pnl is not None:
            delta_pnl = dr.pnl_pct - baseline_b_pnl

        row = {
            "event_id": dr.event_id,
            "symbol": event_row["symbol"],
            "touch_time": event_row["touch_time"],
            "predicted_regime": test_predictions[i],
            "optimal_regime": optimal_label,
            "entry_delay_seconds": dr.delay_seconds,
            "entry_price": dr.entry_price,
            "baseline_b_entry_price": baseline_b_entry_price,
            "pnl_pct": dr.pnl_pct,
            "baseline_b_pnl_pct": baseline_b_pnl,
            "delta_pnl_pct": delta_pnl,
        }

        # Add pre-breakout feature values
        for feat in used_features:
            if feat in event_row.index:
                row[feat] = event_row[feat]
            else:
                row[feat] = None

        rows.append(row)

    df = pd.DataFrame(rows)
    path = output_dir / "pre_breakout_timing_attribution.csv"
    df.to_csv(path, index=False)


def _generate_feature_importance_csv(
    best_classifier: ClassifierResult,
    output_dir: Path,
) -> None:
    """生成 feature_importance.csv — 特征重要性排序。"""
    fi = best_classifier.feature_importance
    # Sort by importance descending
    sorted_fi = sorted(fi.items(), key=lambda x: x[1], reverse=True)

    rows = [
        {"feature": name, "importance": importance, "rank": rank + 1}
        for rank, (name, importance) in enumerate(sorted_fi)
    ]

    df = pd.DataFrame(rows)
    path = output_dir / "feature_importance.csv"
    df.to_csv(path, index=False)


def _generate_classifier_rules_md(
    classifier_results: list[ClassifierResult],
    used_features: list[str],
    output_dir: Path,
) -> None:
    """生成 classifier_rules.md — 规则型分类器的中文规则描述。"""
    # Find the rule-based classifier
    rule_classifier = None
    for cr in classifier_results:
        if cr.name == "RuleBased_DT3":
            rule_classifier = cr
            break

    lines: list[str] = []
    lines.append("# 规则型分类器规则描述")
    lines.append("")
    lines.append("> 基于 DecisionTree (max_depth=3) 提取的人类可读规则")
    lines.append("")

    if rule_classifier is None:
        lines.append("（未找到规则型分类器结果）")
    else:
        # Try to extract rules from the model
        # The rule_classifier doesn't store the model itself, but we can
        # reconstruct from best_params or use extract_rules_text if model available.
        # For now, generate a placeholder that will be filled in tasks 7.2-7.8
        # when the runner integrates the model object.
        lines.append("## 分类器参数")
        lines.append("")
        lines.append(f"- 模型类型: DecisionTree")
        lines.append(f"- max_depth: {rule_classifier.best_params.get('max_depth', 3)}")
        lines.append(f"- 训练样本数: {len(rule_classifier.predictions_train)}")
        lines.append(f"- LOOCV calendar_sum: {rule_classifier.loocv_calendar_sum:+.4f}%")
        lines.append(f"- Train accuracy: {rule_classifier.train_accuracy:.1%}")
        lines.append("")
        lines.append("## 特征重要性（规则型分类器）")
        lines.append("")
        sorted_fi = sorted(
            rule_classifier.feature_importance.items(),
            key=lambda x: x[1],
            reverse=True,
        )
        for rank, (feat, imp) in enumerate(sorted_fi, 1):
            if imp > 0:
                lines.append(f"{rank}. **{feat}**: {imp:.4f}")
        lines.append("")
        lines.append("## 规则描述")
        lines.append("")
        lines.append("（规则文本将在 runner 集成 model 对象后由 `extract_rules_text()` 生成）")

    content = "\n".join(lines) + "\n"
    path = output_dir / "classifier_rules.md"
    path.write_text(content, encoding="utf-8")


def _generate_summary_json(
    classifier_results: list[ClassifierResult],
    best_classifier: ClassifierResult,
    baselines: dict[str, dict],
    oracle_calendar_sum: float,
    bootstrap_ci: dict,
    symbol_results: dict,
    overfitting_check: dict,
    regime_stability: dict,
    go_nogo: str,
    best_pullback_config: PullbackConfig,
    used_features: list[str],
    excluded_features: list[str],
    events: pd.DataFrame,
    output_dir: Path,
    baseline_b_test_calendar_sum: float | None = None,
) -> None:
    """生成 pre_breakout_timing_summary.json — 结构化 summary。"""
    baseline_b_cs = baselines["D5"]["calendar_sum"]
    baseline_b_test_cs = (
        baseline_b_test_calendar_sum
        if baseline_b_test_calendar_sum is not None
        else baseline_b_cs
    )

    # Oracle 实现率
    oracle_realization = (
        best_classifier.test_calendar_sum / oracle_calendar_sum * 100.0
        if oracle_calendar_sum != 0
        else 0.0
    )

    # delay_optimization_ceiling_low 判定
    delay_optimization_ceiling_low = bool(abs(oracle_calendar_sum - baseline_b_cs) < 1.0)

    summary = {
        "experiment": "pre_breakout_timing_classifier",
        "generated_at": datetime.now().isoformat(),
        "n_events": len(events),
        "go_nogo": go_nogo,
        "small_sample_warning": True,
        "delay_optimization_ceiling_low": delay_optimization_ceiling_low,
        "best_classifier": {
            "name": best_classifier.name,
            "best_params": best_classifier.best_params,
            "train_calendar_sum": best_classifier.train_calendar_sum,
            "test_calendar_sum": best_classifier.test_calendar_sum,
            "loocv_calendar_sum": best_classifier.loocv_calendar_sum,
            "train_accuracy": best_classifier.train_accuracy,
            "test_accuracy": best_classifier.test_accuracy,
        },
        "all_classifiers": [
            {
                "name": cr.name,
                "loocv_calendar_sum": cr.loocv_calendar_sum,
                "train_calendar_sum": cr.train_calendar_sum,
                "test_calendar_sum": cr.test_calendar_sum,
                "train_accuracy": cr.train_accuracy,
                "test_accuracy": cr.test_accuracy,
            }
            for cr in classifier_results
        ],
        "baselines": {
            label: {
                "calendar_sum": info["calendar_sum"],
                "trade_count": info["trade_count"],
                "win_rate": info["win_rate"],
                "avg_pnl": info["avg_pnl"],
            }
            for label, info in baselines.items()
        },
        "oracle": {
            "calendar_sum": oracle_calendar_sum,
            "realization_pct": oracle_realization,
            "delay_optimization_ceiling_low": delay_optimization_ceiling_low,
        },
        "bootstrap_ci": bootstrap_ci,
        "symbol_results": symbol_results,
        "overfitting_check": overfitting_check,
        "regime_stability": regime_stability,
        "pullback_config": {
            "pullback_target_atr": best_pullback_config.pullback_target_atr,
            "pullback_window_seconds": best_pullback_config.pullback_window_seconds,
            "start_offset_seconds": best_pullback_config.start_offset_seconds,
        },
        "features": {
            "used": used_features,
            "excluded": excluded_features,
            "n_used": len(used_features),
            "n_excluded": len(excluded_features),
        },
        "reference": {
            "dynamic_entry_timing_calendar_sum": 2.40,
            "note": "dynamic-entry-timing spec 结果（post-breakout 方法，已 No-Go）",
        },
        "go_nogo_detail": {
            "classifier_test_calendar_sum": best_classifier.test_calendar_sum,
            "baseline_b_test_calendar_sum": baseline_b_test_cs,
            "improvement_pct": best_classifier.test_calendar_sum - baseline_b_test_cs,
            "threshold_pct": 0.5,
            "note": "Go/No-Go 判定使用 test set 的 Baseline B（公平对比同一数据集）",
        },
    }

    path = output_dir / "pre_breakout_timing_summary.json"
    path.write_text(
        json.dumps(summary, indent=2, ensure_ascii=False, default=str),
        encoding="utf-8",
    )


def _generate_report_md(
    classifier_results: list[ClassifierResult],
    best_classifier: ClassifierResult,
    baselines: dict[str, dict],
    oracle_calendar_sum: float,
    bootstrap_ci: dict,
    symbol_results: dict,
    overfitting_check: dict,
    regime_stability: dict,
    feature_stats: pd.DataFrame | None,
    used_features: list[str],
    excluded_features: list[str],
    go_nogo: str,
    best_pullback_config: PullbackConfig,
    events: pd.DataFrame,
    optimal_labels: list[str],
    output_dir: Path,
    baseline_b_test_calendar_sum: float | None = None,
) -> None:
    """生成 pre_breakout_timing_report.md — 中文主报告。"""
    from collections import Counter

    # 全量 baseline（用于 Section 4 对照表）
    baseline_b_cs_full = baselines["D5"]["calendar_sum"]
    # test-set-only baseline（用于 Go/No-Go 判定显示）
    baseline_b_cs_test = (
        baseline_b_test_calendar_sum
        if baseline_b_test_calendar_sum is not None
        else baseline_b_cs_full
    )
    test_cs = best_classifier.test_calendar_sum
    # Go/No-Go 判定使用 test-set baseline（公平对比）
    improvement = test_cs - baseline_b_cs_test

    # Oracle 实现率
    oracle_realization = (
        test_cs / oracle_calendar_sum * 100.0
        if oracle_calendar_sum != 0
        else 0.0
    )

    # delay_optimization_ceiling_low
    delay_optimization_ceiling_low = bool(abs(oracle_calendar_sum - baseline_b_cs_full) < 1.0)

    # Label distribution
    label_counts = Counter(optimal_labels)
    n_events = len(events)

    lines: list[str] = []

    # --- 标题 ---
    lines.append("# Pre-Breakout Timing Classifier 实验报告")
    lines.append("")
    lines.append(f"> 生成时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    lines.append(f"> 样本量: {n_events} events (small_sample_warning=true)")
    lines.append(f"> 决策建议: **{go_nogo}**")
    lines.append("")

    # --- 1. 实验设计 ---
    lines.append("## 1. 实验设计")
    lines.append("")
    lines.append("### 1.1 核心假设")
    lines.append("")
    lines.append("不同 pre-breakout 特征组合的 event，其最优入场延迟不同。")
    lines.append("通过在 breakout 触发前就确定入场策略（conditional delay），")
    lines.append("避免 post-breakout tick 观察的噪声问题。")
    lines.append("")
    lines.append("### 1.2 方法")
    lines.append("")
    lines.append("1. 对每个 V6 gate event 在 D=0/5/10/15/pullback 下模拟 V4 执行")
    lines.append("2. 基于 PnL 确定每个 event 的 Optimal_Delay_Label")
    lines.append("3. 使用 pre-breakout 特征训练分类器预测最优 delay regime")
    lines.append("4. Time-split 验证（前 60% train，后 40% test）")
    lines.append("")
    lines.append("### 1.3 使用特征")
    lines.append("")
    lines.append(f"- 使用特征数: {len(used_features)}")
    if used_features:
        lines.append(f"- 特征列表: {', '.join(used_features)}")
    if excluded_features:
        lines.append(f"- 排除特征（缺失率 > 50%）: {', '.join(excluded_features)}")
    lines.append("")

    # --- 2. Optimal_Delay_Label 分布 ---
    lines.append("## 2. Optimal_Delay_Label 分布")
    lines.append("")
    lines.append("| Label | 数量 | 占比 |")
    lines.append("|-------|------|------|")
    for label_name in ["D0", "D5", "D10", "D15", "pullback", "skip"]:
        count = label_counts.get(label_name, 0)
        pct = count / n_events * 100.0 if n_events > 0 else 0.0
        lines.append(f"| {label_name} | {count} | {pct:.1f}% |")
    lines.append("")

    # Oracle analysis
    lines.append(f"**Oracle calendar_sum（理论上限）**: {oracle_calendar_sum:+.4f}%")
    lines.append(f"**Baseline B (D=5s) 全量**: {baseline_b_cs_full:+.4f}%")
    lines.append(f"**Baseline B (D=5s) test set**: {baseline_b_cs_test:+.4f}%")
    lines.append(
        f"**Oracle vs Baseline B 差异**: {oracle_calendar_sum - baseline_b_cs_full:+.4f}%"
    )
    if delay_optimization_ceiling_low:
        lines.append("")
        lines.append(
            "⚠ `delay_optimization_ceiling_low=true`: "
            "Oracle 与 Baseline B 差异 < 1.0%，delay 优化空间有限。"
        )
    lines.append("")

    # --- 3. 分类器对比 ---
    lines.append("## 3. 分类器对比")
    lines.append("")
    lines.append("| 分类器 | LOOCV calendar_sum | Train calendar_sum | "
                 "Test calendar_sum | Train Accuracy |")
    lines.append("|--------|--------------------|--------------------|"
                 "-------------------|----------------|")
    for cr in classifier_results:
        marker = " ★" if cr.name == best_classifier.name else ""
        lines.append(
            f"| {cr.name}{marker} | {cr.loocv_calendar_sum:+.4f}% | "
            f"{cr.train_calendar_sum:+.4f}% | {cr.test_calendar_sum:+.4f}% | "
            f"{cr.train_accuracy:.1%} |"
        )
    lines.append("")
    lines.append(f"**最优分类器**: {best_classifier.name} "
                 f"(LOOCV={best_classifier.loocv_calendar_sum:+.4f}%)")
    lines.append("")

    # --- 4. 结果对比 ---
    lines.append("## 4. 结果对比（Baselines vs Classifier）")
    lines.append("")
    lines.append("| 策略 | Calendar Sum | Trade Count | Win Rate | Avg PnL |")
    lines.append("|------|-------------|-------------|----------|---------|")
    for label, info in baselines.items():
        lines.append(
            f"| Baseline {label} | {info['calendar_sum']:+.4f}% | "
            f"{info['trade_count']} | {info['win_rate']:.1f}% | "
            f"{info['avg_pnl']:.6f} |"
        )
    lines.append(
        f"| **Classifier ({best_classifier.name})** | "
        f"**{test_cs:+.4f}%** | — | — | — |"
    )
    lines.append(f"| Oracle | {oracle_calendar_sum:+.4f}% | — | — | — |")
    lines.append(f"| Reference (dynamic-entry-timing) | +2.40% | — | — | — |")
    lines.append("")
    lines.append(f"**Classifier vs Baseline B (test set) 改善**: {improvement:+.4f}%")
    lines.append(f"**Baseline B test set calendar_sum**: {baseline_b_cs_test:+.4f}%")
    lines.append(f"**Oracle 实现率**: {oracle_realization:.1f}%")
    lines.append("")

    # --- 5. Bootstrap CI ---
    lines.append("## 5. Bootstrap 置信区间")
    lines.append("")
    lines.append(f"- 重采样次数: {bootstrap_ci.get('n_bootstrap', 1000)}")
    lines.append(f"- random_state: {bootstrap_ci.get('random_state', 42)}")
    lines.append("")
    overall_ci = bootstrap_ci.get("overall", {})
    lines.append(
        f"**Overall**: calendar_sum={overall_ci.get('calendar_sum', 0):+.4f}%, "
        f"CI [5th, 95th] = [{overall_ci.get('ci_5', 0):+.4f}%, "
        f"{overall_ci.get('ci_95', 0):+.4f}%]"
    )
    btc_ci = bootstrap_ci.get("BTC", {})
    lines.append(
        f"**BTC** ({btc_ci.get('n_events', 0)} events): "
        f"CI [{btc_ci.get('ci_5', 0):+.4f}%, {btc_ci.get('ci_95', 0):+.4f}%]"
    )
    eth_ci = bootstrap_ci.get("ETH", {})
    lines.append(
        f"**ETH** ({eth_ci.get('n_events', 0)} events): "
        f"CI [{eth_ci.get('ci_5', 0):+.4f}%, {eth_ci.get('ci_95', 0):+.4f}%]"
    )
    lines.append("")

    # --- 6. 分 Symbol 验证 ---
    lines.append("## 6. 分 Symbol 验证")
    lines.append("")
    for sym, sym_data in symbol_results.items():
        test_data = sym_data.get("test", {})
        flag = sym_data.get("symbol_not_superior_flag", False)
        lines.append(f"### {sym}")
        lines.append("")
        lines.append(
            f"- Test events: {test_data.get('n_events', 0)}"
        )
        lines.append(
            f"- Classifier calendar_sum: {test_data.get('classifier_calendar_sum', 0):+.4f}%"
        )
        lines.append(
            f"- Baseline B calendar_sum: {test_data.get('baseline_b_calendar_sum', 0):+.4f}%"
        )
        lines.append(
            f"- Classifier win_rate: {test_data.get('win_rate', 0):.1f}%"
        )
        if flag:
            lines.append(f"- ⚠ `symbol_not_superior_flag_{sym.lower()}=true`")
        else:
            lines.append(f"- ✓ Classifier superior for {sym}")
        lines.append("")

    # --- 7. 过拟合检查 ---
    lines.append("## 7. 过拟合检查")
    lines.append("")
    lines.append(f"- Train calendar_sum: {overfitting_check['train_cs']:+.4f}%")
    lines.append(f"- Test calendar_sum: {overfitting_check['test_cs']:+.4f}%")
    lines.append(f"- LOOCV calendar_sum: {overfitting_check['loocv_cs']:+.4f}%")
    lines.append(f"- Drop (train→test): {overfitting_check['drop_pct']:+.2f}%")
    lines.append(f"- overfitting_flag: {overfitting_check['overfitting_flag']}")
    lines.append(
        f"- loocv_degradation_flag: {overfitting_check['loocv_degradation_flag']}"
    )
    lines.append("")

    # --- 8. Regime 稳定性 ---
    lines.append("## 8. Regime 稳定性分析")
    lines.append("")
    lines.append(
        f"- label_distribution_shift: {regime_stability['label_distribution_shift']}"
    )
    lines.append(f"- Max proportion difference: {regime_stability['max_diff_pp']:.1f}pp")
    lines.append("")
    lines.append("| Regime | Train% | Test% | Δ(pp) |")
    lines.append("|--------|--------|-------|-------|")
    for row in regime_stability.get("per_regime", []):
        flag = " ⚠" if row["diff_pp"] > 15.0 else ""
        lines.append(
            f"| {row['regime']} | {row['train_pct']:.1f}% | "
            f"{row['test_pct']:.1f}% | {row['diff_pp']:.1f}{flag} |"
        )
    lines.append("")

    # --- 9. Feature Importance ---
    lines.append("## 9. Feature Importance")
    lines.append("")
    sorted_fi = sorted(
        best_classifier.feature_importance.items(),
        key=lambda x: x[1],
        reverse=True,
    )
    lines.append("| Rank | Feature | Importance |")
    lines.append("|------|---------|------------|")
    for rank, (feat, imp) in enumerate(sorted_fi, 1):
        lines.append(f"| {rank} | {feat} | {imp:.4f} |")
    lines.append("")

    # --- 10. Go/No-Go 决策 ---
    lines.append("## 10. Go/No-Go 决策")
    lines.append("")
    lines.append(f"### 判定结果: **{go_nogo}**")
    lines.append("")
    lines.append("判定依据:")
    lines.append(f"- Classifier test calendar_sum: {test_cs:+.4f}%")
    lines.append(f"- Baseline B (D=5s) test set calendar_sum: {baseline_b_cs_test:+.4f}%")
    lines.append(f"- 改善幅度: {improvement:+.4f}%")
    lines.append(f"- overfitting_flag: {overfitting_check['overfitting_flag']}")
    lines.append(
        f"- loocv_degradation_flag: {overfitting_check['loocv_degradation_flag']}"
    )
    lines.append("")

    # 判定规则说明
    lines.append("判定规则:")
    lines.append("- **Go**: classifier > baseline_b + 0.5% 且无 overfitting_flag")
    lines.append("- **Conditional Go**: classifier > baseline_b 但改善 < 0.5%，"
                 "或存在 loocv_degradation_flag")
    lines.append("- **No-Go**: classifier ≤ baseline_b 或 overfitting_flag=true")
    lines.append("")

    # --- 11. 下一步行动建议 ---
    lines.append("## 11. 下一步行动建议")
    lines.append("")
    if go_nogo == "Go":
        lines.append("✓ **推荐采用 pre-breakout timing classification**")
        lines.append("")
        lines.append("- 将规则型分类器的规则固化为 live-compatible 的 entry timing 逻辑")
        lines.append("- 参考 `classifier_rules.md` 中的 IF-THEN 规则")
        lines.append("- 在更多 V6 gate events 上持续验证")
    elif go_nogo == "Conditional Go":
        lines.append("~ **有条件采用，需进一步验证**")
        lines.append("")
        lines.append("- 等更多 V6 gate events 积累后重新验证")
        lines.append("- 尝试简化为 2-regime 分类（fast vs slow）")
        lines.append("- 关注 LOOCV 退化和 symbol 差异")
    else:
        lines.append("✗ **不采用 pre-breakout timing classification**")
        lines.append("")
        lines.append("- 确认固定 D=5s 为最优策略")
        lines.append("- 关闭 entry timing 优化方向")
        lines.append("- 资源转向其他研究方向")
    lines.append("")

    # --- 12. 与 dynamic-entry-timing 对比 ---
    lines.append("## 12. 与 dynamic-entry-timing 实验对比")
    lines.append("")
    lines.append("| 维度 | dynamic-entry-timing | pre-breakout-timing-classifier |")
    lines.append("|------|---------------------|-------------------------------|")
    lines.append("| 特征时间点 | Post-breakout（touch 后 5-60s） | "
                 "Pre-breakout（signal bar 时刻） |")
    lines.append("| 特征来源 | 1s bar 实时计算 | "
                 "events_execution_labeled.csv 已有列 |")
    lines.append("| 决策方式 | 步进式规则匹配 | 一次性分类 |")
    lines.append("| Calendar Sum | +2.40% | "
                 f"{test_cs:+.4f}% |")
    lines.append("| 已知问题 | flow_imbalance 为 placeholder | "
                 "116 events 样本量小 |")
    lines.append("| 结论 | No-Go | "
                 f"{go_nogo} |")
    lines.append("")
    lines.append("**分析**: Pre-breakout 方法使用 V6 gate 已验证有效的特征，")
    lines.append("避免了 post-breakout tick 特征的噪声问题。")
    lines.append("但受限于 116 events 的小样本量，统计显著性有限。")
    lines.append("")

    # --- 回调参数 ---
    lines.append("## 附录: 回调入场参数")
    lines.append("")
    lines.append(
        f"- pullback_target_atr: {best_pullback_config.pullback_target_atr}"
    )
    lines.append(
        f"- pullback_window_seconds: {best_pullback_config.pullback_window_seconds}"
    )
    lines.append(
        f"- start_offset_seconds: {best_pullback_config.start_offset_seconds}"
    )
    lines.append("")

    content = "\n".join(lines)
    path = output_dir / "pre_breakout_timing_report.md"
    path.write_text(content, encoding="utf-8")
