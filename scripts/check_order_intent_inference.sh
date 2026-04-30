#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
from __future__ import annotations

import re
import sys
import os
from dataclasses import dataclass
from pathlib import Path

ROOT = Path.cwd()

SCAN_ROOTS = [
    ROOT / "internal",
    ROOT / "web" / "console" / "src",
]

SIDE_PATTERN = re.compile(
    r"""(?:\bside\s*(?:==|===|!=|!==)\s*["'](?:BUY|SELL)["'])|(?:["'](?:BUY|SELL)["']\s*(?:==|===|!=|!==)\s*\bside\b)"""
)

EXCLUDED_PARTS = {
    "node_modules",
    "testdata",
}

EXCLUDED_SUFFIXES = (
    "_test.go",
    ".test.ts",
    ".test.tsx",
)

@dataclass(frozen=True)
class AllowlistEntry:
    path: str
    line_pattern: re.Pattern[str]
    count: int
    reason: str


def allowed(path: str, line_pattern: str, count: int, reason: str) -> AllowlistEntry:
    return AllowlistEntry(path=path, line_pattern=re.compile(line_pattern), count=count, reason=reason)


# Existing, reviewed hits. Counts make identical duplicate lines fail too.
ALLOWLIST = [
    # Canonical domain classifier: the only source of order intent semantics.
    allowed("internal/domain/order_intent.go", r'side == "BUY".*!isExit', 1, "canonical classifier"),
    allowed("internal/domain/order_intent.go", r'side == "SELL".*!isExit', 1, "canonical classifier"),
    allowed("internal/domain/order_intent.go", r'side == "SELL".*isExit', 1, "canonical classifier"),
    allowed("internal/domain/order_intent.go", r'side == "BUY".*isExit', 1, "canonical classifier"),

    # Existing service execution side validation/normalization is an execution boundary, not display semantics.
    allowed("internal/service/backtest.go", r'side != "BUY".*side != "SELL".*side != "LONG".*side != "SHORT"', 1, "backtest side validation"),
    allowed("internal/service/live_execution.go", r'side != "BUY".*side != "SELL"', 1, "execution side validation"),
    allowed("internal/service/live_execution.go", r'side == "SELL"', 1, "execution side normalization"),

    # Existing Monitor fallback is tracked by issue #357 and must not grow.
    allowed("web/console/src/utils/derivation.ts", r'side === "BUY"', 1, "order marker buy/sell visual direction"),
]


def is_scanned_source(path: Path) -> bool:
    if any(part in EXCLUDED_PARTS for part in path.parts):
        return False
    if path.name.endswith(EXCLUDED_SUFFIXES):
        return False
    return path.suffix in {".go", ".ts", ".tsx"}


def matches_sensor(rel: str, line: str) -> bool:
    return bool(SIDE_PATTERN.search(line))


if os.environ.get("ORDER_INTENT_SENSOR_TEST") == "1":
    cases = [
        ("web/console/src/example.ts", 'if (order.intent === "CLOSE_LONG") {', False),
        ("web/console/src/example.ts", 'if (intent === "OPEN_LONG" || intent === "CLOSE_LONG") return "LONG";', False),
        ("web/console/src/example.ts", 'if (side === "BUY") {', True),
        ("internal/example.go", 'if side == "SELL" {', True),
    ]
    for rel, line, want in cases:
        got = matches_sensor(rel, line)
        if got != want:
            print(f"self-test failed for {rel}: {line!r}: got {got}, want {want}", file=sys.stderr)
            sys.exit(1)
    print("Order intent inference sensor self-test passed.")
    sys.exit(0)


unexpected_hits: list[tuple[str, int, str]] = []
allowlist_hits = [0 for _ in ALLOWLIST]
allowlist_locations: list[list[str]] = [[] for _ in ALLOWLIST]

for root in SCAN_ROOTS:
    if not root.exists():
        continue
    for path in root.rglob("*"):
        if not path.is_file() or not is_scanned_source(path):
            continue
        rel = path.relative_to(ROOT).as_posix()
        for lineno, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            line = raw_line.strip()
            if not matches_sensor(rel, line):
                continue
            matched = False
            for idx, entry in enumerate(ALLOWLIST):
                if rel == entry.path and entry.line_pattern.search(line):
                    allowlist_hits[idx] += 1
                    allowlist_locations[idx].append(f"{rel}:{lineno}")
                    matched = True
                    break
            if not matched:
                unexpected_hits.append((rel, lineno, line))

failures: list[str] = []

for rel, lineno, line in unexpected_hits:
    failures.append(f"{rel}:{lineno}\n  {line}\n  allowed=0, found=1")

for idx, entry in enumerate(ALLOWLIST):
    found = allowlist_hits[idx]
    if found != entry.count:
        locations = ", ".join(allowlist_locations[idx]) or entry.path
        failures.append(
            f"{locations}\n"
            f"  allowlist entry count mismatch: {entry.line_pattern.pattern}\n"
            f"  reason={entry.reason}, allowed={entry.count}, found={found}"
        )

if failures:
    print("Order intent inference sensor failed.", file=sys.stderr)
    print("", file=sys.stderr)
    print("Do not add a second order semantics classifier from side/reduceOnly.", file=sys.stderr)
    print("Display/replay/analysis semantics must use domain.ClassifyOrderIntent()", file=sys.stderr)
    print("or backend-provided intent / intentLabel.", file=sys.stderr)
    print("", file=sys.stderr)
    print("If this is an execution safety boundary, update the allowlist with a concrete reason.", file=sys.stderr)
    print("", file=sys.stderr)
    for failure in failures:
        print(failure, file=sys.stderr)
        print("", file=sys.stderr)
    sys.exit(1)

print("Order intent inference sensor passed.")
PY
