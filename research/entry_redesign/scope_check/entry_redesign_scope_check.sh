#!/usr/bin/env bash
# entry_redesign_scope_check.sh — CI wrapper for entry_redesign scope checks.
#
# Calls:
#   16.1 forbidden_words.py — 禁词 ripgrep 扫描
#   16.2 out_of_scope_paths.py — 越权路径扫描
#   16.3 approval_literal.py — approval 字面格式检查
#
# Additionally executes Requirement 5.4 / 5.7 ripgrep commands directly.
#
# Exit codes:
#   0 — all checks pass (or infrastructure failure with warning)
#   1 — at least one check failed (scope violation detected)
#
# Infrastructure failure (python not found, rg not found, etc.) does NOT block
# the pipeline (exit 0) but echoes a warning for PR comment auto-trace
# (Requirement 7.11).
#
# Environment variables:
#   PR_DIFF_FILE          — path to file containing PR diff (optional)
#   PR_DESCRIPTION_FILE   — path to file containing PR description (optional)
#
# If PR_DIFF_FILE is not set and stdin is not a terminal, diff is read from stdin.
#
# Requirements: 5.4, 5.7, 7.11, 7.12

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
SPEC_DIR="${REPO_ROOT}/.kiro/specs/original-t2-entry-logic-redesign"

REQUIREMENTS_MD="${SPEC_DIR}/requirements.md"
DESIGN_MD="${SPEC_DIR}/design.md"
TASKS_MD="${SPEC_DIR}/tasks.md"

# PR diff: read from $PR_DIFF_FILE env var, or stdin, or empty
PR_DIFF_FILE="${PR_DIFF_FILE:-}"
# PR description: read from $PR_DESCRIPTION_FILE env var, or empty
PR_DESCRIPTION_FILE="${PR_DESCRIPTION_FILE:-}"

# Track overall result
FAILED=0

# ---------------------------------------------------------------------------
# Helper: infrastructure failure handler
# Per Requirement 7.11 — does not block merge, but leaves trace.
# ---------------------------------------------------------------------------

infra_failure() {
    local reason="$1"
    echo ""
    echo "::warning::INFRASTRUCTURE FAILURE: ${reason}"
    echo ""
    echo "⚠️  Infrastructure failure detected — not blocking merge per Requirement 7.11."
    echo "    Reason: ${reason}"
    echo "    This MUST be re-run successfully before the next PR merge."
    echo "    Reference: AGENTS §2 Strategy Semantic Sources"
    echo ""
    exit 0
}

# ---------------------------------------------------------------------------
# Locate Python interpreter
# ---------------------------------------------------------------------------

PYTHON=""
if command -v python3 &>/dev/null; then
    PYTHON="python3"
elif command -v python &>/dev/null; then
    PYTHON="python"
else
    infra_failure "Python interpreter not found (neither python3 nor python available)"
fi

# Verify Python can import the scope_check package
if ! "${PYTHON}" -c "import sys; sys.path.insert(0, '${REPO_ROOT}'); import research.entry_redesign.scope_check" 2>/dev/null; then
    infra_failure "Cannot import research.entry_redesign.scope_check package"
fi

# ---------------------------------------------------------------------------
# Locate ripgrep (rg) — used for Requirement 5.4 direct scan
# ---------------------------------------------------------------------------

RG=""
if command -v rg &>/dev/null; then
    RG="rg"
fi

# ---------------------------------------------------------------------------
# Prepare PR diff content
# ---------------------------------------------------------------------------

DIFF_TMPFILE="$(mktemp)"
trap 'rm -f "${DIFF_TMPFILE}"' EXIT

if [[ -n "${PR_DIFF_FILE}" && -f "${PR_DIFF_FILE}" ]]; then
    cp "${PR_DIFF_FILE}" "${DIFF_TMPFILE}"
elif [[ ! -t 0 ]]; then
    cat > "${DIFF_TMPFILE}"
else
    # No diff provided — create empty file
    : > "${DIFF_TMPFILE}"
fi

DIFF_SIZE=$(wc -c < "${DIFF_TMPFILE}" | tr -d ' ')

# ---------------------------------------------------------------------------
# Prepare PR description
# ---------------------------------------------------------------------------

PR_DESC_TMPFILE="$(mktemp)"
trap 'rm -f "${DIFF_TMPFILE}" "${PR_DESC_TMPFILE}"' EXIT

