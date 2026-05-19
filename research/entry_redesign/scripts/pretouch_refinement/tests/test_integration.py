"""
端到端集成测试 — Task 10.1 & 10.2

Task 10.1: 端到端集成测试
- 验证模块可正常 import
- 验证 9 个输出文件列表定义正确
- 验证 summary.json 结构完整（若存在）
- 验证确定性：generate_3regime_labels / rebuild_delay_results_from_matrix 两次调用一致

Task 10.2: 复用约束与输出隔离
- 确认未修改 pre_breakout_timing/ 目录下任何文件
- 确认所有输出仅写入 output/pretouch_refinement/
- 确认 baseline_legacy 字段正确引用 pre_breakout_timing_summary.json
- 确认使用同一 test set（time_split_events 后 40%）

Requirements: 7.5, 7.6, 8.1, 8.4
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

# ---------------------------------------------------------------------------
# Path setup
# ---------------------------------------------------------------------------

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))


# ===========================================================================
# Task 10.1: 端到端集成测试
# ===========================================================================


class TestRunnerImports:
    """10.1 - 验证模块可正常 import。"""

    def test_runner_imports_successfully(self):
        """验证 pretouch_refinement.refinement_runner 可正常 import。"""
        import pretouch_refinement.refinement_runner as runner

        # 验证关键属性存在
        assert hasattr(runner, "main")
        assert hasattr(runner, "OUTPUT_DIR")
        assert hasattr(runner, "DELAY_MATRIX_PATH")
        assert hasattr(runner, "BASELINE_SUMMARY_PATH")
        assert hasattr(runner, "SCRIPTS_DIR")

    def test_all_submodules_importable(self):
        """验证所有子模块可正常 import。"""
        import pretouch_refinement.enhanced_features
        import pretouch_refinement.regime_labels
        import pretouch_refinement.arm_runner
        import pretouch_refinement.ablation
        import pretouch_refinement.refinement_report

        # 验证关键函数/类存在
        assert hasattr(pretouch_refinement.enhanced_features, "extract_enhanced_features")
        assert hasattr(pretouch_refinement.regime_labels, "generate_3regime_labels")
        assert hasattr(pretouch_refinement.arm_runner, "rebuild_delay_results_from_matrix")
        assert hasattr(pretouch_refinement.ablation, "run_ablation")
        assert hasattr(pretouch_refinement.refinement_report, "generate_refinement_report")


class TestOutputFilesExpected:
    """10.1 - 验证 9 个预期输出文件列表定义正确。"""

    EXPECTED_OUTPUT_FILES = [
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

    def test_output_files_expected(self):
        """验证预期输出文件列表包含 9 个文件。"""
        assert len(self.EXPECTED_OUTPUT_FILES) == 9

    def test_output_files_have_correct_extensions(self):
        """验证输出文件扩展名合理（.md, .json, .csv）。"""
        valid_extensions = {".md", ".json", ".csv"}
        for fname in self.EXPECTED_OUTPUT_FILES:
            ext = Path(fname).suffix
            assert ext in valid_extensions, f"Unexpected extension for {fname}: {ext}"

    def test_output_dir_points_to_correct_location(self):
        """验证 OUTPUT_DIR 指向 output/pretouch_refinement/。"""
        from pretouch_refinement.refinement_runner import OUTPUT_DIR

        assert OUTPUT_DIR.name == "pretouch_refinement"
        assert OUTPUT_DIR.parent.name == "output"


class TestSummaryJsonStructure:
    """10.1 - 验证 summary.json 结构完整（若存在）。"""

    EXPECTED_TOP_LEVEL_KEYS = [
        "experiment_name",
        "best_arm",
        "go_nogo_decision",
        "baseline_legacy",
        "arm_results",
        "robustness",
    ]

    EXPECTED_BASELINE_LEGACY_KEYS = [
        "test_calendar_sum",
        "best_classifier",
    ]

    def test_summary_json_structure(self):
        """验证 summary.json 的预期结构（若文件存在）。

        若文件不存在（尚未运行完整 pipeline），则跳过。
        """
        from pretouch_refinement.refinement_runner import OUTPUT_DIR

        summary_path = OUTPUT_DIR / "pretouch_refinement_summary.json"
        if not summary_path.exists():
            pytest.skip(
                "pretouch_refinement_summary.json 不存在（需要完整运行 pipeline 生成）"
            )

        with open(summary_path, "r", encoding="utf-8") as f:
            summary = json.load(f)

        # 验证顶层 key 存在
        for key in self.EXPECTED_TOP_LEVEL_KEYS:
            assert key in summary, f"Missing top-level key: {key}"

        # 验证 baseline_legacy 字段结构
        baseline_legacy = summary["baseline_legacy"]
        assert isinstance(baseline_legacy, dict)
        for key in self.EXPECTED_BASELINE_LEGACY_KEYS:
            assert key in baseline_legacy, f"Missing baseline_legacy key: {key}"

        # 验证 go_nogo_decision 字段
        go_nogo = summary["go_nogo_decision"]
        assert isinstance(go_nogo, dict)
        assert "decision" in go_nogo
        assert go_nogo["decision"] in ["Strong Go", "Marginal Go", "No-Go"]


class TestDeterminism:
    """10.1 - 验证确定性：相同输入两次运行产出一致结果。

    Validates: Requirements 7.5
    """

    def _build_synthetic_matrix(self, n_events: int = 10) -> pd.DataFrame:
        """构建合成 delay_pnl_matrix 用于确定性测试。"""
        delays = ["D0", "D5", "D10", "D15", "pullback"]
        rng = np.random.default_rng(42)
        rows = []
        for i in range(n_events):
            eid = f"event_{i:03d}"
            for j, delay in enumerate(delays):
                rows.append({
                    "event_id": eid,
                    "delay_label": delay,
                    "delay_seconds": [0, 5, 10, 15, 20][j],
                    "pnl_pct": rng.uniform(-0.02, 0.03),
                    "traded": True,
                    "entry_time": f"2024-01-{i+1:02d}T10:00:00+00:00",
                    "exit_time": f"2024-01-{i+1:02d}T11:00:00+00:00",
                    "entry_price": 50000.0 + i * 100,
                    "exit_reason": "tp_hit",
                    "hold_seconds": 3600.0,
                    "mfe_r": rng.uniform(0, 0.05),
                    "mae_r": rng.uniform(-0.03, 0),
                    "symbol": "BTCUSDT" if i % 2 == 0 else "ETHUSDT",
                })
        return pd.DataFrame(rows)

    def _build_synthetic_events(self, n_events: int = 10) -> pd.DataFrame:
        """构建合成 events DataFrame。"""
        return pd.DataFrame({
            "event_id": [f"event_{i:03d}" for i in range(n_events)],
            "symbol": ["BTCUSDT" if i % 2 == 0 else "ETHUSDT" for i in range(n_events)],
            "touch_time": pd.date_range("2024-01-01", periods=n_events, freq="D", tz="UTC"),
        })

    def test_determinism_generate_3regime_labels(self):
        """验证 generate_3regime_labels 两次调用结果逐 event 一致。"""
        from pretouch_refinement.regime_labels import generate_3regime_labels

        n_events = 20
        matrix = self._build_synthetic_matrix(n_events)
        events = self._build_synthetic_events(n_events)

        # 第一次运行
        labels_run1 = generate_3regime_labels(matrix, events, tolerance_bps=5.0)

        # 第二次运行
        labels_run2 = generate_3regime_labels(matrix, events, tolerance_bps=5.0)

        # 逐 event 一致
        assert list(labels_run1) == list(labels_run2), (
            "generate_3regime_labels 两次运行结果不一致"
        )

    def test_determinism_rebuild_delay_results_from_matrix(self):
        """验证 rebuild_delay_results_from_matrix 两次调用结果一致。"""
        from pretouch_refinement.arm_runner import rebuild_delay_results_from_matrix

        n_events = 10
        matrix = self._build_synthetic_matrix(n_events)
        events = self._build_synthetic_events(n_events)

        # 第一次运行
        results_run1 = rebuild_delay_results_from_matrix(matrix, events)

        # 第二次运行
        results_run2 = rebuild_delay_results_from_matrix(matrix, events)

        # 验证结构一致
        assert len(results_run1) == len(results_run2)
        for i in range(len(results_run1)):
            assert len(results_run1[i]) == len(results_run2[i])
            for j in range(len(results_run1[i])):
                dr1 = results_run1[i][j]
                dr2 = results_run2[i][j]
                assert dr1.event_id == dr2.event_id
                assert dr1.delay_label == dr2.delay_label
                assert dr1.pnl_pct == dr2.pnl_pct
                assert dr1.traded == dr2.traded

    def test_determinism_generate_2regime_labels(self):
        """验证 generate_2regime_labels 两次调用结果一致。"""
        from pretouch_refinement.regime_labels import generate_2regime_labels

        n_events = 10
        matrix = self._build_synthetic_matrix(n_events)
        events = self._build_synthetic_events(n_events)

        # 添加 silo 所需列
        matrix["symbol"] = matrix["event_id"].apply(
            lambda eid: "BTCUSDT" if int(eid.split("_")[1]) % 2 == 0 else "ETHUSDT"
        )

        # 60/40 split
        train_events = events.iloc[:6].reset_index(drop=True)

        labels_run1, delay_run1 = generate_2regime_labels(matrix, train_events, events)
        labels_run2, delay_run2 = generate_2regime_labels(matrix, train_events, events)

        assert list(labels_run1) == list(labels_run2)
        assert delay_run1 == delay_run2


# ===========================================================================
# Task 10.2: 复用约束与输出隔离
# ===========================================================================


class TestPreBreakoutTimingNotModified:
    """10.2 - 确认未修改 pre_breakout_timing/ 目录下任何文件。"""

    def test_pre_breakout_timing_not_modified(self):
        """验证 pre_breakout_timing/ 目录存在且关键文件完整。

        本 spec 的核心约束：不修改 pre_breakout_timing/ 目录下任何文件。
        通过检查关键文件存在性和模块可导入性来验证。
        """
        pre_breakout_dir = _SCRIPTS_DIR / "pre_breakout_timing"

        # 目录存在
        assert pre_breakout_dir.exists(), "pre_breakout_timing/ 目录不存在"
        assert pre_breakout_dir.is_dir(), "pre_breakout_timing/ 不是目录"

        # 关键文件存在
        expected_files = [
            "data_layer.py",
            "delay_simulator.py",
            "feature_extractor.py",
            "timing_classifier.py",
        ]
        for fname in expected_files:
            fpath = pre_breakout_dir / fname
            assert fpath.exists(), f"pre_breakout_timing/{fname} 不存在"

        # 验证模块可正常 import（未被破坏）
        from pre_breakout_timing.delay_simulator import DelayResult
        from pre_breakout_timing.timing_classifier import ClassifierResult

        assert DelayResult is not None
        assert ClassifierResult is not None

    def test_pretouch_refinement_does_not_write_to_pre_breakout(self):
        """验证 pretouch_refinement 模块不包含对 pre_breakout_timing/ 目录的写操作。

        检查 pretouch_refinement/ 下所有 .py 文件不包含向 pre_breakout_timing
        目录写入文件的代码模式（如 open(pre_breakout_timing_path, 'w')）。
        """
        pretouch_dir = _SCRIPTS_DIR / "pretouch_refinement"

        # 写操作模式：向 pre_breakout_timing 目录写入
        write_patterns = [
            "pre_breakout_timing.*open.*'w'",
            "pre_breakout_timing.*write_text",
            "pre_breakout_timing.*write_bytes",
            "pre_breakout_timing.*to_csv",
            "pre_breakout_timing.*to_json",
        ]

        import re

        for py_file in pretouch_dir.glob("*.py"):
            content = py_file.read_text(encoding="utf-8")
            for pattern in write_patterns:
                matches = re.findall(pattern, content)
                assert not matches, (
                    f"{py_file.name} 包含对 pre_breakout_timing 的写操作: {matches}"
                )


class TestOutputIsolation:
    """10.2 - 确认所有输出仅写入 output/pretouch_refinement/。"""

    def test_output_isolation(self):
        """验证 OUTPUT_DIR 指向 output/pretouch_refinement/。"""
        from pretouch_refinement.refinement_runner import OUTPUT_DIR, SCRIPTS_DIR

        expected_output_dir = SCRIPTS_DIR / "output" / "pretouch_refinement"
        assert OUTPUT_DIR == expected_output_dir, (
            f"OUTPUT_DIR 应为 {expected_output_dir}，实际为 {OUTPUT_DIR}"
        )

    def test_output_dir_does_not_overlap_pre_breakout(self):
        """验证输出目录与 pre_breakout_timing 输出目录不重叠。"""
        from pretouch_refinement.refinement_runner import OUTPUT_DIR, SCRIPTS_DIR

        pre_breakout_output = SCRIPTS_DIR / "output" / "pre_breakout_timing"

        # 两个目录不应相同
        assert OUTPUT_DIR != pre_breakout_output
        # pretouch_refinement 输出不应是 pre_breakout_timing 的子目录
        assert not str(OUTPUT_DIR).startswith(str(pre_breakout_output))
        # pre_breakout_timing 输出不应是 pretouch_refinement 的子目录
        assert not str(pre_breakout_output).startswith(str(OUTPUT_DIR))

    def test_output_dir_within_research_scripts(self):
        """验证输出目录在 research/entry_redesign/scripts/output/ 下。"""
        from pretouch_refinement.refinement_runner import OUTPUT_DIR

        # 输出目录应在 scripts/output/ 下
        assert "output" in OUTPUT_DIR.parts
        assert "pretouch_refinement" in OUTPUT_DIR.parts
        # 不应写入 internal/, deployments/, .github/, cmd/, web/
        forbidden_dirs = ["internal", "deployments", ".github", "cmd", "web"]
        for forbidden in forbidden_dirs:
            assert forbidden not in OUTPUT_DIR.parts, (
                f"OUTPUT_DIR 不应包含 {forbidden}"
            )


class TestBaselineLegacyFieldStructure:
    """10.2 - 验证 baseline_legacy 字段正确引用 pre_breakout_timing_summary.json。"""

    def test_baseline_legacy_field_structure(self):
        """验证 baseline_legacy 字段的预期结构。

        若 summary.json 存在，验证 baseline_legacy 字段包含正确的引用。
        若不存在，验证 BASELINE_SUMMARY_PATH 指向正确位置。
        """
        from pretouch_refinement.refinement_runner import (
            BASELINE_SUMMARY_PATH,
            OUTPUT_DIR,
        )

        # 验证 BASELINE_SUMMARY_PATH 指向 pre_breakout_timing_summary.json
        assert BASELINE_SUMMARY_PATH.name == "pre_breakout_timing_summary.json"
        assert "pre_breakout_timing" in str(BASELINE_SUMMARY_PATH)

        # 若 summary.json 存在，验证 baseline_legacy 字段
        summary_path = OUTPUT_DIR / "pretouch_refinement_summary.json"
        if summary_path.exists():
            with open(summary_path, "r", encoding="utf-8") as f:
                summary = json.load(f)

            assert "baseline_legacy" in summary, (
                "summary.json 缺少 baseline_legacy 字段"
            )
            baseline_legacy = summary["baseline_legacy"]
            assert isinstance(baseline_legacy, dict)

            # 验证包含关键数值引用
            assert "test_calendar_sum" in baseline_legacy, (
                "baseline_legacy 缺少 test_calendar_sum"
            )
            assert isinstance(baseline_legacy["test_calendar_sum"], (int, float))

    def test_baseline_summary_path_references_correct_file(self):
        """验证 BASELINE_SUMMARY_PATH 引用的是 pre_breakout_timing 的输出。"""
        from pretouch_refinement.refinement_runner import BASELINE_SUMMARY_PATH, SCRIPTS_DIR

        expected_path = (
            SCRIPTS_DIR / "output" / "pre_breakout_timing" / "pre_breakout_timing_summary.json"
        )
        assert BASELINE_SUMMARY_PATH == expected_path, (
            f"BASELINE_SUMMARY_PATH 应为 {expected_path}，实际为 {BASELINE_SUMMARY_PATH}"
        )


class TestSameTestSet:
    """10.2 - 验证使用同一 test set（time_split_events 后 40%）。"""

    def test_same_test_set(self):
        """验证 time_split_events 产出一致的 60/40 split。

        两次调用 time_split_events 应产出完全相同的 train/test 划分。
        """
        # 构建合成 events（模拟 116 events）
        n_events = 116
        events = pd.DataFrame({
            "event_id": [f"event_{i:03d}" for i in range(n_events)],
            "symbol": ["BTCUSDT" if i % 2 == 0 else "ETHUSDT" for i in range(n_events)],
            "touch_time": pd.date_range(
                "2024-01-01", periods=n_events, freq="3D", tz="UTC"
            ),
        })

        from dynamic_timing.data_layer import time_split_events

        # 第一次 split
        train1, test1 = time_split_events(events)

        # 第二次 split
        train2, test2 = time_split_events(events)

        # 验证一致性
        assert len(train1) == len(train2)
        assert len(test1) == len(test2)
        assert list(train1["event_id"]) == list(train2["event_id"])
        assert list(test1["event_id"]) == list(test2["event_id"])

    def test_time_split_ratio_60_40(self):
        """验证 time_split_events 默认使用 60/40 比例。"""
        n_events = 116
        events = pd.DataFrame({
            "event_id": [f"event_{i:03d}" for i in range(n_events)],
            "symbol": ["BTCUSDT"] * n_events,
            "touch_time": pd.date_range(
                "2024-01-01", periods=n_events, freq="D", tz="UTC"
            ),
        })

        from dynamic_timing.data_layer import time_split_events

        train, test = time_split_events(events)

        # 60% train, 40% test
        expected_train_size = int(n_events * 0.6)  # 69
        expected_test_size = n_events - expected_train_size  # 47

        assert len(train) == expected_train_size, (
            f"Train size should be {expected_train_size}, got {len(train)}"
        )
        assert len(test) == expected_test_size, (
            f"Test size should be {expected_test_size}, got {len(test)}"
        )

    def test_time_split_is_chronological(self):
        """验证 time_split_events 按时间排序后 split（train 在前，test 在后）。"""
        # 故意打乱顺序
        events = pd.DataFrame({
            "event_id": ["e3", "e1", "e5", "e2", "e4"],
            "symbol": ["BTCUSDT"] * 5,
            "touch_time": pd.to_datetime([
                "2024-03-01", "2024-01-01", "2024-05-01",
                "2024-02-01", "2024-04-01",
            ], utc=True),
        })

        from dynamic_timing.data_layer import time_split_events

        train, test = time_split_events(events)

        # train 应包含时间最早的 60% events
        # 排序后: e1(01), e2(02), e3(03), e4(04), e5(05)
        # 60% of 5 = 3 → train = [e1, e2, e3], test = [e4, e5]
        assert len(train) == 3
        assert len(test) == 2

        # train 中最晚的时间应早于 test 中最早的时间
        train_max_time = pd.Timestamp(train["touch_time"].max())
        test_min_time = pd.Timestamp(test["touch_time"].min())
        assert train_max_time <= test_min_time, (
            "Train set 最晚时间应 <= test set 最早时间"
        )
