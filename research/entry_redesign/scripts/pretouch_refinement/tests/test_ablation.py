"""
test_ablation — ablation 模块的单元测试

覆盖：
- run_ablation 返回 6 个 AblationResult
- 消融逻辑（特征组正确剔除）
- high_value_group / negative_value_group 标注
- 特征组不在矩阵中的优雅处理
- bootstrap CI bounds 合理性
- generate_ablation_report 文件生成

Requirements: 3.3, 3.4
"""

from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

# Path setup
_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pretouch_refinement.ablation import (
    AblationResult,
    run_ablation,
    generate_ablation_report,
)
from pretouch_refinement.enhanced_features import ENHANCED_FEATURE_GROUPS
from pre_breakout_timing.delay_simulator import DelayResult


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _make_delay_result(
    event_id: str,
    delay_label: str,
    pnl_pct: float | None,
    traded: bool = True,
    entry_time: str = "2024-01-10 08:00:00+00:00",
) -> DelayResult:
    """快速构建 DelayResult 用于测试。"""
    return DelayResult(
        event_id=event_id,
        delay_label=delay_label,
        delay_seconds={"D0": 0, "D5": 5, "D10": 10, "D15": 15, "pullback": 3}.get(
            delay_label, 0
        ),
        entry_time=pd.Timestamp(entry_time) if traded else None,
        entry_price=40000.0 if traded else None,
        pnl_pct=pnl_pct if traded else None,
        exit_reason="TrailingSL" if traded else None,
        exit_time=pd.Timestamp("2024-01-10 09:00:00+00:00") if traded else None,
        hold_seconds=3600.0 if traded else None,
        mfe_r=1.0 if traded else None,
        mae_r=-0.5 if traded else None,
        traded=traded,
    )


def _build_synthetic_data(
    n_train: int = 20,
    n_test: int = 10,
    include_all_enhanced: bool = True,
) -> dict:
    """构建合成数据用于 ablation 测试。

    Parameters
    ----------
    n_train : int
        训练集样本数。
    n_test : int
        测试集样本数。
    include_all_enhanced : bool
        是否包含所有增强特征列。

    Returns
    -------
    dict
        包含 full_features_train, full_features_test, labels,
        delay_results_train, delay_results_test, test_events 等。
    """
    rng = np.random.RandomState(42)

    # 构建特征列：原特征 + 增强特征
    original_features = ["feat_orig_1", "feat_orig_2", "feat_orig_3"]
    enhanced_features = []
    if include_all_enhanced:
        for group_feats in ENHANCED_FEATURE_GROUPS.values():
            enhanced_features.extend(group_feats)

    all_feature_cols = original_features + enhanced_features

    # 生成训练集特征
    train_data = rng.randn(n_train, len(all_feature_cols))
    full_features_train = pd.DataFrame(train_data, columns=all_feature_cols)

    # 生成测试集特征
    test_data = rng.randn(n_test, len(all_feature_cols))
    full_features_test = pd.DataFrame(test_data, columns=all_feature_cols)

    # 标签（5-regime 标签，不含 skip 以简化测试）
    label_choices = ["D0", "D5", "D10", "D15", "pullback"]
    labels = pd.Series(rng.choice(label_choices, size=n_train))

    # 构建 delay_results
    delay_labels = ["D0", "D5", "D10", "D15", "pullback"]

    def _build_delay_results(n: int, prefix: str) -> list[list[DelayResult]]:
        results = []
        for i in range(n):
            symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
            event_id = f"{symbol}_{prefix}_{i:03d}"
            entry_time = f"2024-01-{10 + (i % 20):02d} 08:00:00+00:00"
            event_delays = []
            for dl in delay_labels:
                pnl = rng.uniform(-0.05, 0.05)
                event_delays.append(
                    _make_delay_result(event_id, dl, pnl_pct=pnl, entry_time=entry_time)
                )
            results.append(event_delays)
        return results

    delay_results_train = _build_delay_results(n_train, "train")
    delay_results_test = _build_delay_results(n_test, "test")

    # 构建 test_events
    test_event_rows = []
    for i in range(n_test):
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        test_event_rows.append({
            "event_id": f"{symbol}_test_{i:03d}",
            "symbol": symbol,
            "side": "long",
            "touch_time": f"2024-01-{10 + (i % 20):02d} 08:00:00+00:00",
        })
    test_events = pd.DataFrame(test_event_rows)

    return {
        "full_features_train": full_features_train,
        "full_features_test": full_features_test,
        "labels": labels,
        "delay_results_train": delay_results_train,
        "delay_results_test": delay_results_test,
        "test_events": test_events,
    }


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestRunAblationReturns6Results:
    """验证 run_ablation 返回 6 个 AblationResult 对象。"""

    def test_run_ablation_returns_6_results(self):
        """run_ablation 应返回恰好 6 个 AblationResult（对应 6 个特征组）。"""
        data = _build_synthetic_data(n_train=20, n_test=10)

        results = run_ablation(
            full_features_train=data["full_features_train"],
            full_features_test=data["full_features_test"],
            labels=data["labels"],
            delay_results_train=data["delay_results_train"],
            delay_results_test=data["delay_results_test"],
            test_events=data["test_events"],
            oracle_calendar_sum=10.0,
            baseline_test_cs=2.0,
            best_classifier_name="DecisionTree",
            best_classifier_params={"max_depth": 3},
            regime_schema="5-regime",
            n_bootstrap=10,
            random_state=42,
        )

        assert len(results) == 6
        for r in results:
            assert isinstance(r, AblationResult)


