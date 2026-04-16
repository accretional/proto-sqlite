// Command genproto reads the sqlite EBNF, runs gluon v2's pipeline
// (ParseEBNF → GrammarToAST → compiler.Compile) to produce a
// FileDescriptorProto, serializes it as a FileDescriptorSet, writes
// the bundled .proto to the repo root, and splits individual message
// .proto files into lang/protos/ via proto-merge.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/accretional/gluon/v2/compiler"
	metaparserv2 "github.com/accretional/gluon/v2/metaparser"
	pb "github.com/accretional/gluon/v2/pb"
	"github.com/accretional/merge/descriptor"
	mergeproto "github.com/accretional/merge/proto"
)

func main() {
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	fdsetOut := flag.String("fdset", "lang/sqlite.fdset", "output FileDescriptorSet binary")
	bundledOut := flag.String("bundled", "sqlite.proto", "bundled .proto output in repo root")
	splitDir := flag.String("split-dir", "lang/protos", "output directory for split .proto files")
	prefixMapOut := flag.String("prefix-map", "sqlite/pb/prefix_map.go", "output path for generated message-prefix map")
	pkgName := flag.String("package", "sqlite", "proto package name")
	goPkg := flag.String("go-package", "github.com/accretional/proto-sqlite/sqlite/pb;sqlitepb", "go_package file option (empty = omit)")
	flag.Parse()

	src, err := os.ReadFile(*ebnfPath)
	if err != nil {
		log.Fatalf("read ebnf %s: %v", *ebnfPath, err)
	}

	doc := metaparserv2.WrapString(string(src))
	doc.Name = *ebnfPath

	gd, err := metaparserv2.ParseEBNF(doc)
	if err != nil {
		log.Fatalf("ParseEBNF: %v", err)
	}

	ast, err := compiler.GrammarToAST(gd)
	if err != nil {
		log.Fatalf("GrammarToAST: %v", err)
	}
	ast.Language = *pkgName

	ast.Root = compiler.CollapseCommaList(ast.Root)
	ast.Root = compiler.NameSequence(ast.Root)

	// Collect leading-terminal prefixes and keyword-message literals
	// before StripKeywords discards them. The compiler's OnMessage hook
	// reports each emitted message with the AST node that produced it;
	// for rules and nested wrappers we extract the leading terminal
	// run, and for keyword messages we record the literal token.
	prefixes := map[string][]string{}
	if _, err := compiler.Compile(ast, compiler.Options{
		Package: *pkgName,
		OnMessage: func(fqn string, node *pb.ASTNode) {
			if node.GetKind() == compiler.KindTerminal {
				prefixes[fqn] = []string{node.GetValue()}
				return
			}
			var kids []*pb.ASTNode
			switch node.GetKind() {
			case compiler.KindSequence:
				kids = node.GetChildren()
			case compiler.KindRule:
				body := node.GetChildren()
				if len(body) == 1 && body[0].GetKind() == compiler.KindSequence {
					kids = body[0].GetChildren()
				}
			}
			// Alternation nodes contribute no prefix — their children
			// are variants, not an ordered token run.
			var toks []string
			for _, c := range kids {
				if c.GetKind() != compiler.KindTerminal {
					break
				}
				toks = append(toks, c.GetValue())
			}
			if len(toks) > 0 {
				prefixes[fqn] = toks
			}
		},
	}); err != nil {
		log.Fatalf("compiler.Compile (prefix pass): %v", err)
	}

	ast.Root = compiler.StripKeywords(ast.Root)

	fdp, err := compiler.Compile(ast, compiler.Options{
		Package:   *pkgName,
		GoPackage: *goPkg,
		FileName:  *bundledOut,
	})
	if err != nil {
		log.Fatalf("compiler.Compile: %v", err)
	}
	fmt.Printf("generated %d messages from %d rules\n",
		len(fdp.GetMessageType()), len(gd.GetRules()))

	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}
	blob, err := proto.Marshal(set)
	if err != nil {
		log.Fatalf("marshal fdset: %v", err)
	}
	if err := os.WriteFile(*fdsetOut, blob, 0o644); err != nil {
		log.Fatalf("write %s: %v", *fdsetOut, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *fdsetOut, len(blob))

	protoSrc, err := descriptor.ToString(fdp)
	if err != nil {
		log.Fatalf("descriptor.ToString: %v", err)
	}
	if err := os.WriteFile(*bundledOut, []byte(protoSrc), 0o644); err != nil {
		log.Fatalf("write %s: %v", *bundledOut, err)
	}
	fmt.Printf("wrote %s\n", *bundledOut)

	parsed, err := mergeproto.Parse(protoSrc)
	if err != nil {
		log.Fatalf("proto.Parse: %v", err)
	}
	splits := mergeproto.Split([]*mergeproto.File{parsed})

	if err := os.RemoveAll(*splitDir); err != nil {
		log.Fatalf("rm %s: %v", *splitDir, err)
	}
	if err := os.MkdirAll(*splitDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *splitDir, err)
	}
	for _, sr := range splits {
		outDir := filepath.Join(*splitDir, sr.Dir)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", outDir, err)
		}
		outPath := filepath.Join(outDir, sr.Filename)
		if err := os.WriteFile(outPath, []byte(sr.Content), 0o644); err != nil {
			log.Fatalf("write %s: %v", outPath, err)
		}
	}
	fmt.Printf("wrote %d split files to %s\n", len(splits), *splitDir)

	if err := os.MkdirAll(filepath.Dir(*prefixMapOut), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", filepath.Dir(*prefixMapOut), err)
	}
	if err := os.WriteFile(*prefixMapOut, []byte(formatPrefixMap(prefixes)), 0o644); err != nil {
		log.Fatalf("write %s: %v", *prefixMapOut, err)
	}
	fmt.Printf("wrote %s (%d entries)\n", *prefixMapOut, len(prefixes))
}

// formatPrefixMap renders the FQN → tokens mapping as a generated Go
// source file declaring sqlitepb.MessagePrefix. FQNs are sorted for a
// stable diff.
func formatPrefixMap(prefixes map[string][]string) string {
	keys := make([]string, 0, len(prefixes))
	for k := range prefixes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("// Code generated by genproto. DO NOT EDIT.\n\n")
	b.WriteString("package sqlitepb\n\n")
	b.WriteString("// MessagePrefix maps a fully-qualified proto message name to the\n")
	b.WriteString("// leading terminal tokens that StripKeywords removed from its\n")
	b.WriteString("// schema. The renderer emits these tokens before walking the\n")
	b.WriteString("// message's fields to reconstruct SQL text.\n")
	b.WriteString("var MessagePrefix = map[string][]string{\n")
	for _, k := range keys {
		b.WriteString("\t")
		b.WriteString(strconv.Quote(k))
		b.WriteString(": {")
		for i, tok := range prefixes[k] {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Quote(tok))
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	return b.String()
}
