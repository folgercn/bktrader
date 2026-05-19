"""LedgerCsvWriter — Research_Ledger CSV 单一写盘入口。

固定 22 字段 header 顺序（与 Requirement 4.5 完全一致）：
  entry_ts, exit_ts, symbol, side, entry_price, exit_price, notional,
  raw_pnl, slip_pnl, realistic_pnl, realistic_taker_both_pnl, exit_reason,
  entry_candidate_id, gate_mode, signal_bar_start_ts, trigger_ts,
  entry_delay_seconds, feature_horizon_seconds, trigger_confirmation_id,
  entry_price_mode_id, pretouch_state_band_id, posttouch_quality_band_id

约束：
  - 行排序键：trigger_ts ASC, symbol ASC, side ASC, entry_ts ASC（稳定 tie-breaker）
  - 浮点字段 8 位定点十进制，禁科学计数法，禁 NaN/Inf
  - _ts 字段：ISO-8601 UTC ms（2025-06-01T00:00:00.000Z）
  - UTF-8 无 BOM / LF / 无 trailing whitespace / header 行末不带逗号
  - 单一 writer 入口（禁止多路径拼接）
  - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源

Requirements: 4.5, 4.9
"""

from __future__ import annotations

import math
import pathlib
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Literal, Sequence


# ---------------------------------------------------------------------------
# 固定 header 顺序（22 字段，与 Requirement 4.5 完全一致）
# ---------------------------------------------------------------------------

LEDGER_HEADER: tuple[str, ...] = (
    "entry_ts",
    "exit_ts",
    "symbol",
    "side",
    "entry_price",
    "exit_price",
    "notional",
    "raw_pnl",
    "slip_pnl",
    "realistic_pnl",
    "realistic_taker_both_pnl",
    "exit_reason",
    "entry_candidate_id",
    "gate_mode",
    "signal_bar_start_ts",
    "trigger_ts",
    "entry_delay_seconds",
    "feature_horizon_seconds",
    "trigger_confirmation_id",
    "entry_price_mode_id",
    "pretouch_state_band_id",
    "posttouch_quality_band_id",
)
"""固定 22 字段 header 顺序。"""


# ---------------------------------------------------------------------------
# 浮点字段集合（需要 8 位定点格式化）
# ---------------------------------------------------------------------------

_FLOAT_FIELDS: frozenset[str] = frozenset(
    {
        "entry_price",
        "exit_price",
        "notional",
        "raw_pnl",
        "slip_pnl",
        "realistic_pnl",
        "realistic_taker_both_pnl",
    }
)

# ---------------------------------------------------------------------------
# 时间戳字段集合（需要 ISO-8601 UTC ms 格式化）
# ---------------------------------------------------------------------------

_TS_FIELDS: frozenset[str] = frozenset(
    {
        "entry_ts",
        "exit_ts",
        "signal_bar_start_ts",
        "trigger_ts",
    }
)


# ---------------------------------------------------------------------------
# TradeRecord — ledger CSV 每行一条
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class TradeRecord:
    """Research_Ledger CSV 单行记录（22 字段）。

    所有 _ts 字段为 UTC datetime；浮点字段为 Python float。
    """

    entry_ts: datetime
    exit_ts: datetime
    symbol: str
    side: Literal["long", "short"]
    entry_price: float
    exit_price: float
    notional: float
    raw_pnl: float
    slip_pnl: float
    realistic_pnl: float
    realistic_taker_both_pnl: float
    exit_reason: Literal[
        "signal_exit",
        "initial_stop",
        "breakeven_stop",
        "trail_stop",
        "max_hold_timeout",
        "gate_rejected",
        "runner_aborted",
    ]
    entry_candidate_id: str
    gate_mode: Literal["nogate", "gate001"]
    signal_bar_start_ts: datetime
    trigger_ts: datetime
    entry_delay_seconds: int
    feature_horizon_seconds: int
    trigger_confirmation_id: str
    entry_price_mode_id: str
    pretouch_state_band_id: str
    posttouch_quality_band_id: str


# ---------------------------------------------------------------------------
# 格式化辅助函数
# ---------------------------------------------------------------------------


def _format_ts(dt: datetime) -> str:
    """格式化 datetime 为 ISO-8601 UTC 毫秒精度字符串。

    输出形如 '2025-06-01T00:00:00.000Z'。
    要求输入为 UTC 时区或 naive datetime（视为 UTC）。
    """
    # 确保使用 UTC
    if dt.tzinfo is not None:
        dt = dt.astimezone(timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def _format_float(value: float) -> str:
    """格式化浮点数为 8 位定点十进制字符串。

    禁止科学计数法，禁止 NaN/Inf。

    Raises:
        ValueError: 当 value 为 NaN 或 Inf 时。
    """
    if math.isnan(value) or math.isinf(value):
        raise ValueError(
            f"Ledger 浮点字段禁止 NaN/Inf，收到: {value}"
        )
    return f"{value:.8f}"


# ---------------------------------------------------------------------------
# LedgerCsvWriter — 单一 CSV writer 入口
# ---------------------------------------------------------------------------


class LedgerCsvWriter:
    """Research_Ledger CSV 单一写盘入口。

    保证 P7 (Bit_Identical_Ledger) 的确定性约束：
      - 排序键固定：trigger_ts ASC, symbol ASC, side ASC, entry_ts ASC
      - 浮点字段 8 位定点十进制
      - _ts 字段 ISO-8601 UTC ms
      - UTF-8 无 BOM / LF / 无 trailing whitespace / header 行末不带逗号
      - 单一 writer 入口（禁止多路径拼接）
      - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源
    """

    def write(
        self,
        trades: Sequence[TradeRecord],
        out_path: pathlib.Path,
    ) -> None:
        """将 trades 排序后写入 CSV 文件。

        Args:
            trades: TradeRecord 序列（无需预排序）。
            out_path: 输出 CSV 文件路径。

        Raises:
            ValueError: 当浮点字段包含 NaN/Inf 时。
        """
        # 排序：trigger_ts ASC, symbol ASC, side ASC, entry_ts ASC
        sorted_trades = sorted(
            trades,
            key=lambda t: (t.trigger_ts, t.symbol, t.side, t.entry_ts),
        )

        # 构建行列表
        lines: list[str] = []

        # Header 行（无 trailing whitespace，无末尾逗号）
        lines.append(",".join(LEDGER_HEADER))

        # 数据行
        for trade in sorted_trades:
            row_values: list[str] = []
            for field_name in LEDGER_HEADER:
                value = getattr(trade, field_name)
                if field_name in _TS_FIELDS:
                    row_values.append(_format_ts(value))
                elif field_name in _FLOAT_FIELDS:
                    row_values.append(_format_float(value))
                else:
                    row_values.append(str(value))
            lines.append(",".join(row_values))

        # 写盘：UTF-8 无 BOM / LF 换行 / 无 trailing whitespace
        # 文件末尾以 LF 结束（最后一行后有一个换行符）
        content = "\n".join(lines) + "\n"

        # 确保父目录存在
        out_path.parent.mkdir(parents=True, exist_ok=True)

        # 使用 open() 而非 Path.write_text() 以兼容 Python 3.9
        # newline="" 确保跨平台 LF 换行（不会被 Windows 转为 CRLF）
        with open(out_path, "w", encoding="utf-8", newline="") as f:
            f.write(content)
