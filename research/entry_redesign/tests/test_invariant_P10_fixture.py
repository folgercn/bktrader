"""确定性 fixture 测试：No Live Config Output (P10).

**Validates: Requirements 6.10**

Property 10: No Live Config Output
本 spec 的 Research_Harness MUST NOT 生成 live session.config、sleeve multiplier、
dispatch 配置或 control-plane 操作建议。

测试策略（确定性 fixture，非 property-based）：
1. 静态扫描 `research/entry_redesign/` 下所有 .py 文件，检查禁词集合
   （`session.config` / `sleeve multiplier` / `dispatch` / `control-reset`）
   是否出现在写操作上下文中（排除文档注释、测试文件自身、scope_check 模块中的
   合法引用）。
2. 验证 InvariantChecker 对 `live_output_emitted=false` 的 summary 返回
   P10_count=0 且 live_output_emitted=false。
3. 验证 InvariantChecker 对 `live_output_emitted=true` 的 summary 返回
   P10_count=1 且 live_output_emitted=true（CI 非零退出条件）。

Requirements: 6.10
"""

from __future__ import annotations

import os
import re
from pathlib import Path
from typing import Set

import pandas as pd

from research.entry_redesign.invariants.invariant_checker import InvariantChecker


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

# research/entry_redesign/ 根目录
_ENTRY_REDESIGN_ROOT = Path(__file__).resolve().parent.parent

# P10 禁词集合：live config output 相关的写操作关键词
# 这些词如果出现在非注释、非文档引用的写操作代码中，表示违反 P10
_FORBIDDEN_PATTERNS: list[str] = [
    "session.config",
    "sleeve multiplier",
    "dispatch",
    "control-reset",
]

# 允许包含禁词的路径模式（这些文件中的引用是合法的）：
# - tests/ 目录下的测试文件本身（包含禁词作为测试断言）
# - scope_check/ 目录下的扫描器（包含禁词作为扫描目标）
# - __pycache__/ 编译缓存
_ALLOWED_PATH_PATTERNS: list[str] = [
    "tests/",
    "scope_check/",
    "__pycache__/",
]

# 行级别排除模式：注释行、docstring 引用、字符串常量中的文档引用
# 这些行中出现禁词不算违反
_COMMENT_OR_DOC_PATTERNS: list[re.Pattern[str]] = [
    re.compile(r"^\s*#"),           # Python 单行注释
    re.compile(r'^\s*"""'),         # docstring 开始/结束
    re.compile(r"^\s*'''"),         # docstring 开始/结束（单引号）
    re.compile(r"^\s*#.*禁词"),     # 禁词说明注释
    re.compile(r"^\s*#.*forbidden"),  # forbidden words 说明注释
    re.compile(r"^\s*#.*MUST NOT"),   # requirement 引用注释
    re.compile(r"^\s*#.*P10"),        # P10 说明注释
    re.compile(r"^\s*#.*Requirement"),  # requirement 引用注释
]


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------


def _is_allowed_path(rel_path: str) -> bool:
    """判断文件路径是否在允许列表中（这些文件中的禁词引用是合法的）。"""
    for pattern in _ALLOWED_PATH_PATTERNS:
        if pattern in rel_path:
            return True
    return False


def _is_comment_or_doc_line(line: str) -> bool:
    """判断行是否为注释或 docstring 行。"""
    for pattern in _COMMENT_OR_DOC_PATTERNS:
        if pattern.match(line):
            return True
    return False


def _is_in_docstring_block(lines: list[str], line_idx: int) -> bool:
    """判断指定行是否在 docstring 块内。

    简单启发式：统计该行之前的三引号数量，奇数表示在 docstring 内。
    """
    triple_double_count = 0
    triple_single_count = 0
    for i in range(line_idx):
        stripped = lines[i]
        triple_double_count += stripped.count('"""')
        triple_single_count += stripped.count("'''")
    # 如果三引号计数为奇数，说明当前行在 docstring 块内
    return (triple_double_count % 2 == 1) or (triple_single_count % 2 == 1)


