#!/usr/bin/env bash
# Deploy open-crafters runner + learn stack on the VPS.
#
# Usage (from repo root):
#   ./scripts/vps-deploy.sh
#   SSH_HOST=ubuntu@150.230.131.66 ./scripts/vps-deploy.sh
#   ./scripts/vps-deploy.sh --learn-only
#
set -euo pipefail

SSH_HOST="${SSH_HOST:-ubuntu@150.230.131.66}"
REMOTE_DIR="${REMOTE_DIR:-~/open-crafters}"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/vps-compose.yml}"
LEARN_NO_CACHE="${LEARN_NO_CACHE:-1}"
DEPLOY_LEARN=1
DEPLOY_RUNNER=1

usage() {
  cat <<'EOF'
Usage: scripts/vps-deploy.sh [options]

Options:
  --learn-only     Rebuild and restart only the learn container
  --runner-only    Rebuild grade + runner (skip learn)
  --no-cache-learn Pass --no-cache to the learn image build (default: on)
  --cache-learn    Allow Docker layer cache for learn builds
  -h, --help       Show this help

Environment:
  SSH_HOST         SSH target (default: ubuntu@150.230.131.66)
  REMOTE_DIR       Repo path on the VPS (default: ~/open-crafters)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --learn-only)
      DEPLOY_RUNNER=0
      shift
      ;;
    --runner-only)
      DEPLOY_LEARN=0
      shift
      ;;
    --no-cache-learn)
      LEARN_NO_CACHE=1
      shift
      ;;
    --cache-learn)
      LEARN_NO_CACHE=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

echo "→ Deploying to ${SSH_HOST}:${REMOTE_DIR}"

ssh "${SSH_HOST}" bash -s -- \
  "${REMOTE_DIR}" \
  "${COMPOSE_FILE}" \
  "${DEPLOY_LEARN}" \
  "${DEPLOY_RUNNER}" \
  "${LEARN_NO_CACHE}" <<'REMOTE'
set -euo pipefail
REMOTE_DIR="$1"
COMPOSE_FILE="$2"
DEPLOY_LEARN="$3"
DEPLOY_RUNNER="$4"
LEARN_NO_CACHE="$5"

cd "${REMOTE_DIR}"
git pull

if [[ "$DEPLOY_RUNNER" == "1" ]]; then
  echo "→ Building grade + runner images"
  docker build -t open-crafters-grade:latest -f docker/grade/Dockerfile .
  docker build -t open-crafters-runner:latest -f docker/runner/Dockerfile .
fi

if [[ "$DEPLOY_LEARN" == "1" ]]; then
  echo "→ Building learn image"
  if [[ "$LEARN_NO_CACHE" == "1" ]]; then
    docker build --no-cache -t open-crafters-learn:latest -f docker/learn/Dockerfile .
  else
    docker build -t open-crafters-learn:latest -f docker/learn/Dockerfile .
  fi
fi

export RUNNER_TOKEN
RUNNER_TOKEN="$(docker inspect open-crafters-runner --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep '^RUNNER_TOKEN=' | cut -d= -f2- || true)"
if [[ -z "${RUNNER_TOKEN:-}" && -f .env ]]; then
  RUNNER_TOKEN="$(grep '^RUNNER_TOKEN=' .env | cut -d= -f2- || true)"
fi
export GITHUB_TOKEN
GITHUB_TOKEN="$(cat .github-token 2>/dev/null || true)"

SERVICES=()
[[ "$DEPLOY_LEARN" == "1" ]] && SERVICES+=(learn)
[[ "$DEPLOY_RUNNER" == "1" ]] && SERVICES+=(runner)

if [[ ${#SERVICES[@]} -eq 0 ]]; then
  echo "nothing to deploy"
  exit 0
fi

docker compose -f "$COMPOSE_FILE" up -d "${SERVICES[@]}"
docker compose -f "$COMPOSE_FILE" ps "${SERVICES[@]}"

curl -sf http://127.0.0.1:18081/health | head -c 80 || true
echo
REMOTE

echo "✓ Deploy complete"
