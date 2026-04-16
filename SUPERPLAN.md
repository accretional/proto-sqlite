# SUPERPLAN

Cross-repo plan for eliminating hand-written stmt protos in proto-sqlite by
mechanically deriving them from the EBNF grammar. Spans four repos:

- **proto-sqlite** (this repo) — consumer
- **gluon** (`/Volumes/wd_office_1/repos/gluon`) — hosts `Metaparser` service
- **proto-expr** (`github.com/accretional/proto-expr`) — S-expr body encoding
- **proto-type** (`github.com/accretional/proto-type`) — `Typer.Resolve`

## Vision

```
                                                       ┌─► keywords (dedup'd,
                                                       │    _kw-suffixed on name
                                                       │    collisions)
sqlite.ebnf ─► lexkit.Parse ─► GrammarDescriptor ──────┤
                                (with Expression body) │
                                          │            └─► one DescriptorProto per
                                          │                 production (alt→oneof,
                                          ▼                 rep→repeated, opt→optional,
                                   Metaparser.Proto          nonterm→typed field,
                                          │                 Data.type resolved via Typer)
                                          ▼
                                FileDescriptorProto ─► protoc ─► Go types
                                                                  │
                                                                  ▼
                                                         Sqlite.Query gRPC server
```

The Sqlite service takes a typed `SqlStmtList` over the wire, serializes it
back to SQL text, shells out to the embedded `sqlite3`, returns rows.

## Data model (cross-repo)

### proto-expr — already exists

```proto
message Data { string type = 1; oneof encoding { string text = 2; bytes binary = 3; } }
message Expression {
  message Cell { Expression lhs = 1; Expression rhs = 2; }
  oneof content { string str = 1; string uri = 2; Data data = 3; Cell cell = 4; }
}
```

S-expr encoding conventions for EBNF RHS:
- `("seq" . args)` — concatenation
- `("alt" . args)` — alternation → `oneof`
- `("opt" . body)` — `[x]` → optional / singular presence
- `("rep" . body)` — `{x}` → `repeated`
- `("term" . "BEGIN")` — quoted terminal → keyword message
- `("nonterm" . "savepoint_name")` — identifier → field of that message type

`Cell` is right-nested: `(a b c)` = `Cell{a, Cell{b, Cell{c, nil}}}`.

### proto-type — needs work

Today `typer.proto` references `DescriptorProto` without an import and
`Type` is just `string name`. We'll:
1. Add `import "google/protobuf/descriptor.proto";` to typer.proto.
2. Keep `Type { string name = 1; }` (sufficient for v1 — a fully-qualified
   name is enough to look up a DescriptorProto in a registry).
3. Implement a Go server (`proto-type/cmd/typer`) that resolves names
   against a `FileDescriptorSet` registry. Seed with well-known types
   (`google.protobuf.*`) and let callers register additional sets.

### gluon — two changes

1. **Extend `ProductionDescriptor`** (`grammar.proto`):
   ```proto
   import "github.com/accretional/proto-expr/expression.proto";
   message ProductionDescriptor {
     string name = 1;
     TokenDescriptor token = 2;          // keep; raw RHS string
     proto_expr.Expression body = 3;     // NEW: structured RHS
   }
   ```
   Populate `body` inside `lexkit.Parse` by running the same parser that
   builds the internal `lexkit.Expr` tree, then walking that tree into
   the S-expr encoding above. Keep `token` for backcompat.

2. **Add Metaparser service** (new file `metaparser.proto` in gluon):
   ```proto
   import "google/protobuf/descriptor.proto";
   service Metaparser {
     rpc Proto(LanguageDescriptor) returns (google.protobuf.FileDescriptorProto);
   }
   ```
   Go implementation under `gluon/metaparser/`:
   - For each production, emit `DescriptorProto{name: PascalCase(prod.name)}`.
   - Walk `prod.body`:
     - `seq` children become sequential fields (field numbers 1..N).
     - `alt` becomes a `oneof` with one field per variant.
     - `rep` sets `LABEL_REPEATED` on the wrapped field.
     - `opt` in proto3 is implicit (all singular fields are optional); we
       still set `proto3_optional = true` for clarity on scalar fields.
     - `term "FOO"` → refer to a dedup'd keyword message `message FooKw {}`.
     - `nonterm "foo"` → field typed `.sqlite.Foo`.
     - `data` → call `Typer.Resolve(Data.type)` to get the referenced
       DescriptorProto; embed by name.
   - Emit the union `message SqlStmt { oneof stmt { ... } }` from the
     top-level `sql_stmt` alternation.

## Name-collision rules

- Keyword messages get `_Kw` suffix: `TABLE` → `message TableKw {}`.
  Terminals always get the suffix so there is never ambiguity with a
  nonterminal field named `table`.
- Field names stay snake_case of the production name: nonterm `table_name`
  becomes field `table_name` of type `TableName`.
- Recursive productions (`expr`) are allowed directly — proto3 supports
  recursive message types natively.

## Phases and ordering

1. **proto-type** — add import, implement Typer server + in-memory registry, unit tests.
2. **proto-expr** — no changes; use as-is.
3. **gluon** — extend `ProductionDescriptor`, write `Expr→Expression` serializer, add `metaparser.proto` + Go implementation, tests on the EBNF self-grammar.
4. **proto-sqlite** — call Metaparser on the sqlite `LanguageDescriptor`, write result to `protos/sqlite_stmts.proto`, wire protoc into `build.sh`, use generated types in `sqlite/server.go`.

## Risks / open questions

- **Expr is currently internal to lexkit.** Exposing its shape in proto-expr form may leak implementation detail; acceptable since proto-expr is explicitly designed as a universal AST carrier.
- **Recursive `expr` with 30+ alternation arms** produces a large oneof. Works; noisy. v1 acceptable.
- **Field numbering stability**: Metaparser assigns 1..N in source order. Any edit to `sqlite.ebnf` can renumber fields. Acceptable while the grammar is under active development; before public release we'll need stable field-number assignments (probably a checked-in `.fieldmap` textproto).
- **proto-type scope**: `Type{string name}` is minimal. If dynamic type resolution later needs versioning, parameters, or generics, Type grows. Not blocking.

## Status (see task list for current state)

- ✅ Screenshots, ebnf subset, gengrammar round-trip.
- ⏳ SUPERPLAN (this doc).
- ⏳ proto-type Typer.
- ⏳ gluon ProductionDescriptor.body + Metaparser.
- ⏳ proto-sqlite wiring.
