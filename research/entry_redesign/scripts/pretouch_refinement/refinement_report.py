"""
refinement_report — 对比报告生成 + Go/No-Go 判定

产出 9 个输出文件到 output/pretouch_refinement/。
"""

from __future__ import annotations

import json
import logging
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

import pandas as pd

# ---------------------------------------------------------------------------
# Path setup for importing sibling modules
# ---------------------------------------------------------------------------

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pretouch_refinement.arm_runner import ArmResult
from pretouch_refinement.ablation import AblationResult

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Go/No-Go 阈值
# ---------------------------------------------------------------------------

STRONG_GO_THRESHOLD: float = 3.50  # test calendar_sum > +3.50%
MARGINAL_GO_LOWER: float = 3.10  # test calendar_sum > +3.10%
BASELINE_LEGACY_CS: float = 2.98  # pre-breakout-timing-classifier 结果


# ---------------------------------------------------------------------------
# 报告结构
# ---------------------------------------------------------------------------


@dataclass
class GoNoGoDecision:
    """Go/No-Go 判定结果"""

    decision: str  # "Strong Go" | "Marginal Go" | "No-Go"
    best_arm_name: str
    best_classifier_name: str
    test_calendar_sum: float
    improvement_vs_legacy: float
    overfitting_flag: bool
    loocv_degradation_flag: bool
    no_refinement_superior: bool
    reasoning: str  # 中文判定理由


# ---------------------------------------------------------------------------
# Go/No-Go 判定
# ---------------------------------------------------------------------------


def determine_go_nogo(
    arm_results: list[ArmResult],
    baseline_legacy_cs: float = BASELINE_LEGACY_CS,
    overfitting_flag: bool = False,
    loocv_degradation_flag: bool = False,
) -> GoNoGoDecision:
    """Go/No-Go 判定。

    判定逻辑：
    - Strong Go: best test_cs > +3.50% 且无 overfitting_flag
    - Marginal Go: test_cs ∈ (+3.10%, +3.50%] 或有 loocv_degradation_flag
    - No-Go: test_cs ≤ +3.10%

    Parameters
    ----------
    arm_results : list[ArmResult]
        6 个 arm 的完整结果（含 Baseline）。
    baseline_legacy_cs : float
        Baseline_Legacy 的 test calendar_sum（默认 2.98%）。
    overfitting_flag : bool
        是否存在 overfitting 标志（test < 0.5×train）。
    loocv_degradation_flag : bool
        是否存在 LOOCV degradation 标志。

    Returns
    -------
    GoNoGoDecision
        包含判定结果、最优 arm 信息和中文理由。
    """
    # 1. 找到最优 arm（test_calendar_sum 最高）
    best_arm = max(arm_results, key=lambda r: r.test_calendar_sum)
    best_test_cs = best_arm.test_calendar_sum
    best_arm_name = best_arm.config.name
    best_classifier_name = getattr(best_arm.best_classifier, "name", "Unknown")
    improvement_vs_legacy = best_test_cs - baseline_legacy_cs

    # 2. 检查 no_refinement_superior：所有 treatment arm (A1-A5) 的 test_cs
    #    是否严格低于 baseline_legacy_cs
    treatment_arms = [r for r in arm_results if r.config.name != "Baseline"]
    no_refinement_superior = all(
        r.test_calendar_sum < baseline_legacy_cs for r in treatment_arms
    )

    # 3. 判定逻辑
    if no_refinement_superior:
        decision = "No-Go"
        reasoning = (
            f"所有 treatment arm (A1-A5) 的 test calendar_sum 均严格低于 "
            f"Baseline_Legacy ({baseline_legacy_cs:.2f}%)，"
            f"no_refinement_superior=true。建议直接推进 Path C，跳过 refinement。"
        )
    elif best_test_cs > STRONG_GO_THRESHOLD and not overfitting_flag:
        decision = "Strong Go"
        reasoning = (
            f"最优 arm {best_arm_name} ({best_classifier_name}) 的 "
            f"test calendar_sum = +{best_test_cs:.2f}% > Strong Go 阈值 "
            f"(+{STRONG_GO_THRESHOLD:.2f}%)，且无 overfitting 标志。"
            f"相对 Baseline_Legacy 改善 +{improvement_vs_legacy:.2f}pp。"
            f"建议推进 Path C unified spec。"
        )
    elif best_test_cs > MARGINAL_GO_LOWER or loocv_degradation_flag:
        decision = "Marginal Go"
        reasons: list[str] = []
        if best_test_cs > MARGINAL_GO_LOWER:
            reasons.append(
                f"test calendar_sum = +{best_test_cs:.2f}% 处于 Marginal Go 区间 "
                f"(+{MARGINAL_GO_LOWER:.2f}%, +{STRONG_GO_THRESHOLD:.2f}%]"
            )
        if loocv_degradation_flag:
            reasons.append("存在 LOOCV degradation 标志")
        if overfitting_flag:
            reasons.append("存在 overfitting 标志，降级为 Marginal Go")
        reasoning = (
            f"最优 arm {best_arm_name} ({best_classifier_name})：{'；'.join(reasons)}。"
            f"相对 Baseline_Legacy 改善 +{improvement_vs_legacy:.2f}pp。"
            f"建议等更多 V6 gate events 积累或 Path C 扩大 events 池。"
        )
    else:
        decision = "No-Go"
        reasoning = (
            f"最优 arm {best_arm_name} ({best_classifier_name}) 的 "
            f"test calendar_sum = +{best_test_cs:.2f}% ≤ No-Go 阈值 "
            f"(+{MARGINAL_GO_LOWER:.2f}%)。"
            f"相对 Baseline_Legacy 改善 +{improvement_vs_legacy:.2f}pp，不足以推进。"
            f"建议保持 pre-breakout-timing-classifier 的 Conditional Go 结论。"
        )

    return GoNoGoDecision(
        decision=decision,
        best_arm_name=best_arm_name,
        best_classifier_name=best_classifier_name,
        test_calendar_sum=best_test_cs,
        improvement_vs_legacy=improvement_vs_legacy,
        overfitting_flag=overfitting_flag,
        loocv_degradation_flag=loocv_degradation_flag,
        no_refinement_superior=no_refinement_superior,
        reasoning=reasoning,
    )


