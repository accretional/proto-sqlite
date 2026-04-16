package sqliteembed

import (
	"testing"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

// singleColumnCreateTable builds a typed CREATE TABLE foo (a TEXT).
// One column, no constraints — the simplest shape that exercises
// ColumnDef / TypeName / the LeftParenthesis oneof variant.
func singleColumnCreateTable() *sqlitepb.CreateTableStmt {
	return &sqlitepb.CreateTableStmt{
		TableKeyword: &sqlitepb.TableKeyword{},
		TableName:    &sqlitepb.TableName{Name: &sqlitepb.Name{Value: "foo"}},
		Alt2: &sqlitepb.CreateTableStmt_Alt2{
			Value: &sqlitepb.CreateTableStmt_Alt2_LeftParenthesis_{
				LeftParenthesis: &sqlitepb.CreateTableStmt_Alt2_LeftParenthesis{
					ColumnDef: []*sqlitepb.ColumnDef{{
						ColumnName: &sqlitepb.ColumnName{Name: &sqlitepb.Name{Value: "a"}},
						TypeName:   &sqlitepb.TypeName{Name: &sqlitepb.Name{Value: "TEXT"}},
					}},
					RightParenthesisKeyword: &sqlitepb.RightParenthesisKeyword{},
				},
			},
		},
	}
}

func TestRender_CreateTableSingleColumn(t *testing.T) {
	got, err := RenderSQL(singleColumnCreateTable())
	if err != nil {
		t.Fatal(err)
	}
	want := "CREATE TABLE foo (a TEXT)"
	if got != want {
		t.Errorf("\n got  %q\n want %q", got, want)
	}
}

func TestRender_CreateTableTwoColumns(t *testing.T) {
	stmt := &sqlitepb.CreateTableStmt{
		TableKeyword: &sqlitepb.TableKeyword{},
		TableName:    &sqlitepb.TableName{Name: &sqlitepb.Name{Value: "foo"}},
		Alt2: &sqlitepb.CreateTableStmt_Alt2{
			Value: &sqlitepb.CreateTableStmt_Alt2_LeftParenthesis_{
				LeftParenthesis: &sqlitepb.CreateTableStmt_Alt2_LeftParenthesis{
					ColumnDef: []*sqlitepb.ColumnDef{
						{
							ColumnName: &sqlitepb.ColumnName{Name: &sqlitepb.Name{Value: "a"}},
							TypeName:   &sqlitepb.TypeName{Name: &sqlitepb.Name{Value: "TEXT"}},
						},
						{
							ColumnName: &sqlitepb.ColumnName{Name: &sqlitepb.Name{Value: "b"}},
							TypeName:   &sqlitepb.TypeName{Name: &sqlitepb.Name{Value: "INTEGER"}},
						},
					},
					RightParenthesisKeyword: &sqlitepb.RightParenthesisKeyword{},
				},
			},
		},
	}
	got, err := RenderSQL(stmt)
	if err != nil {
		t.Fatal(err)
	}
	want := "CREATE TABLE foo (a TEXT, b INTEGER)"
	if got != want {
		t.Errorf("\n got  %q\n want %q", got, want)
	}
}
