"""report_generator — 报告生成模块。

职责：
- Go/No-Go 判定逻辑
- 生成所有输出文件：
  - path_c_unified_report.md（中文主报告）
  - path_c_unified_summary.json（结构化 summary）
  - expanded_events_stats.csv（事件池统计）
  - rule_comparison.md（DT3 规则对比报告）
  - path_c_unified_trades.csv（test set trade ledger）
  - bootstrap_results.json（bootstrap CI 详细结果）
- 假设验证（H1/H2/H3）
- 历史对比表（Baseline Legacy, Refinement A2, Path C）
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

import pandas as pd

from .classifier_trainer import (
    DepthComparisonResult,
    RuleComparisonResult,
    TrainingResult,
)
from .delay_simulation_runner import SimulationStats
from .gate_explorer import GateExplorationReport
from .label_generator import LabelStats
from .robustness import RobustnessReport

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Data Classes
# ---------------------------------------------------------------------------


@dataclass
class GoNoGoDecision:
    """Go/No-Go 判定结果。"""

    decision: str  # "Strong Go" | "Marginal Go" | "No-Go"
    test_calendar_sum: float
    ci_lower: float
    overfitting_flag: bool
    reasoning: str
    next_steps: str


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def generate_report(
    gate_report: GateExplorationReport,
    simulation_stats: SimulationStats,
    label_stats: LabelStats,
    training_result: TrainingResult,
    rule_comparison: RuleComparisonResult,
    depth_comparison: DepthComparisonResult,
    robustness: RobustnessReport,
    historical_results: dict,
    output_dir: Path,
) -> GoNoGoDecision:
    """生成完整报告和所有输出文件。

    Parameters
    ----------
    gate_report : GateExplorationReport
        Gate 放宽探索报告。
    simulation_stats : SimulationStats
        Simulation 执行统计。
    label_stats : LabelStats
        标签分布统计。
    training_result : TrainingResult
        DT3 训练结果。
    rule_comparison : RuleComparisonResult
        规则对比结果。
    depth_comparison : DepthComparisonResult
        深度对比结果。
    robustness : RobustnessReport
        稳健性报告。
    historical_results : dict
        从 summary.json 读取的历史结果。
        Expected keys: "baseline_legacy", "refinement_a2"
        Each with: "test_cs", "ci_lower", "ci_upper"
    output_dir : Path
        输出目录。

    Returns
    -------
    GoNoGoDecision
        Go/No-Go 判定结果。
    """
    output_dir.mkdir(parents=True, exist_ok=True)

    # --- Step 1: Go/No-Go 判定 ---
    decision = _determine_go_nogo(
        test_calendar_sum=training_result.test_calendar_sum,
        overfitting_flag=robustness.overfitting_flag,
        ci_lower=robustness.overall_bootstrap.ci_lower,
    )
    logger.info("Go/No-Go decision: %s", decision.decision)

    # --- Step 2: 假设验证 ---
    hypothesis_validation = _validate_hypotheses(
        training_result=training_result,
        robustness=robustness,
        rule_comparison=rule_comparison,
    )

    # --- Step 3: 生成主报告 ---
    _generate_main_report(
        decision=decision,
        gate_report=gate_report,
        simulation_stats=simulation_stats,
        label_stats=label_stats,
        training_result=training_result,
        rule_comparison=rule_comparison,
        depth_comparison=depth_comparison,
        robustness=robustness,
        historical_results=historical_results,
        hypothesis_validation=hypothesis_validation,
        output_dir=output_dir,
    )

    # --- Step 4: 生成 summary.json ---
    _generate_summary_json(
        decision=decision,
        gate_report=gate_report,
        simulation_stats=simulation_stats,
        label_stats=label_stats,
        training_result=training_result,
        rule_comparison=rule_comparison,
        depth_comparison=depth_comparison,
        robustness=robustness,
        historical_results=historical_results,
        hypothesis_validation=hypothesis_validation,
        output_dir=output_dir,
    )

    # --- Step 5: 生成 expanded_events_stats.csv ---
    _generate_events_stats_csv(
        gate_report=gate_report,
        simulation_stats=simulation_stats,
        label_stats=label_stats,
        output_dir=output_dir,
    )

    # --- Step 6: 生成 rule_comparison.md ---
    _generate_rule_comparison_md(
        rule_comparison=rule_comparison,
        depth_comparison=depth_comparison,
        output_dir=output_dir,
    )

    # --- Step 7: 生成 path_c_unified_trades.csv ---
    _generate_trades_csv(
        training_result=training_result,
        output_dir=output_dir,
    )

    # --- Step 8: 生成 bootstrap_results.json ---
    _generate_bootstrap_json(
        robustness=robustness,
        output_dir=output_dir,
    )

    logger.info("所有报告文件已生成到: %s", output_dir)
    return decision


# ---------------------------------------------------------------------------
# Internal — Go/No-Go Decision Logic
# ---------------------------------------------------------------------------


def _determine_go_nogo(
    test_calendar_sum: float,
    overfitting_flag: bool,
    ci_lower: float,
) -> GoNoGoDecision:
    """Go/No-Go 判定逻辑。

    - Strong Go: test_cs > 3.50% AND no overfitting AND ci_lower > 2.0%
    - No-Go: test_cs <= 3.10%
    - Marginal Go: 其他情况
    """
    if test_calendar_sum <= 3.10:
        decision = "No-Go"
        reasoning = (
            f"test calendar_sum = +{test_calendar_sum:.2f}% ≤ +3.10% 阈值，"
            "timing signal 在扩大样本上未达到最低可接受水平。"
        )
        next_steps = (
            "正式关闭 pretouch timing 研究方向；"
            "资源转向其他 alpha 来源（如 volume profile、order flow）。"
        )
    elif (
        test_calendar_sum > 3.50
        and not overfitting_flag
        and ci_lower > 2.0
    ):
        decision = "Strong Go"
        reasoning = (
            f"test calendar_sum = +{test_calendar_sum:.2f}% > +3.50%，"
            f"bootstrap CI lower = +{ci_lower:.2f}% > +2.0%，"
            "无 overfitting flag。timing signal 在扩大样本上稳定且显著。"
        )
        next_steps = (
            "将 3-regime + DT3 规则固化为 live 入场 timing 逻辑；"
            "产出 production integration spec。"
        )
    else:
        decision = "Marginal Go"
        reasons: list[str] = []
        if test_calendar_sum <= 3.50:
            reasons.append(
                f"test_cs = +{test_calendar_sum:.2f}% 未超过 +3.50%"
            )
        if overfitting_flag:
            reasons.append("存在 overfitting flag")
        if ci_lower <= 2.0:
            reasons.append(f"CI lower = +{ci_lower:.2f}% ≤ +2.0%")
        reasoning = (
            f"test calendar_sum = +{test_calendar_sum:.2f}% 处于 Marginal Go 区间。"
            f"原因：{'；'.join(reasons)}。"
        )
        next_steps = (
            "分析瓶颈（gate 质量问题 vs 模型泛化问题）；"
            "考虑进一步扩展时间范围或引入新 symbol；"
            "或等待更多 V6 gate events 积累后重新验证。"
        )

    return GoNoGoDecision(
        decision=decision,
        test_calendar_sum=test_calendar_sum,
        ci_lower=ci_lower,
        overfitting_flag=overfitting_flag,
        reasoning=reasoning,
        next_steps=next_steps,
    )


# ---------------------------------------------------------------------------
# Internal — Hypothesis Validation
# ---------------------------------------------------------------------------


def _validate_hypotheses(
    training_result: TrainingResult,
    robustness: RobustnessReport,
    rule_comparison: RuleComparisonResult,
) -> dict:
    """验证 H1/H2/H3 假设。

    H1 (sample stability): test_cs > 3.10% AND CI width < 3.0pp
    H2 (gate quality): gate_quality.degradation == False
    H3 (rule convergence): rule_stability_score >= 0.5
    """
    test_cs = training_result.test_calendar_sum
    ci_width = robustness.overall_bootstrap.ci_width

    # H1: 样本稳定性
    h1_result = test_cs > 3.10 and ci_width < 3.0
    h1_evidence = (
        f"test_cs = +{test_cs:.2f}% {'>' if test_cs > 3.10 else '≤'} +3.10%；"
        f"CI width = {ci_width:.2f}pp {'<' if ci_width < 3.0 else '≥'} 3.0pp"
    )

    # H2: Gate 质量
    h2_result = not robustness.gate_quality.degradation
    h2_evidence = (
        f"原 116 events 平均 pnl = {robustness.gate_quality.original_mean_pnl:.4f}%，"
        f"新增 events 平均 pnl = {robustness.gate_quality.new_events_mean_pnl:.4f}%，"
        f"差异 = {robustness.gate_quality.pnl_diff:.4f}%，"
        f"p-value = {robustness.gate_quality.p_value:.4f}，"
        f"degradation = {robustness.gate_quality.degradation}"
    )

    # H3: 规则收敛
    h3_result = rule_comparison.stability_score >= 0.5
    h3_evidence = (
        f"Rule Stability Score = {rule_comparison.stability_score:.2f} "
        f"{'≥' if rule_comparison.stability_score >= 0.5 else '<'} 0.5；"
        f"{rule_comparison.diff_summary}"
    )

    return {
        "H1_sample_stability": {"result": h1_result, "evidence": h1_evidence},
        "H2_gate_quality": {"result": h2_result, "evidence": h2_evidence},
        "H3_rule_convergence": {"result": h3_result, "evidence": h3_evidence},
    }


# ---------------------------------------------------------------------------
# Internal — Main Report (path_c_unified_report.md)
# ---------------------------------------------------------------------------


def _generate_main_report(
    decision: GoNoGoDecision,
    gate_report: GateExplorationReport,
    simulation_stats: SimulationStats,
    label_stats: LabelStats,
    training_result: TrainingResult,
    rule_comparison: RuleComparisonResult,
    depth_comparison: DepthComparisonResult,
    robustness: RobustnessReport,
    historical_results: dict,
    hypothesis_validation: dict,
    output_dir: Path,
) -> None:
    """生成中文主报告 path_c_unified_report.md。"""
    lines: list[str] = []

    # --- Header & Disclaimer ---
    lines.append("# Path C Unified Pretouch — 实验报告")
    lines.append("")
    lines.append(f"生成时间：{datetime.now(timezone.utc).isoformat()}")
    lines.append("")
    lines.append("> **声明**：本实验不改变前两个 spec（`pre-breakout-timing-classifier`、")
    lines.append("> `pretouch-classifier-refinement`）的既有部署/发现。")
    lines.append("> 结论仅作为生产集成决策的输入。")
    lines.append("")

    # --- Go/No-Go Decision ---
    lines.append("## Go/No-Go 判定")
    lines.append("")
    lines.append(f"**决策：{decision.decision}**")
    lines.append("")
    lines.append(f"- test calendar_sum: **+{decision.test_calendar_sum:.2f}%**")
    lines.append(f"- bootstrap CI lower (5th): **+{decision.ci_lower:.2f}%**")
    lines.append(
        f"- bootstrap CI upper (95th): "
        f"**+{robustness.overall_bootstrap.ci_upper:.2f}%**"
    )
    lines.append(f"- overfitting_flag: **{decision.overfitting_flag}**")
    lines.append("")
    lines.append(f"**判定理由**：{decision.reasoning}")
    lines.append("")
    lines.append(f"**下一步**：{decision.next_steps}")
    lines.append("")

    # --- Hypothesis Validation ---
    lines.append("## 假设验证")
    lines.append("")
    for h_key, h_val in hypothesis_validation.items():
        status = "✅ 通过" if h_val["result"] else "❌ 未通过"
        lines.append(f"### {h_key}: {status}")
        lines.append("")
        lines.append(f"证据：{h_val['evidence']}")
        lines.append("")

    # --- Historical Comparison Table ---
    lines.append("## 历史对比")
    lines.append("")
    lines.append(_build_comparison_table(training_result, robustness, historical_results))
    lines.append("")

    # --- Event Pool Summary ---
    lines.append("## 事件池概况")
    lines.append("")
    lines.append(f"- 最终事件池大小: **{gate_report.final_n_events}** events")
    lines.append(f"- 包含原 116 events: **{gate_report.includes_original_116}**")
    lines.append(f"- 达到 200 events 目标: **{gate_report.expansion_target_met}**")
    lines.append(f"- 选定策略: {gate_report.selected_strategy}")
    lines.append("")

    # --- Simulation Stats ---
    lines.append("## Simulation 统计")
    lines.append("")
    lines.append(f"- 总 events: {simulation_stats.total_events}")
    lines.append(f"- 成功模拟: {simulation_stats.simulated_events}")
    lines.append(f"- 跳过: {simulation_stats.skipped_events}")
    if simulation_stats.drift_check:
        dc = simulation_stats.drift_check
        lines.append(f"- Drift check: n_compared={dc.n_compared}, "
                     f"max_diff={dc.max_pnl_diff:.6f}, "
                     f"warning={dc.drift_warning}")
    lines.append("")

    # --- Label Distribution ---
    lines.append("## 标签分布")
    lines.append("")
    lines.append("### Train set")
    for regime, count in label_stats.train_distribution.items():
        pct = label_stats.train_pct.get(regime, 0.0)
        lines.append(f"- {regime}: {count} ({pct:.1f}%)")
    lines.append("")
    lines.append("### Test set")
    for regime, count in label_stats.test_distribution.items():
        pct = label_stats.test_pct.get(regime, 0.0)
        lines.append(f"- {regime}: {count} ({pct:.1f}%)")
    lines.append("")
    lines.append(
        f"- label_shift_vs_original: {label_stats.label_shift_vs_original} "
        f"(原 skip={label_stats.original_skip_pct:.1f}%, "
        f"扩展 skip={label_stats.expanded_skip_pct:.1f}%)"
    )
    lines.append("")

    # --- Training Results ---
    lines.append("## DT3 训练结果")
    lines.append("")
    lines.append(f"- LOOCV calendar_sum: +{training_result.loocv_calendar_sum:.2f}%")
    lines.append(f"- Train calendar_sum: +{training_result.train_calendar_sum:.2f}%")
    lines.append(f"- Test calendar_sum: +{training_result.test_calendar_sum:.2f}%")
    lines.append(f"- Accuracy (train): {training_result.accuracy:.2%}")
    lines.append("")
    lines.append("### 决策规则")
    lines.append("")
    lines.append("```")
    lines.append(training_result.rule_text)
    lines.append("```")
    lines.append("")

    # --- Rule Comparison ---
    lines.append("## 规则对比")
    lines.append("")
    lines.append(f"- Rule Stability Score: **{rule_comparison.stability_score:.2f}**")
    lines.append(f"- Instability Warning: {rule_comparison.instability_warning}")
    lines.append(f"- {rule_comparison.diff_summary}")
    lines.append("")

    # --- Depth Comparison ---
    lines.append("## 深度对比")
    lines.append("")
    for depth, cs in sorted(depth_comparison.results_by_depth.items()):
        marker = " ← best" if depth == depth_comparison.best_depth else ""
        lines.append(f"- max_depth={depth}: LOOCV calendar_sum = +{cs:.2f}%{marker}")
    lines.append(f"- DT3 仍为最优: **{depth_comparison.dt3_still_optimal}**")
    lines.append("")

    # --- Robustness ---
    lines.append("## 稳健性验证")
    lines.append("")
    lines.append(
        f"- Overall bootstrap CI: "
        f"[+{robustness.overall_bootstrap.ci_lower:.2f}%, "
        f"+{robustness.overall_bootstrap.ci_upper:.2f}%] "
        f"(width={robustness.overall_bootstrap.ci_width:.2f}pp)"
    )
    lines.append(
        f"- CI width 对比: 原={robustness.ci_width_comparison['original']:.2f}pp, "
        f"扩展={robustness.ci_width_comparison['expanded']:.2f}pp"
    )
    lines.append(f"- Overfitting flag: {robustness.overfitting_flag}")
    lines.append(f"- LOOCV degradation flag: {robustness.loocv_degradation_flag}")
    lines.append(f"- Label distribution shift: {robustness.label_distribution_shift}")
    lines.append(f"- Gate quality degradation: {robustness.gate_quality.degradation}")
    lines.append(f"- Small sample warning: {robustness.small_sample_warning}")
    lines.append("")

    # --- Per-symbol Bootstrap ---
    lines.append("### Per-symbol Bootstrap")
    lines.append("")
    for symbol, br in robustness.symbol_bootstrap.items():
        lines.append(
            f"- {symbol}: mean=+{br.mean:.2f}%, "
            f"CI=[+{br.ci_lower:.2f}%, +{br.ci_upper:.2f}%]"
        )
    lines.append("")

    # Write file
    report_path = output_dir / "path_c_unified_report.md"
    report_path.write_text("\n".join(lines), encoding="utf-8")
    logger.info("主报告已生成: %s", report_path)


# ---------------------------------------------------------------------------
# Internal — Comparison Table
# ---------------------------------------------------------------------------


def _build_comparison_table(
    training_result: TrainingResult,
    robustness: RobustnessReport,
    historical_results: dict,
) -> str:
    """构建历史对比 Markdown 表格。"""
    # Extract historical data with safe defaults
    baseline = historical_results.get("baseline_legacy", {})
    refinement = historical_results.get("refinement_a2", {})

    baseline_cs = baseline.get("test_cs", 2.98)
    baseline_ci_lower = baseline.get("ci_lower", 1.16)
    baseline_ci_upper = baseline.get("ci_upper", 4.73)
    baseline_ci_width = baseline_ci_upper - baseline_ci_lower

    refinement_cs = refinement.get("test_cs", 3.39)
    refinement_ci_lower = refinement.get("ci_lower", 1.60)
    refinement_ci_upper = refinement.get("ci_upper", 5.18)
    refinement_ci_width = refinement_ci_upper - refinement_ci_lower

    path_c_cs = training_result.test_calendar_sum
    path_c_ci_lower = robustness.overall_bootstrap.ci_lower
    path_c_ci_upper = robustness.overall_bootstrap.ci_upper
    path_c_ci_width = robustness.overall_bootstrap.ci_width

    n_events_total = (
        robustness.ci_width_comparison.get("n_events_total", "N/A")
        if isinstance(robustness.ci_width_comparison, dict)
        else "N/A"
    )

    lines = [
        "| 指标 | Baseline Legacy (116 events) | Refinement A2 (116 events) | Path C |",
        "|------|-----|-----|-----|",
        f"| test calendar_sum | +{baseline_cs:.2f}% | +{refinement_cs:.2f}% | +{path_c_cs:.2f}% |",
        f"| bootstrap CI | [{baseline_ci_lower:.2f}%, {baseline_ci_upper:.2f}%] | "
        f"[{refinement_ci_lower:.2f}%, {refinement_ci_upper:.2f}%] | "
        f"[{path_c_ci_lower:.2f}%, {path_c_ci_upper:.2f}%] |",
        f"| CI width | {baseline_ci_width:.2f}pp | {refinement_ci_width:.2f}pp | {path_c_ci_width:.2f}pp |",
        f"| classifier | LogisticRegression | RuleBased_DT3 | RuleBased_DT3 |",
        f"| regime schema | 5-regime | 3-regime | 3-regime |",
    ]

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Internal — Summary JSON
# ---------------------------------------------------------------------------


def _generate_summary_json(
    decision: GoNoGoDecision,
    gate_report: GateExplorationReport,
    simulation_stats: SimulationStats,
    label_stats: LabelStats,
    training_result: TrainingResult,
    rule_comparison: RuleComparisonResult,
    depth_comparison: DepthComparisonResult,
    robustness: RobustnessReport,
    historical_results: dict,
    hypothesis_validation: dict,
    output_dir: Path,
) -> None:
    """生成 path_c_unified_summary.json。"""
    baseline = historical_results.get("baseline_legacy", {})
    refinement = historical_results.get("refinement_a2", {})

    summary = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "spec": "path-c-unified-pretouch",
        "go_nogo_decision": decision.decision,
        "n_events_total": gate_report.final_n_events,
        "n_events_train": sum(label_stats.train_distribution.values()),
        "n_events_test": sum(label_stats.test_distribution.values()),
        "test_calendar_sum": training_result.test_calendar_sum,
        "bootstrap_ci": {
            "lower": robustness.overall_bootstrap.ci_lower,
            "upper": robustness.overall_bootstrap.ci_upper,
            "width": robustness.overall_bootstrap.ci_width,
            "mean": robustness.overall_bootstrap.mean,
        },
        "ci_width_vs_original": robustness.ci_width_comparison,
        "rule_stability_score": rule_comparison.stability_score,
        "gate_quality_degradation": robustness.gate_quality.degradation,
        "overfitting_flag": robustness.overfitting_flag,
        "loocv_degradation_flag": robustness.loocv_degradation_flag,
        "small_sample_warning": robustness.small_sample_warning,
        "hypothesis_validation": hypothesis_validation,
        "comparison": {
            "baseline_legacy": {
                "test_cs": baseline.get("test_cs", 2.98),
                "ci": [
                    baseline.get("ci_lower", 1.16),
                    baseline.get("ci_upper", 4.73),
                ],
            },
            "refinement_a2": {
                "test_cs": refinement.get("test_cs", 3.39),
                "ci": [
                    refinement.get("ci_lower", 1.60),
                    refinement.get("ci_upper", 5.18),
                ],
            },
            "path_c": {
                "test_cs": training_result.test_calendar_sum,
                "ci": [
                    robustness.overall_bootstrap.ci_lower,
                    robustness.overall_bootstrap.ci_upper,
                ],
            },
        },
        "flags": {
            "small_sample_warning": robustness.small_sample_warning,
            "overfitting_flag": robustness.overfitting_flag,
            "loocv_degradation_flag": robustness.loocv_degradation_flag,
            "label_distribution_shift": robustness.label_distribution_shift,
            "gate_quality_degradation": robustness.gate_quality.degradation,
            "rule_instability_warning": rule_comparison.instability_warning,
            "expansion_target_met": gate_report.expansion_target_met,
        },
        "training": {
            "loocv_calendar_sum": training_result.loocv_calendar_sum,
            "train_calendar_sum": training_result.train_calendar_sum,
            "test_calendar_sum": training_result.test_calendar_sum,
            "accuracy": training_result.accuracy,
            "rule_text": training_result.rule_text,
            "feature_importances": training_result.feature_importances,
        },
        "depth_comparison": {
            "results_by_depth": {
                str(k): v for k, v in depth_comparison.results_by_depth.items()
            },
            "best_depth": depth_comparison.best_depth,
            "dt3_still_optimal": depth_comparison.dt3_still_optimal,
        },
        "simulation_stats": {
            "total_events": simulation_stats.total_events,
            "simulated_events": simulation_stats.simulated_events,
            "skipped_events": simulation_stats.skipped_events,
            "per_delay_traded_rate": simulation_stats.per_delay_traded_rate,
        },
        "reasoning": decision.reasoning,
        "next_steps": decision.next_steps,
    }

    summary_path = output_dir / "path_c_unified_summary.json"
    summary_path.write_text(
        json.dumps(summary, indent=2, ensure_ascii=False, default=_json_default),
        encoding="utf-8",
    )
    logger.info("Summary JSON 已生成: %s", summary_path)


def _json_default(obj):
    """Custom JSON serializer for numpy types."""
    import numpy as np
    if isinstance(obj, (np.bool_,)):
        return bool(obj)
    if isinstance(obj, (np.integer,)):
        return int(obj)
    if isinstance(obj, (np.floating,)):
        return float(obj)
    if isinstance(obj, np.ndarray):
        return obj.tolist()
    raise TypeError(f"Object of type {type(obj).__name__} is not JSON serializable")


# ---------------------------------------------------------------------------
# Internal — Events Stats CSV
# ---------------------------------------------------------------------------


def _generate_events_stats_csv(
    gate_report: GateExplorationReport,
    simulation_stats: SimulationStats,
    label_stats: LabelStats,
    output_dir: Path,
) -> None:
    """生成 expanded_events_stats.csv — 事件池统计。"""
    rows: list[dict] = []

    # 每个探索维度一行
    for dim in gate_report.dimensions:
        rows.append(
            {
                "dimension": dim.dimension,
                "description": dim.description,
                "n_events": dim.n_events,
                "overlap_with_original": dim.overlap_with_original,
                "symbol_btc": dim.symbol_distribution.get("BTCUSDT", 0),
                "symbol_eth": dim.symbol_distribution.get("ETHUSDT", 0),
                "side_long": dim.side_distribution.get("long", 0),
                "side_short": dim.side_distribution.get("short", 0),
                "time_range_start": dim.time_range[0] if dim.time_range else "",
                "time_range_end": dim.time_range[1] if dim.time_range else "",
                "sufficient": dim.sufficient,
            }
        )

    # 汇总行
    rows.append(
        {
            "dimension": "FINAL",
            "description": gate_report.selected_strategy,
            "n_events": gate_report.final_n_events,
            "overlap_with_original": 116 if gate_report.includes_original_116 else 0,
            "symbol_btc": "",
            "symbol_eth": "",
            "side_long": "",
            "side_short": "",
            "time_range_start": "",
            "time_range_end": "",
            "sufficient": gate_report.expansion_target_met,
        }
    )

    df = pd.DataFrame(rows)
    stats_path = output_dir / "expanded_events_stats.csv"
    df.to_csv(stats_path, index=False)
    logger.info("Events stats CSV 已生成: %s", stats_path)


# ---------------------------------------------------------------------------
# Internal — Rule Comparison MD
# ---------------------------------------------------------------------------


def _generate_rule_comparison_md(
    rule_comparison: RuleComparisonResult,
    depth_comparison: DepthComparisonResult,
    output_dir: Path,
) -> None:
    """生成 rule_comparison.md — DT3 规则对比报告。"""
    lines: list[str] = []

    lines.append("# DT3 规则对比报告")
    lines.append("")
    lines.append(f"生成时间：{datetime.now(timezone.utc).isoformat()}")
    lines.append("")

    lines.append("## Rule Stability Score")
    lines.append("")
    lines.append(f"**Score: {rule_comparison.stability_score:.2f}**")
    if rule_comparison.instability_warning:
        lines.append("")
        lines.append("⚠ **规则不稳定警告**：score < 0.5，扩展池训练的规则与原 A2 规则差异较大。")
    lines.append("")
    lines.append(f"差异摘要：{rule_comparison.diff_summary}")
    lines.append("")

    lines.append("## 原 A2 DT3 规则")
    lines.append("")
    lines.append("```")
    lines.append(rule_comparison.original_rule_text)
    lines.append("```")
    lines.append("")

    lines.append("## 扩展池 DT3 规则")
    lines.append("")
    lines.append("```")
    lines.append(rule_comparison.expanded_rule_text)
    lines.append("```")
    lines.append("")

    lines.append("## 深度对比（LOOCV）")
    lines.append("")
    lines.append("| max_depth | LOOCV calendar_sum | 备注 |")
    lines.append("|-----------|-------------------|------|")
    for depth, cs in sorted(depth_comparison.results_by_depth.items()):
        note = "← best" if depth == depth_comparison.best_depth else ""
        lines.append(f"| {depth} | +{cs:.2f}% | {note} |")
    lines.append("")
    lines.append(
        f"DT3 仍为最优深度: **{depth_comparison.dt3_still_optimal}** "
        f"(best_depth={depth_comparison.best_depth})"
    )
    lines.append("")

    report_path = output_dir / "rule_comparison.md"
    report_path.write_text("\n".join(lines), encoding="utf-8")
    logger.info("规则对比报告已生成: %s", report_path)


# ---------------------------------------------------------------------------
# Internal — Trades CSV
# ---------------------------------------------------------------------------


def _generate_trades_csv(
    training_result: TrainingResult,
    output_dir: Path,
) -> None:
    """生成 path_c_unified_trades.csv — test set trade ledger。

    Note: The actual trade data is stored in the expanded_delay_pnl_matrix.csv
    (generated by delay_simulation_runner). This file provides a filtered view
    of the test set predictions and their resolved trades.

    Since we only have predictions_test (list of labels) here, we generate
    a minimal ledger. The full trade details are in expanded_delay_pnl_matrix.csv.
    """
    # Generate a minimal predictions ledger
    rows: list[dict] = []
    for i, pred in enumerate(training_result.predictions_test):
        rows.append(
            {
                "test_event_index": i,
                "predicted_regime": pred,
            }
        )

    df = pd.DataFrame(rows)
    trades_path = output_dir / "path_c_unified_trades.csv"
    df.to_csv(trades_path, index=False)
    logger.info("Trades CSV 已生成: %s", trades_path)


# ---------------------------------------------------------------------------
# Internal — Bootstrap Results JSON
# ---------------------------------------------------------------------------


def _generate_bootstrap_json(
    robustness: RobustnessReport,
    output_dir: Path,
) -> None:
    """生成 bootstrap_results.json — 详细 bootstrap 结果。"""
    result = {
        "overall": {
            "n_bootstrap": robustness.overall_bootstrap.n_bootstrap,
            "mean": robustness.overall_bootstrap.mean,
            "ci_lower": robustness.overall_bootstrap.ci_lower,
            "ci_upper": robustness.overall_bootstrap.ci_upper,
            "ci_width": robustness.overall_bootstrap.ci_width,
            "values": robustness.overall_bootstrap.values,
        },
        "per_symbol": {},
        "gate_quality": {
            "original_mean_pnl": robustness.gate_quality.original_mean_pnl,
            "new_events_mean_pnl": robustness.gate_quality.new_events_mean_pnl,
            "pnl_diff": robustness.gate_quality.pnl_diff,
            "p_value": robustness.gate_quality.p_value,
            "degradation": robustness.gate_quality.degradation,
        },
        "ci_width_comparison": robustness.ci_width_comparison,
    }

    for symbol, br in robustness.symbol_bootstrap.items():
        result["per_symbol"][symbol] = {
            "n_bootstrap": br.n_bootstrap,
            "mean": br.mean,
            "ci_lower": br.ci_lower,
            "ci_upper": br.ci_upper,
            "ci_width": br.ci_width,
            "values": br.values,
        }

    bootstrap_path = output_dir / "bootstrap_results.json"
    bootstrap_path.write_text(
        json.dumps(result, indent=2, ensure_ascii=False, default=_json_default), encoding="utf-8"
    )
    logger.info("Bootstrap results JSON 已生成: %s", bootstrap_path)
