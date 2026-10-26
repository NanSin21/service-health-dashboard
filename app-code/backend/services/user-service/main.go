package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/health", healthCheckHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081" // default port
	}
	address := ":" + port
	log.Printf("user-service listening on %s", address)
	log.Fatal(http.ListenAndServe(address, nil))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service": "user-service",
		"status":  "UP",
		"code":    "200",
	})
}
