package sqliteembed

import (
	"testing"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

func TestRender_BeginBare(t *testing.T) {
	got, err := RenderSQL(&sqlitepb.BeginStmt{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "BEGIN" {
		t.Errorf("got %q, want %q", got, "BEGIN")
	}
}

func TestRender_BeginTransaction(t *testing.T) {
	got, err := RenderSQL(&sqlitepb.BeginStmt{
		TransactionKeyword: &sqlitepb.TransactionKeyword{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "BEGIN TRANSACTION" {
		t.Errorf("got %q, want %q", got, "BEGIN TRANSACTION")
	}
}

func TestRender_BeginImmediateTransaction(t *testing.T) {
	got, err := RenderSQL(&sqlitepb.BeginStmt{
		Alt1: &sqlitepb.BeginStmt_Alt1{
			Value: &sqlitepb.BeginStmt_Alt1_ImmediateKeyword{
				ImmediateKeyword: &sqlitepb.ImmediateKeyword{},
			},
		},
		TransactionKeyword: &sqlitepb.TransactionKeyword{},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "BEGIN IMMEDIATE TRANSACTION"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_DropTable(t *testing.T) {
	got, err := RenderSQL(&sqlitepb.DropTableStmt{
		TableName: &sqlitepb.TableName{
			Name: &sqlitepb.Name{XKeyword: &sqlitepb.XKeyword{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "DROP TABLE x" {
		t.Errorf("got %q, want %q", got, "DROP TABLE x")
	}
}

func TestRender_DropTableIfExistsSchemaDot(t *testing.T) {
	// DROP TABLE IF EXISTS x.x
	got, err := RenderSQL(&sqlitepb.DropTableStmt{
		IfExists: &sqlitepb.DropTableStmt_IfExists{},
		Seq1: &sqlitepb.DropTableStmt_Seq1{
			SchemaName: &sqlitepb.SchemaName{
				Name: &sqlitepb.Name{XKeyword: &sqlitepb.XKeyword{}},
			},
			FullStopKeyword: &sqlitepb.FullStopKeyword{},
		},
		TableName: &sqlitepb.TableName{
			Name: &sqlitepb.Name{XKeyword: &sqlitepb.XKeyword{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "DROP TABLE IF EXISTS x.x"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_SqlStmtListWrapping(t *testing.T) {
	list := &sqlitepb.SqlStmtList{
		SqlStmt: []*sqlitepb.SqlStmt{
			{Alt1: &sqlitepb.SqlStmt_Alt1{
				Value: &sqlitepb.SqlStmt_Alt1_BeginStmt{
					BeginStmt: &sqlitepb.BeginStmt{},
				},
			}},
		},
	}
	got, err := RenderSQL(list)
	if err != nil {
		t.Fatal(err)
	}
	if got != "BEGIN" {
		t.Errorf("got %q, want %q", got, "BEGIN")
	}
}