# ---------------------------------------------------------------------------
# 假设验证
# ---------------------------------------------------------------------------


def generate_hypothesis_validation(
    arm_results: list[ArmResult],
    ablation_results: list[AblationResult] | None = None,
) -> str:
    """生成假设验证段落（H1/H2/H3）。

    H1（特征假设）：A1 vs Baseline 的 test_cs 差异 > 0.5%？
    H2（Regime 假设）：A2/A3 vs Baseline 的 test_cs 差异 > 0.5%？
    H3（联合假设）：仅 A4/A5 达到 Strong Go 阈值？

    Parameters
    ----------
    arm_results : list[ArmResult]
        6 个 arm 的完整结果。
    ablation_results : list[AblationResult] | None
        消融实验结果（可选，用于补充分析）。

    Returns
    -------
    str
        中文假设验证段落文本。
    """
    # 按 arm name 建立索引
    arm_map: dict[str, ArmResult] = {r.config.name: r for r in arm_results}

    baseline = arm_map.get("Baseline")
    a1 = arm_map.get("A1")
    a2 = arm_map.get("A2")
    a3 = arm_map.get("A3")
    a4 = arm_map.get("A4")
    a5 = arm_map.get("A5")

    baseline_cs = baseline.test_calendar_sum if baseline else BASELINE_LEGACY_CS

    lines: list[str] = []
    lines.append("## 假设验证")
    lines.append("")

    # --- H1: 特征假设 ---
    lines.append("### H1（特征假设）：增强特征是否显著提升分类器表现？")
    lines.append("")
    if a1 is not None:
        a1_cs = a1.test_calendar_sum
        h1_delta = a1_cs - baseline_cs
        h1_supported = h1_delta > 0.5
        lines.append(
            f"- A1 (原+增强特征, 5-regime) test calendar_sum = +{a1_cs:.2f}%"
        )
        lines.append(f"- Baseline test calendar_sum = +{baseline_cs:.2f}%")
        lines.append(f"- 差异 = +{h1_delta:.2f}pp")
        if h1_supported:
            lines.append(
                f"- **H1 成立**：差异 +{h1_delta:.2f}pp > 0.5%，"
                f"特征信号不足是主要瓶颈之一。"
            )
        else:
            lines.append(
                f"- **H1 不成立**：差异 +{h1_delta:.2f}pp ≤ 0.5%，"
                f"增强特征未能显著提升分类器表现。"
            )
    else:
        lines.append("- A1 结果缺失，无法验证 H1。")
    lines.append("")

    # --- H2: Regime 假设 ---
    lines.append("### H2（Regime 假设）：Regime 简化是否显著提升分类器表现？")
    lines.append("")
    if a2 is not None and a3 is not None:
        a2_cs = a2.test_calendar_sum
        a3_cs = a3.test_calendar_sum
        best_regime_cs = max(a2_cs, a3_cs)
        best_regime_name = "A2 (3-regime)" if a2_cs >= a3_cs else "A3 (2-regime)"
        h2_delta = best_regime_cs - baseline_cs
        h2_supported = h2_delta > 0.5
        lines.append(
            f"- A2 (原特征, 3-regime) test calendar_sum = +{a2_cs:.2f}%"
        )
        lines.append(
            f"- A3 (原特征, 2-regime) test calendar_sum = +{a3_cs:.2f}%"
        )
        lines.append(f"- Baseline test calendar_sum = +{baseline_cs:.2f}%")
        lines.append(
            f"- 最优 Regime 简化 arm: {best_regime_name}，差异 = +{h2_delta:.2f}pp"
        )
        if h2_supported:
            lines.append(
                f"- **H2 成立**：差异 +{h2_delta:.2f}pp > 0.5%，"
                f"类别稀疏是主要瓶颈之一。"
            )
        else:
            lines.append(
                f"- **H2 不成立**：差异 +{h2_delta:.2f}pp ≤ 0.5%，"
                f"Regime 简化未能显著提升分类器表现。"
            )
    else:
        lines.append("- A2/A3 结果缺失，无法验证 H2。")
    lines.append("")

    # --- H3: 联合假设 ---
    lines.append("### H3（联合假设）：仅联合路径达到 Strong Go？")
    lines.append("")
    if a1 is not None and a2 is not None and a3 is not None and a4 is not None and a5 is not None:
        # 检查 A1/A2/A3 是否未达到 Strong Go
        single_path_below = (
            a1.test_calendar_sum <= STRONG_GO_THRESHOLD
            and a2.test_calendar_sum <= STRONG_GO_THRESHOLD
            and a3.test_calendar_sum <= STRONG_GO_THRESHOLD
        )
        # 检查 A4/A5 是否达到 Strong Go
        a4_cs = a4.test_calendar_sum
        a5_cs = a5.test_calendar_sum
        joint_above = (
            a4_cs > STRONG_GO_THRESHOLD or a5_cs > STRONG_GO_THRESHOLD
        )
        h3_supported = single_path_below and joint_above

        lines.append(
            f"- A1 test calendar_sum = +{a1.test_calendar_sum:.2f}% "
            f"({'> ' if a1.test_calendar_sum > STRONG_GO_THRESHOLD else '≤ '}"
            f"Strong Go 阈值 +{STRONG_GO_THRESHOLD:.2f}%)"
        )
        lines.append(
            f"- A2 test calendar_sum = +{a2.test_calendar_sum:.2f}% "
            f"({'> ' if a2.test_calendar_sum > STRONG_GO_THRESHOLD else '≤ '}"
            f"Strong Go 阈值)"
        )
        lines.append(
            f"- A3 test calendar_sum = +{a3.test_calendar_sum:.2f}% "
            f"({'> ' if a3.test_calendar_sum > STRONG_GO_THRESHOLD else '≤ '}"
            f"Strong Go 阈值)"
        )
        lines.append(
            f"- A4 (原+增强, 3-regime) test calendar_sum = +{a4_cs:.2f}% "
            f"({'> ' if a4_cs > STRONG_GO_THRESHOLD else '≤ '}"
            f"Strong Go 阈值)"
        )
        lines.append(
            f"- A5 (原+增强, 2-regime) test calendar_sum = +{a5_cs:.2f}% "
            f"({'> ' if a5_cs > STRONG_GO_THRESHOLD else '≤ '}"
            f"Strong Go 阈值)"
        )
        lines.append("")
        if h3_supported:
            lines.append(
                "- **H3 成立**：仅 A4/A5（联合路径）达到 Strong Go 阈值，"
                "而 A1/A2/A3（单一路径）均未达到。"
                "说明特征增强与 Regime 简化两个瓶颈同时存在且独立贡献。"
            )
        else:
            if not single_path_below:
                lines.append(
                    "- **H3 不成立**：A1/A2/A3 中已有 arm 达到 Strong Go 阈值，"
                    "联合路径非唯一达标方式。"
                )
            elif not joint_above:
                lines.append(
                    "- **H3 不成立**：A4/A5（联合路径）均未达到 Strong Go 阈值，"
                    "联合路径也未能突破。"
                )
            else:
                lines.append("- **H3 不成立**。")
    else:
        lines.append("- A1-A5 结果不完整，无法验证 H3。")
    lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# 声明与保护逻辑 (Task 9.4)
