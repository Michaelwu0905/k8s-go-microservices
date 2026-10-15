package main

import (
	"log"
	"net/http"

	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/driver_client"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

var HttpAddr = ":8081"

func main() {
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
		handleDriversWebSocket(w, r, driverClient)
	})

	registerTripRoutes(http.DefaultServeMux, tripClient)
	registerDriverRoutes(http.DefaultServeMux, driverClient, tripClient)

	log.Printf("Starting api gateway on port %s", HttpAddr)
	log.Fatal(http.ListenAndServe(HttpAddr, nil))
}
