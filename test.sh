#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
./build.sh

if [ -f go.mod ] && compgen -G "**/*_test.go" >/dev/null; then
  echo "[test] go test ./..."
  go test ./...
else
  echo "[test] no tests yet"
fi
echo "[test] OK"
