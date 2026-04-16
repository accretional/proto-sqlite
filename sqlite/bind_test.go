package sqliteembed

import (
	"testing"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

func val(v interface{}) *sqlitepb.Value {
	switch x := v.(type) {
	case string:
		return &sqlitepb.Value{V: &sqlitepb.Value_Text{Text: x}}
	case int:
		return &sqlitepb.Value{V: &sqlitepb.Value_Integer{Integer: int64(x)}}
	case int64:
		return &sqlitepb.Value{V: &sqlitepb.Value_Integer{Integer: x}}
	case float64:
		return &sqlitepb.Value{V: &sqlitepb.Value_Real{Real: x}}
	case []byte:
		return &sqlitepb.Value{V: &sqlitepb.Value_Blob{Blob: x}}
	case nil:
		return &sqlitepb.Value{V: &sqlitepb.Value_Null{Null: true}}
	default:
		panic("unknown type")
	}
}

func TestEscapeSQLValue(t *testing.T) {
	cases := []struct {
		in   *sqlitepb.Value
		want string
	}{
		{val("hello"), "'hello'"},
		{val("it's"), "'it''s'"},
		{val(""), "''"},
		{val(42), "42"},
		{val(int64(-7)), "-7"},
		{val(3.14), "3.14"},
		{val([]byte{0xde, 0xad, 0xbe, 0xef}), "x'deadbeef'"},
		{val([]byte{}), "x''"},
		{val(nil), "NULL"},
	}
	for _, tc := range cases {
		got, err := escapeSQLValue(tc.in)
		if err != nil {
			t.Errorf("escapeSQLValue(%v): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("escapeSQLValue(%v): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBindParams(t *testing.T) {
	cases := []struct {
		sql    string
		params []*sqlitepb.Value
		want   string
	}{
		{
			"SELECT ? + ?",
			[]*sqlitepb.Value{val(1), val(2)},
			"SELECT 1 + 2",
		},
		{
			"INSERT INTO t VALUES (?, ?, ?)",
			[]*sqlitepb.Value{val("a'b"), val(7), val(nil)},
			"INSERT INTO t VALUES ('a''b', 7, NULL)",
		},
		{
			// ? inside a string literal must not be replaced
			"SELECT '?' AS q, ?",
			[]*sqlitepb.Value{val(99)},
			"SELECT '?' AS q, 99",
		},
		{
			// '' inside literal is an escaped quote, not end-of-literal
			"SELECT 'it''s a ?', ?",
			[]*sqlitepb.Value{val("x")},
			"SELECT 'it''s a ?', 'x'",
		},
		{
			"SELECT 1",
			nil,
			"SELECT 1",
		},
	}
	for _, tc := range cases {
		got, err := bindParams(tc.sql, tc.params)
		if err != nil {
			t.Errorf("bindParams(%q): %v", tc.sql, err)
			continue
		}
		if got != tc.want {
			t.Errorf("bindParams(%q):\n  got  %q\n  want %q", tc.sql, got, tc.want)
		}
	}
}

func TestBindParams_Errors(t *testing.T) {
	_, err := bindParams("SELECT ?, ?", []*sqlitepb.Value{val(1)})
	if err == nil {
		t.Error("expected error for too few params")
	}
	_, err = bindParams("SELECT ?", []*sqlitepb.Value{val(1), val(2)})
	if err == nil {
		t.Error("expected error for too many params")
	}
}