# ---------------------------------------------------------------------------

# 保护声明常量
REFINEMENT_NOOP_THRESHOLD: float = 0.05  # 差异 < 0.05% 视为 noop
SMALL_SAMPLE_WARNING: bool = True  # 始终声明小样本警告


def check_refinement_noop(
    best_test_cs: float,
    baseline_legacy_cs: float,
    threshold: float = REFINEMENT_NOOP_THRESHOLD,
) -> bool:
    """检查 refinement 是否为 noop（差异 < 0.05%）。

    Parameters
    ----------
    best_test_cs : float
        最优 arm 的 test calendar_sum。
    baseline_legacy_cs : float
        Baseline_Legacy 的 test calendar_sum。
    threshold : float
        noop 阈值（默认 0.05%）。

    Returns
    -------
    bool
        True 表示 refinement_noop（差异不显著）。
    """
    return abs(best_test_cs - baseline_legacy_cs) < threshold


def check_accuracy_calendar_decoupled(
    best_arm: ArmResult,
    baseline_legacy_cs: float,
    accuracy_threshold: float = 40.0,
) -> bool:
    """检查 accuracy 与 calendar_sum 是否脱钩。

    当 best_arm 的 test accuracy < 40% 但 test calendar_sum 显著高于
    Baseline_Legacy 时，标注 accuracy_calendar_decoupled=true。

    Parameters
    ----------
    best_arm : ArmResult
        最优 arm 结果。
    baseline_legacy_cs : float
        Baseline_Legacy 的 test calendar_sum。
    accuracy_threshold : float
        accuracy 阈值（默认 40%）。

    Returns
    -------
    bool
        True 表示 accuracy 与 calendar_sum 脱钩。
    """
    test_accuracy = getattr(best_arm.best_classifier, "test_accuracy", 100.0)
    test_cs = best_arm.test_calendar_sum
    # accuracy < 40% 但 test_cs 显著高于 baseline（改善 > 0.5%）
    return test_accuracy < accuracy_threshold and (test_cs - baseline_legacy_cs) > 0.5


