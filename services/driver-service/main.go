package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sikozonpc/ride-sharing/shared/rpcsecurity"
	"google.golang.org/grpc"
)

var GrpcAddr = ":9092"
var HttpAddr = ":8082"

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	service, err := NewService(ctx, databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("Initialize driver database: %v", err)
	}
	defer service.pool.Close()

	// Start HTTP server
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), time.Second)
			defer cancel()
			if err := service.pool.Ping(ctx); err != nil {
				http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Request from: %s to %s", r.RemoteAddr, r.URL.Path)
			writeJSON(w, http.StatusOK, map[string]string{"message": "Hello, Driver Service!"})
		})
		log.Println("Starting driver service HTTP on port 8082")
		log.Fatal(http.ListenAndServe(HttpAddr, nil))
	}()

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

	NewGrpcHandler(grpcServer, service)

	log.Printf("Starting gRPC server on port %s", lis.Addr().String())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
