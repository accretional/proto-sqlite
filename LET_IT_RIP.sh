#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
./test.sh

SQLITE=./sqlite/bin/sqlite3
DB=./sqlite/db/example.db
echo "[rip] running queries against $DB"
"$SQLITE" "$DB" 'SELECT id, name, qty FROM widgets ORDER BY id;'
"$SQLITE" "$DB" 'SELECT SUM(qty) AS total FROM widgets;'

echo "[rip] gRPC round-trip"
go run ./sqlite/cmd/ripgrpc
echo "[rip] OK"
