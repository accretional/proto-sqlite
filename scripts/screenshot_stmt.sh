#!/usr/bin/env bash
# Screenshot one sqlite syntax diagram page and dump SVG text strings.
# Usage: screenshot_stmt.sh <stmt-name>    (e.g. alter-table-stmt)
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

STMT="${1:-alter-table-stmt}"
URL="https://sqlite.org/syntax/${STMT}.html"
OUT_PNG="$ROOT/docs/sqlite-parse-img/${STMT}.png"
OUT_TXT="$ROOT/docs/sqlite-parse-img/${STMT}.svg-strings.json"

CHROMERPC_DIR="$ROOT/third_party/chromerpc"
CHROMERPC_BIN="$CHROMERPC_DIR/bin/chromerpc"
AUTOMATE_BIN="$CHROMERPC_DIR/bin/automate"

if [ ! -x "$CHROMERPC_BIN" ] || [ ! -x "$AUTOMATE_BIN" ]; then
  echo "[screenshot] chromerpc not built; run ./setup.sh" >&2
  exit 1
fi

# Find a Chrome binary.
CHROME=""
for c in \
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  "/Applications/Chromium.app/Contents/MacOS/Chromium" \
  "$(command -v google-chrome || true)" \
  "$(command -v chromium || true)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then CHROME="$c"; break; fi
done
if [ -z "$CHROME" ]; then
  echo "[screenshot] no Chrome/Chromium found" >&2
  exit 1
fi

# Render automation textproto from template.
TMPL="$ROOT/lang/automations/screenshot_stmt.textproto.tmpl"
RENDERED=$(mktemp -t "${STMT}.XXXXXX.textproto")
# URL-safe sed: no slashes in URL other than the standard ones; use '|' delimiter.
sed -e "s|__URL__|${URL}|g" -e "s|__OUT__|${OUT_PNG}|g" "$TMPL" > "$RENDERED"

# Start chromerpc server on a free-ish port.
PORT=50551
LOG=$(mktemp -t chromerpc.XXXXXX.log)
echo "[screenshot] launching chromerpc (log: $LOG)"
"$CHROMERPC_BIN" -addr ":$PORT" -chrome "$CHROME" -headless >"$LOG" 2>&1 &
SERVER_PID=$!
cleanup() {
  kill "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
  rm -f "$RENDERED"
}
trap cleanup EXIT

# Wait for server to listen.
for i in $(seq 1 40); do
  if nc -z localhost "$PORT" 2>/dev/null; then break; fi
  sleep 0.25
done
if ! nc -z localhost "$PORT" 2>/dev/null; then
  echo "[screenshot] server failed to start; log:" >&2
  cat "$LOG" >&2
  exit 1
fi

echo "[screenshot] running automation for $STMT"
AUTOMATE_OUT=$(mktemp -t automate.XXXXXX.log)
if ! "$AUTOMATE_BIN" -addr "localhost:$PORT" -input "$RENDERED" -timeout 45s >"$AUTOMATE_OUT" 2>&1; then
  cat "$AUTOMATE_OUT" >&2
  exit 1
fi
cat "$AUTOMATE_OUT"

# Extract the extract_svg_text step's JSON array from the automate output.
# Lines look like:  [extract_svg_text] OK => "[\"ALTER\",\"TABLE\",...]"
python3 - "$AUTOMATE_OUT" "$OUT_TXT" <<'PY'
import json, re, sys
log_path, out_path = sys.argv[1], sys.argv[2]
with open(log_path) as f:
    data = f.read()
m = re.search(r'\[extract_svg_text\] OK => (.+)$', data, re.M)
if not m:
    print("[screenshot] WARN: no extract_svg_text result", file=sys.stderr)
    sys.exit(0)
raw = m.group(1).strip()
# chromerpc wraps script_result as a JSON-ish string; try to unwrap.
try:
    inner = json.loads(raw) if raw.startswith('"') else raw
    arr = json.loads(inner) if isinstance(inner, str) else inner
except Exception:
    arr = raw
with open(out_path, 'w') as f:
    json.dump(arr, f, indent=2)
print(f"[screenshot] wrote {out_path} ({len(arr) if isinstance(arr, list) else '?'} strings)")
PY

echo "[screenshot] wrote $OUT_PNG"
ls -la "$OUT_PNG"
