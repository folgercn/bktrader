"""Approval 字面格式检查。

正则匹配 `AGENTS §3 high-risk approval: <人类用户名> <YYYY-MM-DD>`
（日期 `^\d{4}-\d{2}-\d{2}$`）。

PR 描述缺失该字面值且 PR diff 命中 Requirement 7.1 (a)(b)(c)(d) 任一边界
→ 拒绝合入。

Requirements: 1.7, 7.1, 7.12
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Optional


# ---------------------------------------------------------------------------
# Requirement 7.1 高风险禁区边界 (a)(b)(c)(d)
# ---------------------------------------------------------------------------

# (a) 写操作 / (b) 只读分析产出物 / (c) 只读文档更新 / (d) PR diff 路径 hunk
# 检测 PR diff 中是否命中以下高风险路径
HIGH_RISK_PATH_PATTERNS: list[re.Pattern[str]] = [
    re.compile(r"internal/service/live[^/]*\.go"),
    re.compile(r"internal/service/execution_strategy\.go"),
    re.compile(r"deployments/"),
    re.compile(r"\.github/workflows/"),
]


# ---------------------------------------------------------------------------
# Approval 字面格式正则
# ---------------------------------------------------------------------------

# 匹配 `AGENTS §3 high-risk approval: <username> <YYYY-MM-DD>`
# username: 非空白字符序列
# date: 严格 YYYY-MM-DD 格式
APPROVAL_PATTERN: re.Pattern[str] = re.compile(
    r"AGENTS §3 high-risk approval: (\S+) (\d{4}-\d{2}-\d{2})"
)

# 日期部分的独立校验正则（确保完整匹配）
_DATE_PATTERN: re.Pattern[str] = re.compile(r"^\d{4}-\d{2}-\d{2}$")


# ---------------------------------------------------------------------------
# 结果数据类
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ApprovalCheckResult:
    """Approval 检查结果。

    Attributes:
        approved: 是否通过检查（True = 通过，False = 拒绝）。
        missing_approval: PR 描述中是否缺失 approval 字面值。
        approval_text: 匹配到的 approval 字面值全文（如有）。
    """

    approved: bool
    missing_approval: bool
    approval_text: Optional[str]


# ---------------------------------------------------------------------------
# ApprovalLiteralChecker
# ---------------------------------------------------------------------------


class ApprovalLiteralChecker:
    """检查 PR 描述中是否包含合规的 AGENTS §3 high-risk approval 字面值。

    逻辑：
      1. 如果 PR diff 未命中 Requirement 7.1 (a)(b)(c)(d) 任一高风险边界
         → 无论 approval 是否存在，均通过。
      2. 如果 PR diff 命中高风险边界，且 PR 描述包含合规 approval 字面值
         → 通过。
      3. 如果 PR diff 命中高风险边界，但 PR 描述缺失 approval 字面值
         → 拒绝。

    Usage:
        checker = ApprovalLiteralChecker()
        result = checker.check(pr_description, has_scope_violations=True)
        if not result.approved:
            # 拒绝合入
            ...
    """

    def check(
        self,
        pr_description: str,
        has_scope_violations: bool,
    ) -> ApprovalCheckResult:
        """执行 approval 字面格式检查。

        Args:
            pr_description: PR 描述全文。
            has_scope_violations: PR diff 是否命中 Requirement 7.1
                (a)(b)(c)(d) 任一高风险边界。由外部调用方（如
                out_of_scope_paths.py）预先判定后传入。

        Returns:
            ApprovalCheckResult 包含检查结论。
        """
        # 尝试从 PR 描述中提取 approval 字面值
        approval_text = self._extract_approval(pr_description)

        # 如果没有 scope violations → 无论 approval 是否存在均通过
        if not has_scope_violations:
            return ApprovalCheckResult(
                approved=True,
                missing_approval=approval_text is None,
                approval_text=approval_text,
            )

        # 有 scope violations 且缺失 approval → 拒绝
        if approval_text is None:
            return ApprovalCheckResult(
                approved=False,
                missing_approval=True,
                approval_text=None,
            )

        # 有 scope violations 但 approval 存在 → 通过
        return ApprovalCheckResult(
            approved=True,
            missing_approval=False,
            approval_text=approval_text,
        )

    def _extract_approval(self, text: str) -> Optional[str]:
        """从文本中提取第一个合规的 approval 字面值。

        合规条件：
          - 匹配 `AGENTS §3 high-risk approval: <username> <YYYY-MM-DD>`
          - username 为非空白字符序列
          - date 严格匹配 `^\d{4}-\d{2}-\d{2}$`

        Args:
            text: 待扫描文本（通常为 PR 描述）。

        Returns:
            匹配到的完整 approval 字面值字符串，或 None。
        """
        match = APPROVAL_PATTERN.search(text)
        if match is None:
            return None

        # 二次校验日期格式（虽然正则已保证，但显式验证更安全）
        date_str = match.group(2)
        if not _DATE_PATTERN.match(date_str):
            return None

        return match.group(0)

    @staticmethod
    def has_high_risk_path_hits(pr_diff: str) -> bool:
        """检查 PR diff 是否命中 Requirement 7.1 (a)(b)(c)(d) 任一高风险路径。

        此为辅助方法，供外部调用方在不依赖 out_of_scope_paths.py 时
        独立判定 `has_scope_violations` 参数。

        Args:
            pr_diff: PR diff 全文。

        Returns:
            True 如果命中任一高风险路径模式。
        """
        for pattern in HIGH_RISK_PATH_PATTERNS:
            if pattern.search(pr_diff):
                return True
        return False
