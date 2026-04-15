#!/usr/bin/env bash
# Idempotent project setup: deps, chromerpc, sqlite binary.
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(pwd)"
THIRD_PARTY="$ROOT/third_party"
mkdir -p "$THIRD_PARTY" "$ROOT/docs/sqlite-parse-img" "$ROOT/sqlite/bin" "$ROOT/sqlite/db"

echo "[setup] go version: $(go version)"
case "$(go version)" in
  *"go1.26"*) ;;
  *) echo "[setup] ERROR: go 1.26 required" >&2; exit 1 ;;
esac

# --- chromerpc ---
CHROMERPC_DIR="$THIRD_PARTY/chromerpc"
if [ ! -d "$CHROMERPC_DIR/.git" ]; then
  echo "[setup] cloning chromerpc"
  git clone --depth 1 https://github.com/accretional/chromerpc.git "$CHROMERPC_DIR"
else
  echo "[setup] chromerpc already present"
fi

# Build chromerpc automate binary once.
if [ ! -x "$CHROMERPC_DIR/bin/automate" ] || [ ! -x "$CHROMERPC_DIR/bin/chromerpc" ]; then
  echo "[setup] building chromerpc binaries"
  ( cd "$CHROMERPC_DIR" && mkdir -p bin \
      && go build -o bin/automate ./cmd/automate \
      && go build -o bin/chromerpc ./cmd/chromerpc )
fi

# --- sqlite binary (precompiled) ---
# https://sqlite.org/download.html — macos-arm64 tools bundle.
SQLITE_BIN="$ROOT/sqlite/bin/sqlite3"
if [ ! -x "$SQLITE_BIN" ]; then
  echo "[setup] downloading sqlite3 binary"
  TMP=$(mktemp -d)
  UNAME_S=$(uname -s); UNAME_M=$(uname -m)
  case "$UNAME_S:$UNAME_M" in
    Darwin:arm64|Darwin:*)
      URL="https://sqlite.org/2025/sqlite-tools-osx-x64-3500000.zip" ;;
    Linux:*)
      URL="https://sqlite.org/2025/sqlite-tools-linux-x64-3500000.zip" ;;
    *) echo "[setup] unsupported platform: $UNAME_S/$UNAME_M" >&2; exit 1 ;;
  esac
  ( cd "$TMP" && curl -fsSL "$URL" -o sqlite.zip && unzip -q sqlite.zip )
  # Find the sqlite3 executable inside the extracted tree and copy it.
  FOUND=$(find "$TMP" -type f -name sqlite3 -perm +111 | head -n1 || true)
  if [ -z "$FOUND" ]; then
    # Some archives lack executable bit; fallback to name match.
    FOUND=$(find "$TMP" -type f -name sqlite3 | head -n1 || true)
  fi
  if [ -z "$FOUND" ]; then
    echo "[setup] ERROR: sqlite3 not found in archive" >&2
    exit 1
  fi
  cp "$FOUND" "$SQLITE_BIN"
  chmod +x "$SQLITE_BIN"
  rm -rf "$TMP"
fi
echo "[setup] sqlite: $("$SQLITE_BIN" --version)"

# --- example sqlite db ---
EXAMPLE_DB="$ROOT/sqlite/db/example.db"
if [ ! -f "$EXAMPLE_DB" ]; then
  echo "[setup] creating example db"
  "$SQLITE_BIN" "$EXAMPLE_DB" <<'SQL'
CREATE TABLE IF NOT EXISTS widgets (id INTEGER PRIMARY KEY, name TEXT, qty INTEGER);
INSERT INTO widgets (name, qty) VALUES ('sprocket', 3), ('gizmo', 7), ('cog', 12);
SQL
fi

# --- go module ---
if [ ! -f "$ROOT/go.mod" ]; then
  echo "[setup] initializing go module"
  ( cd "$ROOT" && go mod init github.com/accretional/proto-sqlite )
fi

echo "[setup] OK"
