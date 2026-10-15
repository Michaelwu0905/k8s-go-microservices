package main

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/driver_client"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
	"net/http"
	"time"
)

func handleDriversWebSocket(w http.ResponseWriter, r *http.Request, client *driver_client.DriverServiceClient, trips *trip_client.TripServiceClient) {
	if !requireRole(w, r, "rider") {
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(4096)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stream, err := client.StreamNearbyDrivers(ctx)
	if err != nil {
		return
	}
	defer stream.CloseSend()
	go func() {
		defer cancel()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var input struct {
				Latitude         float32 `json:"latitude"`
				Longitude        float32 `json:"longitude"`
				AssignedDriverID string  `json:"assigned_driver_id"`
			}
			if err = json.Unmarshal(message, &input); err != nil {
				continue
			}
			assigned, err := assignedDriver(r.WithContext(ctx), trips)
			if err != nil {
				return
			}
			if input.AssignedDriverID != "" && input.AssignedDriverID != assigned {
				return
			}
			if err = stream.Send(&pb.FindNearbyDriversRequest{Location: &pb.Location{Latitude: input.Latitude, Longitude: input.Longitude}, AssignedDriverId: assigned}); err != nil {
				return
			}
		}
	}()
	for {
		result, err := stream.Recv()
		if err != nil {
			return
		}
		assigned, err := assignedDriver(r.WithContext(ctx), trips)
		if err != nil {
			return
		}
		visible := make([]*pb.Driver, 0, len(result.NearbyDrivers))
		for _, driver := range result.NearbyDrivers {
			if driver.DriverId == assigned || (driver.Online && driver.DistanceMeters <= 5000) {
				visible = append(visible, driver)
			}
		}
		result.NearbyDrivers = visible
		payload, err := json.Marshal(result.NearbyDrivers)
		if err != nil {
			return
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err = conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}
