# lang/

Pipeline that turns sqlite.org's railroad diagrams into a
`gluon.v2.GrammarDescriptor` and then into a `FileDescriptorProto` of
stmt messages.

## Files

| File | Kind | Author |
|---|---|---|
| `sqlite-lex.textproto`     | hand-authored EBNF lex (retained for docs) | hand-written |
| `sqlite.ebnf`              | EBNF source text                           | hand-written |
| `sqlite-grammar.textproto` | `gluon.v2.GrammarDescriptor`               | **generated** (`cmd/gengrammar`) |
| `sqlite.fdset`             | `FileDescriptorSet` (one entry, `sqlite.proto`) | **generated** (`cmd/genproto`) |
| `protos/sqlite/*.proto`    | per-message split of the stmt protos       | **generated** (`cmd/genproto`) |
| `cmd/gengrammar/main.go`   | EBNF → grammar textproto                   | hand-written |
| `cmd/genproto/main.go`     | EBNF → grammar → AST → `FileDescriptorProto` | hand-written |
| `grammar_test.go`          | grammar textproto sanity check             | hand-written |
| `metaparser_test.go`       | EBNF → Compile e2e via gluon v2 gRPC       | hand-written |
| `automations/`             | chromerpc textproto templates              | hand-written |

## Flow

```
sqlite.org ─► PNG ─► sqlite.ebnf
 (screenshot)        (transcribed from PNGs by a multimodal model)
                      │
                      ▼
                  gengrammar  ──► sqlite-grammar.textproto
                  (Metaparser.EBNF)
                      │
                      ▼
                  genproto    ──► sqlite.fdset + sqlite.proto + protos/sqlite/*.proto
                  (GrammarToAST + compiler.Compile)
```

Both generators call into gluon v2 (`v2/metaparser` and `v2/compiler`),
so the entire pipeline is driven by the same language-agnostic
descriptor shapes exposed as gRPC by `gluon.v2.Metaparser`.

### Step 1 — screenshot

`scripts/screenshot_all.sh` spins up one chromerpc server and drives
Chrome through `automations/screenshot_stmt.textproto.tmpl` once per
stmt. Output: `docs/sqlite-parse-img/<name>.png` +
`<name>.svg-strings.json` (SVG `<text>` nodes extracted via
`evaluate_script`).

### Step 2 — transcribe

Images are fed to a multimodal model that writes EBNF productions into
`sqlite.ebnf`. Diagram labels in ALL CAPS become quoted terminals
(`"BEGIN"`); lowercase/hyphenated labels become nonterminals. The
SVG-string JSON is the ground truth for what text nodes actually
appear in each diagram.

Naming: sqlite's diagrams use hyphens (`alter-table-stmt`). Gluon's
EBNF parser only accepts `[letter|digit|_]` in identifiers, so names
are rewritten to snake_case (`alter_table_stmt`). The mapping is 1:1.

### Step 3 — generate grammar

`go run ./lang/cmd/gengrammar` reads the EBNF and writes the grammar:

```go
src, _ := os.ReadFile("lang/sqlite.ebnf")
doc    := metaparser.WrapString(string(src))
gd,  _ := metaparser.ParseEBNF(doc)
prototext.Marshal(gd) → lang/sqlite-grammar.textproto
```

### Step 4 — generate protos

`go run ./lang/cmd/genproto` reads the same EBNF, runs the full
pipeline, and emits stmt protos:

```go
src, _ := os.ReadFile("lang/sqlite.ebnf")
doc    := metaparser.WrapString(string(src))
gd,  _ := metaparser.ParseEBNF(doc)
ast, _ := compiler.GrammarToAST(gd)
fdp, _ := compiler.Compile(ast, compiler.Options{Package: "sqlite"})
// serialize to lang/sqlite.fdset, sqlite.proto, protos/sqlite/*.proto
```

One proto message per rule; keyword terminals are deduplicated into
empty messages appended after the rule messages.

## Regenerating

```
go run ./lang/cmd/gengrammar   # ebnf  → grammar textproto
go run ./lang/cmd/genproto     # ebnf  → fdset + .proto files
go test ./lang/                # sanity: grammar loads, Compile e2e passes
```

`./test.sh` runs the tests; the generators are invoked manually when
`sqlite.ebnf` changes.