def generate_declarations_section(
    go_nogo: GoNoGoDecision,
    refinement_noop: bool,
    accuracy_calendar_decoupled: bool,
) -> str:
    """生成声明与保护逻辑段落。

    包含：
    - small_sample_warning=true 声明
    - 本 refinement 不改变既有部署/发现的声明
    - refinement_noop 检查结果
    - accuracy_calendar_decoupled 检查结果

    Parameters
    ----------
    go_nogo : GoNoGoDecision
        Go/No-Go 判定结果。
    refinement_noop : bool
        是否为 noop。
    accuracy_calendar_decoupled : bool
        accuracy 与 calendar_sum 是否脱钩。

    Returns
    -------
    str
        中文声明段落文本。
    """
    lines: list[str] = []
    lines.append("## 声明与保护")
    lines.append("")
    lines.append("> **⚠️ 重要声明**")
    lines.append(">")
    lines.append(
        "> 本 refinement 不改变 `pre-breakout-timing-classifier` 的既有部署/发现。"
    )
    lines.append(
        "> 结论仅作为下一阶段 Path C unified spec 的决策输入。"
    )
    lines.append("")
    lines.append("### 保护标志")
    lines.append("")
    lines.append(f"- `small_sample_warning=true`：样本量有限（116 events），"
                 f"所有结论附带 bootstrap 置信区间。")
    lines.append(f"- `refinement_noop={str(refinement_noop).lower()}`"
                 + (f"：最优 arm 与 Baseline_Legacy 差异 < {REFINEMENT_NOOP_THRESHOLD}%，"
                    f"refinement 未产生实质改善。" if refinement_noop
                    else f"：最优 arm 与 Baseline_Legacy 差异 ≥ {REFINEMENT_NOOP_THRESHOLD}%。"))
    lines.append(f"- `accuracy_calendar_decoupled={str(accuracy_calendar_decoupled).lower()}`"
                 + ("：test accuracy 低于 40% 但 calendar_sum 显著高于 Baseline_Legacy，"
                    "说明 accuracy 与 calendar_sum 不必相关，后者是业务真实目标。"
                    if accuracy_calendar_decoupled
                    else "：accuracy 与 calendar_sum 未出现显著脱钩。"))
    lines.append(f"- `no_refinement_superior={str(go_nogo.no_refinement_superior).lower()}`"
                 + ("：所有 treatment arm 均未超越 Baseline_Legacy。"
                    if go_nogo.no_refinement_superior
                    else "：存在 treatment arm 超越 Baseline_Legacy。"))
    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# 报告生成主函数 (Task 9.2)
# ---------------------------------------------------------------------------


