#!/usr/bin/env bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:${PATH:-}"

DEPLOY_PATH=${DEPLOY_PATH:-/opt/bktrader}
COMPOSE_FILE="$DEPLOY_PATH/deployments/docker-compose.prod.yml"
APP_ENV_FILE=${APP_ENV_FILE:-$DEPLOY_PATH/.env}
IMAGE_REPO=${IMAGE_REPO:-ghcr.io/folgercn/bktrader-app}
IMAGE_TAG=${IMAGE_TAG:-latest}
DOCKER_CONFIG_DIR=${DOCKER_CONFIG_DIR:-$DEPLOY_PATH/.docker-ci}

if ! command -v docker >/dev/null 2>&1; then
  echo "docker command not found; install Docker Desktop or another Docker runtime on this Mac." >&2
  exit 127
fi

mkdir -p "$DEPLOY_PATH/deployments" "$DEPLOY_PATH/scripts"
mkdir -p "$DOCKER_CONFIG_DIR"
printf '%s\n' '{"auths":{}}' > "$DOCKER_CONFIG_DIR/config.json"

if [[ -n "${APP_ENV_FILE_CONTENT:-}" ]]; then
  printf '%s
' "$APP_ENV_FILE_CONTENT" > "$APP_ENV_FILE"
fi

export DOCKER_CONFIG="$DOCKER_CONFIG_DIR"

if [[ "$IMAGE_REPO" == ghcr.io/* ]]; then
  if [[ -z "${GHCR_USERNAME:-}" || -z "${GHCR_TOKEN:-}" ]]; then
    echo "Missing GHCR credentials for private image pull. Required env: GHCR_USERNAME and GHCR_TOKEN" >&2
    echo "Current image: ${IMAGE_REPO}:${IMAGE_TAG}" >&2
    exit 1
  fi

  echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USERNAME" --password-stdin

  if ! docker manifest inspect "${IMAGE_REPO}:${IMAGE_TAG}" >/dev/null 2>&1; then
    echo "Unable to access image manifest: ${IMAGE_REPO}:${IMAGE_TAG}" >&2
    echo "Check GHCR token permissions (read:packages), package visibility, image tag, and repository linkage." >&2
    exit 1
  fi
fi

export IMAGE_REPO IMAGE_TAG APP_ENV_FILE

docker compose -f "$COMPOSE_FILE" pull

docker compose -f "$COMPOSE_FILE" up -d

docker image prune -f >/dev/null 2>&1 || true

echo "Deploy finished: ${IMAGE_REPO}:${IMAGE_TAG}"
