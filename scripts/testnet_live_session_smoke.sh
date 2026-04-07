#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [ -f ".env" ]; then
  # shellcheck disable=SC1091
  source .env
fi

API_BASE="${API_BASE:-http://127.0.0.1:8080}"
AUTH_USERNAME="${AUTH_USERNAME:-admin}"
AUTH_PASSWORD="${AUTH_PASSWORD:-}"
ACCOUNT_ID="${ACCOUNT_ID:-live-main}"
STRATEGY_ID="${STRATEGY_ID:-strategy-bk-1d}"
SYMBOL="${SYMBOL:-BTCUSDT}"
SESSION_ID="${SESSION_ID:-}"
AUTO_START="${AUTO_START:-true}"
EXPECT_EXIT_PROFILE="${EXPECT_EXIT_PROFILE:-}"
POLL_TIMEOUT_SECONDS="${POLL_TIMEOUT_SECONDS:-180}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-5}"

if [ -z "$AUTH_PASSWORD" ]; then
  echo "AUTH_PASSWORD is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

login_response="$(
  curl -sS -X POST "$API_BASE/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$AUTH_USERNAME\",\"password\":\"$AUTH_PASSWORD\"}"
)"
token="$(printf '%s' "$login_response" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))' 2>/dev/null || true)"

if [ -z "$token" ]; then
  echo "failed to obtain auth token" >&2
  printf '%s\n' "$login_response" >&2
  exit 1
fi

api_get() {
  curl -sS "$API_BASE$1" -H "Authorization: Bearer $token"
}

api_post() {
  local path="$1"
  local payload="${2:-}"
  if [ -n "$payload" ]; then
    curl -sS -X POST "$API_BASE$path" \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json" \
      -d "$payload"
  else
    curl -sS -X POST "$API_BASE$path" \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json"
  fi
}

current_session_json() {
  local session_id="$1"
  api_get "/api/v1/live/sessions" | python3 -c '
import json, sys
session_id = sys.argv[1]
items = json.load(sys.stdin)
for item in items:
    if item.get("id") == session_id:
        json.dump(item, sys.stdout)
        break
else:
    sys.exit(1)
' "$session_id"
}

print_session_snapshot() {
  python3 -c '
import json, sys
session = json.loads(sys.argv[1])
state = session.get("state") or {}
profile = state.get("lastExecutionProfile") or {}
dispatch = state.get("lastExecutionDispatch") or {}
summary = {
    "sessionId": session.get("id"),
    "status": session.get("status"),
    "symbol": state.get("symbol"),
    "dispatchMode": state.get("dispatchMode"),
    "profile": profile,
    "dispatch": dispatch,
}
print(json.dumps(summary, indent=2, ensure_ascii=False))
' "$1"
}

validate_profile_defaults() {
  python3 -c '
import json, sys
session = json.loads(sys.argv[1])
state = session.get("state") or {}
errors = []
if (state.get("executionPTExitOrderType") or "").upper() != "LIMIT":
    errors.append("executionPTExitOrderType should be LIMIT")
if (state.get("executionPTExitTimeInForce") or "").upper() != "GTX":
    errors.append("executionPTExitTimeInForce should be GTX")
if bool(state.get("executionPTExitPostOnly")) is not True:
    errors.append("executionPTExitPostOnly should be true")
if (state.get("executionPTExitTimeoutFallbackOrderType") or "").upper() != "MARKET":
    errors.append("executionPTExitTimeoutFallbackOrderType should be MARKET")
if (state.get("executionSLExitOrderType") or "").upper() != "MARKET":
    errors.append("executionSLExitOrderType should be MARKET")
if float(state.get("executionSLExitMaxSpreadBps") or 0) < 999:
    errors.append("executionSLExitMaxSpreadBps should be >= 999")
if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
print("validated exit execution profiles on session state")
' "$1"
}

