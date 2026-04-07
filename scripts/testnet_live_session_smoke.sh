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

if [ -z "$AUTH_PASSWORD" ]; then
  echo "AUTH_PASSWORD is required" >&2
  exit 1
fi

login_response="$(
  curl -sS -X POST "$API_BASE/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$AUTH_USERNAME\",\"password\":\"$AUTH_PASSWORD\"}"
)"
token="$(printf '%s' "$login_response" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"

if [ -z "$token" ]; then
  echo "failed to obtain auth token" >&2
  printf '%s\n' "$login_response" >&2
  exit 1
fi

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

create_response="$(
  curl -sS -X POST "$API_BASE/api/v1/live/sessions" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$create_payload"
)"

printf '%s\n' "$create_response"
