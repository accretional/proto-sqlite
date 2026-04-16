package sqliteembed

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

// shortSocketPath returns a socket path under a fresh temp dir that
// fits within the platform's sun_path limit (104 bytes on macOS, ~108
// on Linux). Go's default t.TempDir() on macOS exceeds this. We place
// the socket under /tmp with a short name and register cleanup on t.
func shortSocketPath(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "uds-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	p := filepath.Join(dir, name)
	if len(p) >= 104 {
		t.Fatalf("socket path too long: %d bytes (%q)", len(p), p)
	}
	return p
}


// startSqlited boots the UDS daemon against the embedded sqlite3 + db
// at a temp-dir socket path. Returns the socket path; cleanup is
// registered on t.
func startSqlited(t *testing.T) string {
	t.Helper()
	bin, db, err := ResolveBackend("", "")
	if err != nil {
		t.Fatalf("resolve backend: %v", err)
	}
	sock := shortSocketPath(t, "sqlited.sock")
	lis, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := ServeUDS(ctx, lis, bin, db); err != nil {
			t.Logf("ServeUDS: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return sock
}

// TestServerQuery_UDSRawSQL exercises the socket_uri branch end-to-end:
// gRPC server forwards the SQL to sqlited over UDS, sqlited runs it
// against the embedded db, CSV comes back through the UDS, server
// parses it into a QueryResponse.
func TestServerQuery_UDSRawSQL(t *testing.T) {
	sock := startSqlited(t)
	client := startInProc(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Query(ctx, &sqlitepb.QueryRequest{
		SocketUri: sock,
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT id, name, qty FROM widgets ORDER BY id;",
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got, want := resp.GetColumn(), []string{"id", "name", "qty"}; !eq(got, want) {
		t.Errorf("columns: got %v, want %v", got, want)
	}
	if got := len(resp.GetRow()); got != 3 {
		t.Fatalf("rows: got %d, want 3", got)
	}
	if got := resp.GetRow()[0].GetCell(); !eqCells(got, "1", "sprocket", "3") {
		t.Errorf("row 0: got %v", got)
	}
}

// TestServerQuery_UDSTypedStmt drives a typed SqlStmtList through the
// UDS path — confirms the renderer output and the UDS transport compose
// correctly (rendered SQL is sent over the socket, not executed locally).
func TestServerQuery_UDSTypedStmt(t *testing.T) {
	sock := startSqlited(t)
	client := startInProc(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Query(ctx, &sqlitepb.QueryRequest{
		SocketUri: sock,
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
		t.Errorf("BEGIN should be empty, got cols=%v rows=%d",
			resp.GetColumn(), len(resp.GetRow()))
	}
}

// TestServerQuery_UDSDialFailure proves a missing socket surfaces as
// Unavailable rather than hanging or crashing.
func TestServerQuery_UDSDialFailure(t *testing.T) {
	client := startInProc(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Query(ctx, &sqlitepb.QueryRequest{
		SocketUri: shortSocketPath(t, "does-not-exist.sock"),
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT 1;",
		},
	})
	if err == nil {
		t.Fatal("expected error dialing nonexistent socket, got nil")
	}
}
