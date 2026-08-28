package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type Node struct {
	ID       string    `json:"id"`
	Hostname string    `json:"hostname"`
	CPUCount int       `json:"cpu_count"`
	Role     string    `json:"role"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

type Heartbeat struct {
	ID string `json:"id"`
}

var (
	store   *NodeStore
	nodes   map[string]Node
	nodesMu sync.RWMutex
)

func main() {
	store = NewNodeStore("registry.db")

	loaded, err := store.LoadAll()
	if err != nil {
		log.Fatal(err)
	}

	nodes = loaded

	http.HandleFunc("/nodes", handleNodes)
	http.HandleFunc("/nodes/register", handleRegister)
	http.HandleFunc("/nodes/heartbeat", handleHeartbeat)

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

	role := r.URL.Query().Get("role")

	nodesMu.RLock()
	defer nodesMu.RUnlock()

	log.Println("GET /nodes")

	if role == "" {
		if err := json.NewEncoder(w).Encode(nodes); err != nil {
			http.Error(w, "failed to encode nodes", http.StatusInternalServerError)
		}
		return
	}

	filtered := make(map[string]Node)

	for id, node := range nodes {
		if node.Role == role {
			filtered[id] = node
		}
	}

	if err := json.NewEncoder(w).Encode(filtered); err != nil {
		http.Error(w, "failed to encode nodes", http.StatusInternalServerError)
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var node Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "failed to decode node", http.StatusBadRequest)
		return
	}

	if node.Role == "" {
		node.Role = "compute"
	}

	node.Status = "healthy"
	node.LastSeen = time.Now()

	nodesMu.Lock()
	nodes[node.ID] = node
	nodesMu.Unlock()

	if err := store.Save(node); err != nil {
		http.Error(w, "failed to persist node", http.StatusInternalServerError)
		return
	}

	log.Printf("POST /nodes/register - node %s registered", node.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

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
	if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
		http.Error(w, "failed to decode heartbeat", http.StatusBadRequest)
		return
	}

	nodesMu.Lock()
	node, exists := nodes[heartbeat.ID]
	if exists {
		node.LastSeen = time.Now()
		node.Status = "healthy"
		nodes[heartbeat.ID] = node

		if err := store.Save(node); err != nil {
			log.Printf("failed to persist node %s: %v", heartbeat.ID, err)
		}

		log.Printf("POST /nodes/heartbeat - node %s heartbeat received", heartbeat.ID)

		nodesMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(node)
		return
	}

	log.Printf("POST /nodes/heartbeat - node %s not found", heartbeat.ID)
	nodesMu.Unlock()
	http.Error(w, "node not found", http.StatusNotFound)
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

				if err := store.Save(node); err != nil {
					log.Printf("failed to persist node %s: %v", id, err)
				}
			}
		}
		nodesMu.Unlock()
	}
}
