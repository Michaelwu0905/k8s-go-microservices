package main

import (
	"log"
	"net/http"
	"os"
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
