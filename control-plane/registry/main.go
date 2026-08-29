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
	instanceStore = NewInstanceStore(store.DB())
	iamDB = store.DB()

	loaded, err := store.LoadAll()
	if err != nil {
		log.Fatal(err)
	}

	nodes = loaded

	loadedInst, err := instanceStore.LoadAll()
	if err != nil {
		log.Fatal(err)
	}
	instances = loadedInst

	http.HandleFunc("GET /health", handleHealth)
	http.HandleFunc("GET /nodes", authMiddleware(handleNodes))
	http.HandleFunc("POST /nodes/register", authMiddleware(handleRegister))
	http.HandleFunc("POST /nodes/heartbeat", authMiddleware(handleHeartbeat))
	http.HandleFunc("DELETE /nodes/{id}", authMiddleware(handleNodeByID))
	http.HandleFunc("GET /instances", authMiddleware(handleInstances))
	http.HandleFunc("POST /instances", authMiddleware(handleInstances))
	http.HandleFunc("GET /instances/{id}", authMiddleware(handleInstanceGet))
	http.HandleFunc("DELETE /instances/{id}", authMiddleware(handleInstanceByID))
	http.HandleFunc("POST /iam/keys", authMiddleware(handleIAMKeyCreate))
	http.HandleFunc("DELETE /iam/keys/{key}", authMiddleware(handleIAMKeyDelete))
	http.HandleFunc("GET /iam/keys", authMiddleware(handleIAMKeyList))

	go checkNodesHealth()

	log.Println("node registry listening on :9000")

	if err := http.ListenAndServe(":9000", nil); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"service": "registry",
	})
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

func handleNodeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "node id required", http.StatusBadRequest)
		return
	}

	nodesMu.Lock()
	defer nodesMu.Unlock()

	if _, exists := nodes[id]; !exists {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	delete(nodes, id)
	if err := store.Delete(id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	log.Printf("DELETE /nodes/%s - removed", id)
	w.WriteHeader(http.StatusNoContent)
}

func checkNodesHealth() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		nodesMu.Lock()
		for id, node := range nodes {
			age := now.Sub(node.LastSeen)

			if age > 5*time.Minute && node.Status == "unhealthy" {
				delete(nodes, id)
				if err := store.Delete(id); err != nil {
					log.Printf("failed to delete stale node %s: %v", id, err)
				} else {
					log.Printf("pruned stale node %s", id)
				}
				continue
			}

			if age > 30*time.Second {
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
