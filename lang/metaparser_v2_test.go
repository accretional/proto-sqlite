package lang

import (
	"context"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/accretional/gluon/v2/metaparser"
	v2pb "github.com/accretional/gluon/v2/pb"
)

// TestMetaparserV2E2E drives the full proto-sqlite grammar through
// gluon's v2 Metaparser gRPC service (ReadString → EBNF) over an
// in-memory bufconn listener. It is the proto-sqlite-side counterpart
// to gluon's own e2e tests — same transport, but with the real 550-line
// sqlite.ebnf rather than toy grammars.
func TestMetaparserV2E2E(t *testing.T) {
	client, teardown := startV2Server(t)
	defer teardown()
	ctx := context.Background()

	src, err := os.ReadFile("sqlite.ebnf")
	if err != nil {
		t.Fatalf("read sqlite.ebnf: %v", err)
	}
	doc, err := client.ReadString(ctx, &wrapperspb.StringValue{Value: string(src)})
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	gd, err := client.EBNF(ctx, doc)
	if err != nil {
		t.Fatalf("EBNF: %v", err)
	}
	if got := len(gd.GetRules()); got < 90 {
		t.Fatalf("expected ≥90 rules, got %d", got)
	}
	if gd.GetLex().GetName() != "iso-14977" {
		t.Errorf("lex name: got %q, want iso-14977", gd.GetLex().GetName())
	}

	seen := map[string]bool{}
	for _, r := range gd.GetRules() {
		seen[r.GetName()] = true
	}
	for _, want := range []string{"sql_stmt", "select_stmt", "expr"} {
		if !seen[want] {
			t.Errorf("missing rule %q", want)
		}
	}
}

func startV2Server(t *testing.T) (v2pb.MetaparserClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	v2pb.RegisterMetaparserServer(srv, metaparser.New())

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("server exited: %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return v2pb.NewMetaparserClient(conn), func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}
}
