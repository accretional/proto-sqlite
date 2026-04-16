package sqliteembed

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

// Server implements sqlitepb.SqliteServer against the embedded sqlite3
// binary. Raw-SQL requests are forwarded verbatim; typed SqlStmtList
// requests are not yet supported.
type Server struct {
	sqlitepb.UnimplementedSqliteServer
}

func NewServer() *Server { return &Server{} }

func (s *Server) Query(ctx context.Context, req *sqlitepb.QueryRequest) (*sqlitepb.QueryResponse, error) {
	var sql string
	switch body := req.GetBody().(type) {
	case *sqlitepb.QueryRequest_Sql:
		sql = body.Sql
	case *sqlitepb.QueryRequest_Stmts:
		rendered, err := RenderSQL(body.Stmts)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "render SqlStmtList: %v", err)
		}
		sql = rendered
	default:
		return nil, status.Error(codes.InvalidArgument, "QueryRequest.body is required")
	}

	bin, db, err := extract()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "extract embedded sqlite: %v", err)
	}
	out, err := exec.CommandContext(ctx, bin, "-csv", "-header", db, sql).CombinedOutput()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sqlite3: %v (out=%q)", err, string(out))
	}
	return parseCSV(string(out))
}

// parseCSV turns sqlite3's -csv -header output into a QueryResponse.
// Empty output (statements like INSERT with no returning) produces an
// empty response.
func parseCSV(raw string) (*sqlitepb.QueryResponse, error) {
	trimmed := strings.TrimRight(raw, "\n")
	if trimmed == "" {
		return &sqlitepb.QueryResponse{}, nil
	}
	r := csv.NewReader(strings.NewReader(trimmed))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse sqlite csv output: %w", err)
	}
	resp := &sqlitepb.QueryResponse{Column: records[0]}
	for _, rec := range records[1:] {
		resp.Row = append(resp.Row, &sqlitepb.Row{Cell: rec})
	}
	return resp, nil
}
