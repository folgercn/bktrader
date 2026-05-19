"""Unit tests: Gate snapshot sha256 verification.

确定性 fixture 测试：
  1. 创建 temp file，用 Candidate001SnapshotLoader 加载，验证 sha256 匹配
  2. verify_sha256() 对正确 hash 返回 True，对错误 hash 返回 False
  3. sha256 不匹配时，调用方可触发 RunnerAbortedWriter (abort_reason="parameter_mismatch")

Requirements: 4.4, 4.7
"""

from __future__ import annotations

import hashlib
import json
import pathlib
import tempfile

import pytest

from research.entry_redesign.gate.candidate_001_snapshot_loader import (
    GATE_001_THRESHOLDS,
    Candidate001SnapshotLoader,
    Gate001Thresholds,
)
from research.entry_redesign.snapshot.runner_aborted_writer import (
    RunnerAbortedWriter,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

FIXTURE_CONTENT = b"# Probabilistic V6 Calendar Holdout Validation\n\nThis is a test fixture.\n"
FIXTURE_SHA256 = hashlib.sha256(FIXTURE_CONTENT).hexdigest()
WRONG_SHA256 = "0" * 64  # Guaranteed to differ from any real file hash


@pytest.fixture
def snapshot_file(tmp_path: pathlib.Path) -> pathlib.Path:
    """Create a deterministic temp file simulating the gate snapshot."""
    p = tmp_path / "20260511_probabilistic_v6_calendar_holdout_validation.md"
    p.write_bytes(FIXTURE_CONTENT)
    return p


# ---------------------------------------------------------------------------
# Test 1: Load file and verify sha256 matches
# ---------------------------------------------------------------------------


class TestSnapshotLoaderSha256Match:
    """Candidate001SnapshotLoader 加载文件后 sha256 与手动计算一致。"""

    def test_load_computes_correct_sha256(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """加载文件后，loader.sha256 应与手动 hashlib 计算结果一致。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        thresholds = loader.load()

        # sha256 应与 fixture 内容的手动计算一致
        assert loader.sha256 == FIXTURE_SHA256

    def test_load_returns_gate_thresholds(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """load() 返回 Gate001Thresholds 常量实例。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        thresholds = loader.load()

        assert isinstance(thresholds, Gate001Thresholds)
        assert thresholds.validation_return_over_dd_threshold == 10.0
        assert thresholds.validation_topk_sizing_markov_score_mean_threshold == 0.9
        assert thresholds.validation_topk_sized_return_pct_threshold == 0.5
        assert thresholds == GATE_001_THRESHOLDS

    def test_get_snapshot_ref_after_load(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """load() 后 get_snapshot_ref() 返回正确的 path 和 sha256。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        ref = loader.get_snapshot_ref()
        assert ref["path"] == str(snapshot_file)
        assert ref["sha256"] == FIXTURE_SHA256


# ---------------------------------------------------------------------------
# Test 2: verify_sha256() returns True/False correctly
# ---------------------------------------------------------------------------


class TestVerifySha256:
    """verify_sha256() 对正确 hash 返回 True，对错误 hash 返回 False。"""

    def test_verify_sha256_returns_true_for_correct_hash(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """正确的 expected sha256 → verify_sha256() 返回 True。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        assert loader.verify_sha256(FIXTURE_SHA256) is True

    def test_verify_sha256_returns_true_case_insensitive(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """verify_sha256() 对大写 hex 也应返回 True（内部做 lower）。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        assert loader.verify_sha256(FIXTURE_SHA256.upper()) is True

    def test_verify_sha256_returns_false_for_wrong_hash(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """错误的 expected sha256 → verify_sha256() 返回 False。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        assert loader.verify_sha256(WRONG_SHA256) is False

    def test_verify_sha256_returns_false_for_partial_mismatch(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """仅最后一位不同 → verify_sha256() 返回 False。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        # Flip the last character
        last_char = FIXTURE_SHA256[-1]
        flipped = "0" if last_char != "0" else "1"
        almost_correct = FIXTURE_SHA256[:-1] + flipped

        assert loader.verify_sha256(almost_correct) is False

    def test_verify_sha256_raises_before_load(
        self, snapshot_file: pathlib.Path
    ) -> None:
        """未调用 load() 时 verify_sha256() 应抛 RuntimeError。"""
        loader = Candidate001SnapshotLoader(snapshot_file)

        with pytest.raises(RuntimeError, match="call load\\(\\) first"):
            loader.verify_sha256(FIXTURE_SHA256)


# ---------------------------------------------------------------------------
# Test 3: sha256 mismatch triggers RunnerAbortedWriter with abort_reason="parameter_mismatch"
# ---------------------------------------------------------------------------


class TestSha256MismatchTriggersAbort:
    """sha256 不匹配时，调用方触发 RunnerAbortedWriter (abort_reason="parameter_mismatch")。"""

    def test_mismatch_triggers_abort_write(
        self, snapshot_file: pathlib.Path, tmp_path: pathlib.Path
    ) -> None:
        """sha256 不匹配 → 调用 RunnerAbortedWriter 写 abort JSON。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        # Simulate the caller's logic: verify sha256, if mismatch → abort
        expected_sha256 = WRONG_SHA256
        assert loader.verify_sha256(expected_sha256) is False

        # Caller triggers abort
        candidate_id = "d0_h0_none_market_on_touch_none_none-abcdef012345"
        output_dir = tmp_path / "research_output"
        output_dir.mkdir(parents=True, exist_ok=True)

        abort_writer = RunnerAbortedWriter(output_dir)
        abort_path = abort_writer.write(
            candidate_id=candidate_id,
            abort_reason="parameter_mismatch",
            mismatched_fields=[
                {
                    "field_name": "gate001_snapshot_sha256",
                    "expected": expected_sha256,
                    "observed": loader.sha256,
                }
            ],
            aborted_at_utc_ms="2025-06-15T12:00:00.000Z",
        )

        # Verify abort file was written
        assert abort_path.exists()

        # Verify abort file content
        abort_data = json.loads(abort_path.read_text(encoding="utf-8"))
        assert abort_data["candidate_id"] == candidate_id
        assert abort_data["abort_reason"] == "parameter_mismatch"
        assert len(abort_data["mismatched_fields"]) == 1
        assert abort_data["mismatched_fields"][0]["field_name"] == "gate001_snapshot_sha256"
        assert abort_data["mismatched_fields"][0]["expected"] == expected_sha256
        assert abort_data["mismatched_fields"][0]["observed"] == FIXTURE_SHA256
        assert abort_data["aborted_at_utc_ms"] == "2025-06-15T12:00:00.000Z"

    def test_match_does_not_trigger_abort(
        self, snapshot_file: pathlib.Path, tmp_path: pathlib.Path
    ) -> None:
        """sha256 匹配 → 不触发 abort，abort 文件不存在。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        # Verify sha256 matches
        assert loader.verify_sha256(FIXTURE_SHA256) is True

        # In the normal flow, no abort file is written
        candidate_id = "d0_h0_none_market_on_touch_none_none-abcdef012345"
        abort_filename = f"tmp_entry_redesign_{candidate_id}_runner_aborted.json"
        abort_path = tmp_path / "research_output" / abort_filename

        assert not abort_path.exists()

    def test_abort_file_not_created_on_match(
        self, snapshot_file: pathlib.Path, tmp_path: pathlib.Path
    ) -> None:
        """正常完成时 runner_aborted.json 不存在（禁止空文件占位）。"""
        loader = Candidate001SnapshotLoader(snapshot_file)
        loader.load()

        output_dir = tmp_path / "research_output"
        output_dir.mkdir(parents=True, exist_ok=True)

        # When sha256 matches, the caller does NOT call RunnerAbortedWriter
        assert loader.verify_sha256(FIXTURE_SHA256) is True

        # Verify no abort files exist in output directory
        abort_files = list(output_dir.glob("*_runner_aborted.json"))
        assert len(abort_files) == 0


# ---------------------------------------------------------------------------
# Edge cases
# ---------------------------------------------------------------------------


class TestEdgeCases:
    """边界情况测试。"""

    def test_load_nonexistent_file_raises(self, tmp_path: pathlib.Path) -> None:
        """快照文件不存在 → load() 抛 FileNotFoundError。"""
        nonexistent = tmp_path / "does_not_exist.md"
        loader = Candidate001SnapshotLoader(nonexistent)

        with pytest.raises(FileNotFoundError):
            loader.load()

    def test_sha256_property_raises_before_load(
        self, tmp_path: pathlib.Path
    ) -> None:
        """未调用 load() 时访问 .sha256 属性应抛 RuntimeError。"""
        p = tmp_path / "test.md"
        p.write_bytes(b"content")
        loader = Candidate001SnapshotLoader(p)

        with pytest.raises(RuntimeError, match="call load\\(\\) first"):
            _ = loader.sha256

    def test_get_snapshot_ref_raises_before_load(
        self, tmp_path: pathlib.Path
    ) -> None:
        """未调用 load() 时 get_snapshot_ref() 应抛 RuntimeError。"""
        p = tmp_path / "test.md"
        p.write_bytes(b"content")
        loader = Candidate001SnapshotLoader(p)

        with pytest.raises(RuntimeError, match="call load\\(\\) first"):
            loader.get_snapshot_ref()

    def test_empty_file_has_valid_sha256(self, tmp_path: pathlib.Path) -> None:
        """空文件也应产出有效的 sha256（sha256 of empty bytes）。"""
        p = tmp_path / "empty.md"
        p.write_bytes(b"")
        loader = Candidate001SnapshotLoader(p)
        loader.load()

        expected = hashlib.sha256(b"").hexdigest()
        assert loader.sha256 == expected
        assert loader.verify_sha256(expected) is True
