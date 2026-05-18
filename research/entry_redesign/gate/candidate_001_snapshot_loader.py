"""Candidate001SnapshotLoader — gate 参数快照加载。

快照来源：research/20260511_probabilistic_v6_calendar_holdout_validation.md
三阈值：
  - validation_return_over_dd <= 10
  - validation_topk_sizing_markov_score_mean <= 0.9
  - validation_topk_sized_return_pct >= 0.5

计算快照文件 sha256 并写入 summary JSON 的 gate001_snapshot_ref.{path, sha256}。
sha256 不一致 → 调用 RunnerAbortedWriter (abort_reason="parameter_mismatch")。

Requirements: 4.4, 4.7
"""

from __future__ import annotations

import hashlib
import pathlib
from dataclasses import dataclass


# ---------------------------------------------------------------------------
# Gate 001 三阈值常量（frozen dataclass，不可修改）
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Gate001Thresholds:
    """candidate_001 gate 三阈值参数快照。

    来源: research/20260511_probabilistic_v6_calendar_holdout_validation.md
    """

    validation_return_over_dd_threshold: float = 10.0
    validation_topk_sizing_markov_score_mean_threshold: float = 0.9
    validation_topk_sized_return_pct_threshold: float = 0.5


# 模块级常量实例
GATE_001_THRESHOLDS = Gate001Thresholds()

# 默认快照文件相对路径（相对于项目根目录）
DEFAULT_SNAPSHOT_PATH = "research/20260511_probabilistic_v6_calendar_holdout_validation.md"


# ---------------------------------------------------------------------------
# Candidate001SnapshotLoader
# ---------------------------------------------------------------------------


class Candidate001SnapshotLoader:
    """Gate 参数快照加载器。

    职责：
      1. 读取快照文件并计算 sha256
      2. 返回 gate 三阈值参数
      3. 提供 snapshot_ref dict 供 summary JSON 写入
      4. 校验 sha256 一致性（不一致时由调用方触发 abort）

    用法：
        loader = Candidate001SnapshotLoader(project_root / DEFAULT_SNAPSHOT_PATH)
        thresholds = loader.load()
        ref = loader.get_snapshot_ref()
        # 写入 summary JSON: summary["gate001_snapshot_ref"] = ref

        # 校验 sha256（若有预期值）
        if not loader.verify_sha256(expected_sha256):
            # 调用 RunnerAbortedWriter(abort_reason="parameter_mismatch")
            ...
    """

    def __init__(self, snapshot_path: pathlib.Path) -> None:
        """初始化 loader。

        Args:
            snapshot_path: 快照文件的绝对或相对路径。
                默认值为 DEFAULT_SNAPSHOT_PATH（相对于项目根目录）。
        """
        self._snapshot_path = snapshot_path
        self._sha256: str | None = None
        self._loaded: bool = False

    @property
    def snapshot_path(self) -> pathlib.Path:
        """返回快照文件路径。"""
        return self._snapshot_path

    @property
    def sha256(self) -> str:
        """返回快照文件的 sha256 摘要。

        必须先调用 load()。

        Raises:
            RuntimeError: 尚未调用 load()。
        """
        if self._sha256 is None:
            raise RuntimeError(
                "sha256 not available: call load() first"
            )
        return self._sha256

    def load(self) -> Gate001Thresholds:
        """读取快照文件并计算 sha256，返回 gate 三阈值。

        读取文件内容、计算 sha256 摘要并缓存。
        三阈值为固定常量（来自快照文件的已知参数），不从文件内容动态解析。

        Returns:
            Gate001Thresholds 实例（frozen dataclass）。

        Raises:
            FileNotFoundError: 快照文件不存在。
            OSError: 文件读取失败。
        """
        # 读取文件并计算 sha256
        content = self._snapshot_path.read_bytes()
        self._sha256 = hashlib.sha256(content).hexdigest()
        self._loaded = True

        return GATE_001_THRESHOLDS

    def get_snapshot_ref(self) -> dict[str, str]:
        """返回 snapshot_ref dict，供 summary JSON 的 gate001_snapshot_ref 字段。

        必须先调用 load()。

        Returns:
            {"path": <相对路径字符串>, "sha256": <hex 摘要>}

        Raises:
            RuntimeError: 尚未调用 load()。
        """
        if not self._loaded:
            raise RuntimeError(
                "snapshot_ref not available: call load() first"
            )
        return {
            "path": str(self._snapshot_path),
            "sha256": self._sha256,  # type: ignore[dict-item]
        }

    def verify_sha256(self, expected: str) -> bool:
        """校验 sha256 是否与预期值一致。

        不一致时返回 False，调用方应触发 RunnerAbortedWriter
        (abort_reason="parameter_mismatch")。

        必须先调用 load()。

        Args:
            expected: 预期的 sha256 hex 摘要字符串（64 字符小写 hex）。

        Returns:
            True 如果一致，False 如果不一致。

        Raises:
            RuntimeError: 尚未调用 load()。
        """
        if self._sha256 is None:
            raise RuntimeError(
                "sha256 not available: call load() first"
            )
        return self._sha256 == expected.lower().strip()
