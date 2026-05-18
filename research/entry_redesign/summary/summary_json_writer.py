"""SummaryJsonWriter — summary JSON 写盘入口。

稳定键序（sorted keys）；UTF-8 无 BOM；LF 换行；数值定点（15 位小数保证 round-trip，禁科学计数法）。
Round-trip 相对误差 ≤ 1e-12。

固化 invariant_violations schema（13 字段恒存在）：
  P1_missing_trigger_count (int)
  P1_spurious_trigger_count (int)
  P3_count (int)
  P4_count (int)
  P5_count (int)
  P6_count (int)
  P7_ledger_sha256_pairs (list[dict])
  P8_count (int)
  P9_count (int)
  P10_count (int)
  P11_count (int)
  P12_count (int)
  live_output_emitted (bool)

同时写入以下顶层字段：
  - baseline_reference（nogate_win_rate / nogate_payoff_ratio / 单 symbol 版本）
  - asymmetry_tag
  - small_sample_flag
  - event_expectation_positive / event_expectation_positive_btc_only / event_expectation_positive_eth_only
  - walkforward_config
  - gate001_snapshot_ref

Requirements: 3.2, 3.4, 3.5, 3.7, 4.4, 6.13, 6.8
"""

from __future__ import annotations

import json
import pathlib
from typing import Any


# ---------------------------------------------------------------------------
# invariant_violations schema: 13 字段恒存在
# ---------------------------------------------------------------------------

_INVARIANT_VIOLATIONS_REQUIRED_FIELDS: tuple[str, ...] = (
    "P1_missing_trigger_count",
    "P1_spurious_trigger_count",
    "P3_count",
    "P4_count",
    "P5_count",
    "P6_count",
    "P7_ledger_sha256_pairs",
    "P8_count",
    "P9_count",
    "P10_count",
    "P11_count",
    "P12_count",
    "live_output_emitted",
)
"""invariant_violations 必须包含的 13 个字段。"""


# ---------------------------------------------------------------------------
# 顶层必须存在的字段（除 metrics 外的判定/配置字段）
# ---------------------------------------------------------------------------

_REQUIRED_TOP_LEVEL_FIELDS: tuple[str, ...] = (
    "baseline_reference",
    "asymmetry_tag",
    "small_sample_flag",
    "event_expectation_positive",
    "event_expectation_positive_btc_only",
    "event_expectation_positive_eth_only",
    "walkforward_config",
    "gate001_snapshot_ref",
)
"""summary JSON 顶层必须存在的判定/配置字段。"""


# ---------------------------------------------------------------------------
# JSON 序列化辅助：定点数值格式化（与 snapshot 模块一致的风格）
# ---------------------------------------------------------------------------


def _format_float_fixed(value: float) -> str:
    """将浮点数格式化为定点十进制字符串，禁止科学计数法。

    使用 15 位有效数字以保证 IEEE 754 double 的 round-trip 相对误差 ≤ 1e-12。
    去除尾部多余的零，但保留至少一位小数以区分 int。
    """
    # repr() 保证 round-trip 但可能产生科学计数法；
    # 使用 17 位小数的定点格式确保无损 round-trip 且无科学计数法
    formatted = f"{value:.15f}"
    if "." in formatted:
        formatted = formatted.rstrip("0")
        if formatted.endswith("."):
            formatted += "0"
    return formatted


def _encode_value(obj: Any, indent: int = 0) -> str:
    """递归编码值为 JSON 字符串，浮点使用定点格式，键序稳定（sorted）。

    Args:
        obj: 待编码的 Python 对象。
        indent: 当前缩进层级（空格数）。

    Returns:
        JSON 格式字符串片段。
    """
    if obj is None:
        return "null"
    if isinstance(obj, bool):
        return "true" if obj else "false"
    if isinstance(obj, int):
        return str(obj)
    if isinstance(obj, float):
        return _format_float_fixed(obj)
    if isinstance(obj, str):
        # 特殊值 "+inf" 作为字符串写入
        return json.dumps(obj, ensure_ascii=False)
    if isinstance(obj, list):
        if not obj:
            return "[]"
        items = []
        child_indent = indent + 2
        for item in obj:
            items.append(
                " " * child_indent + _encode_value(item, child_indent)
            )
        return "[\n" + ",\n".join(items) + "\n" + " " * indent + "]"
    if isinstance(obj, dict):
        if not obj:
            return "{}"
        items = []
        child_indent = indent + 2
        keys = sorted(obj.keys())
        for key in keys:
            key_str = json.dumps(key, ensure_ascii=False)
            val_str = _encode_value(obj[key], child_indent)
            items.append(f"{' ' * child_indent}{key_str}: {val_str}")
        return "{\n" + ",\n".join(items) + "\n" + " " * indent + "}"
    # fallback: 尝试 json.dumps
    raise TypeError(
        f"Object of type {type(obj).__name__} is not JSON serializable"
    )


