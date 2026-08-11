package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Node struct represents a node in the cluster.
type Node struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	CPUCount  int       `json:"cpu_count"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen"`
}

// Heartbeat struct for incoming heartbeat requests
type Heartbeat struct {
	ID string `json:"id"`
}

// nodes map with mutex for thread-safe access
var (
	nodes   = make(map[string]Node)
	nodesMu sync.RWMutex
)

func main () {
	
	// register end points
	http.HandleFunc("/nodes", handleNodes)
	http.HandleFunc("/nodes/register", handleRegister)
	http.HandleFunc("/nodes/heartbeat", handleHeartbeat)

	// start health check goroutine
	go checkNodesHealth()

	log.Println("node registry listening on :9000")

	if err := http.ListenAndServe(":9000", nil); err != nil {
		log.Fatal(err)
	}

}

func handleNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodesMu.RLock()
	defer nodesMu.RUnlock()

	log.Println("GET /nodes")
	if err := json.NewEncoder(w).Encode(nodes); err != nil {
		http.Error(w, "failed to encode nodes", http.StatusInternalServerError)
	}

}

func handleRegister(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var node Node
	// decode the request body into a Node struct
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "failed to decode node", http.StatusBadRequest)
		return
	}

	// initialize node with status and last_seen
	node.Status = "healthy"
	node.LastSeen = time.Now()

	// acquire write lock before modifying nodes map
	nodesMu.Lock()
	nodes[node.ID] = node
	nodesMu.Unlock()

	log.Printf("POST /nodes/register - node %s registered", node.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// encode the response body as JSON
	if err := json.NewEncoder(w).Encode(node); err != nil {
		http.Error(w, "failed to encode node", http.StatusInternalServerError)
	}

}

func handleHeartbeat(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var heartbeat Heartbeat
	// decode the request body into a Heartbeat struct
	if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
		http.Error(w, "failed to decode heartbeat", http.StatusBadRequest)
		return
	}

	// acquire write lock before modifying nodes map
	nodesMu.Lock()
	if node, exists := nodes[heartbeat.ID]; exists {
		node.LastSeen = time.Now()
		node.Status = "healthy"
		nodes[heartbeat.ID] = node

		log.Printf("POST /nodes/heartbeat - node %s heartbeat received", heartbeat.ID)

		nodesMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(node)
	} else {
		log.Printf("POST /nodes/heartbeat - node %s not found", heartbeat.ID)
		nodesMu.Unlock()
		http.Error(w, "node not found", http.StatusNotFound)
	}

}

func checkNodesHealth() {
	ticker := time.NewTicker(10 * time.Second)

	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		nodesMu.Lock()
		for id, node := range nodes {
			if now.Sub(node.LastSeen) > 30*time.Second {
				if node.Status == "healthy" {
					log.Printf("node %s became unhealthy", id)
				}

				node.Status = "unhealthy"
				nodes[id] = node
			}
		}
		nodesMu.Unlock()
	}
}