validate_exit_profile_snapshot() {
  local expected="$1"
  python3 -c '
import json, sys
session = json.loads(sys.argv[1])
expected = sys.argv[2].strip().lower()
state = session.get("state") or {}
profile = state.get("lastExecutionProfile") or {}
dispatch = state.get("lastExecutionDispatch") or {}
errors = []
kind = str(profile.get("executionProfile") or "")
if kind.lower() != expected:
    errors.append(f"expected executionProfile={expected}, got {kind or '--'}")
if expected == "pt-exit":
    if str(profile.get("orderType") or "").upper() != "LIMIT":
      errors.append("PT exit should resolve to LIMIT")
    if str(profile.get("timeInForce") or "").upper() != "GTX":
      errors.append("PT exit should resolve to GTX")
    if bool(profile.get("postOnly")) is not True:
      errors.append("PT exit should be postOnly")
    if bool(profile.get("reduceOnly")) is not True:
      errors.append("PT exit should be reduceOnly")
elif expected == "sl-exit":
    if str(profile.get("orderType") or "").upper() != "MARKET":
      errors.append("SL exit should resolve to MARKET")
    if bool(profile.get("reduceOnly")) is not True:
      errors.append("SL exit should be reduceOnly")
if dispatch:
    if str(dispatch.get("executionProfile") or "").lower() != expected:
      errors.append(f"dispatch executionProfile should be {expected}")
    if bool(dispatch.get("reduceOnly")) is not True:
      errors.append("dispatch should remain reduceOnly")
if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
print(f"validated exit profile snapshot for {expected}")
' "$2" "$expected"
}

if [ -z "$SESSION_ID" ]; then
  echo "Creating live session smoke payload for $ACCOUNT_ID / $STRATEGY_ID / $SYMBOL"
  create_payload="$(cat <<JSON
{
  "accountId": "$ACCOUNT_ID",
  "strategyId": "$STRATEGY_ID",
  "signalTimeframe": "1d",
  "executionDataSource": "tick",
  "symbol": "$SYMBOL",
  "defaultOrderQuantity": 0.002,
  "executionEntryOrderType": "MARKET",
  "executionEntryMaxSpreadBps": 8,
  "executionEntryWideSpreadMode": "limit-maker",
  "executionEntryTimeoutFallbackOrderType": "MARKET",
  "executionPTExitOrderType": "LIMIT",
  "executionPTExitTimeInForce": "GTX",
  "executionPTExitPostOnly": true,
  "executionPTExitTimeoutFallbackOrderType": "MARKET",
  "executionSLExitOrderType": "MARKET",
  "executionSLExitMaxSpreadBps": 999,
  "dispatchMode": "manual-review",
  "dispatchCooldownSeconds": 30
}
JSON
)"
  create_response="$(api_post "/api/v1/live/sessions" "$create_payload")"
  SESSION_ID="$(printf '%s' "$create_response" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))' 2>/dev/null || true)"
  if [ -z "$SESSION_ID" ]; then
    echo "failed to create live session" >&2
    printf '%s\n' "$create_response" >&2
    exit 1
  fi
  echo "created session: $SESSION_ID"
else
  echo "using existing session: $SESSION_ID"
fi

if [ "$AUTO_START" = "true" ]; then
  api_post "/api/v1/live/sessions/$SESSION_ID/start" >/dev/null
  echo "started session: $SESSION_ID"
fi

session_json="$(current_session_json "$SESSION_ID")"
print_session_snapshot "$session_json"
validate_profile_defaults "$session_json"

if [ -z "$EXPECT_EXIT_PROFILE" ]; then
  cat <<EOF
Smoke setup complete.
- Session: $SESSION_ID
- PT exit target: LIMIT / GTX / postOnly / reduceOnly
- SL exit target: MARKET / reduceOnly

Set EXPECT_EXIT_PROFILE=pt-exit or EXPECT_EXIT_PROFILE=sl-exit to poll until a real exit proposal/dispatch is observed.
EOF
  exit 0
fi

deadline=$(( $(date +%s) + POLL_TIMEOUT_SECONDS ))
while [ "$(date +%s)" -le "$deadline" ]; do
  session_json="$(current_session_json "$SESSION_ID")"
  current_profile="$(printf '%s' "$session_json" | python3 -c 'import json,sys; s=json.load(sys.stdin); print(((s.get("state") or {}).get("lastExecutionProfile") or {}).get("executionProfile",""))')"
  if [ "${current_profile,,}" = "${EXPECT_EXIT_PROFILE,,}" ]; then
    print_session_snapshot "$session_json"
    validate_exit_profile_snapshot "$EXPECT_EXIT_PROFILE" "$session_json"
    exit 0
  fi
  sleep "$POLL_INTERVAL_SECONDS"
done

echo "timed out waiting for execution profile: $EXPECT_EXIT_PROFILE" >&2
print_session_snapshot "$session_json" >&2
exit 1
