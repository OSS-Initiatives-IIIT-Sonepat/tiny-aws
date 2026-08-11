package main

import (
	"encoding/json"
	"log"
	"net/http"
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

// nodes map 
var nodes = make(map[string]Node)

func main () {
	
	// register end points
	http.HandleFunc("/nodes", handleNodes)
	http.HandleFunc("/nodes/register", handleRegister)
	http.HandleFunc("/nodes/heartbeat", handleHeartbeat)

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

	// add the node to the nodes map
	nodes[node.ID] = node

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

	// update node's last_seen and status
	if node, exists := nodes[heartbeat.ID]; exists {
		node.LastSeen = time.Now()
		node.Status = "healthy"
		nodes[heartbeat.ID] = node

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(node)
	} else {
		http.Error(w, "node not found", http.StatusNotFound)
	}

}