#!/usr/bin/env bash
set -euo pipefail

# Compile all .py files under research/entry_redesign/ using py_compile.
# Any compilation failure causes immediate non-zero exit (set -e).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

count=0

while IFS= read -r -d '' pyfile; do
    python3 -m py_compile "$pyfile"
    count=$((count + 1))
done < <(find "$PACKAGE_ROOT" -name '*.py' -not -path '*/__pycache__/*' -print0 | sort -z)

echo "compile_all.sh: ${count} Python files compiled successfully."