def generate_refinement_report(
    arm_results: list[ArmResult],
    ablation_results: list[AblationResult],
    go_nogo: GoNoGoDecision,
    label_distributions: pd.DataFrame,
    pit_audit_entries: list,
    baseline_legacy_summary: dict,
    robustness_results: dict,
    output_dir: Path,
    best_global_delay: str = "D0",
) -> None:
    """生成全部 9 个输出文件。

    本函数直接生成以下 4 个文件：
    1. pretouch_refinement_report.md（中文主报告）
    2. pretouch_refinement_summary.json（结构化 summary）
    3. feature_importance_best_arm.csv（最优 arm 特征重要性）
    4. pretouch_refinement_trades.csv（最优 arm test trade ledger）

    并验证以下 5 个文件已由前序步骤生成：
    5. arm_comparison.csv（由 arm_runner 生成）
    6. ablation_results.csv（由 ablation 生成）
    7. ablation_report.md（由 ablation 生成）
    8. regime_label_distributions.csv（由 refinement_runner Step 2 生成）
    9. pretouch_features_pit_audit.md（由 refinement_runner Step 3 生成）

    Parameters
    ----------
    arm_results : list[ArmResult]
        6 个 arm 的完整结果。
    ablation_results : list[AblationResult]
        消融实验结果（可能为空列表）。
    go_nogo : GoNoGoDecision
        Go/No-Go 判定结果。
    label_distributions : pd.DataFrame
        标签分布数据。
    pit_audit_entries : list
        PIT audit 记录列表。
    baseline_legacy_summary : dict
        Baseline_Legacy 的 summary.json 内容。
    robustness_results : dict
        稳健性验证结果。
    output_dir : Path
        输出目录。
    best_global_delay : str
        Best_Global_Delay（默认 "D0"）。
    """
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    # 找到最优 arm
    best_arm = max(arm_results, key=lambda r: r.test_calendar_sum)
    baseline_legacy_cs = baseline_legacy_summary.get(
        "best_classifier", {}
    ).get("test_calendar_sum", BASELINE_LEGACY_CS)

    # --- 声明与保护逻辑检查 (Task 9.4) ---
    refinement_noop = check_refinement_noop(
        best_arm.test_calendar_sum, baseline_legacy_cs
    )
    accuracy_calendar_decoupled = check_accuracy_calendar_decoupled(
        best_arm, baseline_legacy_cs
    )

    # --- 1. 生成 pretouch_refinement_report.md ---
    _generate_main_report(
        arm_results=arm_results,
        ablation_results=ablation_results,
        go_nogo=go_nogo,
        baseline_legacy_summary=baseline_legacy_summary,
        robustness_results=robustness_results,
        refinement_noop=refinement_noop,
        accuracy_calendar_decoupled=accuracy_calendar_decoupled,
        output_dir=output_dir,
    )

    # --- 2. 生成 pretouch_refinement_summary.json ---
    _generate_summary_json(
        arm_results=arm_results,
        ablation_results=ablation_results,
        go_nogo=go_nogo,
        baseline_legacy_summary=baseline_legacy_summary,
        robustness_results=robustness_results,
        refinement_noop=refinement_noop,
        accuracy_calendar_decoupled=accuracy_calendar_decoupled,
        best_global_delay=best_global_delay,
        output_dir=output_dir,
    )

    # --- 3. 生成 feature_importance_best_arm.csv ---
    _generate_feature_importance_csv(best_arm, output_dir)

    # --- 4. 生成 pretouch_refinement_trades.csv ---
    _generate_trades_csv(best_arm, output_dir)

    # --- 5-9. 验证前序步骤已生成的文件 ---
    expected_files = [
        "arm_comparison.csv",
        "ablation_results.csv",
        "ablation_report.md",
        "regime_label_distributions.csv",
        "pretouch_features_pit_audit.md",
    ]
    missing_files: list[str] = []
    for fname in expected_files:
        fpath = output_dir / fname
        if not fpath.exists():
            missing_files.append(fname)
            logger.warning(f"预期文件缺失: {fpath}")

    if missing_files:
        logger.warning(
            f"以下文件未由前序步骤生成（可能因消融跳过等原因）: {missing_files}"
        )

    # 汇总输出
    all_output_files = [
        "pretouch_refinement_report.md",
        "pretouch_refinement_summary.json",
        "arm_comparison.csv",
        "ablation_results.csv",
        "ablation_report.md",
        "regime_label_distributions.csv",
        "feature_importance_best_arm.csv",
        "pretouch_features_pit_audit.md",
        "pretouch_refinement_trades.csv",
    ]
    existing_count = sum(
        1 for f in all_output_files if (output_dir / f).exists()
    )
    logger.info(
        f"报告生成完成: {existing_count}/{len(all_output_files)} 个文件已就绪"
    )


# ---------------------------------------------------------------------------
# 内部函数：主报告生成
# ---------------------------------------------------------------------------