if [[ -n "${PR_DESCRIPTION_FILE}" && -f "${PR_DESCRIPTION_FILE}" ]]; then
    cp "${PR_DESCRIPTION_FILE}" "${PR_DESC_TMPFILE}"
else
    : > "${PR_DESC_TMPFILE}"
fi

# ---------------------------------------------------------------------------
# Check 1: forbidden_words.py (Task 16.1)
# ---------------------------------------------------------------------------

echo "═══════════════════════════════════════════════════════════════"
echo "  [1/4] Running forbidden_words.py scanner..."
echo "═══════════════════════════════════════════════════════════════"

# Build file list for scanning
SCAN_FILES_JSON="["
FIRST=1
for f in "${REQUIREMENTS_MD}" "${DESIGN_MD}" "${TASKS_MD}"; do
    if [[ -f "$f" ]]; then
        [[ ${FIRST} -eq 0 ]] && SCAN_FILES_JSON+=","
        SCAN_FILES_JSON+="\"${f}\""
        FIRST=0
    fi
done
if [[ ${DIFF_SIZE} -gt 0 ]]; then
    [[ ${FIRST} -eq 0 ]] && SCAN_FILES_JSON+=","
    SCAN_FILES_JSON+="\"${DIFF_TMPFILE}\""
fi
SCAN_FILES_JSON+="]"

if ! FORBIDDEN_RESULT=$("${PYTHON}" << PYEOF
import sys, json
sys.path.insert(0, '${REPO_ROOT}')
from research.entry_redesign.scope_check.forbidden_words import ForbiddenWordsScanner

scanner = ForbiddenWordsScanner()
files = json.loads('${SCAN_FILES_JSON}')
hits = scanner.scan(files)
if hits:
    for h in hits:
        print(f'  \u274c {h.file_path}:{h.line_number}: matched [{h.matched_word}]')
    sys.exit(1)
else:
    print('  \u2705 No forbidden words found.')
    sys.exit(0)
PYEOF
2>&1); then
    echo "${FORBIDDEN_RESULT}"
    echo ""
    echo "  → forbidden_words check FAILED"
    FAILED=1
else
    echo "${FORBIDDEN_RESULT}"
fi

echo ""

# ---------------------------------------------------------------------------
# Check 2: out_of_scope_paths.py (Task 16.2)
# ---------------------------------------------------------------------------

echo "═══════════════════════════════════════════════════════════════"
echo "  [2/4] Running out_of_scope_paths.py scanner..."
echo "═══════════════════════════════════════════════════════════════"

if [[ ${DIFF_SIZE} -gt 0 ]]; then
    if ! SCOPE_RESULT=$("${PYTHON}" << PYEOF
import sys
sys.path.insert(0, '${REPO_ROOT}')
from research.entry_redesign.scope_check.out_of_scope_paths import OutOfScopePathsScanner

with open('${DIFF_TMPFILE}', 'r', encoding='utf-8') as f:
    diff_text = f.read()

scanner = OutOfScopePathsScanner()
violations = scanner.scan_diff(diff_text)
if violations:
    for v in violations:
        print(f'  \u274c Line {v.line_in_diff}: [{v.pattern_matched}] matched in {v.file_path}')
    sys.exit(1)
else:
    print('  \u2705 No out-of-scope path violations found.')
    sys.exit(0)
PYEOF
    2>&1); then
        echo "${SCOPE_RESULT}"
        echo ""
        echo "  → out_of_scope_paths check FAILED"
        FAILED=1
    else
        echo "${SCOPE_RESULT}"
    fi
else
    echo "  ⚠️  No PR diff provided — skipping out_of_scope_paths check."
fi

echo ""

# ---------------------------------------------------------------------------
# Check 3: approval_literal.py (Task 16.3)
# ---------------------------------------------------------------------------

echo "═══════════════════════════════════════════════════════════════"
echo "  [3/4] Running approval_literal.py checker..."
echo "═══════════════════════════════════════════════════════════════"

