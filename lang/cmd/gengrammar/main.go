// Command gengrammar reads lang/sqlite-lex.textproto + lang/sqlite.ebnf
// and emits lang/sqlite-grammar.textproto via gluon's lexkit.Parse.
//
// No gluon extension required — we reuse its EBNF meta-language verbatim.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/accretional/gluon/lexkit"
)

func main() {
	lexPath := flag.String("lex", "lang/sqlite-lex.textproto", "lex descriptor textproto")
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	outPath := flag.String("out", "lang/sqlite-grammar.textproto", "grammar descriptor output")
	flag.Parse()

	lex, err := lexkit.LoadLex(*lexPath)
	if err != nil {
		log.Fatalf("load lex %s: %v", *lexPath, err)
	}
	src, err := os.ReadFile(*ebnfPath)
	if err != nil {
		log.Fatalf("read ebnf %s: %v", *ebnfPath, err)
	}
	gd, err := lexkit.Parse(string(src), lex)
	if err != nil {
		log.Fatalf("lexkit.Parse: %v", err)
	}
	out := lexkit.ToTextproto(gd)
	if err := os.WriteFile(*outPath, []byte(out), 0o644); err != nil {
		log.Fatalf("write %s: %v", *outPath, err)
	}
	fmt.Printf("wrote %s (%d productions)\n", *outPath, len(gd.Productions))
}