def _generate_main_report(
    arm_results: list[ArmResult],
    ablation_results: list[AblationResult],
    go_nogo: GoNoGoDecision,
    baseline_legacy_summary: dict,
    robustness_results: dict,
    refinement_noop: bool,
    accuracy_calendar_decoupled: bool,
    output_dir: Path,
) -> None:
    """生成 pretouch_refinement_report.md 中文主报告。"""
    best_arm = max(arm_results, key=lambda r: r.test_calendar_sum)
    baseline_legacy_cs = baseline_legacy_summary.get(
        "best_classifier", {}
    ).get("test_calendar_sum", BASELINE_LEGACY_CS)
    oracle_cs = baseline_legacy_summary.get("oracle_calendar_sum", 0.0)

    lines: list[str] = []

    # --- Header + Disclaimer (Req 7.7) ---
    lines.append("# Pretouch Classifier Refinement 报告")
    lines.append("")
    lines.append(f"生成时间: {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')}")
    lines.append("")
    lines.append("> **⚠️ 声明**")
    lines.append(">")
    lines.append(
        "> 本 refinement 不改变 `pre-breakout-timing-classifier` 的既有部署/发现。"
    )
    lines.append(
        "> 结论仅作为下一阶段 Path C unified spec 的决策输入。"
    )
    lines.append(">")
    lines.append(f"> `small_sample_warning=true`：样本量有限（116 events），"
                 f"所有结论附带 bootstrap 置信区间。")
    lines.append("")
    lines.append("---")
    lines.append("")

    # --- Go/No-Go 判定 ---
    lines.append("## Go/No-Go 判定")
    lines.append("")
    lines.append(f"**判定结果: {go_nogo.decision}**")
    lines.append("")
    lines.append(f"- 最优 arm: **{go_nogo.best_arm_name}** ({go_nogo.best_classifier_name})")
    lines.append(f"- Test calendar_sum: **+{go_nogo.test_calendar_sum:.4f}%**")
    lines.append(f"- 相对 Baseline_Legacy 改善: **{go_nogo.improvement_vs_legacy:+.4f}pp**")
    lines.append(f"- Overfitting flag: {go_nogo.overfitting_flag}")
    lines.append(f"- LOOCV degradation flag: {go_nogo.loocv_degradation_flag}")
    lines.append(f"- No refinement superior: {go_nogo.no_refinement_superior}")
    lines.append("")
    lines.append(f"**理由**: {go_nogo.reasoning}")
    lines.append("")

    # Bootstrap CI
    bootstrap_ci = robustness_results.get("bootstrap_ci", {})
    if bootstrap_ci:
        lines.append(
            f"- Bootstrap 90% CI: [{bootstrap_ci.get('ci_lower', 0):.4f}%, "
            f"{bootstrap_ci.get('ci_upper', 0):.4f}%]"
        )
        lines.append("")

    # --- 假设验证 ---
    hypothesis_text = generate_hypothesis_validation(arm_results, ablation_results)
    lines.append(hypothesis_text)

    # --- 消融分析摘要 ---
    lines.append("## 消融分析摘要")
    lines.append("")
    if ablation_results:
        high_value = [r for r in ablation_results if r.high_value_group]
        negative_value = [r for r in ablation_results if r.negative_value_group]

        lines.append(f"| 特征组 | Delta test_cs | 95% CI | 高价值 | 负价值 |")
        lines.append(f"|--------|--------------|--------|--------|--------|")
        for ar in ablation_results:
            hv = "✓" if ar.high_value_group else ""
            nv = "✓" if ar.negative_value_group else ""
            lines.append(
                f"| {ar.group_name} | {ar.delta_test_cs:+.4f}% | "
                f"[{ar.bootstrap_ci_lower:+.4f}, {ar.bootstrap_ci_upper:+.4f}] | "
                f"{hv} | {nv} |"
            )
        lines.append("")

        if high_value:
            lines.append(f"**高价值特征组** (剔除后下降 > 0.3%): "
                         f"{', '.join(r.group_name for r in high_value)}")
            lines.append("")
        if negative_value:
            lines.append(f"**负价值特征组** (剔除后不变或上升，建议 Path C 排除): "
                         f"{', '.join(r.group_name for r in negative_value)}")
            lines.append("")
    else:
        lines.append("消融实验已跳过（最优 arm 不含增强特征）。")
        lines.append("")

    # --- Oracle 实现率对比 ---
    lines.append("## Oracle 实现率对比")
    lines.append("")
    if oracle_cs > 0:
        best_oracle_rate = best_arm.oracle_realization_rate
        baseline_oracle_rate = (baseline_legacy_cs / oracle_cs) * 100.0
        lines.append(f"| 指标 | Best Arm ({best_arm.config.name}) | Baseline_Legacy |")
        lines.append(f"|------|------|------|")
        lines.append(f"| Test calendar_sum | +{best_arm.test_calendar_sum:.4f}% | +{baseline_legacy_cs:.4f}% |")
        lines.append(f"| Oracle calendar_sum | +{oracle_cs:.4f}% | +{oracle_cs:.4f}% |")
        lines.append(f"| Oracle 实现率 | {best_oracle_rate:.2f}% | {baseline_oracle_rate:.2f}% |")
        lines.append(f"| 实现率改善 | {best_oracle_rate - baseline_oracle_rate:+.2f}pp | — |")
    else:
        lines.append("Oracle calendar_sum 不可用，无法计算实现率。")
    lines.append("")

    # --- Baseline_Legacy 对比表 ---
    lines.append("## 与 Baseline_Legacy 数值对照表")
    lines.append("")
    lines.append(f"| Arm | Best Classifier | Test CS | vs Legacy (pp) | vs Legacy (%) | Oracle 实现率 |")
    lines.append(f"|-----|----------------|---------|----------------|---------------|--------------|")

    # Baseline_Legacy 行
    lines.append(
        f"| Baseline_Legacy | "
        f"{baseline_legacy_summary.get('best_classifier', {}).get('name', 'LR')} | "
        f"+{baseline_legacy_cs:.4f}% | — | — | "
        f"{(baseline_legacy_cs / oracle_cs * 100.0) if oracle_cs > 0 else 0:.2f}% |"
    )

    # 各 arm 行
    for ar in arm_results:
        clf_name = getattr(ar.best_classifier, "name", "Unknown")
        delta_pp = ar.test_calendar_sum - baseline_legacy_cs
        delta_pct = (delta_pp / baseline_legacy_cs * 100.0) if baseline_legacy_cs != 0 else 0.0
        lines.append(
            f"| {ar.config.name} | {clf_name} | "
            f"+{ar.test_calendar_sum:.4f}% | "
            f"{delta_pp:+.4f} | "
            f"{delta_pct:+.2f}% | "
            f"{ar.oracle_realization_rate:.2f}% |"
        )
    lines.append("")

    # --- 声明与保护逻辑 (Task 9.4) ---
    declarations_text = generate_declarations_section(
        go_nogo, refinement_noop, accuracy_calendar_decoupled
    )
    lines.append(declarations_text)

    # --- 下一步建议 ---
    lines.append("## 下一步建议")
    lines.append("")
    if go_nogo.decision == "Strong Go":
        lines.append("1. 把最优 arm 的特征+regime 组合固化到 Path C unified spec")
        lines.append("2. 把高价值特征组列为 Path C 必要特征")
        if ablation_results:
            negative_groups = [r.group_name for r in ablation_results if r.negative_value_group]
            if negative_groups:
                lines.append(f"3. 排除负价值特征组: {', '.join(negative_groups)}")
    elif go_nogo.decision == "Marginal Go":
        lines.append("1. 等更多 V6 gate events 积累或 Path C 扩大 events 池")
        lines.append("2. 保留当前 Conditional Go 部署不变")
        lines.append("3. 考虑在 Path C 中验证最优 arm 的组合")
    else:  # No-Go
        if go_nogo.no_refinement_superior:
            lines.append("1. 直接推进 Path C unified spec，跳过 refinement")
            lines.append("2. 保持 pre-breakout-timing-classifier 的 Conditional Go 结论")
        else:
            lines.append("1. 正式关闭 pre-breakout timing refinement 方向")
            lines.append("2. 保持 pre-breakout-timing-classifier 的 Conditional Go 结论")
            lines.append("3. 资源转向 Path C（unified pretouch classifier）或其他研究方向")
    lines.append("")

    # --- refinement_noop 检查 ---
    if refinement_noop:
        lines.append("---")
        lines.append("")
        lines.append(
            f"⚠️ **refinement_noop=true**: 最优 arm 与 Baseline_Legacy 差异 < "
            f"{REFINEMENT_NOOP_THRESHOLD}%，refinement 未产生实质改善。"
        )
        lines.append("")

    # 写入文件
    report_path = output_dir / "pretouch_refinement_report.md"
    report_path.write_text("\n".join(lines), encoding="utf-8")
    logger.info(f"主报告已生成: {report_path}")


