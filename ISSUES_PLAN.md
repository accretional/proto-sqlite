# Issues Remediation Plan

Addresses GitHub issues #1, #2, #3 filed against proto-sqlite by proto-indexer.
Constraint: no sqlite driver switch — the embedded sqlite3 CLI binary remains the
execution backend throughout.

## Ordering

**#1 → #2 → #3.** Each issue is independently mergeable. Issue #2's proto type
change to `Row.cell` touches the most test sites and should land before #3 adds
more callers depending on the stable output format.

---

## Issue #1 — Per-request `db_path`

Allow `QueryRequest` callers to target an arbitrary database file instead of the
embedded `example.db`.

### Proto change (`sqlite_service.proto`)

```proto
message QueryRequest {
  string socket_uri = 1;
  oneof body { string sql = 2; SqlStmtList stmts = 3; }
  string db_path = 4;  // empty → embedded example.db
}
```

### Server change (`sqlite/server.go`)

Replace `extract()` with `ResolveBackend("", req.GetDbPath())`. The existing
`ResolveBackend` already treats an empty string as "use the embedded default" —
no new logic required there.

### UDS scope

`db_path` is intentionally **not** threaded through the UDS wire. The `sqlited`
daemon receives `--db` at startup; its database is fixed per-process. When
`socket_uri` is set, `db_path` is ignored. Document this in the field comment.

### Security note

Add a comment that `db_path` accepts arbitrary filesystem paths and is for
trusted callers only. No server-side allowlist is required for the internal
proto-indexer use case, but the field comment must make this explicit.

### Tests

No existing test exercises `db_path`. Add a case in `server_test.go` that
creates a temp db file with a known schema, issues a query with `db_path` set,
and verifies the correct db was queried.

### Files touched

- `sqlite_service.proto` — add field 4
- `sqlite/server.go` — plumb `req.GetDbPath()` into `ResolveBackend`
- `sqlite/pb/` — regenerated

---

## Issue #2 — Binary-safe output via `.mode quote`

Replace the `-csv -header` invocation with `.mode quote` so BLOB columns survive
the CLI round-trip. Change `Row.cell` from `repeated string` to `repeated bytes`.

### CLI invocation change

**Before:**
```go
exec.CommandContext(ctx, bin, "-csv", "-header", db, sql)
```

**After:**
```go
exec.CommandContext(ctx, bin, "-cmd", ".headers on", "-cmd", ".mode quote", db, sql)
```

`-cmd` flags run before the SQL argument. Apply the same change in
`handleUDSConn` in `sqlite/uds.go`.

### Output format

With `.headers on` + `.mode quote`, sqlite3 emits:

```
col1,col2,col3
'text value',42,X'deadbeef'
NULL,'it''s a quote',3.14
```

- **Header line**: bare comma-separated names, no quoting — split on `,` directly.
- **Value lines**: SQL literal per cell, comma-separated. Cell forms:
  - `'...'` — text; `''` inside is a literal single-quote
  - `X'...'` / `x'...'` — blob; hex payload to decode to `[]byte`
  - `NULL` — null value
  - bare integer or float literal

### Parser: replace `parseCSV` with `parseQuote`

`parseCSV` (uses `encoding/csv`) is replaced by `parseQuote`, a small
quote-aware scanner. Logic for each value line:

```
state: between-cells
  '       → enter text literal; scan until unescaped '
              ('' inside literal = one literal quote, not end-of-token)
  X' / x' → enter blob literal; scan hex chars until '
  N       → expect NULL keyword
  -/digit → scan integer or real until , or EOL
  ,       → emit cell, advance to next cell
  EOL     → emit last cell, advance to next row
```

Return type per cell: `[]byte`. Text and number cells return their string
representation as bytes. Blob cells return decoded binary (not the hex string).
Null cells return `nil`.

### Proto change — `Row.cell`

```proto
// Before
message Row { repeated string cell = 1; }

// After
message Row { repeated bytes cell = 1; }
```

`string` and `bytes` are wire-identical in proto3 (field 1, wire type LEN), so
there is no on-wire breakage. Generated Go changes from `[]string` to `[][]byte`.

