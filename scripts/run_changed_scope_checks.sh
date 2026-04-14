#!/usr/bin/env bash
set -euo pipefail

MODE="working-tree"
BASE_REF=""
CI_MODE="false"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --base)
      MODE="base"
      BASE_REF="${2:-}"
      shift 2
      ;;
    --staged)
      MODE="staged"
      shift
      ;;
    --working-tree)
      MODE="working-tree"
      shift
      ;;
    --ci)
      CI_MODE="true"
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [ "$MODE" = "base" ] && [ -z "$BASE_REF" ]; then
  echo "--base requires a git ref." >&2
  exit 1
fi

CHANGED_FILES=()
if [ "$MODE" = "base" ]; then
  CHANGED_OUTPUT="$(git diff --name-only --diff-filter=ACMR "${BASE_REF}...HEAD")"
elif [ "$MODE" = "staged" ]; then
  CHANGED_OUTPUT="$(git diff --cached --name-only --diff-filter=ACMR)"
else
  CHANGED_OUTPUT="$(
    {
      git diff --name-only --diff-filter=ACMR HEAD
      git ls-files --others --exclude-standard
    } | sort -u
  )"
fi

if [ -n "$CHANGED_OUTPUT" ]; then
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    CHANGED_FILES+=("$line")
  done <<EOF
$CHANGED_OUTPUT
EOF
fi

if [ "${#CHANGED_FILES[@]}" -eq 0 ]; then
  echo "No changed files detected for scope checks."
  exit 0
fi

printf 'Changed files for scope checks:\n'
printf ' - %s\n' "${CHANGED_FILES[@]}"
printf '\n'

HAS_BACKEND="false"
HAS_FRONTEND="false"
HAS_MIGRATIONS="false"
DOCS_ONLY="true"
HAS_HIGH_RISK="false"
HAS_HARNESS="false"

GO_FILES=()
MIGRATION_FILES=()

for path in "${CHANGED_FILES[@]}"; do
  case "$path" in
    *.md|docs/*|README.md|AGENTS.md|.github/pull_request_template.md)
      ;;
    *)
      DOCS_ONLY="false"
      ;;
  esac

  case "$path" in
    cmd/*|internal/*|go.mod|go.sum)
      HAS_BACKEND="true"
      ;;
  esac

  case "$path" in
    web/console/*)
      HAS_FRONTEND="true"
      ;;
  esac

  case "$path" in
    db/migrations/*)
      HAS_BACKEND="true"
      HAS_MIGRATIONS="true"
      HAS_HIGH_RISK="true"
      MIGRATION_FILES+=("$path")
      ;;
  esac

  case "$path" in
    internal/service/live*.go|internal/service/execution_strategy.go|.github/workflows/*|deployments/*|web/console/src/store/useTradingStore.ts|web/console/src/hooks/useTradingActions.ts|web/console/src/modals/LiveSessionModal.tsx)
      HAS_HIGH_RISK="true"
      ;;
  esac

  case "$path" in
    .github/*|docs/*|scripts/*|AGENTS.md)
      HAS_HARNESS="true"
      ;;
  esac

  case "$path" in
    *.go)
      GO_FILES+=("$path")
      ;;
  esac
done

printf 'Detected scope flags:\n'
printf ' - docs_only=%s\n' "$DOCS_ONLY"
printf ' - has_harness=%s\n' "$HAS_HARNESS"
printf ' - has_backend=%s\n' "$HAS_BACKEND"
printf ' - has_frontend=%s\n' "$HAS_FRONTEND"
printf ' - has_migrations=%s\n' "$HAS_MIGRATIONS"
printf ' - has_high_risk=%s\n' "$HAS_HIGH_RISK"
printf '\n'

echo "Running repository safety sensors..."
bash scripts/check_high_risk_defaults.sh
bash scripts/check_env_safety.sh

if [ "$HAS_MIGRATIONS" = "true" ]; then
  echo ""
  echo "Running migration safety sensor..."
  python3 scripts/check_migration_safety.py "${MIGRATION_FILES[@]}"
fi

if [ "$DOCS_ONLY" = "true" ]; then
  echo ""
  echo "Docs-only change detected; skipping Go and frontend builds."
  exit 0
fi

if [ "$HAS_BACKEND" = "true" ]; then
  echo ""
  echo "Running backend checks..."
  if [ "${#GO_FILES[@]}" -gt 0 ]; then
    unformatted="$(gofmt -l "${GO_FILES[@]}")"
    if [ -n "$unformatted" ]; then
      echo "The following changed Go files are not gofmt-formatted:" >&2
      printf '%s\n' "$unformatted" >&2
      exit 1
    fi
  fi
  go test ./...
  go build ./cmd/platform-api
  go build ./cmd/db-migrate
fi

if [ "$HAS_FRONTEND" = "true" ]; then
  echo ""
  echo "Running frontend checks..."
  (
    cd web/console
    if [ ! -d node_modules ] || [ "${BKTRADER_FORCE_NPM_CI:-0}" = "1" ] || [ "$CI_MODE" = "true" ]; then
      npm ci
    fi
    npm run build
  )
fi

echo ""
echo "Changed-scope checks passed."
