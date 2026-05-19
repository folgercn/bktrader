"""越权路径扫描器 — 检测 PR diff 中是否出现 AGENTS §3 高风险禁区路径。

扫描 PR diff 中是否出现以下路径 hunk：
- internal/service/live*.go
- internal/service/execution_strategy.go
- deployments/
- .github/workflows/
- cmd/
- web/

唯一豁免：反引号字面量（最长 120 字符、不含代码片段与行号）作为"禁区标识"。

Requirements: 4.6, 7.1, 7.11
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import List


@dataclass(frozen=True)
class PathViolation:
    """越权路径违反记录。"""

    file_path: str
    pattern_matched: str
    line_in_diff: int


# 禁止路径模式列表 — 每项为 (compiled regex, human-readable pattern description)
FORBIDDEN_PATH_PATTERNS: List[tuple[re.Pattern[str], str]] = [
    (
        re.compile(r"internal/service/live[^/]*\.go"),
        "internal/service/live*.go",
    ),
    (
        re.compile(r"internal/service/execution_strategy\.go"),
        "internal/service/execution_strategy.go",
    ),
    (
        re.compile(r"deployments/"),
        "deployments/",
    ),
    (
        re.compile(r"\.github/workflows/"),
        ".github/workflows/",
    ),
    (
        re.compile(r"cmd/"),
        "cmd/",
    ),
    (
        re.compile(r"web/"),
        "web/",
    ),
]

# 匹配 diff hunk header: --- a/path 或 +++ b/path
_DIFF_FILE_HEADER_RE = re.compile(r"^(?:\+\+\+|---)\s+[ab]/(.+)$")

# 匹配反引号字面量豁免：`...` 内容最长 120 字符、不含代码片段与行号
# 代码片段特征：含 func / if / for / return / := / -> / import / package 等关键字
# 行号特征：含 :数字 或 L数字 模式
_BACKTICK_LITERAL_RE = re.compile(r"`([^`]{1,120})`")
_CODE_SNIPPET_RE = re.compile(
    r"(?:func\s|if\s|for\s|return\s|:=|->|import\s|package\s|"
    r"class\s|def\s|var\s|const\s|let\s|struct\s|\{|\}|;\s*$)"
)
_LINE_NUMBER_RE = re.compile(r"(?::\d+|L\d+|line\s+\d+)", re.IGNORECASE)


class OutOfScopePathsScanner:
    """扫描 PR diff 文本中的越权路径。

    检测 diff 中是否出现 AGENTS §3 高风险禁区路径的 hunk，
    唯一豁免为反引号字面量（最长 120 字符、不含代码片段与行号）。
    """

    def __init__(self) -> None:
        self._patterns = FORBIDDEN_PATH_PATTERNS

    def scan_diff(self, diff_text: str) -> List[PathViolation]:
        """扫描 diff 文本，返回所有越权路径违反。

        Args:
            diff_text: 完整的 PR diff 文本（unified diff 格式）。

        Returns:
            PathViolation 列表，每项记录一个违反的文件路径、
            匹配的模式、以及在 diff 中的行号。
        """
        violations: List[PathViolation] = []
        lines = diff_text.splitlines()

        for line_idx, line in enumerate(lines, start=1):
            # 检查 diff file header (--- a/path, +++ b/path)
            header_match = _DIFF_FILE_HEADER_RE.match(line)
            if header_match:
                file_path = header_match.group(1)
                for pattern, description in self._patterns:
                    if pattern.search(file_path):
                        violations.append(
                            PathViolation(
                                file_path=file_path,
                                pattern_matched=description,
                                line_in_diff=line_idx,
                            )
                        )
                        break  # 一行只报一个 pattern
                continue

            # 对非 header 行，检查是否包含禁止路径
            # 先移除豁免的反引号字面量
            cleaned_line = self._remove_exempt_backtick_literals(line)

            # 在清理后的行中检查禁止路径
            for pattern, description in self._patterns:
                if pattern.search(cleaned_line):
                    violations.append(
                        PathViolation(
                            file_path=self._extract_path_from_line(
                                line, pattern
                            ),
                            pattern_matched=description,
                            line_in_diff=line_idx,
                        )
                    )
                    break  # 一行只报一个 pattern

        return violations

    def _remove_exempt_backtick_literals(self, line: str) -> str:
        """移除行中符合豁免条件的反引号字面量。

        豁免条件：
        - 反引号内容最长 120 字符
        - 不含代码片段（func/if/for/return/:=/->等关键字或大括号/分号）
        - 不含行号（:数字 / L数字 / line 数字）
        """

        def _is_exempt(match: re.Match[str]) -> str:
            content = match.group(1)
            # 长度已由正则限制 <= 120
            # 检查是否含代码片段
            if _CODE_SNIPPET_RE.search(content):
                return match.group(0)  # 不豁免，保留原文
            # 检查是否含行号
            if _LINE_NUMBER_RE.search(content):
                return match.group(0)  # 不豁免，保留原文
            # 符合豁免条件，替换为空
            return ""

        return _BACKTICK_LITERAL_RE.sub(_is_exempt, line)

    def _extract_path_from_line(
        self, line: str, pattern: re.Pattern[str]
    ) -> str:
        """从行中提取匹配的路径片段。"""
        match = pattern.search(line)
        if match:
            return match.group(0)
        return ""
