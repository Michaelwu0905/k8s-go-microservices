package main

import (
	"net/http"
	"os"
	"strings"
)

func allowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	configured := os.Getenv("ALLOWED_ORIGINS")
	if configured == "" {
		configured = "http://localhost:3000,http://127.0.0.1:3000"
	}
	for _, allowed := range strings.Split(configured, ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}
func enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Add("Vary", "Origin")
		if origin != "" {
			if !allowedOrigin(origin) {
				authError(w, http.StatusForbidden, "不允许的请求来源")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		handler(w, r)
	}
}
