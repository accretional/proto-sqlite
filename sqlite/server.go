package sqliteembed

import (
	"context"
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

	if uri := req.GetSocketUri(); uri != "" {
		out, err := queryOverUDS(ctx, uri, sql)
		if err != nil {
			return nil, status.Errorf(codes.Unavailable, "uds query: %v", err)
		}
		return parseQuote(out)
	}

	bin, db, err := ResolveBackend("", req.GetDbPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve sqlite backend: %v", err)
	}
	out, err := exec.CommandContext(ctx, bin, "-cmd", ".headers on", "-cmd", ".mode quote", db, sql).CombinedOutput()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sqlite3: %v (out=%q)", err, string(out))
	}
	return parseQuote(string(out))
}

// parseQuote turns sqlite3's ".headers on" + ".mode quote" output into
// a QueryResponse. Empty output produces an empty response.
//
// Header line: bare comma-separated column names (no quoting).
// Value lines: SQL literals per cell — 'text', X'hex', NULL, or a bare
// number. Text uses '' to escape a literal single-quote inside.
func parseQuote(raw string) (*sqlitepb.QueryResponse, error) {
	trimmed := strings.TrimRight(raw, "\n")
	if trimmed == "" {
		return &sqlitepb.QueryResponse{}, nil
	}
	lines := strings.Split(trimmed, "\n")
	// Header line: column names, also emitted as SQL literals in .mode quote.
	rawCols, _, err := parseQuoteLine(lines[0])
	if err != nil {
		return nil, fmt.Errorf("parse quote header %q: %w", lines[0], err)
	}
	cols := make([]string, len(rawCols))
	for i, c := range rawCols {
		cols[i] = string(c)
	}
	resp := &sqlitepb.QueryResponse{Column: cols}
	for _, line := range lines[1:] {
		cells, nulls, err := parseQuoteLine(line)
		if err != nil {
			return nil, fmt.Errorf("parse quote line %q: %w", line, err)
		}
		resp.Row = append(resp.Row, &sqlitepb.Row{Cell: cells, CellNull: nulls})
	}
	return resp, nil
}
