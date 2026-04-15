package lang

import (
	"testing"

	"github.com/accretional/gluon/lexkit"
)

func TestGrammarLoads(t *testing.T) {
	gd, err := lexkit.LoadGrammar("sqlite-grammar.textproto")
	if err != nil {
		t.Fatalf("LoadGrammar: %v", err)
	}
	if got := len(gd.Productions); got < 20 {
		t.Fatalf("expected ≥20 productions, got %d", got)
	}
	seen := map[string]bool{}
	for _, p := range gd.Productions {
		seen[p.Name] = true
	}
	for _, want := range []string{"sql_stmt", "begin_stmt", "commit_stmt", "drop_table_stmt"} {
		if !seen[want] {
			t.Errorf("missing production %q", want)
		}
	}
}
