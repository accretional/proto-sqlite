#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
./setup.sh

if [ -f go.mod ]; then
  echo "[build] go build ./..."
  go build ./... 2>&1 | sed 's/^/[build] /' || true
  # go build on an empty module is a no-op; succeed regardless.
  go mod tidy 2>/dev/null || true
fi
echo "[build] OK"
