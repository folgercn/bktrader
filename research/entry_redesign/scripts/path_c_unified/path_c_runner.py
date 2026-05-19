"""path_c_runner — Path C Unified Pretouch 主入口。

串联完整 10 步流程：
1. 加载原 116 events（复用 data_layer）
2. 探索 gate 放宽策略
3. 加载扩展事件池
4. 加载 bars cache
5. 执行 multi-delay simulation + 一致性校验
6. 生成 3-regime 标签 + time-split
7. 训练 DT3 + LOOCV + 规则提取
8. 规则对比 + 深度对比
9. 稳健性验证
10. 历史结果对比 + 报告生成 + Go/No-Go

执行方式：
    cd research/entry_redesign/scripts
    python -m path_c_unified.path_c_runner
"""

from __future__ import annotations

import json
import logging
import sys
import time
from pathlib import Path

import pandas as pd

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parents[4]
SCRIPTS_DIR = PROJECT_ROOT / "research" / "entry_redesign" / "scripts"

# 确保 scripts 目录在 sys.path 中（支持 pre_breakout_timing / pretouch_refinement import）
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

OUTPUT_DIR = SCRIPTS_DIR / "output" / "path_c_unified"
ORIGINAL_MATRIX_PATH = SCRIPTS_DIR / "output" / "pre_breakout_timing" / "delay_pnl_matrix.csv"
BASELINE_SUMMARY_PATH = SCRIPTS_DIR / "output" / "pre_breakout_timing" / "pre_breakout_timing_summary.json"
REFINEMENT_SUMMARY_PATH = SCRIPTS_DIR / "output" / "pretouch_refinement" / "pretouch_refinement_summary.json"

# 确定性随机种子
RANDOM_STATE = 42

# ---------------------------------------------------------------------------
# Logging setup
# ---------------------------------------------------------------------------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("path_c_runner")


# ---------------------------------------------------------------------------
# Historical results loading
# ---------------------------------------------------------------------------
def _load_historical_results() -> dict:
    """从 summary.json 文件读取历史结果。

    若文件不存在，使用已知默认值（从 requirements 文档中确认）。
    """
    historical: dict = {
        "baseline_legacy": {
            "test_cs": 2.98,
            "ci_lower": 1.16,
            "ci_upper": 4.73,
        },
        "refinement_a2": {
            "test_cs": 3.39,
            "ci_lower": 1.60,
            "ci_upper": 5.18,
        },
    }

    # 尝试从 baseline summary 读取
    if BASELINE_SUMMARY_PATH.exists():
        try:
            with open(BASELINE_SUMMARY_PATH, "r", encoding="utf-8") as f:
                baseline_data = json.load(f)
            historical["baseline_legacy"] = {
                "test_cs": baseline_data.get("best_classifier", {}).get(
                    "test_calendar_sum", 2.98
                ),
                "ci_lower": baseline_data.get("bootstrap_ci", {})
                .get("overall", {})
                .get("ci_5", 1.16),
                "ci_upper": baseline_data.get("bootstrap_ci", {})
                .get("overall", {})
                .get("ci_95", 4.73),
            }
            logger.info("Baseline summary loaded from: %s", BASELINE_SUMMARY_PATH)
        except Exception as e:
            logger.warning("Failed to load baseline summary: %s", e)
    else:
        logger.warning(
            "Baseline summary not found: %s, using defaults", BASELINE_SUMMARY_PATH
        )

    # 尝试从 refinement summary 读取
    if REFINEMENT_SUMMARY_PATH.exists():
        try:
            with open(REFINEMENT_SUMMARY_PATH, "r", encoding="utf-8") as f:
                refinement_data = json.load(f)
            historical["refinement_a2"] = {
                "test_cs": refinement_data.get("test_calendar_sum", 3.39),
                "ci_lower": refinement_data.get("robustness", {})
                .get("bootstrap_ci", {})
                .get("ci_lower", 1.60),
                "ci_upper": refinement_data.get("robustness", {})
                .get("bootstrap_ci", {})
                .get("ci_upper", 5.18),
            }
            logger.info("Refinement summary loaded from: %s", REFINEMENT_SUMMARY_PATH)
        except Exception as e:
            logger.warning("Failed to load refinement summary: %s", e)
    else:
        logger.warning(
            "Refinement summary not found: %s, using defaults",
            REFINEMENT_SUMMARY_PATH,
        )

    return historical


