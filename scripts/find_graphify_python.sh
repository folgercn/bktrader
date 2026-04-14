#!/usr/bin/env bash
set -euo pipefail

supports_graphify() {
  local candidate="$1"
  [ -n "$candidate" ] || return 1
  command -v "$candidate" >/dev/null 2>&1 || return 1
  "$candidate" - <<'PY' >/dev/null 2>&1
from graphify.watch import _rebuild_code
PY
}

if [ -n "${BKTRADER_GRAPHIFY_PYTHON:-}" ] && supports_graphify "${BKTRADER_GRAPHIFY_PYTHON}"; then
  command -v "${BKTRADER_GRAPHIFY_PYTHON}"
  exit 0
fi

for candidate in \
  python3.12 \
  /usr/local/bin/python3.12 \
  /usr/local/opt/python@3.12/bin/python3.12 \
  python3 \
  /usr/local/bin/python3 \
  python3.11 \
  /usr/local/bin/python3.11 \
  /usr/local/opt/python@3.11/bin/python3.11 \
  python
do
  if supports_graphify "$candidate"; then
    command -v "$candidate"
    exit 0
  fi
done

echo "No Python interpreter with graphify.watch available." >&2
exit 1
