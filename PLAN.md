# Plan

## Setup (done)

- [x] Project scaffold: `lang/`, `sqlite/`, `protos/`, `docs/sqlite-parse-img/`, `scripts/`, `third_party/`
- [x] `setup.sh` / `build.sh` / `test.sh` / `LET_IT_RIP.sh` — chained + idempotent
- [x] `go.mod` at `github.com/accretional/proto-sqlite`, go 1.26
- [x] sqlite3 3.50.0 downloaded to `sqlite/bin/sqlite3`; example db at `sqlite/db/example.db`
- [x] Go embed (`sqlite/embed.go`) with passing test (`SELECT SUM(qty) FROM widgets` → 22)
- [x] chromerpc cloned + built (server + automate binaries)
- [x] Screenshot validated on `alter-table-stmt`: PNG in `docs/sqlite-parse-img/` + 37 SVG text strings extracted to `.svg-strings.json`

## Remaining work

### Phase 1 — Screenshot & string extract every stmt

- [ ] `scripts/screenshot_all_stmts.sh` — iterate the stmt list from CLAUDE.md, reusing one chromerpc server. Also screenshot each referenced intermediate (column-def, conflict-clause, expr, …). Keep images bounding-box-clipped via `evaluate_script` + CDP `Page.captureScreenshot` `clip` param (add a `clipped_screenshot` step upstream to chromerpc if the existing `screenshot` step doesn't expose clip).
- [ ] Store per-stmt output: `docs/sqlite-parse-img/<name>.png` + `<name>.svg-strings.json`.

### Phase 2 — gluon descriptors

- [ ] `sqlite-lex.textproto` — `gluon.LexDescriptor`. Model on `gluon/lexkit/ebnf_grammar.textproto`. Whitespace, terminals, comment delimiters.
- [ ] `sqlite-grammar.textproto` — `gluon.GrammarDescriptor` referencing the lex descriptor; one `ProductionDescriptor` per stmt + intermediate.
- [ ] Wire gluon as a go dep in `go.mod`; write a small test that loads both textprotos via `prototext.Unmarshal`.

### Phase 3 — keyword + symbol protos

- [ ] `protos/keywords.proto`: `enum Keyword { KEYWORD_UNSPECIFIED = 0; ALTER = 1; TABLE = 2; ... }` plus one `message ALTER {}` etc. per keyword. Generate from the union of SVG text strings where `isupper(s)` across all pages.
- [ ] `protos/symbols.proto`: same pattern for non-alphanumeric terminals (`.`, `(`, `)`, `,`, …). Likely hand-curated from extracted strings.

### Phase 4 — stmt protos

- [ ] One message per stmt (`AlterTable`, `CreateTable`, `Select`, …) with fields in source order: keyword-messages, name strings, intermediate messages, repeated as needed. Multimodal-model transcription from the PNGs; validate fields against the SVG-string lists.
- [ ] `protos/sql_stmt_list.proto`: `message SqlStmtList { repeated SqlStmt stmts = 1; }` with `oneof` over all stmt types (including optional `Explain` / `ExplainQueryPlan` wrapper).

### Phase 5 — gRPC service

- [ ] `protos/sqlite_service.proto`: `service Sqlite { rpc Query(SqlStmtList) returns (QueryResult); }`
- [ ] `sqlite/server.go`: embed sqlite binary + example db (already done); serialize `SqlStmtList` back to SQL text, shell out to embedded sqlite3, return rows.
- [ ] Wire into `LET_IT_RIP.sh` so it spins up the gRPC server and round-trips a `SqlStmtList` call.

### Phase 6 — Validation

- [ ] Golden tests: parse representative SQL → `SqlStmtList` textproto (eventually via lex+parse built from the gluon grammar). For now, hand-crafted textprotos into the server.
- [ ] `test.sh` runs full `go test ./...` across lang/, sqlite/, and any proto validation.

## Open questions

- Does `chromerpc`'s `screenshot` step already support a `clip` rect? If not, upstream a small patch or use `evaluate_script` to scroll+crop via a canvas.
- How to handle the `expr` production (recursive, large)? Probably encode as a proto `oneof` over literal / column-ref / binop / funccall / subquery, with a separate grammar file for the full precedence table.
