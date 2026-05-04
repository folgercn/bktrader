#!/usr/bin/env bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:${PATH:-}"

DEPLOY_PATH=${DEPLOY_PATH:-/opt/bktrader}
COMPOSE_FILE="$DEPLOY_PATH/deployments/docker-compose.prod.yml"
APP_ENV_FILE=${APP_ENV_FILE:-$DEPLOY_PATH/.env}
IMAGE_REPO=${IMAGE_REPO:-ghcr.io/folgercn/bktrader-app}
IMAGE_TAG=${IMAGE_TAG:-latest}
DOCKER_CONFIG_DIR=${DOCKER_CONFIG_DIR:-$DEPLOY_PATH/.docker-ci}
DEPLOY_SERVICES=${DEPLOY_SERVICES:-}

if ! command -v docker >/dev/null 2>&1; then
  echo "docker command not found; install Docker Desktop or another Docker runtime on this Mac." >&2
  exit 127
fi

mkdir -p "$DEPLOY_PATH/deployments" "$DEPLOY_PATH/scripts" "$DEPLOY_PATH/logs"
mkdir -p "$DOCKER_CONFIG_DIR"
chmod 750 "$DEPLOY_PATH/logs"
chmod 700 "$DOCKER_CONFIG_DIR"

if [[ -n "${APP_ENV_FILE_CONTENT:-}" ]]; then
  printf '%s
' "$APP_ENV_FILE_CONTENT" > "$APP_ENV_FILE"
fi

env_file_value() {
  local key=$1
  [[ -f "$APP_ENV_FILE" ]] || return 0
  awk -F= -v key="$key" '
    $0 !~ /^[[:space:]]*#/ && $1 == key {
      value = substr($0, length($1) + 2)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      gsub(/^["'\'']|["'\'']$/, "", value)
      print value
    }
  ' "$APP_ENV_FILE" | tail -1
}

effective_app_env=${APP_ENV:-$(env_file_value APP_ENV)}
effective_supervisor_token=${SUPERVISOR_BEARER_TOKEN:-$(env_file_value SUPERVISOR_BEARER_TOKEN)}
if [[ "$effective_app_env" == "production" && "$effective_supervisor_token" == "change-this-supervisor-probe-token" ]]; then
  echo "Refusing production deploy with placeholder SUPERVISOR_BEARER_TOKEN; set a strong random token in $APP_ENV_FILE." >&2
  exit 3
fi
if [[ -n "$effective_app_env" ]]; then
  export APP_ENV="$effective_app_env"
fi
if [[ -n "$effective_supervisor_token" ]]; then
  export SUPERVISOR_BEARER_TOKEN="$effective_supervisor_token"
fi

export DOCKER_CONFIG="$DOCKER_CONFIG_DIR"

if [[ "$IMAGE_REPO" == ghcr.io/* ]]; then
  if [[ -z "${GHCR_USERNAME:-}" || -z "${GHCR_TOKEN:-}" ]]; then
    echo "Missing GHCR credentials for private image pull. Required env: GHCR_USERNAME and GHCR_TOKEN" >&2
    echo "Current image: ${IMAGE_REPO}:${IMAGE_TAG}" >&2
    exit 1
  fi

  auth_b64="$(printf '%s' "${GHCR_USERNAME}:${GHCR_TOKEN}" | base64 | tr -d '\r\n')"
  printf '{"auths":{"ghcr.io":{"auth":"%s"}}}\n' "$auth_b64" > "$DOCKER_CONFIG_DIR/config.json"
  chmod 600 "$DOCKER_CONFIG_DIR/config.json"

  if ! docker manifest inspect "${IMAGE_REPO}:${IMAGE_TAG}" >/dev/null 2>&1; then
    echo "Unable to access image manifest: ${IMAGE_REPO}:${IMAGE_TAG}" >&2
    echo "Check GHCR token permissions (read:packages), package visibility, image tag, and repository linkage." >&2
    exit 1
  fi
fi

export IMAGE_REPO IMAGE_TAG APP_ENV_FILE

echo "Deploy image: ${IMAGE_REPO}:${IMAGE_TAG}"
if [[ -n "$DEPLOY_SERVICES" ]]; then
  echo "Deploy service input: ${DEPLOY_SERVICES}"
else
  echo "Deploy service input: <all compose services>"
fi

deploy_args=()
if [[ -n "$DEPLOY_SERVICES" ]]; then
  for service in $DEPLOY_SERVICES; do
    case "$service" in
      platform-api|live-runner|signal-runtime-runner|notification-worker|supervisor)
        deploy_args+=("$service")
        ;;
      *)
        echo "Unsupported DEPLOY_SERVICES entry: $service" >&2
        echo "Allowed services: platform-api live-runner signal-runtime-runner notification-worker supervisor" >&2
        exit 2
        ;;
    esac
  done
fi

if [[ ${#deploy_args[@]} -eq 0 ]]; then
  echo "Deploying all compose services"
  docker-compose -f "$COMPOSE_FILE" pull
  docker-compose -f "$COMPOSE_FILE" up -d --remove-orphans
else
  echo "Deploying compose services: ${deploy_args[*]}"
  docker-compose -f "$COMPOSE_FILE" pull "${deploy_args[@]}"
  docker-compose -f "$COMPOSE_FILE" up -d --remove-orphans "${deploy_args[@]}"
fi

docker image prune -f >/dev/null 2>&1 || true

echo "Deploy finished: ${IMAGE_REPO}:${IMAGE_TAG}"
