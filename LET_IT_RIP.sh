#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
./test.sh

SQLITE=./sqlite/bin/sqlite3
DB=./sqlite/db/example.db
echo "[rip] running queries against $DB"
"$SQLITE" "$DB" 'SELECT id, name, qty FROM widgets ORDER BY id;'
"$SQLITE" "$DB" 'SELECT SUM(qty) AS total FROM widgets;'

echo "[rip] gRPC round-trip (in-proc sqlite3)"
go run ./sqlite/cmd/ripgrpc

echo "[rip] gRPC round-trip over UDS"
# UDS paths are capped at ~104 bytes on macOS; keep under /tmp.
SOCKDIR="$(mktemp -d /tmp/uds-XXXX)"
SOCK="$SOCKDIR/s.sock"
# Build first so $SQLITED_PID is the actual daemon, not a `go run` wrapper
# (killing the wrapper doesn't reap the child and leaves the stdout pipe
# open, hanging the script).
go build -o "$SOCKDIR/sqlited" ./sqlite/cmd/sqlited
"$SOCKDIR/sqlited" -socket "$SOCK" -bin "$SQLITE" -db "$DB" &
SQLITED_PID=$!
trap 'kill $SQLITED_PID 2>/dev/null || true; wait $SQLITED_PID 2>/dev/null || true; rm -rf "$SOCKDIR"' EXIT
for _ in 1 2 3 4 5 6 7 8 9 10; do
  [ -S "$SOCK" ] && break
  sleep 0.2
done
go run ./sqlite/cmd/ripgrpc -socket "$SOCK"
echo "[rip] OK"
