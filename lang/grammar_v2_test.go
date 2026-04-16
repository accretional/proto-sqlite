package lang

import (
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"

	v2pb "github.com/accretional/gluon/v2/pb"
)

// TestGrammarV2Loads confirms the v2 grammar textproto produced by
// cmd/gengrammarv2 round-trips through prototext and contains the
// expected top-level rules.
func TestGrammarV2Loads(t *testing.T) {
	blob, err := os.ReadFile("sqlite-grammar-v2.textproto")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var gd v2pb.GrammarDescriptor
	if err := prototext.Unmarshal(blob, &gd); err != nil {
		t.Fatalf("prototext.Unmarshal: %v", err)
	}
	if got := len(gd.GetRules()); got < 90 {
		t.Fatalf("expected ≥90 rules, got %d", got)
	}
	if gd.GetLex().GetName() != "iso-14977" {
		t.Errorf("lex name: got %q, want iso-14977", gd.GetLex().GetName())
	}

	seen := map[string]bool{}
	for _, r := range gd.GetRules() {
		seen[r.GetName()] = true
	}
	for _, want := range []string{
		"sql_stmt_list", "sql_stmt", "begin_stmt", "commit_stmt",
		"drop_table_stmt", "create_table_stmt", "select_stmt", "expr",
	} {
		if !seen[want] {
			t.Errorf("missing rule %q", want)
		}
	}
}
