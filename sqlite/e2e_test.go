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

// TestRender_SqlStmtListSemicolonSeparator pins the only non-comma
// separator in the generated FieldSeparator map: ";" between members
// of sql_stmt on SqlStmtList. A BEGIN + COMMIT pair must render as
// "BEGIN; COMMIT" — the semicolon is dropped by CollapseCommaList
// and reintroduced by the renderer.
func TestRender_SqlStmtListSemicolonSeparator(t *testing.T) {
	list := &sqlitepb.SqlStmtList{
		SqlStmt: []*sqlitepb.SqlStmt{
			{
				Alt1: &sqlitepb.SqlStmt_Alt1{
					Value: &sqlitepb.SqlStmt_Alt1_BeginStmt{
						BeginStmt: &sqlitepb.BeginStmt{},
					},
				},
			},
			{
				Alt1: &sqlitepb.SqlStmt_Alt1{
					Value: &sqlitepb.SqlStmt_Alt1_CommitStmt{
						CommitStmt: &sqlitepb.CommitStmt{
							Alt1: &sqlitepb.CommitStmt_Alt1{
								Value: &sqlitepb.CommitStmt_Alt1_CommitKeyword{
									CommitKeyword: &sqlitepb.CommitKeyword{},
								},
							},
						},
					},
				},
			},
		},
	}
	got, err := RenderSQL(list)
	if err != nil {
		t.Fatal(err)
	}
	want := "BEGIN; COMMIT"
	if got != want {
		t.Errorf("\n got  %q\n want %q", got, want)
	}
}

// strLiteralExpr wraps a raw SQL string-literal token (quotes included)
// in the Expr → LiteralValue → StringLiteral chain. The StringLiteral's
// scalar `value` is emitted verbatim by the renderer, so callers must
// supply the surrounding single quotes.
func strLiteralExpr(tok string) *sqlitepb.Expr {
	return &sqlitepb.Expr{
		Value: &sqlitepb.Expr_LiteralValue{
			LiteralValue: &sqlitepb.LiteralValue{
				Value: &sqlitepb.LiteralValue_StringLiteral{
					StringLiteral: &sqlitepb.StringLiteral{Value: tok},
				},
			},
		},
	}
}

// insertTwoRows builds:
//
//	INSERT INTO stress_ins (a, b) VALUES ('hello', 'world'), ('foo', 'bar')
//
// This shape touches four distinct separator/prefix paths at once:
//   - ","  in the column-name list (InsertStmt.LeftParenthesis.column_name)
//   - "VALUES (" prefix on InsertStmt.Alt2.ValuesLeftParenthesis
//   - ","  between exprs inside the first values tuple
//   - ", (" prefix on each CommaLeftParenthesis tuple, plus ","
//     between its inner exprs
func insertTwoRows() *sqlitepb.InsertStmt {
	return &sqlitepb.InsertStmt{
		Alt1: &sqlitepb.InsertStmt_Alt1{
			Value: &sqlitepb.InsertStmt_Alt1_InsertKeyword{
				InsertKeyword: &sqlitepb.InsertKeyword{},
			},
		},
		IntoKeyword: &sqlitepb.IntoKeyword{},
		TableName:   &sqlitepb.TableName{Name: &sqlitepb.Name{Value: "stress_ins"}},
		LeftParenthesis: &sqlitepb.InsertStmt_LeftParenthesis{
			ColumnName: []*sqlitepb.ColumnName{
				{Name: &sqlitepb.Name{Value: "a"}},
				{Name: &sqlitepb.Name{Value: "b"}},
			},
			RightParenthesisKeyword: &sqlitepb.RightParenthesisKeyword{},
		},
		Alt2: &sqlitepb.InsertStmt_Alt2{
			Value: &sqlitepb.InsertStmt_Alt2_ValuesLeftParenthesis_{
				ValuesLeftParenthesis: &sqlitepb.InsertStmt_Alt2_ValuesLeftParenthesis{
					Expr: []*sqlitepb.Expr{
						strLiteralExpr("'hello'"),
						strLiteralExpr("'world'"),
					},
					RightParenthesisKeyword: &sqlitepb.RightParenthesisKeyword{},
					CommaLeftParenthesis: []*sqlitepb.InsertStmt_Alt2_ValuesLeftParenthesis_CommaLeftParenthesis{{
						Expr: []*sqlitepb.Expr{
							strLiteralExpr("'foo'"),
							strLiteralExpr("'bar'"),
						},
						RightParenthesisKeyword: &sqlitepb.RightParenthesisKeyword{},
					}},
				},
			},
		},
	}
}

func TestRender_InsertValuesTwoRows(t *testing.T) {
	got, err := RenderSQL(insertTwoRows())
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO stress_ins (a, b) VALUES ('hello', 'world'), ('foo', 'bar')"
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
