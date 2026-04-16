# proto-sqlite

Elevating sqlite into a fully protobuf-encoded, grpc-compatible interface similar to googlesql.

## Goal

Encode sqlite's language / structure into protobuf at a deep, language-spec level.

The base production rule / parse structure is `sql-stmt-list`, consisting of `sql-stmt;sql-stmt...` from 0 to N statements. `sql-stmt` is `EXPLAIN [ QUERY PLAN ]` followed by one of:

alter-table-stmt analyze-stmt attach-stmt begin-stmt commit-stmt create-index-stmt create-table-stmt create-trigger-stmt create-view-stmt create-virtual-table-stmt delete-stmt delete-stmt-limited detach-stmt drop-index-stmt drop-table-stmt drop-trigger-stmt drop-view-stmt insert-stmt pragma-stmt reindex-stmt release-stmt rollback-stmt savepoint-stmt select-stmt update-stmt update-stmt-limited vacuum-stmt

Structure per stmt is documented at `https://sqlite.org/syntax/<name>.html`. URL list: `sqlite-doc-urls.csv`.

## Approach

The stmt messages are *mechanically derived* from SQLite's EBNF grammar — not hand-maintained. `lang/cmd/genproto` drives a gluon v2 pipeline (`ParseEBNF` → `GrammarToAST` → AST transforms → `Compile`) that turns `lang/sqlite.ebnf` into a `FileDescriptorProto`; protoc emits the Go types; the `Sqlite.Query` server renders typed `SqlStmtList` instances back to SQL via proto reflection and shells out to an embedded `sqlite3`.

See **[GLUON_GUIDE.md](GLUON_GUIDE.md)** for the full pipeline, the role each gluon API plays, and what proto-sqlite contributes locally (`scalarizeX`, the reflection renderer).

The initial syntax-diagram capture work used `github.com/accretional/chromerpc` to screenshot each `https://sqlite.org/syntax/<name>.html` diagram into `docs/sqlite-parse-img/<NAME>.png`; those images guided the hand-transcription of `lang/sqlite.ebnf` but are no longer in the build path.

## Directory Layout

- `lang/` — `sqlite.ebnf`, the genproto/gengrammar drivers, and split protos
- `lang/protos/sqlite/` — generated per-message `.proto` files
- `sqlite.proto` — generated bundled proto at the repo root
- `sqlite/` — `Sqlite.Query` gRPC server, reflection renderer, embedded sqlite binary + example db
- `sqlite/pb/` — generated Go types + `prefix_map.go`
- `docs/sqlite-parse-img/` — per-stmt syntax screenshots (historical reference)
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
