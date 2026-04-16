// Command sqlited is the UDS-attached sqlite3 daemon. It listens on a
// Unix Domain Socket and, for each connection, reads one framed SQL
// statement and replies with CSV output produced by the sqlite3 CLI.
//
// This is the far side of QueryRequest.socket_uri: the gRPC service's
// Query handler dials this daemon when a request specifies a socket.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	sqliteembed "github.com/accretional/proto-sqlite/sqlite"
)

func main() {
	socket := flag.String("socket", "", "Unix domain socket path to listen on (required)")
	bin := flag.String("bin", "", "path to the sqlite3 binary (default: embedded)")
	db := flag.String("db", "", "path to the sqlite database file (default: embedded example.db)")
	flag.Parse()

	if *socket == "" {
		log.Fatal("--socket is required")
	}

	resolvedBin, resolvedDB, err := sqliteembed.ResolveBackend(*bin, *db)
	if err != nil {
		log.Fatalf("resolve sqlite backend: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*socket), 0o755); err != nil {
		log.Fatalf("mkdir socket dir: %v", err)
	}
	_ = os.Remove(*socket)

	lis, err := net.Listen("unix", *socket)
	if err != nil {
		log.Fatalf("listen unix %s: %v", *socket, err)
	}
	defer os.Remove(*socket)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("sqlited listening on %s (bin=%s db=%s)", *socket, resolvedBin, resolvedDB)
	if err := sqliteembed.ServeUDS(ctx, lis, resolvedBin, resolvedDB); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
