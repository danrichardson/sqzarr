#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_USER="${DEPLOY_USER:-root}"
REMOTE_SRC_DIR="${REMOTE_SRC_DIR:-/opt/sqzarr-src}"
REMOTE_BIN_PATH="${REMOTE_BIN_PATH:-/usr/local/bin/sqzarr}"
SERVICE_NAME="${SERVICE_NAME:-sqzarr}"

if [[ -z "$DEPLOY_HOST" ]]; then
  echo "DEPLOY_HOST is required."
  echo "Set it and re-run, for example:"
  echo "  DEPLOY_HOST=192.168.29.211 ./deploy.sh"
  exit 1
fi

cd "$ROOT_DIR"

echo "[1/3] Building frontend..."
(
  cd frontend
  npm run build
)

echo "[2/3] Copying source to server..."
scp -r internal cmd go.mod go.sum "$DEPLOY_USER@$DEPLOY_HOST:$REMOTE_SRC_DIR/"

echo "[3/3] Building binary and restarting service..."
ssh "$DEPLOY_USER@$DEPLOY_HOST" \
  "cd '$REMOTE_SRC_DIR' && go build -trimpath -ldflags='-s -w' -o '$REMOTE_BIN_PATH' ./cmd/sqzarr/ && systemctl restart '$SERVICE_NAME' && systemctl is-active '$SERVICE_NAME'"

echo "DEPLOY OK"
