package sqliteembed

import (
	"context"
	"net"
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
	if !eq(first, []string{"1", "sprocket", "3"}) {
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
