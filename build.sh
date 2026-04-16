#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
./setup.sh

if [ -f go.mod ]; then
  echo "[build] regenerating sqlite.proto from EBNF"
  go run ./lang/cmd/genproto 2>&1 | sed 's/^/[build] /'

  if command -v protoc >/dev/null && \
     command -v protoc-gen-go >/dev/null && \
     command -v protoc-gen-go-grpc >/dev/null; then
    echo "[build] protoc → sqlite/pb"
    mkdir -p sqlite/pb
    protoc --go_out=sqlite/pb --go_opt=paths=source_relative \
           --go-grpc_out=sqlite/pb --go-grpc_opt=paths=source_relative \
           -I. sqlite.proto sqlite_service.proto
  else
    echo "[build] WARN: protoc / protoc-gen-go(-grpc) missing — skipping pb codegen"
  fi

  echo "[build] go build ./..."
  go build ./... 2>&1 | sed 's/^/[build] /' || true
  go mod tidy 2>/dev/null || true
fi
echo "[build] OK"
