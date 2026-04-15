# proto-sqlite

Elevating sqlite into a fully protobuf-encoded, grpc-compatible interface similar to googlesql.

## Goal

Encode sqlite's language / structure into protobuf at a deep, language-spec level.

The base production rule / parse structure is `sql-stmt-list`, consisting of `sql-stmt;sql-stmt...` from 0 to N statements. `sql-stmt` is `EXPLAIN [ QUERY PLAN ]` followed by one of:

alter-table-stmt analyze-stmt attach-stmt begin-stmt commit-stmt create-index-stmt create-table-stmt create-trigger-stmt create-view-stmt create-virtual-table-stmt delete-stmt delete-stmt-limited detach-stmt drop-index-stmt drop-table-stmt drop-trigger-stmt drop-view-stmt insert-stmt pragma-stmt reindex-stmt release-stmt rollback-stmt savepoint-stmt select-stmt update-stmt update-stmt-limited vacuum-stmt

Structure per stmt is documented at `https://sqlite.org/syntax/<name>.html`. URL list: `sqlite-doc-urls.csv`.

## Tasks

1. **Screenshot each syntax diagram** into `docs/sqlite-parse-img/<NAME>.png` (bounding-box-clipped). SVG text can also be extracted directly from the DOM. Feed screenshots to multimodal models to transcribe into production rules. In the SVG: nodes listed in "References" are production rules; other labels are usually user-provided strings.
2. **Convert to gluon descriptors**: build `sqlite-lex.textproto` and `sqlite-grammar.textproto` (see `github.com/accretional/gluon` — `LexDescriptor` in `lex.proto`, `GrammarDescriptor` in `grammar.proto`).
3. **Keyword/symbol enums + empty messages**: `keywords.proto` (enum + one empty `message ALTER {}` etc. per keyword), `symbols.proto` (same pattern for symbols). Pattern borrowed from googlesql.
4. **Stmt messages**: e.g. `message AlterTable { Alter alter = 1; Table table = 2; string schema_name = 3; Dot dot = 4; string table_name = 5; ... }` with intermediate parses (`ColumnDef`, `ColumnConstraint`, …) as their own messages.
5. **gRPC server in Go**: `go:embed` the sqlite binary + an example db, expose `service Sqlite { rpc Query(SqlStmtList) returns (...) }`.

## Directory Layout

- `lang/` — parsing logic, screenshot-to-grammar pipeline, textproto outputs
- `sqlite/` — protobuf service, gRPC server, embedded sqlite binary + example db
- `protos/` — shared .proto files (keywords, symbols, stmts, service)
- `docs/sqlite-parse-img/` — per-stmt syntax screenshots
- `scripts/` — helper scripts invoked by the top-level shell scripts
- `third_party/` — vendored/cloned external tools (chromerpc, sqlite binary)

## Build Discipline

- **NEVER build/test/run code outside of `setup.sh`, `build.sh`, `test.sh`, `LET_IT_RIP.sh`.** Add any new command to the relevant script.
- **NEVER commit or push without running these.**
- Scripts are idempotent and chained: `build.sh` runs `setup.sh`, `test.sh` runs `build.sh`, `LET_IT_RIP.sh` runs `test.sh`.
- `LET_IT_RIP.sh` executes queries against the actual embedded sqlite db.

## Tooling

- Go 1.26.
- Screenshots: `github.com/accretional/chromerpc` (the repo is named `chromerpc`, not `chrome-rpc`). Automation is declared in textproto `AutomationSequence` files; run with `go run ./cmd/automate -input <file>.textproto`. Step types include `navigate`, `set_viewport`, `wait_for_selector`, `screenshot`, `full_page_screenshot`, `evaluate_script`. `evaluate_script` results come back in `StepResult.script_result` — useful for extracting SVG `<text>` content directly.
- Gluon descriptors: `github.com/accretional/gluon` — reuse its proto types rather than reinventing.

## References

- sqlite docs: https://sqlite.org/
- gluon: https://github.com/accretional/gluon (sibling at `/Volumes/wd_office_1/repos/gluon`)
- chromerpc: https://github.com/accretional/chromerpc
