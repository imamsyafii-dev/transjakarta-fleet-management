package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status": "ok",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	address := ":8080"
	log.Printf("server listening on %s", address)

	if err := http.ListenAndServe(address, mux); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