def _scan_py_files_for_forbidden_patterns(
    root: Path,
) -> list[tuple[str, int, str, str]]:
    """扫描 root 下所有 .py 文件，查找禁词集合的写操作命中。

    Returns:
        List of (relative_path, line_number, matched_pattern, line_content)
        tuples. Empty list means no violations.
    """
    violations: list[tuple[str, int, str, str]] = []

    for dirpath, _dirnames, filenames in os.walk(root):
        for filename in filenames:
            if not filename.endswith(".py"):
                continue

            filepath = Path(dirpath) / filename
            rel_path = str(filepath.relative_to(root))

            # 跳过允许列表中的路径
            if _is_allowed_path(rel_path):
                continue

            try:
                with open(filepath, "r", encoding="utf-8", errors="replace") as f:
                    lines = f.readlines()
            except OSError:
                continue

            for line_idx, line in enumerate(lines):
                # 跳过注释行和 docstring 行
                if _is_comment_or_doc_line(line):
                    continue

                # 跳过 docstring 块内的行
                if _is_in_docstring_block(lines, line_idx):
                    continue

                # 检查每个禁词
                line_lower = line.lower()
                for pattern in _FORBIDDEN_PATTERNS:
                    if pattern.lower() in line_lower:
                        violations.append((
                            rel_path,
                            line_idx + 1,
                            pattern,
                            line.rstrip("\n\r"),
                        ))

    return violations


def _collect_all_py_files(root: Path) -> Set[str]:
    """收集 root 下所有 .py 文件的相对路径。"""
    py_files: Set[str] = set()
    for dirpath, _dirnames, filenames in os.walk(root):
        for filename in filenames:
            if filename.endswith(".py"):
                filepath = Path(dirpath) / filename
                rel_path = str(filepath.relative_to(root))
                py_files.add(rel_path)
    return py_files


# ---------------------------------------------------------------------------
# Test: 静态扫描禁词
# ---------------------------------------------------------------------------


def test_p10_no_forbidden_live_config_patterns_in_source() -> None:
    """P10: 扫描 research/entry_redesign/ 下所有 .py 源码文件，
    确认禁词集合（session.config / sleeve multiplier / dispatch / control-reset）
    不出现在写操作代码中。

    **Validates: Requirements 6.10**

    排除范围：
    - tests/ 目录（测试文件本身包含禁词作为断言目标）
    - scope_check/ 目录（扫描器包含禁词作为扫描目标）
    - 注释行和 docstring 块中的引用
    """
    violations = _scan_py_files_for_forbidden_patterns(_ENTRY_REDESIGN_ROOT)

    if violations:
        msg_lines = [
            "P10 violated: forbidden live config output patterns found in source code:",
            "",
        ]
        for rel_path, line_no, pattern, content in violations:
            msg_lines.append(
                f"  {rel_path}:{line_no} matched '{pattern}': {content}"
            )
        msg_lines.append("")
        msg_lines.append(
            "These patterns indicate potential live config output generation, "
            "which violates Requirement 6.10 (No Live Config Output)."
        )
        raise AssertionError("\n".join(msg_lines))


# ---------------------------------------------------------------------------
# Test: InvariantChecker 对 live_output_emitted=false 的行为
# ---------------------------------------------------------------------------


