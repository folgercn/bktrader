"""RankingMarkdownWriter — Entry_Candidate 排名报告 markdown 写盘入口。

排名维度（Requirement 3.8）：
  - 主键：nogate_calendar_normalized_return_pct DESC
  - 副键：nogate_per_trade_quality_bps_over_notional DESC
  - tie-breaker：candidate_id 字典序 ASC

每行同步 asymmetry_tag 标注（Requirement 3.7）：
  - asymmetry_tag ∈ {eth_only_positive, btc_only_positive, all_symbols_positive, none}
  - 禁止把 asymmetry_tag == "eth_only_positive" 的候选在 BTCUSDT 维度声称为"全 symbol 正"

确定性约束：
  - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源
  - UTF-8 无 BOM / LF 换行
  - 浮点字段 8 位定点十进制，禁科学计数法

Requirements: 3.8, 3.7
"""

from __future__ import annotations

import pathlib
from typing import Sequence


# ---------------------------------------------------------------------------
# 排序辅助
# ---------------------------------------------------------------------------


def _sort_key(candidate: dict) -> tuple[float, float, str]:
    """生成排序键。

    排序规则：
      1. nogate_calendar_normalized_return_pct DESC → 取负值实现降序
      2. nogate_per_trade_quality_bps_over_notional DESC → 取负值实现降序
      3. candidate_id 字典序 ASC → 原值

    Args:
        candidate: 包含排名所需字段的字典。

    Returns:
        三元组排序键。
    """
    cal_return = float(candidate["nogate_calendar_normalized_return_pct"])
    quality_bps = float(candidate["nogate_per_trade_quality_bps_over_notional"])
    candidate_id = str(candidate["candidate_id"])
    return (-cal_return, -quality_bps, candidate_id)


# ---------------------------------------------------------------------------
# 格式化辅助
# ---------------------------------------------------------------------------


def _format_float_8dp(value: float) -> str:
    """格式化浮点数为 8 位定点十进制字符串，禁止科学计数法。"""
    return f"{value:.8f}"


# ---------------------------------------------------------------------------
# RankingMarkdownWriter
# ---------------------------------------------------------------------------


class RankingMarkdownWriter:
    """Entry_Candidate 排名报告 markdown 写盘入口。

    排名维度固定为：
      - 主键：nogate_calendar_normalized_return_pct DESC
      - 副键：nogate_per_trade_quality_bps_over_notional DESC
      - tie-breaker：candidate_id 字典序 ASC

    每行同步 asymmetry_tag 标注（Requirement 3.7）。

    保证确定性约束：
      - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源
      - UTF-8 无 BOM / LF 换行
      - 浮点字段 8 位定点十进制，禁科学计数法
    """

    def write(
        self,
        candidates: list[dict],
        output_path: pathlib.Path,
    ) -> None:
        """将 Entry_Candidate 排名写入 markdown 文件。

        Args:
            candidates: 字典列表，每个字典 MUST 包含以下字段：
                - candidate_id: str
                - nogate_calendar_normalized_return_pct: float
                - nogate_per_trade_quality_bps_over_notional: float
                - asymmetry_tag: str (∈ {eth_only_positive, btc_only_positive,
                                        all_symbols_positive, none})
            output_path: 输出 markdown 文件路径。

        Raises:
            KeyError: 当候选字典缺少必需字段时。
        """
        # 按排名规则排序
        sorted_candidates = sorted(candidates, key=_sort_key)

        # 构建 markdown 内容
        lines: list[str] = []

        # 标题
        lines.append("# Entry Candidate Ranking")
        lines.append("")
        lines.append(
            "排名维度：主键 `nogate_calendar_normalized_return_pct` DESC、"
            "副键 `nogate_per_trade_quality_bps_over_notional` DESC、"
            "tie-breaker `candidate_id` 字典序 ASC。"
        )
        lines.append("")

        # 表头
        lines.append(
            "| Rank | candidate_id | nogate_calendar_normalized_return_pct "
            "| nogate_per_trade_quality_bps_over_notional | asymmetry_tag |"
        )
        lines.append(
            "| ---: | :--- | ---: | ---: | :--- |"
        )

        # 数据行
        for rank, candidate in enumerate(sorted_candidates, start=1):
            candidate_id = str(candidate["candidate_id"])
            cal_return = _format_float_8dp(
                float(candidate["nogate_calendar_normalized_return_pct"])
            )
            quality_bps = _format_float_8dp(
                float(candidate["nogate_per_trade_quality_bps_over_notional"])
            )
            asymmetry_tag = str(candidate["asymmetry_tag"])
            lines.append(
                f"| {rank} | {candidate_id} | {cal_return} "
                f"| {quality_bps} | {asymmetry_tag} |"
            )

        # 末尾空行
        lines.append("")

        # 写盘：UTF-8 无 BOM / LF 换行
        content = "\n".join(lines)

        # 确保父目录存在
        output_path.parent.mkdir(parents=True, exist_ok=True)

        # newline="" 确保跨平台 LF 换行
        with open(output_path, "w", encoding="utf-8", newline="") as f:
            f.write(content)
