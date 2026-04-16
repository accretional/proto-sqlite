package lang

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/accretional/gluon/lexkit"
	"github.com/accretional/gluon/metaparser"
	pb "github.com/accretional/gluon/pb"
)

func TestMetaparserBuild(t *testing.T) {
	lex, err := lexkit.LoadLex("sqlite-lex.textproto")
	if err != nil {
		t.Fatalf("LoadLex: %v", err)
	}
	src, err := os.ReadFile("sqlite.ebnf")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	gd, err := lexkit.Parse(string(src), lex)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	ld := &pb.LanguageDescriptor{Name: "sqlite", Grammar: gd}
	fdp, err := metaparser.Build(ld)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got := len(fdp.GetMessageType()); got < 100 {
		t.Errorf("expected ≥100 messages, got %d", got)
	}

	byName := map[string]bool{}
	for _, m := range fdp.GetMessageType() {
		byName[m.GetName()] = true
	}
	for _, want := range []string{
		"SqlStmtList", "SqlStmt", "SelectStmt", "SelectCore",
		"InsertStmt", "UpdateStmt", "DeleteStmt",
		"CreateTableStmt", "AlterTableStmt", "Expr",
	} {
		if !byName[want] {
			t.Errorf("missing message %q", want)
		}
	}
}

func TestMetaparserProtoc(t *testing.T) {
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not on PATH")
	}
	lex, err := lexkit.LoadLex("sqlite-lex.textproto")
	if err != nil {
		t.Fatalf("LoadLex: %v", err)
	}
	src, err := os.ReadFile("sqlite.ebnf")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	gd, err := lexkit.Parse(string(src), lex)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	ld := &pb.LanguageDescriptor{Name: "sqlite", Grammar: gd}
	fdp, err := metaparser.Build(ld)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	dir := t.TempDir()
	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}
	blob, err := proto.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	setPath := filepath.Join(dir, "set.pb")
	if err := os.WriteFile(setPath, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("protoc",
		"--descriptor_set_in="+setPath,
		"--descriptor_set_out="+filepath.Join(dir, "out.pb"),
		fdp.GetName(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc rejected: %v\n%s", err, out)
	}
}