def test_p10_invariant_checker_live_output_false() -> None:
    """P10: InvariantChecker 对 live_output_emitted=false 的 summary
    返回 P10_count=0 且 live_output_emitted=false。

    **Validates: Requirements 6.10**
    """
    checker = InvariantChecker()

    # 构造最小合法 summary（live_output_emitted=false）
    summary: dict = {
        "live_output_emitted": False,
        # P9 需要的字段
        "nogate_per_trade_quality_bps_over_notional": None,
        "nogate_trade_count": 0,
        "gate001_per_trade_quality_bps_over_notional": None,
        "gate001_trade_count": 0,
        # P12 需要的字段
        "nogate_active_silo_sum_pct": 0.0,
        "nogate_calendar_normalized_return_pct": 0.0,
        "nogate_active_months": 0,
        "nogate_empty_months": 22,
        "gate001_active_silo_sum_pct": 0.0,
        "gate001_calendar_normalized_return_pct": 0.0,
        "gate001_active_months": 0,
        "gate001_empty_months": 22,
    }

    # 空 ledger
    ledger = pd.DataFrame()

    violations = checker.check(ledger, summary)

    assert violations["P10_count"] == 0, (
        f"P10_count should be 0 when live_output_emitted=false, "
        f"got {violations['P10_count']}"
    )
    assert violations["live_output_emitted"] is False, (
        "live_output_emitted should be False in violations dict "
        "when summary has live_output_emitted=false"
    )
    assert not InvariantChecker.has_violations(violations), (
        "has_violations() should return False when no violations exist"
    )


# ---------------------------------------------------------------------------
# Test: InvariantChecker 对 live_output_emitted=true 的行为
# ---------------------------------------------------------------------------


def test_p10_invariant_checker_live_output_true() -> None:
    """P10: InvariantChecker 对 live_output_emitted=true 的 summary
    返回 P10_count=1 且 live_output_emitted=true（CI 非零退出条件）。

    **Validates: Requirements 6.10**
    """
    checker = InvariantChecker()

    # 构造 summary（live_output_emitted=true → 违反 P10）
    summary: dict = {
        "live_output_emitted": True,
        # P9 需要的字段
        "nogate_per_trade_quality_bps_over_notional": None,
        "nogate_trade_count": 0,
        "gate001_per_trade_quality_bps_over_notional": None,
        "gate001_trade_count": 0,
        # P12 需要的字段
        "nogate_active_silo_sum_pct": 0.0,
        "nogate_calendar_normalized_return_pct": 0.0,
        "nogate_active_months": 0,
        "nogate_empty_months": 22,
        "gate001_active_silo_sum_pct": 0.0,
        "gate001_calendar_normalized_return_pct": 0.0,
        "gate001_active_months": 0,
        "gate001_empty_months": 22,
    }

    # 空 ledger
    ledger = pd.DataFrame()

    violations = checker.check(ledger, summary)

    assert violations["P10_count"] == 1, (
        f"P10_count should be 1 when live_output_emitted=true, "
        f"got {violations['P10_count']}"
    )
    assert violations["live_output_emitted"] is True, (
        "live_output_emitted should be True in violations dict "
        "when summary has live_output_emitted=true"
    )
    assert InvariantChecker.has_violations(violations), (
        "has_violations() should return True when P10 is violated "
        "(live_output_emitted=true triggers CI non-zero exit)"
    )


# ---------------------------------------------------------------------------
# Test: live_output_emitted 字段缺失时默认行为
# ---------------------------------------------------------------------------


def test_p10_invariant_checker_live_output_missing_defaults_false() -> None:
    """P10: 当 summary 中 live_output_emitted 字段缺失时，
    InvariantChecker 应默认视为 false（不违反）。

    **Validates: Requirements 6.10**
    """
    checker = InvariantChecker()

    # 构造 summary（不包含 live_output_emitted 字段）
    summary: dict = {
        # P9 需要的字段
        "nogate_per_trade_quality_bps_over_notional": None,
        "nogate_trade_count": 0,
        "gate001_per_trade_quality_bps_over_notional": None,
        "gate001_trade_count": 0,
        # P12 需要的字段
        "nogate_active_silo_sum_pct": 0.0,
        "nogate_calendar_normalized_return_pct": 0.0,
        "nogate_active_months": 0,
        "nogate_empty_months": 22,
        "gate001_active_silo_sum_pct": 0.0,
        "gate001_calendar_normalized_return_pct": 0.0,
        "gate001_active_months": 0,
        "gate001_empty_months": 22,
    }

    ledger = pd.DataFrame()

    violations = checker.check(ledger, summary)

    assert violations["P10_count"] == 0, (
        f"P10_count should be 0 when live_output_emitted is missing from summary, "
        f"got {violations['P10_count']}"
    )
    assert violations["live_output_emitted"] is False, (
        "live_output_emitted should default to False when missing from summary"
    )


