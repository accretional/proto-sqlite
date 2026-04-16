// Command gengrammarv2 reads lang/sqlite.ebnf and emits a v2
// GrammarDescriptor textproto via gluon's v2 Metaparser.ParseEBNF.
//
// This parallels lang/cmd/gengrammar (which uses v1 lexkit.Parse) and is
// the first piece of the proto-sqlite → gluon v2 migration. The v1
// pipeline still drives the compiler path (genproto) until gluon v2
// ships its own compiler; see gluon/v2/README.md for the plan.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/protobuf/encoding/prototext"

	metaparserv2 "github.com/accretional/gluon/v2/metaparser"
)

func main() {
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	outPath := flag.String("out", "lang/sqlite-grammar-v2.textproto", "v2 grammar descriptor output")
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

	out, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(gd)
	if err != nil {
		log.Fatalf("prototext.Marshal: %v", err)
	}
	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		log.Fatalf("write %s: %v", *outPath, err)
	}
	fmt.Printf("wrote %s (%d rules)\n", *outPath, len(gd.GetRules()))
}
