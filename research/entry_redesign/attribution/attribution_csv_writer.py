"""AttributionCsvWriter — per-symbol-per-month 归因表 CSV 写盘入口。

固定字段顺序：
  symbol, year_month,
  nogate_trade_count, nogate_realistic_pnl_pct,
  nogate_per_trade_quality_bps_over_notional,
  gate001_trade_count, gate001_realistic_pnl_pct,
  gate001_per_trade_quality_bps_over_notional,
  baseline_delta_nogate_realistic_pnl_pct,
  baseline_delta_nogate_per_trade_quality_bps_over_notional,
  entry_effect_bps, gate_effect_bps, sizing_effect_bps,
  layer_dependency, pullback_limit_unfilled_count

约束：
  - 排序键：symbol ASC, year_month ASC
  - 浮点字段 8 位定点十进制，禁科学计数法，禁 NaN/Inf
  - UTF-8 无 BOM / LF / 无 trailing whitespace / header 行末不带逗号
  - sizing_effect_bps 恒为 0.0（sizing 层固定，本 spec 不修改）
  - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源

Requirements: 3.6, 5.1, 5.3
"""

from __future__ import annotations

import math
import pathlib
from typing import Sequence


# ---------------------------------------------------------------------------
# 固定 header 顺序（15 字段）
# ---------------------------------------------------------------------------

ATTRIBUTION_HEADER: tuple[str, ...] = (
    "symbol",
    "year_month",
    "nogate_trade_count",
    "nogate_realistic_pnl_pct",
    "nogate_per_trade_quality_bps_over_notional",
    "gate001_trade_count",
    "gate001_realistic_pnl_pct",
    "gate001_per_trade_quality_bps_over_notional",
    "baseline_delta_nogate_realistic_pnl_pct",
    "baseline_delta_nogate_per_trade_quality_bps_over_notional",
    "entry_effect_bps",
    "gate_effect_bps",
    "sizing_effect_bps",
    "layer_dependency",
    "pullback_limit_unfilled_count",
)
"""固定 15 字段 header 顺序。"""


# ---------------------------------------------------------------------------
# 浮点字段集合（需要 8 位定点格式化）
# ---------------------------------------------------------------------------

_FLOAT_FIELDS: frozenset[str] = frozenset(
    {
        "nogate_realistic_pnl_pct",
        "nogate_per_trade_quality_bps_over_notional",
        "gate001_realistic_pnl_pct",
        "gate001_per_trade_quality_bps_over_notional",
        "baseline_delta_nogate_realistic_pnl_pct",
        "baseline_delta_nogate_per_trade_quality_bps_over_notional",
        "entry_effect_bps",
        "gate_effect_bps",
        "sizing_effect_bps",
    }
)

# ---------------------------------------------------------------------------
# 整数字段集合
# ---------------------------------------------------------------------------

_INT_FIELDS: frozenset[str] = frozenset(
    {
        "nogate_trade_count",
        "gate001_trade_count",
        "pullback_limit_unfilled_count",
    }
)


# ---------------------------------------------------------------------------
# 格式化辅助函数
# ---------------------------------------------------------------------------


def _format_float(value: float) -> str:
    """格式化浮点数为 8 位定点十进制字符串。

    禁止科学计数法，禁止 NaN/Inf。

    Raises:
        ValueError: 当 value 为 NaN 或 Inf 时。
    """
    if math.isnan(value) or math.isinf(value):
        raise ValueError(
            f"Attribution 浮点字段禁止 NaN/Inf，收到: {value}"
        )
    return f"{value:.8f}"


# ---------------------------------------------------------------------------
# AttributionCsvWriter — 归因表 CSV 写盘入口
# ---------------------------------------------------------------------------


class AttributionCsvWriter:
    """per-symbol-per-month 归因表 CSV 写盘入口。

    保证确定性约束：
      - 排序键固定：symbol ASC, year_month ASC
      - 浮点字段 8 位定点十进制
      - UTF-8 无 BOM / LF / 无 trailing whitespace / header 行末不带逗号
      - sizing_effect_bps 恒为 0.0
      - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源
    """

    def write(
        self,
        rows: list[dict],
        path: pathlib.Path,
    ) -> None:
        """将归因行排序后写入 CSV 文件。

        Args:
            rows: 字典列表，每个字典包含 ATTRIBUTION_HEADER 中的所有字段。
            path: 输出 CSV 文件路径。

        Raises:
            ValueError: 当浮点字段包含 NaN/Inf 时。
            KeyError: 当行缺少必需字段时。
        """
        # 排序：symbol ASC, year_month ASC
        sorted_rows = sorted(
            rows,
            key=lambda r: (r["symbol"], r["year_month"]),
        )

        # 构建行列表
        lines: list[str] = []

        # Header 行（无 trailing whitespace，无末尾逗号）
        lines.append(",".join(ATTRIBUTION_HEADER))

        # 数据行
        for row in sorted_rows:
            row_values: list[str] = []
            for field_name in ATTRIBUTION_HEADER:
                value = row[field_name]
                if field_name in _FLOAT_FIELDS:
                    row_values.append(_format_float(float(value)))
                elif field_name in _INT_FIELDS:
                    row_values.append(str(int(value)))
                else:
                    # 字符串字段：symbol, year_month, layer_dependency
                    row_values.append(str(value))
            lines.append(",".join(row_values))

        # 写盘：UTF-8 无 BOM / LF 换行 / 无 trailing whitespace
        # 文件末尾以 LF 结束（最后一行后有一个换行符）
        content = "\n".join(lines) + "\n"

        # 确保父目录存在
        path.parent.mkdir(parents=True, exist_ok=True)

        # newline="" 确保跨平台 LF 换行（不会被 Windows 转为 CRLF）
        with open(path, "w", encoding="utf-8", newline="") as f:
            f.write(content)
