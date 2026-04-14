#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


COMMENT_BLOCK_RE = re.compile(r"/\*.*?\*/", re.S)
COMMENT_LINE_RE = re.compile(r"--[^\n]*")

CRITICAL_PATTERNS = (
    ("drop_table_like", re.compile(r"\bdrop\s+(table|schema|database)\b", re.I)),
    ("truncate", re.compile(r"\btruncate\b", re.I)),
    ("lock_table", re.compile(r"\block\s+table\b", re.I)),
    ("alter_drop_column", re.compile(r"\balter\s+table\b.*\bdrop\s+column\b", re.I | re.S)),
)

DELETE_RE = re.compile(r"^\s*delete\s+from\b", re.I | re.S)
UPDATE_RE = re.compile(r"^\s*update\b", re.I | re.S)
WHERE_RE = re.compile(r"\bwhere\b", re.I)


def strip_sql_comments(text: str) -> str:
    text = COMMENT_BLOCK_RE.sub("", text)
    return COMMENT_LINE_RE.sub("", text)


def statement_line(text: str, start: int) -> int:
    return text.count("\n", 0, start) + 1


def split_statements(text: str) -> list[tuple[int, str]]:
    statements: list[tuple[int, str]] = []
    start = 0
    for idx, char in enumerate(text):
        if char != ";":
            continue
        stmt = text[start:idx].strip()
        if stmt:
            statements.append((statement_line(text, start), stmt))
        start = idx + 1
    tail = text[start:].strip()
    if tail:
        statements.append((statement_line(text, start), tail))
    return statements


def find_issues(path: Path) -> list[str]:
    raw = path.read_text(encoding="utf-8")
    text = strip_sql_comments(raw)
    issues: list[str] = []

    for line_no, statement in split_statements(text):
        for name, pattern in CRITICAL_PATTERNS:
            if pattern.search(statement):
                issues.append(f"{path}:{line_no}: suspicious migration pattern: {name}")

        if DELETE_RE.search(statement) and not WHERE_RE.search(statement):
            issues.append(f"{path}:{line_no}: DELETE statement without WHERE")

        if UPDATE_RE.search(statement) and not WHERE_RE.search(statement):
            issues.append(f"{path}:{line_no}: UPDATE statement without WHERE")

    return issues


def collect_files(args: argparse.Namespace) -> list[Path]:
    if args.files:
        files = [Path(item) for item in args.files]
    else:
        files = sorted(Path("db/migrations").glob("*.sql"))
    return [path for path in files if path.exists() and path.suffix == ".sql"]


def main() -> int:
    parser = argparse.ArgumentParser(description="Check migration files for obvious unsafe SQL patterns.")
    parser.add_argument("files", nargs="*", help="Optional migration files to inspect.")
    args = parser.parse_args()

    files = collect_files(args)
    if not files:
        print("No migration files to inspect.")
        return 0

    findings: list[str] = []
    for path in files:
        findings.extend(find_issues(path))

    if findings:
        print("Migration safety check failed:")
        for item in findings:
            print(f"- {item}")
        print("")
        print("Review the flagged statements carefully before merging.")
        return 1

    print(f"Migration safety check passed for {len(files)} file(s).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
