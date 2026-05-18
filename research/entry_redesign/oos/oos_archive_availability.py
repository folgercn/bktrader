"""OosArchiveAvailabilityWriter — R2 非重叠 OOS 窗口 archive 可用性查询与写盘。

非重叠 OOS 窗口: 2024-01 ~ 2025-05（17 个月）。
MUST 不与 2025-06 ~ 2026-04 calendar 重叠。

当 1s tick archive 不可用时，写出:
  research/tmp_entry_redesign_<candidate_id>_oos_archive_availability.json

字段:
  - query_command: 用于查 archive 的字面命令 (str)
  - query_timestamp_utc: ISO-8601 UTC 时间戳 (str)
  - archive_root: archive 根目录路径 (str)
  - missing_symbol_months[]: 缺失的 (symbol, year_month) 列表
  - evidence_snippet: 命令输出摘录 (str)

如果所有 archive 均存在，则不创建该文件（无空文件占位）。

Requirements: 5.6
"""

from __future__ import annotations

import json
import pathlib
from typing import Literal


# ---------------------------------------------------------------------------
# OOS 窗口常量
# ---------------------------------------------------------------------------

# 非重叠 OOS 窗口: 2024-01 ~ 2025-05（17 个月）
# MUST 不与 2025-06 ~ 2026-04 calendar 重叠
OOS_WINDOW_START_YEAR: int = 2024
OOS_WINDOW_START_MONTH: int = 1
OOS_WINDOW_END_YEAR: int = 2025
OOS_WINDOW_END_MONTH: int = 5

# 目标 symbols
OOS_SYMBOLS: list[str] = ["BTCUSDT", "ETHUSDT"]


def _generate_oos_year_months() -> list[str]:
    """生成 OOS 窗口内所有 year_month 字符串 (YYYY-MM 格式)。

    覆盖 2024-01 ~ 2025-05，共 17 个月。
    """
    months: list[str] = []
    year = OOS_WINDOW_START_YEAR
    month = OOS_WINDOW_START_MONTH
    while (year, month) <= (OOS_WINDOW_END_YEAR, OOS_WINDOW_END_MONTH):
        months.append(f"{year:04d}-{month:02d}")
        month += 1
        if month > 12:
            month = 1
            year += 1
    return months


OOS_YEAR_MONTHS: list[str] = _generate_oos_year_months()

# 验证: 17 个月
assert len(OOS_YEAR_MONTHS) == 17, (
    f"OOS window must be 17 months, got {len(OOS_YEAR_MONTHS)}"
)

# 验证: 不与 2025-06 ~ 2026-04 calendar 重叠
_CALENDAR_START = "2025-06"
assert all(ym < _CALENDAR_START for ym in OOS_YEAR_MONTHS), (
    "OOS window MUST NOT overlap with 2025-06 ~ 2026-04 calendar"
)


# ---------------------------------------------------------------------------
# OosArchiveAvailabilityWriter
# ---------------------------------------------------------------------------


class OosArchiveAvailabilityWriter:
    """R2 非重叠 OOS 窗口 1s tick archive 可用性检查与写盘。

    检查 archive_root 下是否存在每个 (symbol, year_month) 组合的
    1s tick archive 文件。如果有任何缺失，写出 JSON 报告。

    OOS 窗口: 2024-01 ~ 2025-05（17 个月），symbols: ["BTCUSDT", "ETHUSDT"]。
    MUST 不与 2025-06 ~ 2026-04 calendar 重叠。

    如果所有 archive 均存在，则不创建文件（无空文件占位）。

    JSON 写盘要求:
      - 稳定键序（按字段声明顺序固定）
      - UTF-8 无 BOM
      - LF 换行
      - 禁止 datetime.now() / os.getpid()
    """

    def __init__(self, output_dir: pathlib.Path) -> None:
        """初始化 writer。

        Args:
            output_dir: 输出目录路径（通常为 research/ 根目录）。
        """
        self._output_dir = output_dir

    def check_and_write(
        self,
        candidate_id: str,
        archive_root: str,
        query_timestamp_utc: str,
        output_dir: pathlib.Path | None = None,
    ) -> pathlib.Path | None:
        """检查 OOS 窗口 1s tick archive 可用性，缺失时写 JSON。

        对每个 (symbol, year_month) 组合检查 archive 文件是否存在。
        archive 文件路径模式: {archive_root}/{symbol}/{year_month}/
        （目录存在且非空即视为可用）

        Args:
            candidate_id: Entry_Candidate 的唯一标识，满足正则
                ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$。
            archive_root: archive 根目录路径字符串。
            query_timestamp_utc: ISO-8601 UTC 时间戳字符串
                （形如 "2025-06-01T00:00:00.000Z"）。
                由调用方传入，MUST NOT 在本方法内通过 datetime.now() 生成。
            output_dir: 可选的输出目录覆盖。默认使用构造时传入的 output_dir。

        Returns:
            如果有缺失 archive，返回写入文件的路径。
            如果所有 archive 均存在，返回 None（不创建文件）。
        """
        effective_output_dir = output_dir if output_dir is not None else self._output_dir
        archive_path = pathlib.Path(archive_root)

        # 检查每个 (symbol, year_month) 组合
        missing_symbol_months: list[dict[str, str]] = []
        for symbol in OOS_SYMBOLS:
            for year_month in OOS_YEAR_MONTHS:
                symbol_month_dir = archive_path / symbol / year_month
                if not symbol_month_dir.exists() or not any(
                    symbol_month_dir.iterdir()
                    if symbol_month_dir.is_dir()
                    else iter([])
                ):
                    missing_symbol_months.append(
                        {"symbol": symbol, "year_month": year_month}
                    )

        # 如果所有 archive 均存在，不创建文件
        if not missing_symbol_months:
            return None

        # 构建 query_command（用于查 archive 的字面命令）
        query_command = (
            f"ls -la {archive_root}/{{BTCUSDT,ETHUSDT}}/{{2024-01..2025-05}}/"
        )

        # 构建 evidence_snippet
        missing_count = len(missing_symbol_months)
        total_count = len(OOS_SYMBOLS) * len(OOS_YEAR_MONTHS)
        first_missing = missing_symbol_months[0]
        evidence_snippet = (
            f"{missing_count}/{total_count} symbol-month archives missing. "
            f"First missing: {first_missing['symbol']}/{first_missing['year_month']}. "
            f"Archive root: {archive_root}"
        )

        # 构建 JSON 数据（固定键序）
        data: dict = {
            "query_command": query_command,
            "query_timestamp_utc": query_timestamp_utc,
            "archive_root": archive_root,
            "missing_symbol_months": missing_symbol_months,
            "evidence_snippet": evidence_snippet,
        }

        # 输出路径
        filename = (
            f"tmp_entry_redesign_{candidate_id}_oos_archive_availability.json"
        )
        out_path = effective_output_dir / filename

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