# ---------------------------------------------------------------------------
# 内部函数：Summary JSON 生成
# ---------------------------------------------------------------------------


def _generate_summary_json(
    arm_results: list[ArmResult],
    ablation_results: list[AblationResult],
    go_nogo: GoNoGoDecision,
    baseline_legacy_summary: dict,
    robustness_results: dict,
    refinement_noop: bool,
    accuracy_calendar_decoupled: bool,
    best_global_delay: str,
    output_dir: Path,
) -> None:
    """生成 pretouch_refinement_summary.json。"""
    best_arm = max(arm_results, key=lambda r: r.test_calendar_sum)
    baseline_legacy_cs = baseline_legacy_summary.get(
        "best_classifier", {}
    ).get("test_calendar_sum", BASELINE_LEGACY_CS)
    oracle_cs = baseline_legacy_summary.get("oracle_calendar_sum", 0.0)

    # 构建 arm 对比摘要
    arm_comparison_summary = []
    for ar in arm_results:
        clf_name = getattr(ar.best_classifier, "name", "Unknown")
        arm_comparison_summary.append({
            "arm_name": ar.config.name,
            "feature_set": ar.config.feature_set,
            "regime_schema": ar.config.regime_schema,
            "best_classifier": clf_name,
            "test_calendar_sum": ar.test_calendar_sum,
            "oracle_realization_rate": ar.oracle_realization_rate,
            "improvement_vs_legacy": ar.test_calendar_sum - baseline_legacy_cs,
        })

    # 构建消融摘要
    ablation_summary = []
    for ab in ablation_results:
        ablation_summary.append({
            "group_name": ab.group_name,
            "features_removed": ab.features_removed,
            "delta_test_cs": ab.delta_test_cs,
            "high_value_group": ab.high_value_group,
            "negative_value_group": ab.negative_value_group,
            "bootstrap_ci": [ab.bootstrap_ci_lower, ab.bootstrap_ci_upper],
        })

    # 构建 baseline_legacy 字段 (Req 8.1)
    baseline_legacy_field = {
        "test_calendar_sum": baseline_legacy_cs,
        "best_classifier_name": baseline_legacy_summary.get(
            "best_classifier", {}
        ).get("name", "LogisticRegression"),
        "bootstrap_ci_overall": baseline_legacy_summary.get(
            "bootstrap_ci", {}
        ).get("overall", {}),
        "symbol_results": baseline_legacy_summary.get("symbol_results", {}),
        "regime_stability": baseline_legacy_summary.get("regime_stability", {}),
    }

    summary = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "spec": "pretouch-classifier-refinement",
        "go_nogo_decision": go_nogo.decision,
        "best_arm": best_arm.config.name,
        "best_classifier": getattr(best_arm.best_classifier, "name", "Unknown"),
        "test_calendar_sum": best_arm.test_calendar_sum,
        "oracle_calendar_sum": oracle_cs,
        "oracle_realization_rate": best_arm.oracle_realization_rate,
        "improvement_vs_legacy": best_arm.test_calendar_sum - baseline_legacy_cs,
        "best_global_delay": best_global_delay,
        "baseline_legacy": baseline_legacy_field,
        "robustness": {
            "bootstrap_ci": robustness_results.get("bootstrap_ci", {}),
            "symbol_bootstrap": robustness_results.get("symbol_bootstrap", {}),
            "overfitting_check": robustness_results.get("overfitting_check", {}),
            "regime_stability": robustness_results.get("regime_stability", {}),
        },
        "flags": {
            "small_sample_warning": SMALL_SAMPLE_WARNING,
            "refinement_noop": refinement_noop,
            "accuracy_calendar_decoupled": accuracy_calendar_decoupled,
            "no_refinement_superior": go_nogo.no_refinement_superior,
            "overfitting_flag": go_nogo.overfitting_flag,
            "loocv_degradation_flag": go_nogo.loocv_degradation_flag,
        },
        "arm_comparison": arm_comparison_summary,
        "ablation": ablation_summary,
        "reasoning": go_nogo.reasoning,
    }

    json_path = output_dir / "pretouch_refinement_summary.json"
    with open(json_path, "w", encoding="utf-8") as f:
        json.dump(summary, f, ensure_ascii=False, indent=2)
    logger.info(f"Summary JSON 已生成: {json_path}")