### Null representation

`bytes` fields cannot distinguish nil from `[]byte{}` without extra signalling.
SQL NULL must be distinguishable from an empty blob. Add a parallel field:

```proto
message Row {
  repeated bytes cell      = 1;
  repeated bool  cell_null = 2;  // true at index i means cell[i] is SQL NULL
}
```

`cell_null` is populated for every row. Callers that do not care about NULL can
ignore it.

### Test sites to update

`server_test.go` and `uds_test.go` use `GetCell()` as `[]string`. After the
change, comparisons become:

```go
// Before
eq(got, []string{"1", "sprocket", "3"})
got != "typed_ct"

// After
string(got[0]) == "1"
string(got) != "typed_ct"
```

Search: `GetCell`, `Row{`, `eq(`, `.Cell` across `sqlite/*_test.go`.

### Files touched

- `sqlite_service.proto` — `Row.cell` type + add `cell_null`
- `sqlite/server.go` — CLI args + replace `parseCSV` with `parseQuote`
- `sqlite/uds.go` — CLI args in `handleUDSConn`
- `sqlite/server_test.go`, `sqlite/uds_test.go` — string→bytes comparisons
- `sqlite/pb/` — regenerated

---

## Issue #3 — Parameter binding

Allow callers to pass typed values that the server substitutes for `?`
placeholders, eliminating caller-side escaping and the SQL-injection surface.

### Proto additions (`sqlite_service.proto`)

```proto
message Value {
  oneof v {
    string text    = 1;
    int64  integer = 2;
    double real    = 3;
    bytes  blob    = 4;
    bool   null    = 5;  // true = SQL NULL; false is not a valid state
  }
}

message QueryRequest {
  string socket_uri    = 1;
  oneof body { string sql = 2; SqlStmtList stmts = 3; }
  string db_path       = 4;
  repeated Value param = 5;
}
```

### Server-side binding (`sqlite/server.go` + new `sqlite/bind.go`)

After SQL is rendered from `body`, if `len(req.GetParam()) > 0`, call
`bindParams(sql, params)` before exec. Extract into `sqlite/bind.go` so it can
be unit-tested independently.

`bindParams` does a single left-to-right scan, replacing `?` tokens that are
**not inside string literals** with the next param's escaped form:

| Value type | Escaped form                                    |
|------------|-------------------------------------------------|
| `text`     | `'` + replace all `'` with `''` + `'`          |
| `integer`  | `strconv.FormatInt(n, 10)`                      |
| `real`     | `strconv.FormatFloat(f, 'g', -1, 64)`           |
| `blob`     | `x'` + `hex.EncodeToString(b)` + `'`           |
| `null`     | `NULL`                                          |

The `?` scanner must skip `'...'` literals in the SQL (using a `''`-aware state
machine, same as the `parseQuote` output scanner) to avoid replacing `?`
characters that appear inside string values. Return an error if the param count
does not match the `?` count.

Named parameters (`:name`, `@name`, `$name`) are out of scope — positional `?`
only.

### UDS path

`bindParams` runs in the gRPC server before SQL is sent over UDS, so the daemon
receives already-substituted SQL. No UDS wire format change is needed.

### Transactions

`BEGIN; INSERT ...; INSERT ...; COMMIT;` as a single `Query` call already works
because the entire payload goes to one sqlite3 invocation. Cross-call sessions
are impossible without a persistent connection and are explicitly out of scope.
Document this in the `QueryRequest` comment.

### Files touched

- `sqlite_service.proto` — add `Value` message + `param` field 5
- `sqlite/bind.go` — new file: `bindParams` + `escapeSQLValue`
- `sqlite/bind_test.go` — new file: unit tests for escaping and `?` counting
- `sqlite/server.go` — call `bindParams` after SQL render
- `sqlite/pb/` — regenerated

---

## Build / regeneration notes

Each issue requires running `build.sh` after the proto change to regenerate
`sqlite/pb/`. Field numbers chosen (4, 5) do not conflict with existing fields.
No existing message is removed or renumbered, so proto-level wire compatibility
is maintained across all three issues.
