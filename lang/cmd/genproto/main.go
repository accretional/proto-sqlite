// Command genproto reads the sqlite grammar textproto, runs gluon's
// Metaparser to emit a FileDescriptorProto, serializes it as a
// FileDescriptorSet, and renders .proto source text via proto-merge.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/accretional/gluon/lexkit"
	"github.com/accretional/gluon/metaparser"
	pb "github.com/accretional/gluon/pb"
	"github.com/accretional/merge/descriptor"
)

func main() {
	lexPath := flag.String("lex", "lang/sqlite-lex.textproto", "lex descriptor textproto")
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	fdsetOut := flag.String("fdset", "lang/sqlite.fdset", "output FileDescriptorSet binary")
	protoDir := flag.String("proto-dir", "lang/protos", "output directory for .proto files")
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
	if err := os.WriteFile(*fdsetOut, blob, 0o644); err != nil {
		log.Fatalf("write %s: %v", *fdsetOut, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *fdsetOut, len(blob))

	protoSrc, err := descriptor.ToString(fdp)
	if err != nil {
		log.Fatalf("descriptor.ToString: %v", err)
	}
	if err := os.MkdirAll(*protoDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *protoDir, err)
	}
	protoPath := filepath.Join(*protoDir, fdp.GetName())
	if err := os.WriteFile(protoPath, []byte(protoSrc), 0o644); err != nil {
		log.Fatalf("write %s: %v", protoPath, err)
	}
	fmt.Printf("wrote %s\n", protoPath)
}
