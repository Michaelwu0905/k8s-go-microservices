package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/driver_client"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return allowedOrigin(r.Header.Get("Origin")) }}

var HttpAddr = ":8081"

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	auth, err := NewAuthStore(ctx, databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("Initialize authentication: %v", err)
	}
	defer auth.pool.Close()

	// Initialize gRPC clients
	driverClient, err := driver_client.NewDriverServiceClient()
	if err != nil {
		log.Fatal(err)
	}
	defer driverClient.Close()

	tripClient, err := trip_client.NewTripServiceClient()
	if err != nil {
		log.Fatal(err)
	}
	defer tripClient.Close()

	// Get services (for debugging purposes)
	http.HandleFunc("/services", enableCORS(handleGetServices))

	// Live stream driver locations
	http.HandleFunc("/ws/drivers", func(w http.ResponseWriter, r *http.Request) {
		handleDriversWebSocket(w, r, driverClient, tripClient)
	})

	registerTripRoutes(http.DefaultServeMux, tripClient)
	registerDriverRoutes(http.DefaultServeMux, driverClient, tripClient)
	auth.registerRoutes(http.DefaultServeMux, driverClient)

	log.Printf("Starting api gateway on port %s", HttpAddr)
	log.Fatal(http.ListenAndServe(HttpAddr, auth.middleware(http.DefaultServeMux)))
}
