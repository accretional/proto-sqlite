// Command genproto reads the sqlite grammar textproto, runs gluon's
// Metaparser to emit a FileDescriptorProto, and writes a serialized
// FileDescriptorSet that protoc can consume via --descriptor_set_in.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/accretional/gluon/lexkit"
	"github.com/accretional/gluon/metaparser"
	pb "github.com/accretional/gluon/pb"
)

func main() {
	lexPath := flag.String("lex", "lang/sqlite-lex.textproto", "lex descriptor textproto")
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	outPath := flag.String("out", "lang/sqlite.fdset", "output FileDescriptorSet binary")
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

	ld := &pb.LanguageDescriptor{
		Name:    "sqlite",
		Grammar: gd,
	}
	fdp, err := metaparser.Build(ld)
	if err != nil {
		log.Fatalf("metaparser.Build: %v", err)
	}
	fmt.Printf("generated %d messages from %d productions\n",
		len(fdp.GetMessageType()), len(gd.GetProductions()))

	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}
	blob, err := proto.Marshal(set)
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(*outPath, blob, 0o644); err != nil {
		log.Fatalf("write %s: %v", *outPath, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *outPath, len(blob))
}
