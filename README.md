# proto-sqlite

A fully protobuf-encoded, gRPC-compatible interface to SQLite — in the spirit
of googlesql, but derived mechanically from SQLite's EBNF grammar rather
than hand-maintained.

The wire format for `Sqlite.Query` is a typed `SqlStmtList` message whose
shape mirrors the SQLite syntax specification at the language level. A
server embeds the `sqlite3` binary and an example db, serializes typed
requests back to SQL text via proto reflection, and returns the rows.
A raw-SQL escape hatch on the same oneof is kept for convenience.

## How it's built

```
lang/sqlite.ebnf
      │
      │   gluon v2: ParseEBNF → GrammarToAST
      │   AST transforms:  CollapseCommaList → NameSequence →
      │                    scalarizeX (proto-sqlite local) →
      │                    StripKeywords
      │   gluon v2: Compile
      ▼
FileDescriptorProto ──► sqlite.proto, lang/protos/sqlite/*.proto
                    └─► protoc → sqlite/pb/*.pb.go
      │
      ▼
Typed SqlStmtList ──► RenderSQL (reflection) ──► sqlite3 ──► QueryResponse
```

One ~160-line driver (`lang/cmd/genproto`) runs the whole pipeline;
editing `lang/sqlite.ebnf` and re-running `./build.sh` regenerates all
222 proto messages, a keyword-prefix map, and the generated Go types.
The SQL renderer is ~100 lines and covers every production reflectively —
adding a statement is zero renderer work.

See **[GLUON_GUIDE.md](GLUON_GUIDE.md)** for the full pipeline, the role
each gluon API plays, and what this integration demonstrates about
gluon's capabilities.

## Quick start

```
./setup.sh        # fetch sqlite binary + chromerpc (idempotent)
./build.sh        # regenerate protos; compile Go
./test.sh         # build + go test ./...
./LET_IT_RIP.sh   # build + test + live query against embedded sqlite
```

The scripts chain: `LET_IT_RIP.sh` ⊃ `test.sh` ⊃ `build.sh` ⊃ `setup.sh`.
There are no other entry points — do not build or test outside these.

## Using the service

```go
import (
    sqliteembed "github.com/accretional/proto-sqlite/sqlite"
    sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

srv := sqliteembed.NewServer()

// Typed request — renders to "DROP TABLE IF EXISTS ephemeral"
resp, err := srv.Query(ctx, &sqlitepb.QueryRequest{
    Body: &sqlitepb.QueryRequest_Stmts{
        Stmts: &sqlitepb.SqlStmtList{
            SqlStmt: []*sqlitepb.SqlStmt{{
                Alt1: &sqlitepb.SqlStmt_Alt1{
                    Value: &sqlitepb.SqlStmt_Alt1_DropTableStmt{
                        DropTableStmt: &sqlitepb.DropTableStmt{
                            IfExists:  &sqlitepb.DropTableStmt_IfExists{},
                            TableName: &sqlitepb.TableName{
                                Name: &sqlitepb.Name{Value: "ephemeral"},
                            },
                        },
                    },
                },
            }},
        },
    },
})

// Or the raw-SQL escape hatch
resp, err = srv.Query(ctx, &sqlitepb.QueryRequest{
    Body: &sqlitepb.QueryRequest_Sql{
        Sql: "SELECT id, name, qty FROM widgets ORDER BY id;",
    },
})
```

Standalone server: `go run ./sqlite/cmd/server` (listens on `:50051`).
In-process round-trip demo: `go run ./sqlite/cmd/ripgrpc`.

## Repo layout

| Path | Role |
|---|---|
| `lang/sqlite.ebnf` | Source grammar (human-edited) |
| `lang/cmd/genproto/` | Pipeline driver + `scalarizeX` transform |
| `lang/cmd/gengrammar/` | Grammar round-trip check |
| `sqlite.proto`, `lang/protos/sqlite/*.proto` | Generated protos |
| `lang/sqlite.fdset` | Serialized `FileDescriptorSet` |
| `sqlite/pb/` | Generated Go types + `prefix_map.go` |
| `sqlite/render.go` | Reflection-based SQL serializer |
| `sqlite/server.go` | `Sqlite.Query` gRPC server |
| `sqlite/cmd/server/` | Standalone server |
| `sqlite/cmd/ripgrpc/` | In-process round-trip demo |
| `docs/sqlite-parse-img/` | Per-stmt syntax screenshots (historical) |
| `docs/ORIGINAL_README.md` | Original project brief |
| `third_party/` | Vendored chromerpc + sqlite binary |

## Further reading

- **[GLUON_GUIDE.md](GLUON_GUIDE.md)** — how proto-sqlite and gluon compose
- **[SUPERPLAN.md](SUPERPLAN.md)** — cross-repo plan across gluon, proto-expr, proto-type
- **[CLAUDE.md](CLAUDE.md)** — build discipline and working conventions
- **[docs/ORIGINAL_README.md](docs/ORIGINAL_README.md)** — the original brief

## Dependencies

- Go 1.26
- [gluon](https://github.com/accretional/gluon) — EBNF parsing + AST-to-proto compiler
- [chromerpc](https://github.com/accretional/chromerpc) — used to capture the SQLite railroad diagrams
- `sqlite3` binary — downloaded by `setup.sh` and embedded via `go:embed`
