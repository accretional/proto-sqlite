// Command ripgrpc starts the Sqlite gRPC server in-process, runs a
// query over the wire, and prints the result. Used by LET_IT_RIP.sh
// to prove the end-to-end service works.
package main

import (
	"context"
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
		Body: &sqlitepb.QueryRequest_Sql{
			Sql: "SELECT id, name, qty FROM widgets ORDER BY id;",
		},
	})
	if err != nil {
		log.Fatalf("Query: %v", err)
	}
	fmt.Printf("columns: %v\n", resp.GetColumn())
	for _, r := range resp.GetRow() {
		fmt.Printf("row: %v\n", r.GetCell())
	}
}
