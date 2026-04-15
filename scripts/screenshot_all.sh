#!/usr/bin/env bash
# Screenshot every sqlite syntax diagram we care about into docs/sqlite-parse-img/.
# Reuses a single chromerpc server across all pages.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

# Top-level sql-stmt alternatives (from CLAUDE.md) + referenced intermediates.
STMTS=(
  sql-stmt-list
  sql-stmt
  alter-table-stmt analyze-stmt attach-stmt begin-stmt commit-stmt
  create-index-stmt create-table-stmt create-trigger-stmt create-view-stmt
  create-virtual-table-stmt delete-stmt delete-stmt-limited detach-stmt
  drop-index-stmt drop-table-stmt drop-trigger-stmt drop-view-stmt
  insert-stmt pragma-stmt reindex-stmt release-stmt rollback-stmt
  savepoint-stmt select-stmt update-stmt update-stmt-limited vacuum-stmt
  # Common intermediates referenced across stmts.
  column-def column-constraint table-constraint conflict-clause expr
  type-name signed-number literal-value numeric-literal
  qualified-table-name indexed-column foreign-key-clause
  result-column join-clause join-operator join-constraint
  compound-operator ordering-term with-clause common-table-expression
  select-core factored-select-stmt table-or-subquery
  upsert-clause returning-clause filter-clause over-clause
  window-defn frame-spec raise-function
)

CHROMERPC_DIR="$ROOT/third_party/chromerpc"
CHROMERPC_BIN="$CHROMERPC_DIR/bin/chromerpc"
AUTOMATE_BIN="$CHROMERPC_DIR/bin/automate"
[ -x "$CHROMERPC_BIN" ] && [ -x "$AUTOMATE_BIN" ] || { echo "run ./setup.sh first" >&2; exit 1; }

# Locate Chrome.
CHROME=""
for c in \
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  "/Applications/Chromium.app/Contents/MacOS/Chromium" \
  "$(command -v google-chrome || true)" \
  "$(command -v chromium || true)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then CHROME="$c"; break; fi
done
[ -n "$CHROME" ] || { echo "no Chrome found" >&2; exit 1; }

PORT=50552
LOG=$(mktemp -t chromerpc.XXXXXX.log)
echo "[batch] launching chromerpc (log: $LOG)"
"$CHROMERPC_BIN" -addr ":$PORT" -chrome "$CHROME" -headless >"$LOG" 2>&1 &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true; wait "$SERVER_PID" 2>/dev/null || true' EXIT

for i in $(seq 1 60); do
  nc -z localhost "$PORT" 2>/dev/null && break
  sleep 0.25
done
nc -z localhost "$PORT" 2>/dev/null || { echo "server failed; log:"; cat "$LOG"; exit 1; }

TMPL="$ROOT/lang/automations/screenshot_stmt.textproto.tmpl"
OUT_DIR="$ROOT/docs/sqlite-parse-img"
mkdir -p "$OUT_DIR"

FORCE="${FORCE:-0}"
ok=0; skip=0; fail=0
for stmt in "${STMTS[@]}"; do
  png="$OUT_DIR/${stmt}.png"
  json="$OUT_DIR/${stmt}.svg-strings.json"
  if [ "$FORCE" != "1" ] && [ -s "$png" ] && [ -s "$json" ]; then
    skip=$((skip+1)); continue
  fi

  url="https://sqlite.org/syntax/${stmt}.html"
  rendered=$(mktemp -t "${stmt}.XXXXXX.textproto")
  sed -e "s|__URL__|${url}|g" -e "s|__OUT__|${png}|g" "$TMPL" > "$rendered"
  aout=$(mktemp -t automate.XXXXXX.log)
  if "$AUTOMATE_BIN" -addr "localhost:$PORT" -input "$rendered" -timeout 45s >"$aout" 2>&1; then
    python3 - "$aout" "$json" <<'PY'
import json, re, sys
log, out = sys.argv[1], sys.argv[2]
with open(log) as f: data = f.read()
m = re.search(r'\[extract_svg_text\] OK => (.+)$', data, re.M)
if not m:
    open(out,'w').write('[]'); sys.exit(0)
raw = m.group(1).strip()
try:
    inner = json.loads(raw) if raw.startswith('"') else raw
    arr = json.loads(inner) if isinstance(inner, str) else inner
except Exception:
    arr = []
json.dump(arr, open(out,'w'), indent=2)
PY
    ok=$((ok+1)); echo "[batch] ok  $stmt"
  else
    fail=$((fail+1)); echo "[batch] FAIL $stmt"
    tail -n3 "$aout" | sed 's/^/    /'
  fi
  rm -f "$rendered" "$aout"
done

echo "[batch] done: ok=$ok skip=$skip fail=$fail"
