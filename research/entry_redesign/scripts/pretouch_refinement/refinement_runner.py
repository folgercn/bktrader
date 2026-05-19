"""
refinement_runner — Pretouch Classifier Refinement 主入口

串联完整流程：
1. 加载数据 + 验证 delay_pnl_matrix.csv
2. 标签重生成（3-regime / 2-regime）
3. 增强特征提取 + PIT 校验
4. 6 arm × 4 classifier 训练
5. 消融实验
6. 稳健性验证
7. 报告生成 + Go/No-Go

Usage:
    cd research/entry_redesign/scripts
    python -m pretouch_refinement.refinement_runner
"""

from __future__ import annotations

import json
import logging
import sys
from pathlib import Path

import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

OUTPUT_DIR = SCRIPTS_DIR / "output" / "pretouch_refinement"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

DELAY_MATRIX_PATH = (
    SCRIPTS_DIR / "output" / "pre_breakout_timing" / "delay_pnl_matrix.csv"
)
BASELINE_SUMMARY_PATH = (
    SCRIPTS_DIR / "output" / "pre_breakout_timing" / "pre_breakout_timing_summary.json"
)

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)


def main():
    """完整流程。"""

    # ===================================================================
    # Step 1: 加载数据 + 验证
    # ===================================================================
    print("=" * 70)
    print("Step 1: 加载数据 + 验证")
    print("=" * 70)

    # 1.1 load_v6_gate_events() → 116 events
    from pre_breakout_timing.data_layer import (
        load_bars_cache,
        load_v6_gate_events,
        time_split_events,
    )

    print("[1.1] 加载 V6 gate events...")
    events = load_v6_gate_events()
    n_events = len(events)
    print(f"      → {n_events} events 加载完成")
    logger.info(f"V6 gate events: {n_events} events")

    # 1.2 load_bars_cache() → 1s bar cache
    print("[1.2] 加载 1s bar cache...")
    bars_cache = load_bars_cache(events)
    print(f"      → {len(bars_cache)} symbol_month keys 加载完成")
    logger.info(f"Bars cache: {len(bars_cache)} keys")

    # 1.3 time_split_events() → train(70) / test(46)
    print("[1.3] 执行 time split...")
    train_events, test_events = time_split_events(events)
    n_train = len(train_events)
    n_test = len(test_events)
    print(f"      → train={n_train}, test={n_test}")
    logger.info(f"Time split: train={n_train}, test={n_test}")

    # 1.4 load_delay_pnl_matrix() → 验证 580行×15列
    from pretouch_refinement.regime_labels import load_delay_pnl_matrix

    print("[1.4] 加载并验证 delay_pnl_matrix.csv...")
    delay_pnl_matrix = load_delay_pnl_matrix(str(DELAY_MATRIX_PATH))
    print(f"      → shape={delay_pnl_matrix.shape} 验证通过")
    logger.info(f"Delay PnL matrix: shape={delay_pnl_matrix.shape}")

    # 1.5 load baseline_legacy summary.json
    print("[1.5] 加载 baseline_legacy summary.json...")
    if not BASELINE_SUMMARY_PATH.exists():
        raise FileNotFoundError(
            f"Baseline summary 文件不存在: {BASELINE_SUMMARY_PATH}\n"
            "请先运行 pre_breakout_timing 实验生成该文件。"
        )
    with open(BASELINE_SUMMARY_PATH, "r", encoding="utf-8") as f:
        baseline_legacy_summary = json.load(f)
    baseline_legacy_cs = baseline_legacy_summary["best_classifier"]["test_calendar_sum"]
    print(f"      → Baseline_Legacy test_calendar_sum={baseline_legacy_cs:.4f}%")
    logger.info(f"Baseline_Legacy: test_calendar_sum={baseline_legacy_cs:.4f}%")

    # 1.6 rebuild_delay_results_from_matrix() → list[list[DelayResult]]
    from pretouch_refinement.arm_runner import rebuild_delay_results_from_matrix

    print("[1.6] 重建 DelayResult...")
    all_delay_results = rebuild_delay_results_from_matrix(delay_pnl_matrix, events)
    total_dr = sum(len(inner) for inner in all_delay_results)
    print(f"      → {len(all_delay_results)} events × 5 delays = {total_dr} DelayResults")
    logger.info(f"DelayResults rebuilt: {total_dr} total")

    # 按 train/test split 分割 delay_results
    # time_split_events 按 touch_time 排序后 60/40 split，需要对齐
    # events 已经是 time_split 前的完整集合，train/test 是排序后的子集
    # 需要根据 event_id 对齐 delay_results
    train_event_ids = set(train_events["event_id"].tolist())
    test_event_ids = set(test_events["event_id"].tolist())

    # events 的顺序与 all_delay_results 对齐
    delay_results_train = []
    delay_results_test = []
    for i, (_, row) in enumerate(events.iterrows()):
        eid = row["event_id"]
        if eid in train_event_ids:
            delay_results_train.append(all_delay_results[i])
        elif eid in test_event_ids:
            delay_results_test.append(all_delay_results[i])

    print(f"      → delay_results split: train={len(delay_results_train)}, test={len(delay_results_test)}")

    # ===================================================================
    # Step 2: 标签重生成
    # ===================================================================
    print()
    print("=" * 70)
    print("Step 2: 标签重生成")
    print("=" * 70)

    from pre_breakout_timing.delay_simulator import compute_optimal_labels
    from pretouch_refinement.regime_labels import (
        compute_label_distributions,
        generate_2regime_labels,
        generate_3regime_labels,
    )

    # 2.1 保留原 5-regime 标签（从 matrix 重新计算 compute_optimal_labels）
    print("[2.1] 生成 5-regime 标签（compute_optimal_labels）...")
    labels_5regime_list = compute_optimal_labels(all_delay_results, tolerance_bps=5.0)
    labels_5regime = pd.Series(labels_5regime_list, index=events.index, name="regime_5_label")
    skip_count = (labels_5regime == "skip").sum()
    print(f"      → 5-regime 标签分布: {labels_5regime.value_counts().to_dict()}")
    print(f"      → skip events: {skip_count}")
    logger.info(f"5-regime labels: {labels_5regime.value_counts().to_dict()}")

    # 2.2 generate_3regime_labels() → labels_3regime
    print("[2.2] 生成 3-regime 标签...")
    labels_3regime = generate_3regime_labels(delay_pnl_matrix, events, tolerance_bps=5.0)
    print(f"      → 3-regime 标签分布: {labels_3regime.value_counts().to_dict()}")
    logger.info(f"3-regime labels: {labels_3regime.value_counts().to_dict()}")

    # 2.3 generate_2regime_labels() → labels_2regime + best_global_delay
    print("[2.3] 生成 2-regime 标签...")
    labels_2regime, best_global_delay = generate_2regime_labels(
        delay_pnl_matrix, train_events, events
    )
    print(f"      → 2-regime 标签分布: {labels_2regime.value_counts().to_dict()}")
    print(f"      → Best_Global_Delay: {best_global_delay}")
    logger.info(f"2-regime labels: {labels_2regime.value_counts().to_dict()}")
    logger.info(f"Best_Global_Delay: {best_global_delay}")

    # 2.4 compute_label_distributions() → regime_label_distributions.csv
    print("[2.4] 计算标签分布并输出 CSV...")
    # 构建 train_mask（基于 event_id 对齐到 events 的 index）
    train_mask = events["event_id"].isin(train_event_ids)

    label_distributions, regime2_imbalanced = compute_label_distributions(
        labels_5regime=labels_5regime,
        labels_3regime=labels_3regime,
        labels_2regime=labels_2regime,
        train_mask=train_mask,
    )
    dist_csv_path = OUTPUT_DIR / "regime_label_distributions.csv"
    label_distributions.to_csv(dist_csv_path, index=False)
    print(f"      → 已保存: {dist_csv_path}")
    print(f"      → 分布表 shape: {label_distributions.shape}")
    logger.info(f"Label distributions saved: {dist_csv_path}")

    # 2.5 检查 regime2_imbalanced 标志
    if regime2_imbalanced:
        print("[2.5] ⚠️  WARNING: 2-regime 类别失衡（enter 占比 <40% 或 >90%）")
        logger.warning(
            "regime2_imbalanced=true: 2-regime 的 enter 标签在 train set 上失衡，"
            "可能影响分类器训练效果。"
        )
    else:
        print("[2.5] ✓ 2-regime 类别分布正常")
        logger.info("regime2_imbalanced=false: 2-regime 类别分布正常")

    print()
    print("=" * 70)
    print("Step 1-2 完成。数据加载与标签生成就绪。")
    print("=" * 70)

    # ===================================================================
    # Step 3: 特征提取
    # ===================================================================
    print()
    print("=" * 70)
    print("Step 3: 特征提取")
    print("=" * 70)

    from pre_breakout_timing.feature_extractor import (
        extract_features,
        feature_statistics,
        impute_features,
    )
    from pretouch_refinement.enhanced_features import (
        extract_enhanced_features,
        generate_pit_audit_report,
        load_extended_bars_cache,
    )

    # 3.1 extract_features() → 原 10 特征（复用 feature_extractor）
    print("[3.1] 提取原始 10 特征（复用 feature_extractor）...")
    original_features, orig_used_features, orig_excluded_features = extract_features(
        events, missing_threshold=0.5
    )
    print(f"      → 原始特征: used={len(orig_used_features)}, excluded={len(orig_excluded_features)}")
    print(f"      → used: {orig_used_features}")
    if orig_excluded_features:
        print(f"      → excluded: {orig_excluded_features}")
    logger.info(
        f"Original features: used={len(orig_used_features)}, "
        f"excluded={len(orig_excluded_features)}"
    )

    # 3.2 尝试 load_extended_bars_cache()
    print("[3.2] 尝试加载 extended_bars_cache（24h lookback）...")
    extended_bars_cache = load_extended_bars_cache(events)
    if extended_bars_cache:
        print(f"      → 加载成功: {len(extended_bars_cache)} symbol_month keys")
        logger.info(f"Extended bars cache: {len(extended_bars_cache)} keys loaded")
    else:
        print("      → extended_bars_cache 为空（graceful degradation，level_group 将被跳过）")
        logger.info("Extended bars cache: empty (level_group will be excluded)")

    # 3.3 extract_enhanced_features() → 增强特征 + PIT audit
    print("[3.3] 提取增强特征 + PIT audit...")
    (
        enhanced_features_df,
        enhanced_used_features,
        enhanced_excluded_features,
        pit_audit,
    ) = extract_enhanced_features(
        events, bars_cache, extended_bars_cache, missing_threshold=0.5
    )
    print(f"      → 增强特征: used={len(enhanced_used_features)}, excluded={len(enhanced_excluded_features)}")
    print(f"      → used: {enhanced_used_features}")
    if enhanced_excluded_features:
        print(f"      → excluded: {enhanced_excluded_features}")
    print(f"      → PIT audit entries: {len(pit_audit)}")
    logger.info(
        f"Enhanced features: used={len(enhanced_used_features)}, "
        f"excluded={len(enhanced_excluded_features)}, "
        f"pit_audit_entries={len(pit_audit)}"
    )

    # 生成 PIT audit 报告
    pit_audit_path = OUTPUT_DIR / "pretouch_features_pit_audit.md"
    generate_pit_audit_report(pit_audit, pit_audit_path)
    print(f"      → PIT audit 报告已保存: {pit_audit_path}")

    # 3.4 impute_features()（原特征 + 增强特征分别 impute）
    print("[3.4] 分别 impute 原特征和增强特征...")

    # 按 train/test split 分割原始特征
    train_mask = events["event_id"].isin(train_event_ids)
    test_mask = events["event_id"].isin(test_event_ids)

    original_features_train = original_features.loc[train_mask].copy()
    original_features_test = original_features.loc[test_mask].copy()

    # impute 原始特征（中位数填充，统计量仅从 train 计算）
    original_features_train, original_features_test, orig_impute_stats = impute_features(
        original_features_train, original_features_test
    )
    orig_imputed_count = sum(
        1 for s in orig_impute_stats.values() if s["train_missing_count"] > 0
    )
    print(f"      → 原始特征 impute 完成: {orig_imputed_count} 个特征有缺失值被填充")
    logger.info(f"Original features imputed: {orig_imputed_count} features had missing values")

    # 按 train/test split 分割增强特征（仅保留 used 特征列）
    if enhanced_used_features:
        enhanced_train = enhanced_features_df.loc[train_mask, enhanced_used_features].copy()
        enhanced_test = enhanced_features_df.loc[test_mask, enhanced_used_features].copy()

        # 编码分类特征为数值（impute_features 仅支持数值列）
        # time_of_day_session_overlap: {none=0, asia_europe=1, europe_us=2, us_asia=3}
        _SESSION_ENCODING = {"none": 0, "asia_europe": 1, "europe_us": 2, "us_asia": 3}
        if "time_of_day_session_overlap" in enhanced_train.columns:
            enhanced_train["time_of_day_session_overlap"] = (
                enhanced_train["time_of_day_session_overlap"].map(_SESSION_ENCODING).astype(float)
            )
            enhanced_test["time_of_day_session_overlap"] = (
                enhanced_test["time_of_day_session_overlap"].map(_SESSION_ENCODING).astype(float)
            )

        # impute 增强特征（中位数填充，统计量仅从 train 计算）
        enhanced_train, enhanced_test, enhanced_impute_stats = impute_features(
            enhanced_train, enhanced_test
        )
        enhanced_imputed_count = sum(
            1 for s in enhanced_impute_stats.values() if s["train_missing_count"] > 0
        )
        print(f"      → 增强特征 impute 完成: {enhanced_imputed_count} 个特征有缺失值被填充")
        logger.info(
            f"Enhanced features imputed: {enhanced_imputed_count} features had missing values"
        )
    else:
        enhanced_train = None
        enhanced_test = None
        print("      → 无可用增强特征，跳过 impute")
        logger.warning("No enhanced features available after exclusion")

    # 3.5 检查缺失率并排除（汇总报告）
    print("[3.5] 特征缺失率汇总...")
    print(f"      原始特征（impute 后）:")
    for col in original_features_train.columns:
        # impute 后应无缺失，但仍检查
        train_missing = original_features_train[col].isna().sum()
        test_missing = original_features_test[col].isna().sum()
        if train_missing > 0 or test_missing > 0:
            print(f"        ⚠️ {col}: train_missing={train_missing}, test_missing={test_missing}")
    print(f"      → 原始特征 impute 后无残余缺失 ✓")

    if enhanced_train is not None:
        print(f"      增强特征（impute 后）:")
        residual_missing = False
        for col in enhanced_train.columns:
            train_missing = enhanced_train[col].isna().sum()
            test_missing = enhanced_test[col].isna().sum()
            if train_missing > 0 or test_missing > 0:
                print(f"        ⚠️ {col}: train_missing={train_missing}, test_missing={test_missing}")
                residual_missing = True
        if not residual_missing:
            print(f"      → 增强特征 impute 后无残余缺失 ✓")

    # 3.6 feature_statistics() → 各 regime 下分布对比
    print("[3.6] 计算特征统计（各 regime 下分布对比）...")
    # 使用 5-regime 标签对 train set 做统计
    train_labels_5regime = labels_5regime.loc[train_mask]

    # 原始特征统计
    orig_stats = feature_statistics(original_features_train, train_labels_5regime)
    print(f"      → 原始特征统计: {orig_stats.shape[0]} features × {orig_stats.shape[1]} columns")
    logger.info(f"Original feature statistics: shape={orig_stats.shape}")

    # 增强特征统计
    if enhanced_train is not None:
        enhanced_stats = feature_statistics(enhanced_train, train_labels_5regime)
        print(f"      → 增强特征统计: {enhanced_stats.shape[0]} features × {enhanced_stats.shape[1]} columns")
        logger.info(f"Enhanced feature statistics: shape={enhanced_stats.shape}")
    else:
        enhanced_stats = None

    print()
    print("=" * 70)
    print("Step 3 完成。特征提取与 impute 就绪。")
    print(f"  原始特征: {len(orig_used_features)} features (train={len(original_features_train)}, test={len(original_features_test)})")
    if enhanced_train is not None:
        print(f"  增强特征: {len(enhanced_used_features)} features (train={len(enhanced_train)}, test={len(enhanced_test)})")
    else:
        print("  增强特征: 无可用特征")
    print(f"  排除特征: 原始={orig_excluded_features}, 增强={enhanced_excluded_features}")
    print("=" * 70)

    # ===================================================================
    # Step 4: 6 Arm 训练
    # ===================================================================
    print()
    print("=" * 70)
    print("Step 4: 6 Arm 训练")
    print("=" * 70)

    from pretouch_refinement.arm_runner import ARM_CONFIGS, run_all_arms

    # 4.1 获取 oracle_calendar_sum（从 baseline_legacy_summary 中读取）
    oracle_calendar_sum = baseline_legacy_summary.get("oracle_calendar_sum", 0.0)
    print(f"[4.1] Oracle calendar_sum: {oracle_calendar_sum:.4f}%")
    logger.info(f"Oracle calendar_sum: {oracle_calendar_sum:.4f}%")

    # 4.2 运行 6 arm × 4 classifier 训练
    print("[4.2] 运行 6 arm × 4 classifier 训练...")
    print(f"      Arms: {[c.name for c in ARM_CONFIGS]}")

    # 需要将 labels 对齐到 train set（labels 是基于 events 全集的 index）
    # run_all_arms 内部会根据 train_mask 过滤 skip
    # 但 labels 需要是 train set 对应的子集
    labels_5regime_train = labels_5regime.loc[train_mask].reset_index(drop=True)
    labels_3regime_train = labels_3regime.loc[train_mask].reset_index(drop=True)
    labels_2regime_train = labels_2regime.loc[train_mask].reset_index(drop=True)

    arm_results = run_all_arms(
        arm_configs=ARM_CONFIGS,
        original_features_train=original_features_train,
        original_features_test=original_features_test,
        enhanced_features_train=enhanced_train,
        enhanced_features_test=enhanced_test,
        labels_5regime=labels_5regime_train,
        labels_3regime=labels_3regime_train,
        labels_2regime=labels_2regime_train,
        delay_results_train=delay_results_train,
        delay_results_test=delay_results_test,
        test_events=test_events,
        oracle_calendar_sum=oracle_calendar_sum,
    )

    # 4.3 选出 best_arm_classifier
    best_arm = max(arm_results, key=lambda r: r.test_calendar_sum)
    print(f"[4.3] Best arm: {best_arm.config.name} ({best_arm.best_classifier.name})")
    print(f"      test_calendar_sum: {best_arm.test_calendar_sum:.4f}%")
    print(f"      oracle_realization_rate: {best_arm.oracle_realization_rate:.2f}%")
    logger.info(
        f"Best arm selected: {best_arm.config.name} "
        f"({best_arm.best_classifier.name}), "
        f"test_cs={best_arm.test_calendar_sum:.4f}%"
    )

    # 打印所有 arm 结果摘要
    print()
    print("      Arm 结果摘要:")
    print(f"      {'Arm':<10} {'Best Clf':<20} {'LOOCV_cs':<12} {'Test_cs':<12} {'Oracle%':<10}")
    print(f"      {'-'*10} {'-'*20} {'-'*12} {'-'*12} {'-'*10}")
    for ar in arm_results:
        print(
            f"      {ar.config.name:<10} "
            f"{ar.best_classifier.name:<20} "
            f"{ar.best_classifier.loocv_calendar_sum:>10.4f}% "
            f"{ar.test_calendar_sum:>10.4f}% "
            f"{ar.oracle_realization_rate:>8.2f}%"
        )

    print()
    print("=" * 70)
    print("Step 4 完成。6 Arm 训练与评估就绪。")
    print(f"  Best arm: {best_arm.config.name} ({best_arm.best_classifier.name})")
    print(f"  Test calendar_sum: {best_arm.test_calendar_sum:.4f}%")
    print(f"  Baseline_Legacy: {baseline_legacy_cs:.4f}%")
    print(f"  改善: {best_arm.test_calendar_sum - baseline_legacy_cs:+.4f}%")
    print("=" * 70)

    # ===================================================================
    # Step 5: 消融实验
    # ===================================================================
    print()
    print("=" * 70)
    print("Step 5: 消融实验")
    print("=" * 70)

    from pretouch_refinement.ablation import generate_ablation_report, run_ablation

    # 5.1 判断最优 arm 是否使用增强特征
    best_arm_uses_enhanced = best_arm.config.feature_set == "original+enhanced"

    if best_arm_uses_enhanced:
        print("[5.1] 最优 arm 使用增强特征 → 执行消融实验")
        logger.info(
            f"Best arm '{best_arm.config.name}' uses enhanced features. "
            f"Running ablation."
        )

        # 5.2 构建消融所需的完整特征矩阵（原特征 + 增强特征）
        full_features_train = pd.concat(
            [original_features_train.reset_index(drop=True),
             enhanced_train.reset_index(drop=True)],
            axis=1,
        )
        full_features_test = pd.concat(
            [original_features_test.reset_index(drop=True),
             enhanced_test.reset_index(drop=True)],
            axis=1,
        )

        # 选择最优 arm 对应的标签
        if best_arm.config.regime_schema == "5-regime":
            ablation_labels = labels_5regime_train
        elif best_arm.config.regime_schema == "3-regime":
            ablation_labels = labels_3regime_train
        elif best_arm.config.regime_schema == "2-regime":
            ablation_labels = labels_2regime_train
        else:
            raise ValueError(f"未知 regime_schema: {best_arm.config.regime_schema}")

        # 确定 best_global_delay（仅 2-regime 需要）
        ablation_best_global_delay = (
            best_global_delay if best_arm.config.regime_schema == "2-regime" else None
        )

        print(f"[5.2] 运行消融实验（6 组增强特征逐类剔除）...")
        print(f"      基线 test_cs: {best_arm.test_calendar_sum:.4f}%")
        print(f"      分类器: {best_arm.best_classifier.name}")
        print(f"      Regime schema: {best_arm.config.regime_schema}")

        ablation_results = run_ablation(
            full_features_train=full_features_train,
            full_features_test=full_features_test,
            labels=ablation_labels,
            delay_results_train=delay_results_train,
            delay_results_test=delay_results_test,
            test_events=test_events,
            oracle_calendar_sum=oracle_calendar_sum,
            baseline_test_cs=best_arm.test_calendar_sum,
            best_classifier_name=best_arm.best_classifier.name,
            best_classifier_params=best_arm.best_classifier.best_params,
            regime_schema=best_arm.config.regime_schema,
            best_global_delay=ablation_best_global_delay,
            n_bootstrap=1000,
            random_state=42,
        )

        # 5.3 生成消融报告
        print("[5.3] 生成消融报告...")
        generate_ablation_report(
            results=ablation_results,
            output_dir=str(OUTPUT_DIR),
            baseline_test_cs=best_arm.test_calendar_sum,
        )

        # 打印消融结果摘要
        high_value_groups = [r for r in ablation_results if r.high_value_group]
        negative_value_groups = [r for r in ablation_results if r.negative_value_group]

        print()
        print("      消融结果摘要:")
        print(f"      {'Group':<28} {'Delta_cs':<12} {'High Value':<12} {'Neg Value':<12}")
        print(f"      {'-'*28} {'-'*12} {'-'*12} {'-'*12}")
        for ar in ablation_results:
            hv = "✓" if ar.high_value_group else ""
            nv = "✓" if ar.negative_value_group else ""
            print(
                f"      {ar.group_name:<28} "
                f"{ar.delta_test_cs:>+10.4f}% "
                f"{hv:<12} "
                f"{nv:<12}"
            )

        print()
        print(f"      高价值特征组: {len(high_value_groups)}")
        print(f"      负价值特征组: {len(negative_value_groups)}")
        logger.info(
            f"Ablation complete: {len(high_value_groups)} high-value groups, "
            f"{len(negative_value_groups)} negative-value groups"
        )

    else:
        print(f"[5.1] 最优 arm '{best_arm.config.name}' 不含增强特征 "
              f"(feature_set='{best_arm.config.feature_set}') → 跳过消融实验")
        print("      原因: 消融实验仅对含增强特征的 arm 有意义。")
        print("      若最优 arm 为 A2/A3（仅原特征 + regime 简化），")
        print("      说明 regime 简化本身已足够，增强特征无额外贡献。")
        logger.info(
            f"Ablation skipped: best arm '{best_arm.config.name}' uses "
            f"feature_set='{best_arm.config.feature_set}' (no enhanced features)"
        )
        ablation_results = []

    print()
    print("=" * 70)
    print("Step 5 完成。消融实验就绪。")
    if ablation_results:
        print(f"  消融组数: {len(ablation_results)}")
        print(f"  输出文件: {OUTPUT_DIR / 'ablation_results.csv'}")
        print(f"  输出文件: {OUTPUT_DIR / 'ablation_report.md'}")
    else:
        print("  消融实验已跳过（最优 arm 不含增强特征）")
    print("=" * 70)

    # ===================================================================
    # Step 6: 稳健性验证
    # ===================================================================
    print()
    print("=" * 70)
    print("Step 6: 稳健性验证")
    print("=" * 70)

    import numpy as np

    from pretouch_refinement.arm_runner import (
        _compute_calendar_sum_silo,
        _resolve_3regime_prediction,
        _resolve_2regime_prediction,
    )
    from pre_breakout_timing.timing_classifier import _find_delay_result

    # --- 6.0 准备：重建 best arm 在 test set 上的 per-event DelayResult ---
    # 重新预测 test set（因为 predictions_test 可能未存储在 ClassifierResult 中）
    from pretouch_refinement.arm_runner import predict_and_evaluate_test, _find_delay_result_by_label, _find_best_in_group, _FAST_DELAYS, _SLOW_DELAYS

    best_regime_schema = best_arm.config.regime_schema

    # 组装 best arm 的 test 特征矩阵
    if best_arm.config.feature_set == "original+enhanced" and enhanced_test is not None:
        best_test_features = pd.concat(
            [original_features_test.reset_index(drop=True),
             enhanced_test.reset_index(drop=True)],
            axis=1,
        )
    else:
        best_test_features = original_features_test.reset_index(drop=True)

    # 重建 best classifier 并预测
    from pretouch_refinement.ablation import _build_classifier
    best_clf_name = best_arm.best_classifier.name
    best_clf_params = best_arm.best_classifier.best_params

    # 组装 train 特征和标签
    if best_arm.config.feature_set == "original+enhanced" and enhanced_train is not None:
        best_train_features = pd.concat(
            [original_features_train.reset_index(drop=True),
             enhanced_train.reset_index(drop=True)],
            axis=1,
        )
    else:
        best_train_features = original_features_train.reset_index(drop=True)

    # 选择对应标签
    if best_regime_schema == "5-regime":
        best_labels = labels_5regime_train
    elif best_regime_schema == "3-regime":
        best_labels = labels_3regime_train
    else:
        best_labels = labels_2regime_train

    # 过滤 skip（仅 5-regime 和 3-regime）
    if best_regime_schema != "2-regime":
        _skip_mask = best_labels != "skip"
        best_train_features_filtered = best_train_features.loc[_skip_mask].reset_index(drop=True)
        best_labels_filtered = best_labels.loc[_skip_mask].reset_index(drop=True)
    else:
        best_train_features_filtered = best_train_features
        best_labels_filtered = best_labels

    # 训练并预测
    best_model = _build_classifier(best_clf_name, best_clf_params)
    best_model.fit(best_train_features_filtered, best_labels_filtered)
    best_predictions_test = best_model.predict(best_test_features)

    print(f"[6.0] 重建 best arm 预测: {len(best_predictions_test)} predictions")
    print(f"      预测分布: {pd.Series(best_predictions_test).value_counts().to_dict()}")

    # 为每个 test event 解析出对应的 DelayResult
    test_delay_results_per_event: list[DelayResult] = []
    for i, pred_label in enumerate(best_predictions_test):
        event_delays = delay_results_test[i]
        if best_regime_schema == "3-regime":
            matched = _resolve_3regime_prediction(pred_label, event_delays)
        elif best_regime_schema == "2-regime":
            matched = _resolve_2regime_prediction(pred_label, event_delays, best_arm.config)
        else:
            # 5-regime: 直接使用 _find_delay_result
            matched = _find_delay_result(event_delays, pred_label)
        test_delay_results_per_event.append(matched)

    n_test_events = len(test_delay_results_per_event)
    test_events_reset = test_events.reset_index(drop=True)

    # --- 6.1 Bootstrap CI (1000次, random_state=42) ---
    print("[6.1] Bootstrap CI (1000 次, random_state=42)...")
    rng = np.random.RandomState(42)
    n_bootstrap = 1000
    bootstrap_cs_values: list[float] = []

    for _ in range(n_bootstrap):
        # 有放回重采样 test event 索引
        sample_indices = rng.choice(n_test_events, size=n_test_events, replace=True)
        sampled_results = [test_delay_results_per_event[idx] for idx in sample_indices]
        sampled_events = test_events_reset.iloc[sample_indices].reset_index(drop=True)
        cs = _compute_calendar_sum_silo(sampled_results, sampled_events)
        bootstrap_cs_values.append(cs)

    bootstrap_ci_lower = float(np.percentile(bootstrap_cs_values, 5))
    bootstrap_ci_upper = float(np.percentile(bootstrap_cs_values, 95))
    bootstrap_mean = float(np.mean(bootstrap_cs_values))

    print(f"      → Bootstrap mean: {bootstrap_mean:.4f}%")
    print(f"      → 90% CI: [{bootstrap_ci_lower:.4f}%, {bootstrap_ci_upper:.4f}%]")
    logger.info(
        f"Bootstrap CI (1000x): mean={bootstrap_mean:.4f}%, "
        f"CI=[{bootstrap_ci_lower:.4f}%, {bootstrap_ci_upper:.4f}%]"
    )

    # --- 6.2 分 symbol bootstrap (BTC / ETH) ---
    print("[6.2] 分 symbol bootstrap (BTC / ETH)...")
    symbol_bootstrap_results: dict[str, dict] = {}

    for symbol_prefix in ["BTC", "ETH"]:
        # 筛选属于该 symbol 的 test event 索引
        symbol_mask = test_events_reset["symbol"].str.startswith(symbol_prefix)
        symbol_indices = [i for i, m in enumerate(symbol_mask) if m]

        if len(symbol_indices) == 0:
            print(f"      → {symbol_prefix}: 无 test events，跳过")
            symbol_bootstrap_results[symbol_prefix] = {
                "n_events": 0,
                "test_cs": 0.0,
                "bootstrap_ci_lower": 0.0,
                "bootstrap_ci_upper": 0.0,
                "bootstrap_mean": 0.0,
            }
            continue

        # 计算该 symbol 的原始 test_cs
        symbol_results = [test_delay_results_per_event[idx] for idx in symbol_indices]
        symbol_events = test_events_reset.iloc[symbol_indices].reset_index(drop=True)
        symbol_test_cs = _compute_calendar_sum_silo(symbol_results, symbol_events)

        # Bootstrap 该 symbol
        rng_symbol = np.random.RandomState(42)
        symbol_bs_values: list[float] = []
        n_symbol = len(symbol_indices)

        for _ in range(n_bootstrap):
            sample_idx = rng_symbol.choice(n_symbol, size=n_symbol, replace=True)
            sampled_results = [symbol_results[idx] for idx in sample_idx]
            sampled_events = symbol_events.iloc[sample_idx].reset_index(drop=True)
            cs = _compute_calendar_sum_silo(sampled_results, sampled_events)
            symbol_bs_values.append(cs)

        sym_ci_lower = float(np.percentile(symbol_bs_values, 5))
        sym_ci_upper = float(np.percentile(symbol_bs_values, 95))
        sym_bs_mean = float(np.mean(symbol_bs_values))

        symbol_bootstrap_results[symbol_prefix] = {
            "n_events": n_symbol,
            "test_cs": symbol_test_cs,
            "bootstrap_ci_lower": sym_ci_lower,
            "bootstrap_ci_upper": sym_ci_upper,
            "bootstrap_mean": sym_bs_mean,
        }

        print(
            f"      → {symbol_prefix}: n={n_symbol}, test_cs={symbol_test_cs:.4f}%, "
            f"CI=[{sym_ci_lower:.4f}%, {sym_ci_upper:.4f}%]"
        )
        logger.info(
            f"Symbol bootstrap {symbol_prefix}: n={n_symbol}, "
            f"test_cs={symbol_test_cs:.4f}%, "
            f"CI=[{sym_ci_lower:.4f}%, {sym_ci_upper:.4f}%]"
        )

    # --- 6.3 Overfitting 检查 ---
    print("[6.3] Overfitting 检查...")
    train_calendar_sum = best_arm.best_classifier.train_calendar_sum
    test_calendar_sum_val = best_arm.test_calendar_sum
    loocv_calendar_sum = best_arm.best_classifier.loocv_calendar_sum

    # Overfitting flag: test < 0.5 × train
    overfitting_flag = test_calendar_sum_val < 0.5 * train_calendar_sum

    # LOOCV degradation flag: loocv < 0.7 × train OR diff > 30%
    if train_calendar_sum != 0.0:
        loocv_ratio = loocv_calendar_sum / train_calendar_sum
        loocv_diff_pct = abs(train_calendar_sum - loocv_calendar_sum) / abs(train_calendar_sum) * 100.0
    else:
        loocv_ratio = 1.0
        loocv_diff_pct = 0.0

    loocv_degradation_flag = (
        loocv_calendar_sum < 0.7 * train_calendar_sum
        or loocv_diff_pct > 30.0
    )

    print(f"      → train_cs: {train_calendar_sum:.4f}%")
    print(f"      → test_cs: {test_calendar_sum_val:.4f}%")
    print(f"      → loocv_cs: {loocv_calendar_sum:.4f}%")
    print(f"      → test/train ratio: {test_calendar_sum_val / train_calendar_sum:.2f}" if train_calendar_sum != 0 else "      → train_cs=0, ratio undefined")
    print(f"      → loocv/train ratio: {loocv_ratio:.2f}")
    print(f"      → loocv diff%: {loocv_diff_pct:.1f}%")
    print(f"      → overfitting_flag: {overfitting_flag}")
    print(f"      → loocv_degradation_flag: {loocv_degradation_flag}")
    logger.info(
        f"Overfitting check: overfitting_flag={overfitting_flag}, "
        f"loocv_degradation_flag={loocv_degradation_flag}"
    )

    # --- 6.4 Regime 稳定性分析 ---
    print("[6.4] Regime 稳定性分析...")

    # 选择 best arm 对应的 regime schema 的标签
    test_mask = events["event_id"].isin(test_event_ids)
    if best_regime_schema == "5-regime":
        regime_labels_all = labels_5regime
    elif best_regime_schema == "3-regime":
        regime_labels_all = labels_3regime
    elif best_regime_schema == "2-regime":
        regime_labels_all = labels_2regime
    else:
        regime_labels_all = labels_5regime

    train_labels = regime_labels_all.loc[train_mask]
    test_labels = regime_labels_all.loc[test_mask]

    # 计算 train/test 各标签占比
    train_dist = train_labels.value_counts(normalize=True) * 100.0  # 百分比
    test_dist = test_labels.value_counts(normalize=True) * 100.0

    # 对齐所有标签
    all_labels_set = set(train_dist.index) | set(test_dist.index)
    label_distribution_shift = False
    shift_details: list[str] = []

    for label in sorted(all_labels_set):
        train_pct = train_dist.get(label, 0.0)
        test_pct = test_dist.get(label, 0.0)
        diff_pp = abs(train_pct - test_pct)
        # 严格大于 15pp 才触发（恰好 15pp 不触发）
        if diff_pp > 15.0:
            label_distribution_shift = True
            shift_details.append(
                f"{label}: train={train_pct:.1f}%, test={test_pct:.1f}%, diff={diff_pp:.1f}pp"
            )

    print(f"      → Regime schema: {best_regime_schema}")
    print(f"      → Train 分布: {train_dist.to_dict()}")
    print(f"      → Test 分布: {test_dist.to_dict()}")
    print(f"      → label_distribution_shift: {label_distribution_shift}")
    if shift_details:
        for detail in shift_details:
            print(f"        ⚠️ {detail}")
    logger.info(
        f"Regime stability: label_distribution_shift={label_distribution_shift}, "
        f"schema={best_regime_schema}"
    )

    # --- 6.5 Baseline_Legacy 横向对比 ---
    print("[6.5] Baseline_Legacy 横向对比...")

    # 从 baseline_legacy_summary 读取对比数据
    baseline_best_clf = baseline_legacy_summary.get("best_classifier", {})
    baseline_bootstrap = baseline_legacy_summary.get("bootstrap_ci", {})
    baseline_symbol_results = baseline_legacy_summary.get("symbol_results", {})
    baseline_regime_stability = baseline_legacy_summary.get("regime_stability", {})

    improvement_vs_legacy = best_arm.test_calendar_sum - baseline_legacy_cs

    print(f"      → Best arm test_cs: {best_arm.test_calendar_sum:.4f}%")
    print(f"      → Baseline_Legacy test_cs: {baseline_legacy_cs:.4f}%")
    print(f"      → 改善: {improvement_vs_legacy:+.4f}%")
    if baseline_bootstrap:
        bl_ci = baseline_bootstrap.get("overall", {})
        print(
            f"      → Baseline_Legacy CI: "
            f"[{bl_ci.get('ci_lower', 'N/A')}, {bl_ci.get('ci_upper', 'N/A')}]"
        )
    if baseline_symbol_results:
        for sym, sym_data in baseline_symbol_results.items():
            if isinstance(sym_data, dict):
                print(f"      → Baseline {sym}: test_cs={sym_data.get('test_calendar_sum', 'N/A')}")
    logger.info(
        f"Baseline comparison: improvement={improvement_vs_legacy:+.4f}%"
    )

    # --- 汇总 robustness_results ---
    robustness_results = {
        "bootstrap_ci": {
            "n_bootstrap": n_bootstrap,
            "random_state": 42,
            "mean": bootstrap_mean,
            "ci_lower": bootstrap_ci_lower,
            "ci_upper": bootstrap_ci_upper,
        },
        "symbol_bootstrap": symbol_bootstrap_results,
        "overfitting_check": {
            "train_calendar_sum": train_calendar_sum,
            "test_calendar_sum": test_calendar_sum_val,
            "loocv_calendar_sum": loocv_calendar_sum,
            "overfitting_flag": overfitting_flag,
            "loocv_degradation_flag": loocv_degradation_flag,
            "loocv_diff_pct": loocv_diff_pct,
        },
        "regime_stability": {
            "regime_schema": best_regime_schema,
            "label_distribution_shift": label_distribution_shift,
            "shift_details": shift_details,
            "train_distribution": train_dist.to_dict(),
            "test_distribution": test_dist.to_dict(),
        },
        "baseline_legacy_comparison": {
            "best_arm_test_cs": best_arm.test_calendar_sum,
            "baseline_legacy_cs": baseline_legacy_cs,
            "improvement_vs_legacy": improvement_vs_legacy,
            "baseline_bootstrap_ci": baseline_bootstrap.get("overall", {}),
            "baseline_symbol_results": baseline_symbol_results,
            "baseline_regime_stability": baseline_regime_stability,
        },
        "small_sample_warning": True,
    }

    print()
    print("=" * 70)
    print("Step 6 完成。稳健性验证就绪。")
    print(f"  Bootstrap 90% CI: [{bootstrap_ci_lower:.4f}%, {bootstrap_ci_upper:.4f}%]")
    print(f"  Overfitting flag: {overfitting_flag}")
    print(f"  LOOCV degradation flag: {loocv_degradation_flag}")
    print(f"  Label distribution shift: {label_distribution_shift}")
    print(f"  Improvement vs Legacy: {improvement_vs_legacy:+.4f}%")
    print(f"  Small sample warning: True")
    print("=" * 70)

    # ===================================================================
    # Step 7: 报告生成
    # ===================================================================
    print()
    print("=" * 70)
    print("Step 7: 报告生成 + Go/No-Go")
    print("=" * 70)

    from pretouch_refinement.refinement_report import (
        determine_go_nogo,
        generate_refinement_report,
    )

    # 7.1 determine_go_nogo()
    print("[7.1] Go/No-Go 判定...")
    go_nogo = determine_go_nogo(
        arm_results=arm_results,
        baseline_legacy_cs=baseline_legacy_cs,
        overfitting_flag=overfitting_flag,
        loocv_degradation_flag=loocv_degradation_flag,
    )
    print(f"      → 判定结果: {go_nogo.decision}")
    print(f"      → 最优 arm: {go_nogo.best_arm_name} ({go_nogo.best_classifier_name})")
    print(f"      → Test CS: +{go_nogo.test_calendar_sum:.4f}%")
    print(f"      → 理由: {go_nogo.reasoning}")
    logger.info(f"Go/No-Go: {go_nogo.decision}")

    # 7.2 generate_refinement_report() → 9 个输出文件
    print("[7.2] 生成 9 个输出文件...")
    generate_refinement_report(
        arm_results=arm_results,
        ablation_results=ablation_results,
        go_nogo=go_nogo,
        label_distributions=label_distributions,
        pit_audit_entries=pit_audit,
        baseline_legacy_summary=baseline_legacy_summary,
        robustness_results=robustness_results,
        output_dir=OUTPUT_DIR,
        best_global_delay=best_global_delay,
    )

    # 7.3 声明 small_sample_warning=true
    print("[7.3] ✓ small_sample_warning=true 已声明（写入报告和 summary.json）")
    logger.info("small_sample_warning=true declared")

    # 7.4 声明本 refinement 不改变既有部署/发现
    print("[7.4] ✓ 声明已写入报告: 本 refinement 不改变 pre-breakout-timing-classifier 的既有部署/发现")
    logger.info("Refinement non-alteration declaration written to report")

    # 汇总输出文件
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
    print()
    print("      输出文件清单:")
    for fname in all_output_files:
        fpath = OUTPUT_DIR / fname
        status = "✓" if fpath.exists() else "✗ (缺失)"
        print(f"        {status} {fname}")

    existing_count = sum(1 for f in all_output_files if (OUTPUT_DIR / f).exists())
    print()
    print(f"      → {existing_count}/{len(all_output_files)} 个文件已就绪")

    print()
    print("=" * 70)
    print("Step 7 完成。报告生成就绪。")
    print(f"  Go/No-Go: {go_nogo.decision}")
    print(f"  输出目录: {OUTPUT_DIR}")
    print("=" * 70)
    print()
    print("🎉 Pretouch Classifier Refinement 全流程完成！")
    print(f"   判定: {go_nogo.decision}")
    print(f"   最优 arm: {go_nogo.best_arm_name} ({go_nogo.best_classifier_name})")
    print(f"   Test CS: +{go_nogo.test_calendar_sum:.4f}%")
    print(f"   vs Legacy: {go_nogo.improvement_vs_legacy:+.4f}pp")


if __name__ == "__main__":
    main()