def _load_original_a2_rule_text() -> str:
    """尝试从 pretouch_refinement_summary.json 或报告中提取原 A2 DT3 规则文本。

    若无法获取，返回占位文本。
    """
    # 原 A2 规则文本未存储在 summary.json 中
    # 使用占位文本并标注
    placeholder = (
        "（原 A2 DT3 规则文本未存储在 pretouch_refinement 输出中，"
        "无法进行精确规则对比。此处使用占位文本。"
        "若需精确对比，请重新运行 pretouch_refinement 实验并保存规则文本。）"
    )
    logger.warning("原 A2 DT3 规则文本不可用，使用占位文本进行规则对比")
    return placeholder


# ---------------------------------------------------------------------------
# Main pipeline
# ---------------------------------------------------------------------------
def main() -> None:
    """Path C Unified Pretouch 主流程。

    Steps:
    1. 加载原 116 events（复用 data_layer）
    2. 探索 gate 放宽策略
    3. 加载扩展事件池
    4. 加载 bars cache
    5. 执行 multi-delay simulation + 一致性校验
    6. 生成 3-regime 标签 + time-split
    7. 训练 DT3 + LOOCV + 规则提取
    8. 规则对比 + 深度对比
    9. 稳健性验证
    10. 历史结果对比 + 报告生成 + Go/No-Go
    """
    pipeline_start = time.time()
    logger.info("=" * 70)
    logger.info("Path C Unified Pretouch — 开始执行")
    logger.info("=" * 70)
    logger.info("输出目录: %s", OUTPUT_DIR)
    logger.info("随机种子: %d", RANDOM_STATE)

    # 确保输出目录存在
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    # =========================================================================
    # Step 1: 加载原 116 events
    # =========================================================================
    logger.info("")
    logger.info("[Step 1/10] 加载原 116 events...")
    t0 = time.time()

    from pre_breakout_timing.data_layer import load_v6_gate_events

    original_events = load_v6_gate_events()
    original_events = original_events.sort_values("touch_time").reset_index(drop=True)
    n_original = len(original_events)

    logger.info(
        "  原始事件池加载完成: %d events (%.1fs)", n_original, time.time() - t0
    )

    # =========================================================================
    # Step 2: 探索 gate 放宽策略
    # =========================================================================
    logger.info("")
    logger.info("[Step 2/10] 探索 gate 放宽策略...")
    t0 = time.time()

    from path_c_unified.gate_explorer import explore_gate_relaxation

    gate_report = explore_gate_relaxation(original_events)

    logger.info(
        "  Gate 探索完成: 选定 %d events, 达标=%s (%.1fs)",
        gate_report.final_n_events,
        gate_report.expansion_target_met,
        time.time() - t0,
    )

    # =========================================================================
    # Step 3: 加载扩展事件池
    # =========================================================================
    logger.info("")
    logger.info("[Step 3/10] 加载扩展事件池...")
    t0 = time.time()

    from path_c_unified.expanded_data_layer import load_expanded_events

    expanded_events = load_expanded_events(original_events)
    n_expanded = len(expanded_events)

    logger.info(
        "  扩展事件池加载完成: %d events (%.1fs)", n_expanded, time.time() - t0
    )

    # =========================================================================
    # Step 4: 加载 bars cache
    # =========================================================================
    logger.info("")
    logger.info("[Step 4/10] 加载 bars cache...")
    t0 = time.time()

    from path_c_unified.expanded_data_layer import load_expanded_bars_cache

    bars_cache = load_expanded_bars_cache(expanded_events)

    logger.info(
        "  Bars cache 加载完成: %d 个 (symbol, month) 组合 (%.1fs)",
        len(bars_cache),
        time.time() - t0,
    )

    # =========================================================================
    # Step 5: Multi-delay simulation + 一致性校验
    # =========================================================================
    logger.info("")
    logger.info("[Step 5/10] 执行 multi-delay simulation...")
    t0 = time.time()

    from path_c_unified.delay_simulation_runner import run_multi_delay_simulation

    delay_pnl_matrix, sim_stats, all_delay_results = run_multi_delay_simulation(
        events=expanded_events,
        bars_cache=bars_cache,
        pullback_params=None,  # 使用 DEFAULT_PULLBACK_PARAMS
        original_matrix_path=ORIGINAL_MATRIX_PATH,
    )

    # 保存 expanded_delay_pnl_matrix.csv
    matrix_output_path = OUTPUT_DIR / "expanded_delay_pnl_matrix.csv"
    delay_pnl_matrix.to_csv(matrix_output_path, index=False)
    logger.info("  expanded_delay_pnl_matrix.csv 已保存: %s", matrix_output_path)

    # 保存 skipped_events.csv（如果有跳过的 events）
    if sim_stats.skipped_events > 0:
        skipped_ids = []
        for i, event_results in enumerate(all_delay_results):
            if event_results and event_results[0].exit_reason == "NoData":
                skipped_ids.append(
                    {
                        "event_id": expanded_events.iloc[i]["event_id"],
                        "symbol": expanded_events.iloc[i]["symbol"],
                        "touch_time": expanded_events.iloc[i]["touch_time"],
                        "reason": "bars_cache_unavailable",
                    }
                )
        if skipped_ids:
            skipped_df = pd.DataFrame(skipped_ids)
            skipped_path = OUTPUT_DIR / "skipped_events.csv"
            skipped_df.to_csv(skipped_path, index=False)
            logger.info("  skipped_events.csv 已保存: %d events", len(skipped_ids))

    logger.info(
        "  Simulation 完成: total=%d, simulated=%d, skipped=%d (%.1fs)",
        sim_stats.total_events,
        sim_stats.simulated_events,
        sim_stats.skipped_events,
        time.time() - t0,
    )
    if sim_stats.drift_check:
        dc = sim_stats.drift_check
        logger.info(
            "  Drift check: n_compared=%d, max_diff=%.6f%%, warning=%s",
            dc.n_compared,
            dc.max_pnl_diff * 100,
            dc.drift_warning,
        )

    # =========================================================================
    # Step 6: 生成 3-regime 标签 + time-split
    # =========================================================================
    logger.info("")
    logger.info("[Step 6/10] 生成 3-regime 标签 + time-split...")
    t0 = time.time()

    from path_c_unified.label_generator import generate_labels_and_split

    train_events, test_events, train_labels, test_labels, label_stats = (
        generate_labels_and_split(
            delay_pnl_matrix=delay_pnl_matrix,
            events=expanded_events,
            original_events=original_events,
            tolerance_bps=5.0,
            train_ratio=0.6,
        )
    )

    logger.info(
        "  标签生成完成: train=%d, test=%d (%.1fs)",
        len(train_events),
        len(test_events),
        time.time() - t0,
    )
    logger.info(
        "  Train 分布: %s", label_stats.train_distribution
    )
    logger.info(
        "  Test 分布: %s", label_stats.test_distribution
    )
    logger.info(
        "  Label shift vs original: %s (orig_skip=%.1f%%, expanded_skip=%.1f%%)",
        label_stats.label_shift_vs_original,
        label_stats.original_skip_pct,
        label_stats.expanded_skip_pct,
    )

    # =========================================================================
    # Step 7: 训练 DT3 + LOOCV + 规则提取
    # =========================================================================
    logger.info("")
    logger.info("[Step 7/10] 训练 DT3 + LOOCV + 规则提取...")
    t0 = time.time()

    from path_c_unified.classifier_trainer import train_and_evaluate

    train_event_ids = set(train_events["event_id"].astype(str).tolist())
    test_event_ids = set(test_events["event_id"].astype(str).tolist())

    training_result = train_and_evaluate(
        train_events=train_events,
        test_events=test_events,
        train_labels=train_labels,
        all_delay_results=all_delay_results,
        train_event_ids=train_event_ids,
        test_event_ids=test_event_ids,
    )

    logger.info(
        "  DT3 训练完成: loocv_cs=+%.2f%%, train_cs=+%.2f%%, test_cs=+%.2f%% (%.1fs)",
        training_result.loocv_calendar_sum,
        training_result.train_calendar_sum,
        training_result.test_calendar_sum,
        time.time() - t0,
    )

    # =========================================================================
    # Step 8: 规则对比 + 深度对比
    # =========================================================================
    logger.info("")
    logger.info("[Step 8/10] 规则对比 + 深度对比...")
    t0 = time.time()

    from path_c_unified.classifier_trainer import compare_depths, compare_rules

    # 获取原 A2 规则文本
    original_a2_rule_text = _load_original_a2_rule_text()

    # 规则对比
    rule_comparison = compare_rules(
        expanded_rule_text=training_result.rule_text,
        original_a2_rule_text=original_a2_rule_text,
    )

    logger.info(
        "  Rule Stability Score: %.2f, instability_warning=%s",
        rule_comparison.stability_score,
        rule_comparison.instability_warning,
    )

    # 深度对比 — 需要准备 train features（排除 skip）
    from pre_breakout_timing.feature_extractor import extract_features, impute_features
    from path_c_unified.classifier_trainer import _extract_delay_results_by_ids

    train_features, used_features, _ = extract_features(train_events)
    # 使用 test_features 仅用于 impute（获取 imputer）
    test_features_raw = train_events[used_features].copy()  # placeholder
    train_features_imputed, _, _ = impute_features(train_features, test_features_raw)

    # 过滤 skip
    train_mask = train_labels != "skip"
    train_features_filtered = train_features_imputed.loc[train_mask].reset_index(
        drop=True
    )
    train_labels_filtered = train_labels.loc[train_mask].reset_index(drop=True)

    # 获取 train delay results（排除 skip）
    delay_results_train = _extract_delay_results_by_ids(
        all_delay_results, train_event_ids, train_events
    )
    delay_results_train_filtered = [
        delay_results_train[i] for i, keep in enumerate(train_mask) if keep
    ]

    depth_comparison = compare_depths(
        train_features=train_features_filtered,
        train_labels=train_labels_filtered,
        delay_results_train=delay_results_train_filtered,
        train_events=train_events.loc[train_mask].reset_index(drop=True),
    )

    logger.info(
        "  深度对比完成: best_depth=%d, dt3_still_optimal=%s (%.1fs)",
        depth_comparison.best_depth,
        depth_comparison.dt3_still_optimal,
        time.time() - t0,
    )

    # =========================================================================
    # Step 9: 稳健性验证
    # =========================================================================
    logger.info("")
    logger.info("[Step 9/10] 稳健性验证...")
    t0 = time.time()

    from path_c_unified.robustness import run_robustness_checks
    from path_c_unified.classifier_trainer import (
        _extract_delay_results_by_ids,
        _resolve_3regime_predictions,
    )
    import numpy as np

    # 获取 test set 的 resolved delay results（用于 bootstrap）
    delay_results_test = _extract_delay_results_by_ids(
        all_delay_results, test_event_ids, test_events
    )
    test_predictions = np.array(training_result.predictions_test)
    test_resolved_results = _resolve_3regime_predictions(
        test_predictions, delay_results_test
    )

    original_event_ids = set(original_events["event_id"].astype(str).tolist())

    robustness_report = run_robustness_checks(
        test_delay_results=test_resolved_results,
        test_events=test_events,
        train_calendar_sum=training_result.train_calendar_sum,
        loocv_calendar_sum=training_result.loocv_calendar_sum,
        test_calendar_sum=training_result.test_calendar_sum,
        train_labels=train_labels,
        test_labels=test_labels,
        original_event_ids=original_event_ids,
        n_bootstrap=1000,
        random_state=RANDOM_STATE,
    )

    logger.info(
        "  稳健性验证完成: CI=[+%.2f%%, +%.2f%%], width=%.2fpp (%.1fs)",
        robustness_report.overall_bootstrap.ci_lower,
        robustness_report.overall_bootstrap.ci_upper,
        robustness_report.overall_bootstrap.ci_width,
        time.time() - t0,
    )
    logger.info(
        "  Flags: overfitting=%s, loocv_degradation=%s, gate_degradation=%s, "
        "small_sample=%s",
        robustness_report.overfitting_flag,
        robustness_report.loocv_degradation_flag,
        robustness_report.gate_quality.degradation,
        robustness_report.small_sample_warning,
    )

    # =========================================================================
    # Step 10: 历史结果对比 + 报告生成 + Go/No-Go
    # =========================================================================
    logger.info("")
    logger.info("[Step 10/10] 历史结果对比 + 报告生成...")
    t0 = time.time()

    from path_c_unified.report_generator import generate_report

    historical_results = _load_historical_results()

    decision = generate_report(
        gate_report=gate_report,
        simulation_stats=sim_stats,
        label_stats=label_stats,
        training_result=training_result,
        rule_comparison=rule_comparison,
        depth_comparison=depth_comparison,
        robustness=robustness_report,
        historical_results=historical_results,
        output_dir=OUTPUT_DIR,
    )

    logger.info("  报告生成完成 (%.1fs)", time.time() - t0)

    # =========================================================================
    # Summary
    # =========================================================================
    total_elapsed = time.time() - pipeline_start
    logger.info("")
    logger.info("=" * 70)
    logger.info("Path C Unified Pretouch — 执行完成")
    logger.info("=" * 70)
    logger.info("  Go/No-Go 决策: %s", decision.decision)
    logger.info("  Test calendar_sum: +%.2f%%", decision.test_calendar_sum)
    logger.info("  Bootstrap CI lower: +%.2f%%", decision.ci_lower)
    logger.info("  事件池大小: %d events", gate_report.final_n_events)
    logger.info("  总耗时: %.1fs", total_elapsed)
    logger.info("  输出目录: %s", OUTPUT_DIR)
    logger.info("=" * 70)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    main()
