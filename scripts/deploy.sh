#!/usr/bin/env bash
set -euo pipefail

DEPLOY_PATH=${DEPLOY_PATH:-/opt/bktrader}
COMPOSE_FILE="$DEPLOY_PATH/deployments/docker-compose.prod.yml"
APP_ENV_FILE=${APP_ENV_FILE:-$DEPLOY_PATH/.env}
IMAGE_REPO=${IMAGE_REPO:-ghcr.io/wuyaocheng/bktrader}
IMAGE_TAG=${IMAGE_TAG:-latest}

mkdir -p "$DEPLOY_PATH/deployments" "$DEPLOY_PATH/scripts"

if [[ -n "${APP_ENV_FILE_CONTENT:-}" ]]; then
  printf '%s
' "$APP_ENV_FILE_CONTENT" > "$APP_ENV_FILE"
fi

if [[ -n "${GHCR_USERNAME:-}" && -n "${GHCR_TOKEN:-}" ]]; then
  echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USERNAME" --password-stdin
fi

export IMAGE_REPO IMAGE_TAG APP_ENV_FILE

docker compose -f "$COMPOSE_FILE" pull

docker compose -f "$COMPOSE_FILE" up -d

docker image prune -f >/dev/null 2>&1 || true

echo "Deploy finished: ${IMAGE_REPO}:${IMAGE_TAG}"