# ---------------------------------------------------------------------------
# 内部函数：Feature Importance CSV
# ---------------------------------------------------------------------------


def _generate_feature_importance_csv(
    best_arm: ArmResult,
    output_dir: Path,
) -> None:
    """生成 feature_importance_best_arm.csv。

    从最优 arm 的 best_classifier 中提取 feature_importance 字典，
    按重要性降序排列输出。
    """
    feature_importance = getattr(best_arm.best_classifier, "feature_importance", {})

    if not feature_importance:
        # 若无 feature_importance（如 LR 无直接 importance），生成空文件
        df = pd.DataFrame(columns=["feature", "importance", "rank"])
    else:
        # 按重要性降序排列
        sorted_features = sorted(
            feature_importance.items(), key=lambda x: abs(x[1]), reverse=True
        )
        df = pd.DataFrame(sorted_features, columns=["feature", "importance"])
        df["rank"] = range(1, len(df) + 1)

    csv_path = output_dir / "feature_importance_best_arm.csv"
    df.to_csv(csv_path, index=False)
    logger.info(f"Feature importance CSV 已生成: {csv_path} ({len(df)} features)")


# ---------------------------------------------------------------------------
# 内部函数：Trades CSV
# ---------------------------------------------------------------------------


def _generate_trades_csv(
    best_arm: ArmResult,
    output_dir: Path,
) -> None:
    """生成 pretouch_refinement_trades.csv。

    从最优 arm 的 best_classifier 中提取 test set 预测结果，
    构建 trade ledger。
    """
    predictions_test = getattr(best_arm.best_classifier, "predictions_test", None)

    if predictions_test is None or len(predictions_test) == 0:
        # 无预测结果，生成空文件
        df = pd.DataFrame(columns=[
            "event_index", "predicted_label", "action",
        ])
    else:
        records = []
        for i, pred in enumerate(predictions_test):
            action = "skip" if pred == "skip" else "trade"
            records.append({
                "event_index": i,
                "predicted_label": pred,
                "action": action,
            })
        df = pd.DataFrame(records)

    csv_path = output_dir / "pretouch_refinement_trades.csv"
    df.to_csv(csv_path, index=False)
    logger.info(f"Trades CSV 已生成: {csv_path} ({len(df)} rows)")
