package sqliteembed

import (
	"strings"
	"testing"
)

func TestQueryWidgets(t *testing.T) {
	out, err := Query("SELECT SUM(qty) FROM widgets;")
	if err != nil {
		t.Fatalf("Query: %v (out=%q)", err, out)
	}
	got := strings.TrimSpace(out)
	if got != "22" {
		t.Fatalf("want 22, got %q", got)
	}
}