if ! APPROVAL_RESULT=$("${PYTHON}" << PYEOF
import sys
sys.path.insert(0, '${REPO_ROOT}')
from research.entry_redesign.scope_check.approval_literal import ApprovalLiteralChecker
from research.entry_redesign.scope_check.out_of_scope_paths import OutOfScopePathsScanner

with open('${PR_DESC_TMPFILE}', 'r', encoding='utf-8') as f:
    pr_description = f.read()

with open('${DIFF_TMPFILE}', 'r', encoding='utf-8') as f:
    pr_diff = f.read()

# Determine if there are scope violations in the diff
scanner = OutOfScopePathsScanner()
has_violations = len(scanner.scan_diff(pr_diff)) > 0

checker = ApprovalLiteralChecker()
result = checker.check(pr_description, has_scope_violations=has_violations)

if not result.approved:
    print('  \u274c PR diff touches high-risk paths but PR description is missing approval literal.')
    print('     Required format: AGENTS \u00a73 high-risk approval: <username> <YYYY-MM-DD>')
    sys.exit(1)
else:
    if result.approval_text:
        print(f'  \u2705 Approval found: {result.approval_text}')
    else:
        print('  \u2705 No high-risk paths touched \u2014 approval not required.')
    sys.exit(0)
PYEOF
2>&1); then
    echo "${APPROVAL_RESULT}"
    echo ""
    echo "  → approval_literal check FAILED"
    FAILED=1
else
    echo "${APPROVAL_RESULT}"
fi

echo ""

# ---------------------------------------------------------------------------
# Check 4: Requirement 5.4 / 5.7 — direct ripgrep command
# Per Requirement 5.4: R0 scope check ripgrep pattern.
# Per Requirement 5.7: re-executed at every R1/R2 PR merge.
# ---------------------------------------------------------------------------

echo "═══════════════════════════════════════════════════════════════"
echo "  [4/4] Running Requirement 5.4 / 5.7 ripgrep scope scan..."
echo "═══════════════════════════════════════════════════════════════"

# Requirement 5.4 ripgrep pattern
REQ_5_4_PATTERN='(live[_ -]?migration|control[_ -]?reset|auto[_ -]?dispatch|sleeve[_ -]?multiplier|dispatchMode|session\.config|gate[_ -]?改造|event[_ -]?source[_ -]?替換|R[3-5]\b)'

# Build target file list
RG_TARGETS=()
[[ -f "${REQUIREMENTS_MD}" ]] && RG_TARGETS+=("${REQUIREMENTS_MD}")
[[ -f "${DESIGN_MD}" ]] && RG_TARGETS+=("${DESIGN_MD}")
[[ -f "${TASKS_MD}" ]] && RG_TARGETS+=("${TASKS_MD}")
[[ ${DIFF_SIZE} -gt 0 ]] && RG_TARGETS+=("${DIFF_TMPFILE}")

if [[ ${#RG_TARGETS[@]} -eq 0 ]]; then
    echo "  ⚠️  No target files for Requirement 5.4 ripgrep scan."
elif [[ -n "${RG}" ]]; then
    # Use ripgrep (preferred)
    RG_OUTPUT=""
    if RG_OUTPUT=$("${RG}" -n -i "${REQ_5_4_PATTERN}" "${RG_TARGETS[@]}" 2>&1); then
        echo "  ❌ Requirement 5.4 ripgrep scan found matches:"
        echo "${RG_OUTPUT}" | head -20
        echo ""
        echo "  → Requirement 5.4 / 5.7 scope check FAILED"
        FAILED=1
    else
        echo "  ✅ Requirement 5.4 / 5.7 ripgrep scan — no violations found."
    fi
elif command -v grep &>/dev/null; then
    # Fallback: use grep -E -i -n
    GREP_OUTPUT=""
    if GREP_OUTPUT=$(grep -n -i -E "${REQ_5_4_PATTERN}" "${RG_TARGETS[@]}" 2>&1); then
        echo "  ❌ Requirement 5.4 grep scan found matches:"
        echo "${GREP_OUTPUT}" | head -20
        echo ""
        echo "  → Requirement 5.4 / 5.7 scope check FAILED"
        FAILED=1
    else
        echo "  ✅ Requirement 5.4 / 5.7 grep scan — no violations found."
    fi
else
    infra_failure "Neither rg nor grep available for Requirement 5.4 / 5.7 scan"
fi

echo ""

# ---------------------------------------------------------------------------
# Final result
# ---------------------------------------------------------------------------

echo "═══════════════════════════════════════════════════════════════"
if [[ ${FAILED} -ne 0 ]]; then
    echo "  ❌ entry_redesign_scope_check FAILED — PR must not merge."
    echo "     Reference: AGENTS §2 Strategy Semantic Sources"
    echo "═══════════════════════════════════════════════════════════════"
    exit 1
else
    echo "  ✅ entry_redesign_scope_check PASSED — all checks clean."
    echo "═══════════════════════════════════════════════════════════════"
    exit 0
fi
