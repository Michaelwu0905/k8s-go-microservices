package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"github.com/sikozonpc/ride-sharing/shared/types"
)

type Service struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// This is a dummy function to test the K8s DNS
func handleGetServices(w http.ResponseWriter, r *http.Request) {
	// Logging the pod name to visually see the replica scaling load balancing
	podName := os.Getenv("HOSTNAME") // k8s sets this automatically
	log.Printf("Pod %s handling request from: %s to %s", podName, r.RemoteAddr, r.URL.Path)

	var services []Service
	services = append(services, Service{
		URL:  "http://localhost:8082",
		Name: "driver-service",
	})

	err := writeJSON(w, http.StatusOK, services)
	if err != nil {
		return
	}
}

func handleCreateTrip(w http.ResponseWriter, r *http.Request, tripClient *trip_client.TripServiceClient) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Pickup      types.Location `json:"pickup"`
		Destination types.Location `json:"destination"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := tripClient.CreateTrip(r.Context(), &pb.CreateTripRequest{
		StartLocation: &pb.Coordinate{
			Latitude:  float32(reqBody.Pickup.Latitude),
			Longitude: float32(reqBody.Pickup.Longitude),
		},
		EndLocation: &pb.Coordinate{
			Latitude:  float32(reqBody.Destination.Latitude),
			Longitude: float32(reqBody.Destination.Longitude),
		},
	})
	if err != nil {
		http.Error(w, "Failed to create trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleAcceptTrip(w http.ResponseWriter, r *http.Request, tripClient *trip_client.TripServiceClient) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		TripID   string `json:"trip_id"`
		DriverID string `json:"driver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := tripClient.AcceptTrip(r.Context(), &pb.AcceptTripRequest{
		TripId:   reqBody.TripID,
		DriverId: reqBody.DriverID,
	})
	if err != nil {
		http.Error(w, "Failed to accept trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleGetTrip(w http.ResponseWriter, r *http.Request, tripClient *trip_client.TripServiceClient) {
	tripID := r.PathValue("id")

	resp, err := tripClient.GetTrip(r.Context(), &pb.GetTripRequest{TripId: tripID})
	if err != nil {
		http.Error(w, "Failed to get trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
