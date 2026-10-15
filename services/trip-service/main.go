package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/sikozonpc/ride-sharing/shared/rpcsecurity"
	"google.golang.org/grpc"
)

var GrpcAddr = ":9093"

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	store, err := NewTripStore(ctx, databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("Initialize PostgreSQL: %v", err)
	}
	defer store.pool.Close()
	// Start gRPC server
	lis, err := net.Listen("tcp", GrpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	options, err := rpcsecurity.ServerOptions()
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer(options...)

	NewGrpcHandler(grpcServer, store)

	log.Printf("Starting gRPC server Trip service on port %s", lis.Addr().String())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
