package sqliteembed

import (
	"context"
	"net"
	"os/exec"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

func startInProc(t *testing.T) sqlitepb.SqliteClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	sqlitepb.RegisterSqliteServer(srv, NewServer())
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc server stopped: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)

	dial := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}
	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(dial),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return sqlitepb.NewSqliteClient(conn)
}

func TestServerQuery_RawSQL(t *testing.T) {
	client := startInProc(t)

	resp, err := client.Query(context.Background(), &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT id, name, qty FROM widgets ORDER BY id;",
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	wantCols := []string{"id", "name", "qty"}
	if got, want := resp.GetColumn(), wantCols; !eq(got, want) {
		t.Errorf("columns: got %v, want %v", got, want)
	}
	if got := len(resp.GetRow()); got != 3 {
		t.Fatalf("rows: got %d, want 3", got)
	}
	// example db: (1, sprocket, 3), (2, gizmo, 7), (3, cog, 12)
	first := resp.GetRow()[0].GetCell()
	if !eqCells(first, "1", "sprocket", "3") {
		t.Errorf("row 0: got %v", first)
	}
}

func TestServerQuery_TypedBegin(t *testing.T) {
	// Typed SqlStmtList → rendered to "BEGIN" → executed against the
	// example db. BEGIN returns no rows but also no error.
	client := startInProc(t)

	resp, err := client.Query(context.Background(), &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Stmts{
			Stmts: &sqlitepb.SqlStmtList{
				SqlStmt: []*sqlitepb.SqlStmt{{
					Alt1: &sqlitepb.SqlStmt_Alt1{
						Value: &sqlitepb.SqlStmt_Alt1_BeginStmt{
							BeginStmt: &sqlitepb.BeginStmt{},
						},
					},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(resp.GetColumn()) != 0 || len(resp.GetRow()) != 0 {
		t.Errorf("BEGIN should have empty result, got cols=%v rows=%d",
			resp.GetColumn(), len(resp.GetRow()))
	}
}

func TestServerQuery_TypedDropTableIfExists(t *testing.T) {
	// Typed SqlStmtList carrying a real user-supplied table name
	// (scalarized from the grammar's `name = "x"` placeholder) →
	// rendered to "DROP TABLE IF EXISTS ephemeral" → executed against
	// the example db as a no-op.
	client := startInProc(t)

	resp, err := client.Query(context.Background(), &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Stmts{
			Stmts: &sqlitepb.SqlStmtList{
				SqlStmt: []*sqlitepb.SqlStmt{{
					Alt1: &sqlitepb.SqlStmt_Alt1{
						Value: &sqlitepb.SqlStmt_Alt1_DropTableStmt{
							DropTableStmt: &sqlitepb.DropTableStmt{
								IfExists: &sqlitepb.DropTableStmt_IfExists{},
								TableName: &sqlitepb.TableName{
									Name: &sqlitepb.Name{Value: "ephemeral"},
								},
							},
						},
					},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(resp.GetColumn()) != 0 || len(resp.GetRow()) != 0 {
		t.Errorf("DROP TABLE IF EXISTS should be empty, got cols=%v rows=%d",
			resp.GetColumn(), len(resp.GetRow()))
	}
}

func TestServerQuery_TypedCreateTableTwoColumns(t *testing.T) {
	// Typed SqlStmtList → rendered to "CREATE TABLE <name> (a TEXT, b
	// INTEGER)" → executed against the example db. Verifies the
	// separator-interleave path end-to-end: without it the rendered SQL
	// would be "(a TEXT b INTEGER)" and sqlite3 would reject it.
	client := startInProc(t)

	ctx := context.Background()
	resp, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Stmts{
			Stmts: &sqlitepb.SqlStmtList{
				SqlStmt: []*sqlitepb.SqlStmt{{
					Alt1: &sqlitepb.SqlStmt_Alt1{
						Value: &sqlitepb.SqlStmt_Alt1_CreateTableStmt{
							CreateTableStmt: &sqlitepb.CreateTableStmt{
								TableKeyword: &sqlitepb.TableKeyword{},
								TableName:    &sqlitepb.TableName{Name: &sqlitepb.Name{Value: "typed_ct"}},
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
							},
						},
					},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Query CREATE TABLE: %v", err)
	}
	if len(resp.GetColumn()) != 0 || len(resp.GetRow()) != 0 {
		t.Errorf("CREATE TABLE should be empty, got cols=%v rows=%d",
			resp.GetColumn(), len(resp.GetRow()))
	}

	// Confirm via raw SQL that the table actually landed.
	check, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT name FROM sqlite_master WHERE type='table' AND name='typed_ct';",
		},
	})
	if err != nil {
		t.Fatalf("Query sqlite_master: %v", err)
	}
	if n := len(check.GetRow()); n != 1 {
		t.Fatalf("expected 1 row for typed_ct, got %d", n)
	}
	if got := check.GetRow()[0].GetCell()[0]; string(got) != "typed_ct" {
		t.Errorf("name: got %q, want %q", got, "typed_ct")
	}
}

func TestServerQuery_TypedTwoStmtSemicolonSeparator(t *testing.T) {
	// Typed SqlStmtList with two statements → renders to
	// "CREATE TABLE stress_sep (x TEXT); DROP TABLE stress_sep".
	// Exercises the non-comma separator path (";" between sql_stmt
	// entries) end-to-end: sqlite3 sees and executes both statements,
	// and a follow-up sqlite_master probe confirms the table is gone.
	client := startInProc(t)

	ctx := context.Background()
	resp, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Stmts{
			Stmts: &sqlitepb.SqlStmtList{
				SqlStmt: []*sqlitepb.SqlStmt{
					{
						Alt1: &sqlitepb.SqlStmt_Alt1{
							Value: &sqlitepb.SqlStmt_Alt1_CreateTableStmt{
								CreateTableStmt: &sqlitepb.CreateTableStmt{
									TableKeyword: &sqlitepb.TableKeyword{},
									TableName:    &sqlitepb.TableName{Name: &sqlitepb.Name{Value: "stress_sep"}},
									Alt2: &sqlitepb.CreateTableStmt_Alt2{
										Value: &sqlitepb.CreateTableStmt_Alt2_LeftParenthesis_{
											LeftParenthesis: &sqlitepb.CreateTableStmt_Alt2_LeftParenthesis{
												ColumnDef: []*sqlitepb.ColumnDef{{
													ColumnName: &sqlitepb.ColumnName{Name: &sqlitepb.Name{Value: "x"}},
													TypeName:   &sqlitepb.TypeName{Name: &sqlitepb.Name{Value: "TEXT"}},
												}},
												RightParenthesisKeyword: &sqlitepb.RightParenthesisKeyword{},
											},
										},
									},
								},
							},
						},
					},
					{
						Alt1: &sqlitepb.SqlStmt_Alt1{
							Value: &sqlitepb.SqlStmt_Alt1_DropTableStmt{
								DropTableStmt: &sqlitepb.DropTableStmt{
									TableName: &sqlitepb.TableName{Name: &sqlitepb.Name{Value: "stress_sep"}},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Query two-stmt list: %v", err)
	}
	if len(resp.GetColumn()) != 0 || len(resp.GetRow()) != 0 {
		t.Errorf("CREATE+DROP should be empty, got cols=%v rows=%d",
			resp.GetColumn(), len(resp.GetRow()))
	}

	check, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT name FROM sqlite_master WHERE type='table' AND name='stress_sep';",
		},
	})
	if err != nil {
		t.Fatalf("Query sqlite_master: %v", err)
	}
	if n := len(check.GetRow()); n != 0 {
		t.Errorf("stress_sep should be dropped; got %d sqlite_master rows", n)
	}
}

func TestServerQuery_TypedInsertValuesTwoRows(t *testing.T) {
	// Typed INSERT INTO stress_ins (a, b) VALUES ('hello','world'),
	// ('foo','bar') — exercises three separator/prefix paths in one
	// statement: the column-name list, the first values tuple expr
	// list, and the repeated CommaLeftParenthesis tuple (each of which
	// carries its own inner expr list). Setup and verification use raw
	// SQL so the typed INSERT is the only subject under test.
	client := startInProc(t)
	ctx := context.Background()

	if _, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "CREATE TABLE stress_ins (a TEXT, b TEXT);",
		},
	}); err != nil {
		t.Fatalf("setup CREATE: %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.Query(ctx, &sqlitepb.QueryRequest{
			Body: &sqlitepb.QueryRequest_Sql{Sql: "DROP TABLE stress_ins;"},
		})
	})

	resp, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Stmts{
			Stmts: &sqlitepb.SqlStmtList{
				SqlStmt: []*sqlitepb.SqlStmt{{
					Alt1: &sqlitepb.SqlStmt_Alt1{
						Value: &sqlitepb.SqlStmt_Alt1_InsertStmt{
							InsertStmt: insertTwoRows(),
						},
					},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("typed INSERT: %v", err)
	}
	if len(resp.GetColumn()) != 0 || len(resp.GetRow()) != 0 {
		t.Errorf("INSERT should be empty, got cols=%v rows=%d",
			resp.GetColumn(), len(resp.GetRow()))
	}

	check, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT a, b FROM stress_ins ORDER BY a;",
		},
	})
	if err != nil {
		t.Fatalf("verify SELECT: %v", err)
	}
	if n := len(check.GetRow()); n != 2 {
		t.Fatalf("expected 2 rows, got %d", n)
	}
	r0 := check.GetRow()[0].GetCell()
	r1 := check.GetRow()[1].GetCell()
	if !eqCells(r0, "foo", "bar") {
		t.Errorf("row 0: got %v, want [foo bar]", r0)
	}
	if !eqCells(r1, "hello", "world") {
		t.Errorf("row 1: got %v, want [hello world]", r1)
	}
}

func TestServerQuery_DbPath(t *testing.T) {
	// Create a temp db with a known schema distinct from the embedded
	// example.db, issue a query with db_path set, and verify the right
	// db was queried.
	bin, _, err := ResolveBackend("", "")
	if err != nil {
		t.Fatalf("resolve backend: %v", err)
	}
	dir := t.TempDir()
	dbPath := dir + "/custom.db"
	setup := "CREATE TABLE things (label TEXT); INSERT INTO things VALUES ('alpha'),('beta');"
	if out, err2 := exec.CommandContext(context.Background(), bin, dbPath, setup).CombinedOutput(); err2 != nil {
		t.Fatalf("setup custom db: %v (out=%q)", err2, out)
	}

	client := startInProc(t)
	resp, err := client.Query(context.Background(), &sqlitepb.QueryRequest{
		DbPath: dbPath,
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT label FROM things ORDER BY label;",
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got, want := resp.GetColumn(), []string{"label"}; !eq(got, want) {
		t.Errorf("columns: got %v, want %v", got, want)
	}
	if n := len(resp.GetRow()); n != 2 {
		t.Fatalf("rows: got %d, want 2", n)
	}
	if got := resp.GetRow()[0].GetCell(); !eqCells(got, "alpha") {
		t.Errorf("row 0: got %v, want [alpha]", got)
	}
	if got := resp.GetRow()[1].GetCell(); !eqCells(got, "beta") {
		t.Errorf("row 1: got %v, want [beta]", got)
	}
}

func TestServerQuery_ParamBinding(t *testing.T) {
	// End-to-end: param binding substitutes ? placeholders before exec.
	// Uses the embedded example db (widgets table: id, name, qty).
	client := startInProc(t)
	ctx := context.Background()

	// text param filters by name
	resp, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body:  &sqlitepb.QueryRequest_Sql{Sql: "SELECT id, qty FROM widgets WHERE name = ?;"},
		Param: []*sqlitepb.Value{{V: &sqlitepb.Value_Text{Text: "gizmo"}}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if n := len(resp.GetRow()); n != 1 {
		t.Fatalf("rows: got %d, want 1", n)
	}
	if got := resp.GetRow()[0].GetCell(); !eqCells(got, "2", "7") {
		t.Errorf("row: got %v, want [2 7]", got)
	}

	// integer param filters by qty; quote-inside-text param is safe
	resp2, err := client.Query(ctx, &sqlitepb.QueryRequest{
		Body:  &sqlitepb.QueryRequest_Sql{Sql: "SELECT name FROM widgets WHERE qty > ?;"},
		Param: []*sqlitepb.Value{{V: &sqlitepb.Value_Integer{Integer: 5}}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if n := len(resp2.GetRow()); n != 2 {
		t.Fatalf("rows: got %d, want 2", n)
	}
}

func TestServerQuery_EmptyBody(t *testing.T) {
	client := startInProc(t)

	_, err := client.Query(context.Background(), &sqlitepb.QueryRequest{})
	if err == nil {
		t.Fatal("want InvalidArgument error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("code: got %v, want InvalidArgument", status.Code(err))
	}
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func eqCells(got [][]byte, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if string(got[i]) != want[i] {
			return false
		}
	}
	return true
}