class TestAblationFeaturesCorrectlyRemoved:
    """验证每组消融时正确剔除了对应特征。"""

    def test_ablation_features_correctly_removed(self):
        """每个 AblationResult 的 features_removed 应匹配 ENHANCED_FEATURE_GROUPS 中的特征。"""
        data = _build_synthetic_data(n_train=20, n_test=10)

        results = run_ablation(
            full_features_train=data["full_features_train"],
            full_features_test=data["full_features_test"],
            labels=data["labels"],
            delay_results_train=data["delay_results_train"],
            delay_results_test=data["delay_results_test"],
            test_events=data["test_events"],
            oracle_calendar_sum=10.0,
            baseline_test_cs=2.0,
            best_classifier_name="DecisionTree",
            best_classifier_params={"max_depth": 3},
            regime_schema="5-regime",
            n_bootstrap=10,
            random_state=42,
        )

        # 验证每组的 group_name 和 features_removed 与 ENHANCED_FEATURE_GROUPS 一致
        group_names = list(ENHANCED_FEATURE_GROUPS.keys())
        assert len(results) == len(group_names)

        for result in results:
            assert result.group_name in group_names
            expected_features = ENHANCED_FEATURE_GROUPS[result.group_name]
            # features_removed 应该是 expected_features 中实际存在于矩阵中的子集
            for feat in result.features_removed:
                assert feat in expected_features


class TestHighValueGroupAnnotation:
    """验证 high_value_group=True 当 delta < -0.3。"""

    def test_high_value_group_annotation(self):
        """当 delta_test_cs < -0.3 时，high_value_group 应为 True。"""
        data = _build_synthetic_data(n_train=20, n_test=10)

        # 使用一个较高的 baseline_test_cs 使得消融后 delta 可能 < -0.3
        # 由于合成数据的随机性，我们直接验证标注逻辑的一致性
        results = run_ablation(
            full_features_train=data["full_features_train"],
            full_features_test=data["full_features_test"],
            labels=data["labels"],
            delay_results_train=data["delay_results_train"],
            delay_results_test=data["delay_results_test"],
            test_events=data["test_events"],
            oracle_calendar_sum=10.0,
            baseline_test_cs=5.0,  # 高 baseline 使 delta 更可能为负
            best_classifier_name="DecisionTree",
            best_classifier_params={"max_depth": 3},
            regime_schema="5-regime",
            n_bootstrap=10,
            random_state=42,
        )

        for r in results:
            # 验证标注逻辑一致性：high_value_group iff delta < -0.3
            if r.delta_test_cs < -0.3:
                assert r.high_value_group is True, (
                    f"group '{r.group_name}': delta={r.delta_test_cs:.4f} < -0.3 "
                    f"but high_value_group={r.high_value_group}"
                )
            else:
                assert r.high_value_group is False, (
                    f"group '{r.group_name}': delta={r.delta_test_cs:.4f} >= -0.3 "
                    f"but high_value_group={r.high_value_group}"
                )


class TestNegativeValueGroupAnnotation:
    """验证 negative_value_group=True 当 delta >= 0。"""

    def test_negative_value_group_annotation(self):
        """当 delta_test_cs >= 0 时，negative_value_group 应为 True。"""
        data = _build_synthetic_data(n_train=20, n_test=10)

        results = run_ablation(
            full_features_train=data["full_features_train"],
            full_features_test=data["full_features_test"],
            labels=data["labels"],
            delay_results_train=data["delay_results_train"],
            delay_results_test=data["delay_results_test"],
            test_events=data["test_events"],
            oracle_calendar_sum=10.0,
            baseline_test_cs=2.0,
            best_classifier_name="DecisionTree",
            best_classifier_params={"max_depth": 3},
            regime_schema="5-regime",
            n_bootstrap=10,
            random_state=42,
        )

        for r in results:
            # 验证标注逻辑一致性：negative_value_group iff delta >= 0
            if r.delta_test_cs >= 0.0:
                assert r.negative_value_group is True, (
                    f"group '{r.group_name}': delta={r.delta_test_cs:.4f} >= 0 "
                    f"but negative_value_group={r.negative_value_group}"
                )
            else:
                assert r.negative_value_group is False, (
                    f"group '{r.group_name}': delta={r.delta_test_cs:.4f} < 0 "
                    f"but negative_value_group={r.negative_value_group}"
                )


