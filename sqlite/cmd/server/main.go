// Command server runs the Sqlite gRPC service against the embedded
// sqlite3 binary + example db.
package main

import (
	"flag"
	"log"
	"net"

	"google.golang.org/grpc"

	sqliteembed "github.com/accretional/proto-sqlite/sqlite"
	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

func main() {
	addr := flag.String("addr", ":50051", "listen address")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	srv := grpc.NewServer()
	sqlitepb.RegisterSqliteServer(srv, sqliteembed.NewServer())
	log.Printf("sqlite grpc server listening on %s", *addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
