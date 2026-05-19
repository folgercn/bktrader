"""RunnerAbortedWriter — 运行中 abort 记录写盘。

运行中检测到 Entry_Candidate 的任何字段与 RunnerParameterSnapshot 不一致、
walk-forward 历史不足、invariant 违反或 IO 错误时，终止该候选并写入
runner_aborted.json。

abort_reason 封闭枚举:
  {parameter_mismatch, insufficient_walkforward_history, invariant_violation, io_error}

abort 后 MUST NOT 产出 ledger / summary / attribution / <YYYYMMDD> markdown。

输出路径: research/tmp_entry_redesign_<candidate_id>_runner_aborted.json
字段: candidate_id, abort_reason, mismatched_fields[], aborted_at_utc_ms

Requirements: 4.7, 4.8, 4.10
"""

from __future__ import annotations

import json
import pathlib
from typing import Literal


# ---------------------------------------------------------------------------
# abort_reason 封闭枚举
# ---------------------------------------------------------------------------

AbortReason = Literal[
    "parameter_mismatch",
    "insufficient_walkforward_history",
    "invariant_violation",
    "io_error",
]

VALID_ABORT_REASONS: frozenset[str] = frozenset(
    {
        "parameter_mismatch",
        "insufficient_walkforward_history",
        "invariant_violation",
        "io_error",
    }
)


# ---------------------------------------------------------------------------
# RunnerAbortedWriter
# ---------------------------------------------------------------------------


class RunnerAbortedWriter:
    """运行中 abort 记录 writer。

    写 research/tmp_entry_redesign_<candidate_id>_runner_aborted.json。

    abort 后 MUST NOT 产出 ledger / summary / attribution / <YYYYMMDD> markdown。
    与 runner_rejected_combinations.json 互斥：同一 candidate_id 不得同时出现。
    正常完成时允许该文件不存在（禁止空文件占位）。

    JSON 写盘要求：
      - 稳定键序（按字段声明顺序固定）
      - UTF-8 无 BOM
      - LF 换行
      - 数值定点
      - 禁止 datetime.now() / os.getpid()
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
        abort_reason: AbortReason,
        mismatched_fields: list[dict[str, str]],
        aborted_at_utc_ms: str,
    ) -> pathlib.Path:
        """写入 runner_aborted.json。

        Args:
            candidate_id: Entry_Candidate 的唯一标识，满足正则
                ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$。
            abort_reason: 封闭枚举值，必须为
                {parameter_mismatch, insufficient_walkforward_history,
                 invariant_violation, io_error} 之一。
            mismatched_fields: 不一致字段列表，每项为
                {"field_name": str, "expected": str, "observed": str}。
                当 abort_reason 非 parameter_mismatch 时可为空列表。
            aborted_at_utc_ms: ISO-8601 UTC 毫秒精度时间戳字符串
                （形如 "2025-06-01T00:00:00.000Z"）。
                由调用方传入，MUST NOT 在本方法内通过 datetime.now() 生成。

        Returns:
            写入文件的绝对路径。

        Raises:
            ValueError: abort_reason 不在封闭枚举中。
            ValueError: mismatched_fields 中某项缺少必需键。
        """
        # 校验 abort_reason
        if abort_reason not in VALID_ABORT_REASONS:
            raise ValueError(
                f"abort_reason must be one of {sorted(VALID_ABORT_REASONS)}, "
                f"got {abort_reason!r}"
            )

        # 校验 mismatched_fields 结构
        for i, field in enumerate(mismatched_fields):
            for required_key in ("field_name", "expected", "observed"):
                if required_key not in field:
                    raise ValueError(
                        f"mismatched_fields[{i}] missing required key "
                        f"{required_key!r}: {field!r}"
                    )

        # 构建 JSON 数据（固定键序）
        data: dict = {
            "candidate_id": candidate_id,
            "abort_reason": abort_reason,
            "mismatched_fields": [
                {
                    "field_name": f["field_name"],
                    "expected": f["expected"],
                    "observed": f["observed"],
                }
                for f in mismatched_fields
            ],
            "aborted_at_utc_ms": aborted_at_utc_ms,
        }

        # 输出路径
        filename = f"tmp_entry_redesign_{candidate_id}_runner_aborted.json"
        out_path = self._output_dir / filename

        # 确保输出目录存在
        out_path.parent.mkdir(parents=True, exist_ok=True)

        # 写盘：稳定键序、UTF-8 无 BOM、LF 换行
        json_str = json.dumps(data, ensure_ascii=False, indent=2, sort_keys=False)
        # 确保 LF 结尾
        if not json_str.endswith("\n"):
            json_str += "\n"

        # 使用二进制写入确保 LF 换行（跨平台一致）
        out_path.write_bytes(json_str.encode("utf-8"))

        return out_path
