"""RunnerRejectedCombinationsWriter — 运行前静态拒绝记录。

只记录运行前静态拒绝（例如 H > D、token 映射失败），
reject_reason ∈ {H_gt_D, token_mapping_error}。

与 runner_aborted.json 互斥：同一 candidate_id 不得同时出现在两份文件中。
正常完成时允许两份文件均不存在（禁止空文件占位）。

被记录的候选 MUST NOT 产出 ledger / summary / attribution / markdown 任一 artifact。

输出路径：research/tmp_entry_redesign_<candidate_id>_runner_rejected_combinations.json
JSON 格式：稳定键序、UTF-8 无 BOM、LF 结尾。

Requirements: 2.7, 4.10
"""

from __future__ import annotations

import json
import pathlib
from dataclasses import dataclass
from typing import Literal, TYPE_CHECKING

if TYPE_CHECKING:
    from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec


# ---------------------------------------------------------------------------
# reject_reason 封闭枚举
# ---------------------------------------------------------------------------

RejectReason = Literal["H_gt_D", "token_mapping_error"]

_VALID_REJECT_REASONS: frozenset[str] = frozenset({"H_gt_D", "token_mapping_error"})


# ---------------------------------------------------------------------------
# RunnerRejectedCombination 数据结构
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class RunnerRejectedCombination:
    """单条静态拒绝记录。

    Attributes:
        candidate_id: 被拒绝的候选 ID（满足正则
            ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$）。
        reject_reason: 拒绝原因，封闭枚举 {H_gt_D, token_mapping_error}。
        spec: 被拒绝的 EntryCandidateSpec 六元组字段。
        rejected_at_utc_ms: ISO-8601 UTC 毫秒精度时间戳
            （形如 2025-06-01T00:00:00.000Z）。
    """

    candidate_id: str
    reject_reason: RejectReason
    spec: dict  # EntryCandidateSpec 六元组字段的 dict 表示
    rejected_at_utc_ms: str


# ---------------------------------------------------------------------------
# RunnerRejectedCombinationsWriter
# ---------------------------------------------------------------------------


class RunnerRejectedCombinationsWriter:
    """运行前静态拒绝记录 writer。

    只在确实发生静态拒绝时写文件；正常完成时不创建空文件。
    与 RunnerAbortedWriter 互斥：同一 candidate_id 不得同时出现在两份文件中。

    Usage:
        writer = RunnerRejectedCombinationsWriter(output_dir=Path("research"))
        writer.write(
            candidate_id="d5_h15_tcnone_epmot_prnone_ponone-abcdef012345",
            reject_reason="H_gt_D",
            spec=my_spec,
            rejected_at_utc_ms="2025-06-01T00:00:00.000Z",
        )
    """

    def __init__(self, output_dir: pathlib.Path) -> None:
        """初始化 writer。

        Args:
            output_dir: 输出目录路径（通常为 research/ 根目录）。
        """
        self._output_dir = output_dir

    def write(
        self,
        candidate_id: str,
        reject_reason: RejectReason,
        spec: "EntryCandidateSpec",
        rejected_at_utc_ms: str,
    ) -> pathlib.Path:
        """写入静态拒绝记录 JSON 文件。

        仅在确实发生静态拒绝时调用。不得为"证明未发生拒绝"而创建空文件。

        Args:
            candidate_id: 被拒绝的候选 ID。
            reject_reason: 拒绝原因，必须为 {H_gt_D, token_mapping_error} 之一。
            spec: 被拒绝的 EntryCandidateSpec 实例。
            rejected_at_utc_ms: ISO-8601 UTC 毫秒精度时间戳。

        Returns:
            写入的 JSON 文件路径。

        Raises:
            ValueError: reject_reason 不在封闭枚举中。
        """
        if reject_reason not in _VALID_REJECT_REASONS:
            raise ValueError(
                f"reject_reason must be one of {sorted(_VALID_REJECT_REASONS)}, "
                f"got {reject_reason!r}"
            )

        # 构建 spec 字段 dict（六元组所有字段，稳定键序）
        spec_dict = {
            "entry_delay_seconds": spec.entry_delay_seconds,
            "entry_price_mode_id": spec.entry_price_mode_id,
            "feature_horizon_seconds": spec.feature_horizon_seconds,
            "posttouch_quality_band_id": spec.posttouch_quality_band_id,
            "pretouch_state_band_id": spec.pretouch_state_band_id,
            "trigger_confirmation_id": spec.trigger_confirmation_id,
        }

        # 构建输出 JSON 内容（稳定键序）
        record = {
            "candidate_id": candidate_id,
            "reject_reason": reject_reason,
            "rejected_at_utc_ms": rejected_at_utc_ms,
            "spec": spec_dict,
        }

        # 输出路径
        filename = (
            f"tmp_entry_redesign_{candidate_id}_runner_rejected_combinations.json"
        )
        out_path = self._output_dir / filename

        # 确保输出目录存在
        out_path.parent.mkdir(parents=True, exist_ok=True)

        # 写入 JSON：稳定键序、UTF-8 无 BOM、LF 结尾
        json_content = json.dumps(record, sort_keys=True, ensure_ascii=False, indent=2)
        # 确保 LF 结尾（无 trailing whitespace）
        if not json_content.endswith("\n"):
            json_content += "\n"

        # 使用 open() 显式控制 newline="\n" 以确保 LF 换行（兼容 Python 3.9）
        with open(out_path, "w", encoding="utf-8", newline="\n") as f:
            f.write(json_content)

        return out_path
