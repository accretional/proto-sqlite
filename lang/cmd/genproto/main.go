// Command genproto reads the sqlite grammar textproto, runs gluon's
// Metaparser to emit a FileDescriptorProto, serializes it as a
// FileDescriptorSet, writes the bundled .proto to the repo root, and
// splits individual message .proto files into lang/protos/ via proto-merge.
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
	mergeproto "github.com/accretional/merge/proto"
)

func main() {
	lexPath := flag.String("lex", "lang/sqlite-lex.textproto", "lex descriptor textproto")
	ebnfPath := flag.String("ebnf", "lang/sqlite.ebnf", "EBNF source")
	fdsetOut := flag.String("fdset", "lang/sqlite.fdset", "output FileDescriptorSet binary")
	bundledOut := flag.String("bundled", "sqlite.proto", "bundled .proto output in repo root")
	splitDir := flag.String("split-dir", "lang/protos", "output directory for split .proto files")
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

	if err := os.WriteFile(*bundledOut, []byte(protoSrc), 0o644); err != nil {
		log.Fatalf("write %s: %v", *bundledOut, err)
	}
	fmt.Printf("wrote %s\n", *bundledOut)

	parsed, err := mergeproto.Parse(protoSrc)
	if err != nil {
		log.Fatalf("proto.Parse: %v", err)
	}
	splits := mergeproto.Split([]*mergeproto.File{parsed})

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
