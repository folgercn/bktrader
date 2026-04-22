#!/usr/bin/env python3
"""Check SQL migration files for common safety issues.

NOTE: This is an **advisory sensor** (warn-only). It scans all historical
migration files and prints warnings, but does NOT block CI. Its purpose is
to surface potential risks for human review, not to enforce a hard gate.

To promote this to a blocking check in the future, change `exit_code = 0`
to `exit_code = 1` in main() when issues are found.

Checks performed:
1. Dangerous DROP TABLE / DROP COLUMN without IF EXISTS
2. ADD COLUMN without IF NOT EXISTS (idempotency)
3. Duplicate migration sequence numbers
4. TRUNCATE or DELETE without WHERE (data wipe risk)
"""

import re
import sys
from pathlib import Path

MIGRATIONS_DIR = Path(__file__).resolve().parents[1] / "db" / "migrations"

# Patterns that are dangerous without guards
DANGEROUS_PATTERNS = [
    (
        r"\bDROP\s+TABLE\b(?!\s+IF\s+EXISTS)",
        "DROP TABLE without IF EXISTS — 生产环境可能因表不存在而失败",
    ),
    (
        r"\bDROP\s+COLUMN\b(?!\s+IF\s+EXISTS)",
        "DROP COLUMN without IF EXISTS — 生产环境可能因列不存在而失败",
    ),
    (
        r"\bADD\s+COLUMN\b(?!\s+IF\s+NOT\s+EXISTS)",
        "ADD COLUMN without IF NOT EXISTS — migration 不幂等，重复执行会报错",
    ),
    (
        r"\bTRUNCATE\s+(?:TABLE\s+)?(?!.*\bIF\b)",
        "TRUNCATE TABLE — 整表清空，确认是否有意为之",
    ),
    (
        r"\bDELETE\s+FROM\b(?!.*\bWHERE\b)",
        "DELETE FROM without WHERE — 全表删除，确认是否有意为之",
    ),
]


def check_dangerous_patterns(filepath: Path) -> list[str]:
    """Check a single SQL file for dangerous patterns."""
    issues = []
    content = filepath.read_text(encoding="utf-8")

    # Only check the +migrate Up section (skip Down)
    up_section = content
    down_marker = re.search(r"--\s*\+migrate\s+Down", content, re.IGNORECASE)
    if down_marker:
        up_section = content[: down_marker.start()]

    for pattern, message in DANGEROUS_PATTERNS:
        matches = list(re.finditer(pattern, up_section, re.IGNORECASE))
        for match in matches:
            line_num = up_section[: match.start()].count("\n") + 1
            issues.append(f"  L{line_num}: {message}")
            issues.append(f"    → {match.group(0).strip()}")

    return issues


def check_duplicate_sequence_numbers(sql_files: list[Path]) -> list[str]:
    """Check for duplicate migration sequence number prefixes."""
    issues = []
    seen: dict[str, list[str]] = {}

    for f in sql_files:
        prefix = f.name.split("_", 1)[0]
        seen.setdefault(prefix, []).append(f.name)

    for prefix, files in sorted(seen.items()):
        if len(files) > 1:
            issues.append(f"  序号 {prefix} 被多个文件使用:")
            for name in files:
                issues.append(f"    → {name}")

    return issues


def main() -> int:
    if not MIGRATIONS_DIR.is_dir():
        print(f"Migration directory not found: {MIGRATIONS_DIR}")
        return 1

    sql_files = sorted(MIGRATIONS_DIR.glob("*.sql"))
    if not sql_files:
        print("No SQL migration files found.")
        return 0

    all_issues: dict[str, list[str]] = {}
    exit_code = 0

    # Check each file for dangerous patterns
    for sql_file in sql_files:
        issues = check_dangerous_patterns(sql_file)
        if issues:
            all_issues[sql_file.name] = issues

    # Check for duplicate sequence numbers
    dup_issues = check_duplicate_sequence_numbers(sql_files)
    if dup_issues:
        all_issues["[序号重复]"] = dup_issues

    if all_issues:
        print("")
        print("🚨 [MIGRATION SAFETY CHECK] 🚨")
        print("")
        for filename, issues in all_issues.items():
            print(f"  {filename}:")
            for issue in issues:
                print(f"  {issue}")
            print("")
        print("请确认以上问题是否为有意行为。如果是新增 migration，请修复后再提交。")
        print("如果是已有的历史 migration，可忽略此警告。")

        # === ADVISORY SENSOR: warn-only, does NOT block CI ===
        # This scans all historical migration files. To evolve into a
        # blocking gate, change to: exit_code = 1 (and ideally scope
        # to only newly-added migration files in the current PR).
        exit_code = 0
        print("")
        print("⚠️  [ADVISORY SENSOR] 当前为警告模式（不阻塞 CI）。")
        print("    此检查仅提示潜在风险供人工 review，不具备强制约束力。")
        print("    对新增 migration 文件请人工确认安全性。")
    else:
        print("✅ Migration safety check passed.")

    return exit_code


if __name__ == "__main__":
    sys.exit(main())