# ---------------------------------------------------------------------------
# Test: 确认 research/entry_redesign/ 下所有 artifact 路径不含 live 配置
# ---------------------------------------------------------------------------


def test_p10_no_live_artifact_paths() -> None:
    """P10: 确认 research/entry_redesign/ 下不存在任何以 live 配置命名的
    artifact 文件（如 session_config.json、dispatch_config.yaml 等）。

    **Validates: Requirements 6.10**

    扫描 research/entry_redesign/ 下所有文件名，确认不存在以下模式：
    - *session_config*
    - *session.config*
    - *dispatch_config*
    - *sleeve_multiplier*
    - *control_reset* (非 runner_aborted 上下文)
    """
    # 禁止的 artifact 文件名模式
    forbidden_artifact_patterns: list[re.Pattern[str]] = [
        re.compile(r"session[_.]config", re.IGNORECASE),
        re.compile(r"dispatch[_.]config", re.IGNORECASE),
        re.compile(r"sleeve[_.]multiplier", re.IGNORECASE),
        re.compile(r"live[_.]session", re.IGNORECASE),
    ]

    violations: list[str] = []

    for dirpath, _dirnames, filenames in os.walk(_ENTRY_REDESIGN_ROOT):
        for filename in filenames:
            # 跳过 __pycache__
            if "__pycache__" in dirpath:
                continue

            for pattern in forbidden_artifact_patterns:
                if pattern.search(filename):
                    filepath = Path(dirpath) / filename
                    rel_path = str(filepath.relative_to(_ENTRY_REDESIGN_ROOT))
                    violations.append(
                        f"  {rel_path} matches forbidden artifact pattern "
                        f"'{pattern.pattern}'"
                    )

    if violations:
        msg_lines = [
            "P10 violated: live config artifact files found in "
            "research/entry_redesign/:",
            "",
        ] + violations + [
            "",
            "The Research_Harness MUST NOT generate live session.config, "
            "sleeve multiplier, dispatch config, or control-plane artifacts.",
        ]
        raise AssertionError("\n".join(msg_lines))


# ---------------------------------------------------------------------------
# Test: 确认 live_output_emitted 在 InvariantChecker schema 中恒存在
# ---------------------------------------------------------------------------


def test_p10_live_output_emitted_always_in_schema() -> None:
    """P10: 确认 InvariantChecker.check() 返回的 violations dict
    中 live_output_emitted 字段恒存在（即使值为 false）。

    **Validates: Requirements 6.10, 6.13**
    """
    checker = InvariantChecker()

    # 测试多种 summary 输入
    test_cases = [
        {"live_output_emitted": False},
        {"live_output_emitted": True},
        {},  # 字段缺失
    ]

    for summary in test_cases:
        # 补充 P9/P12 所需最小字段
        full_summary = {
            "nogate_per_trade_quality_bps_over_notional": None,
            "nogate_trade_count": 0,
            "gate001_per_trade_quality_bps_over_notional": None,
            "gate001_trade_count": 0,
            "nogate_active_silo_sum_pct": 0.0,
            "nogate_calendar_normalized_return_pct": 0.0,
            "nogate_active_months": 0,
            "nogate_empty_months": 22,
            "gate001_active_silo_sum_pct": 0.0,
            "gate001_calendar_normalized_return_pct": 0.0,
            "gate001_active_months": 0,
            "gate001_empty_months": 22,
            **summary,
        }

        ledger = pd.DataFrame()
        violations = checker.check(ledger, full_summary)

        assert "live_output_emitted" in violations, (
            f"live_output_emitted field MUST always be present in "
            f"invariant_violations schema (summary input: {summary})"
        )
        assert "P10_count" in violations, (
            f"P10_count field MUST always be present in "
            f"invariant_violations schema (summary input: {summary})"
        )
