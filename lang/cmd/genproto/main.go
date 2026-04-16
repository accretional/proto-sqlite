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

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/accretional/gluon/v2/compiler"
	metaparserv2 "github.com/accretional/gluon/v2/metaparser"
	"github.com/accretional/merge/descriptor"
	mergeproto "github.com/accretional/merge/proto"
)

func main() {
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	fdsetOut := flag.String("fdset", "lang/sqlite.fdset", "output FileDescriptorSet binary")
	bundledOut := flag.String("bundled", "sqlite.proto", "bundled .proto output in repo root")
	splitDir := flag.String("split-dir", "lang/protos", "output directory for split .proto files")
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
}