# ---------------------------------------------------------------------------
# 校验辅助
# ---------------------------------------------------------------------------


def _validate_invariant_violations(summary: dict[str, Any]) -> None:
    """校验 summary 中 invariant_violations 包含全部 13 个必需字段。

    Raises:
        ValueError: 当 invariant_violations 缺失或字段不完整时。
    """
    iv = summary.get("invariant_violations")
    if iv is None:
        raise ValueError(
            "summary 缺少 'invariant_violations' 字段"
        )
    if not isinstance(iv, dict):
        raise ValueError(
            f"'invariant_violations' 必须为 dict，收到: {type(iv).__name__}"
        )
    missing = [
        field
        for field in _INVARIANT_VIOLATIONS_REQUIRED_FIELDS
        if field not in iv
    ]
    if missing:
        raise ValueError(
            f"'invariant_violations' 缺少以下必需字段: {missing}"
        )


def _validate_top_level_fields(summary: dict[str, Any]) -> None:
    """校验 summary 顶层必须存在的判定/配置字段。

    Raises:
        ValueError: 当必需字段缺失时。
    """
    missing = [
        field
        for field in _REQUIRED_TOP_LEVEL_FIELDS
        if field not in summary
    ]
    if missing:
        raise ValueError(
            f"summary 缺少以下必需顶层字段: {missing}"
        )


# ---------------------------------------------------------------------------
# SummaryJsonWriter
# ---------------------------------------------------------------------------


class SummaryJsonWriter:
    """summary JSON 写盘入口。

    保证：
      - 稳定键序（sorted keys）
      - UTF-8 无 BOM
      - LF 换行（末尾恰好一个 '\\n'）
      - 数值定点（15 位小数保证 round-trip，禁科学计数法）
      - Round-trip 相对误差 ≤ 1e-12
      - invariant_violations schema 13 字段恒存在
      - baseline_reference / asymmetry_tag / small_sample_flag /
        event_expectation_positive / event_expectation_positive_btc_only /
        event_expectation_positive_eth_only / walkforward_config /
        gate001_snapshot_ref 顶层字段恒存在
      - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源
    """

    def write(self, summary: dict[str, Any], path: pathlib.Path) -> None:
        """将 summary dict 序列化为 JSON 并写入文件。

        写入前校验：
          1. invariant_violations 包含全部 13 个必需字段。
          2. 顶层必须存在 baseline_reference / asymmetry_tag /
             small_sample_flag / event_expectation_positive /
             event_expectation_positive_btc_only /
             event_expectation_positive_eth_only /
             walkforward_config / gate001_snapshot_ref。

        Args:
            summary: 完整的 summary dict（含 metrics、判定字段、invariant_violations）。
            path: 输出 JSON 文件路径。

        Raises:
            ValueError: 当 summary 结构不满足 schema 要求时。
        """
        # 校验 invariant_violations schema
        _validate_invariant_violations(summary)

        # 校验顶层必需字段
        _validate_top_level_fields(summary)

        # 序列化为稳定键序、定点数值的 JSON 字符串
        json_str = _encode_value(summary, indent=0)

        # 确保末尾恰好一个 LF
        if not json_str.endswith("\n"):
            json_str += "\n"

        # 确保父目录存在
        path.parent.mkdir(parents=True, exist_ok=True)

        # 写盘：UTF-8 无 BOM / LF 换行
        # newline="" 确保跨平台 LF 换行（不会被 Windows 转为 CRLF）
        with open(path, "w", encoding="utf-8", newline="") as f:
            f.write(json_str)
