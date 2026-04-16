# SUPERPLAN

Cross-repo plan for eliminating hand-written stmt protos in proto-sqlite by
mechanically deriving them from the EBNF grammar. Spans four repos:

- **proto-sqlite** (this repo) — consumer
- **gluon** (`/Volumes/wd_office_1/repos/gluon`) — hosts `Metaparser`,
  `Transformer`, and the v2 descriptor protos
- **proto-expr** (`github.com/accretional/proto-expr`) — hosts the
  `Protosh.Run` scripting runtime and the `Data` / `ScriptDescriptor` types
- **proto-type** (`github.com/accretional/proto-type`) — hosts `Typer.Resolve`

## Vision

The end-to-end pipeline is a sequence of gRPC calls threaded together
by proto-expr's Protosh runtime:

```
sqlite.ebnf
  │
  ▼                                          ┌──────────────────────────┐
Metaparser.ReadBytes → TextDescriptor        │  Metaparser.Transform    │
Metaparser.ReadString → DocumentDescriptor   │  runs a ScriptDescriptor │
Metaparser.EBNF → GrammarDescriptor          │  over the ASTDescriptor: │
Metaparser.CST → ASTDescriptor ──────────────►  • Filter / Replace via  │
                                             │    astkit://<Method>     │
                                             │  • Resolve proto types   │
                                             │    via typer://Resolve   │
                                             │  • Lower to proto via    │
                                             │    protoc://Compile      │
                                             └──────────────┬───────────┘
                                                            │
                                                            ▼
                                                   FileDescriptorProto
                                                            │
                                                 protoc → Go types
                                                            │
                                                            ▼
                                                Sqlite.Query gRPC server
```

The Sqlite service takes a typed `SqlStmtList` over the wire, serializes it
back to SQL text, shells out to the embedded `sqlite3`, returns rows.

## Architecture (v2)

### proto-expr — the driver

Provides `Data`, `ScriptDescriptor`, `StatementDescriptor`,
`DispatchDescriptor`, and the `Protosh.Run` runtime. A script is a
sequence of statements over a register map: Import, const/mutable
Variable, Dispatch, Expression (not yet implemented).

Dispatch URIs are exact-match and pluggable — each host registers
handlers before calling `Run`. The convention for per-dispatch params
is to pack them into `Data.type` as `k=v,k2=v2`; the runtime preserves
caller-supplied `Data.type` across register resolution so the script
can pass small parameters without an extra plumbing layer. See
`proto-expr/README.md` for the detailed contract.

### gluon v2 — Metaparser + astkit

- `v2/metaparser` exposes 5 RPCs: `ReadBytes`, `ReadString`, `EBNF`,
  `CST`, `Transform`.
- `Transform(ASTDescriptor, script_textproto)` embeds Protosh as a
  library. It pre-populates the `ast` register with the request's
  `ASTNode` (bare, not the full descriptor) and wires the Transformer
  service under URIs of the form `astkit://<Method>`. See
  `v2/metaparser/transform.go`.
- `v2/astkit` is a language-agnostic set of `ASTNode` tree operations
  (Walk, Find, FindAll, Count, ReplaceKind, ReplaceValue, Filter) with
  both a pure-Go API and a gRPC `Transformer` service. It's the first
  set of handlers the Transform RPC wires in.

### proto-type — typed lookups

`Typer.Resolve(Type{name}) → DescriptorProto` against a
`FileDescriptorSet` registry. Used by the (TBD) proto-lowering
handler to embed typed fields when the grammar references a non-local
type.

### proto-sqlite — consumer

Calls `Metaparser.ReadString` → `EBNF` → `CST` → `Transform` on
`sqlite.ebnf` + a lowering script, writes the resulting
`FileDescriptorProto` to `protos/sqlite_stmts.proto`, runs protoc,
uses the generated types in `sqlite/server.go`.

## S-expression conventions (EBNF RHS)

Used inside `ProductionDescriptor.body` (proto-expr `Expression`):

- `("seq" . args)` — concatenation
- `("alt" . args)` — alternation → `oneof`
- `("opt" . body)` — `[x]` → optional / singular presence
- `("rep" . body)` — `{x}` → `repeated`
- `("term" . "BEGIN")` — quoted terminal → keyword message
- `("nonterm" . "savepoint_name")` — identifier → field of that message type

`Cell` is right-nested: `(a b c)` = `Cell{a, Cell{b, Cell{c, nil}}}`.

## Name-collision rules

- Keyword messages get `_Kw` suffix: `TABLE` → `message TableKw {}`.
  Terminals always get the suffix so there is never ambiguity with a
  nonterminal field named `table`.
- Field names stay snake_case of the production name: nonterm `table_name`
  becomes field `table_name` of type `TableName`.
- Recursive productions (`expr`) are allowed directly — proto3 supports
  recursive message types natively.

## Status

- ✅ Screenshots, EBNF subset, gengrammar round-trip.
- ✅ proto-type Typer service (task #10).
- ✅ gluon `ProductionDescriptor.body` + v1 Metaparser (tasks #11, #12).
- ✅ gluon v2 descriptor protos, ReadBytes/ReadString/EBNF/CST RPCs
  (tasks #14–#18).
- ✅ gluon v2 `astkit` package + Transformer service (tasks #19, #20).
- ✅ proto-expr `Protosh.Run` scripting runtime (task #21).
- ✅ gluon v2 `Metaparser.Transform` RPC wrapping Protosh.Run (task #22).
- ✅ gluon v2 `compiler` package + `Compile` RPC + `protoc://Compile`
  Transform handler (task #23). Lowers a schema-shaped `ASTDescriptor`
  to a `FileDescriptorProto`; `compiler.GrammarToAST` bridges from a
  flat `GrammarDescriptor` for callers that hold a grammar.
- ✅ proto-sqlite cutover to v2 pipeline (task #23, phase D).
  `lang/cmd/gengrammar` and `lang/cmd/genproto` now call gluon v2
  (`ParseEBNF`, `GrammarToAST`, `compiler.Compile`); all v1 gluon
  imports are gone. Output is byte-identical to the previous v1 run.
- ⏳ Task #8 — stmt protos + Sqlite.Query — is the consumer end.

## Risks / open questions

- **Recursive `expr` with 30+ alternation arms** produces a large
  oneof. Works; noisy. v1 acceptable.
- **Field numbering stability**: lowering assigns 1..N in source
  order. Any edit to `sqlite.ebnf` can renumber fields. Acceptable
  while the grammar is under active development; before public
  release we'll need stable assignments (probably a checked-in
  `.fieldmap` textproto).
- **proto-type scope**: `Type{string name}` is minimal. If dynamic
  type resolution later needs versioning, parameters, or generics,
  `Type` grows. Not blocking.
