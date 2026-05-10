#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$ROOT_DIR/dist"
OUT="$DIST_DIR/sqzarr-linux-amd64"

mkdir -p "$DIST_DIR"

cd "$ROOT_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' -o "$OUT" ./cmd/sqzarr/

echo "BUILD OK"
ls -lh "$OUT"
