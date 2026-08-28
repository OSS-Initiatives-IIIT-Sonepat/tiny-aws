package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

type Node struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

var counter uint64

func main() {
	registryURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")

	http.HandleFunc("/schedule", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp, err := http.Get(registryURL + "/nodes?role=compute")
		if err != nil {
			http.Error(w, "failed to reach registry", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "failed to read registry response", http.StatusBadGateway)
			return
		}

		var nodes map[string]Node
		if err := json.Unmarshal(body, &nodes); err != nil {
			http.Error(w, "failed to decode registry response", http.StatusBadGateway)
			return
		}

		healthy := make([]Node, 0)
		for _, node := range nodes {
			if node.Status == "healthy" {
				healthy = append(healthy, node)
			}
		}

		if len(healthy) == 0 {
			http.Error(w, "no healthy compute nodes", http.StatusServiceUnavailable)
			return
		}

		idx := atomic.AddUint64(&counter, 1) % uint64(len(healthy))
		selected := healthy[idx]

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"node_id": selected.ID,
		})
	})

	log.Println("scheduler listening on :9001")
	log.Fatal(http.ListenAndServe(":9001", nil))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
