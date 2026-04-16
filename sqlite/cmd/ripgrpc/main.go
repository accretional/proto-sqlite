// Command ripgrpc starts the Sqlite gRPC server in-process, runs a
// query over the wire, and prints the result. Used by LET_IT_RIP.sh
// to prove the end-to-end service works.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sqliteembed "github.com/accretional/proto-sqlite/sqlite"
	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

func main() {
	socket := flag.String("socket", "", "UDS path to a running sqlited; if set, QueryRequest.socket_uri is populated and the gRPC server proxies SQL over this UDS")
	flag.Parse()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	sqlitepb.RegisterSqliteServer(srv, sqliteembed.NewServer())
	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Printf("serve: %v", err)
		}
	}()
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	cli := sqlitepb.NewSqliteClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Query(ctx, &sqlitepb.QueryRequest{
		SocketUri: *socket,
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT id, name, qty FROM widgets ORDER BY id;",
		},
	})
	if err != nil {
		log.Fatalf("Query: %v", err)
	}
	if *socket != "" {
		fmt.Printf("(via uds: %s)\n", *socket)
	}
	fmt.Printf("columns: %v\n", resp.GetColumn())
	for _, r := range resp.GetRow() {
		fmt.Printf("row: %v\n", r.GetCell())
	}
}