class TestAblationGroupWithNoFeaturesInMatrix:
    """验证当特征组的特征不在矩阵中时的优雅处理。"""

    def test_ablation_group_with_no_features_in_matrix(self):
        """当某组特征不在特征矩阵中时，应返回 delta=0 且 negative_value_group=True。"""
        data = _build_synthetic_data(n_train=20, n_test=10)

        # 从特征矩阵中移除 volume_group 的特征
        volume_features = ENHANCED_FEATURE_GROUPS["volume_group"]
        features_train_no_volume = data["full_features_train"].drop(
            columns=volume_features, errors="ignore"
        )
        features_test_no_volume = data["full_features_test"].drop(
            columns=volume_features, errors="ignore"
        )

        results = run_ablation(
            full_features_train=features_train_no_volume,
            full_features_test=features_test_no_volume,
            labels=data["labels"],
            delay_results_train=data["delay_results_train"],
            delay_results_test=data["delay_results_test"],
            test_events=data["test_events"],
            oracle_calendar_sum=10.0,
            baseline_test_cs=2.0,
            best_classifier_name="DecisionTree",
            best_classifier_params={"max_depth": 3},
            regime_schema="5-regime",
            n_bootstrap=10,
            random_state=42,
        )

        # 找到 volume_group 的结果
        volume_result = next(
            r for r in results if r.group_name == "volume_group"
        )

        # 特征不在矩阵中时：delta=0, negative_value_group=True, high_value_group=False
        assert volume_result.delta_test_cs == 0.0
        assert volume_result.negative_value_group is True
        assert volume_result.high_value_group is False
        assert volume_result.test_calendar_sum == 2.0  # 等于 baseline


class TestBootstrapCIBounds:
    """验证 bootstrap CI 的合理性：CI lower <= delta <= CI upper（近似）。"""

    def test_bootstrap_ci_bounds(self):
        """bootstrap CI lower 应 <= delta_test_cs <= CI upper（对于有特征的组）。

        注意：由于 bootstrap 是对 test set 重采样，CI 反映的是不确定性，
        delta_test_cs 本身不一定严格在 CI 内（CI 是 bootstrap 分布的 percentile），
        但对于合理的 bootstrap，delta 应大致在 CI 范围附近。
        这里验证 CI lower <= CI upper 且 CI 范围合理。
        """
        data = _build_synthetic_data(n_train=20, n_test=10)

        results = run_ablation(
            full_features_train=data["full_features_train"],
            full_features_test=data["full_features_test"],
            labels=data["labels"],
            delay_results_train=data["delay_results_train"],
            delay_results_test=data["delay_results_test"],
            test_events=data["test_events"],
            oracle_calendar_sum=10.0,
            baseline_test_cs=2.0,
            best_classifier_name="DecisionTree",
            best_classifier_params={"max_depth": 3},
            regime_schema="5-regime",
            n_bootstrap=10,
            random_state=42,
        )

        for r in results:
            # CI lower 应 <= CI upper
            assert r.bootstrap_ci_lower <= r.bootstrap_ci_upper, (
                f"group '{r.group_name}': CI lower ({r.bootstrap_ci_lower}) > "
                f"CI upper ({r.bootstrap_ci_upper})"
            )

            # 对于有特征的组（delta != 0 的情况），CI 范围应有限
            if r.delta_test_cs != 0.0:
                ci_range = r.bootstrap_ci_upper - r.bootstrap_ci_lower
                # CI 范围不应过大（合理性检查）
                assert ci_range < 100.0, (
                    f"group '{r.group_name}': CI range ({ci_range}) unreasonably large"
                )


class TestGenerateAblationReportCreatesFiles:
    """验证 generate_ablation_report 生成 CSV 和 MD 文件。"""

    def test_generate_ablation_report_creates_files(self, tmp_path):
        """generate_ablation_report 应在 output_dir 中创建 ablation_results.csv 和 ablation_report.md。"""
        # 构建合成 AblationResult 列表
        results = []
        for group_name, features in ENHANCED_FEATURE_GROUPS.items():
            results.append(
                AblationResult(
                    group_name=group_name,
                    features_removed=features,
                    test_calendar_sum=2.5,
                    loocv_calendar_sum=2.3,
                    oracle_realization_rate=25.0,
                    delta_test_cs=-0.5,
                    high_value_group=True,
                    negative_value_group=False,
                    bootstrap_ci_lower=-0.8,
                    bootstrap_ci_upper=-0.2,
                )
            )

        output_dir = str(tmp_path)
        generate_ablation_report(results, output_dir)

        # 验证文件生成
        csv_path = tmp_path / "ablation_results.csv"
        md_path = tmp_path / "ablation_report.md"

        assert csv_path.exists(), "ablation_results.csv not created"
        assert md_path.exists(), "ablation_report.md not created"

        # 验证 CSV 内容基本正确
        df = pd.read_csv(csv_path)
        assert len(df) == 6
        assert "group_name" in df.columns
        assert "delta_test_cs" in df.columns
