package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// Node struct represents a node in the rust cluster.
type Node struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	CPUCount int    `json:"cpu_count"`
}

// nodes map 
var nodes = make(map[string]Node)

func main () {
	
	// register end points
	http.HandleFunc("/nodes", handleNodes)
	http.HandleFunc("/nodes/register", handleRegister)


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

	// add the node to the nodes map
	nodes[node.ID] = node

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// encode the response body as JSON
	if err := json.NewEncoder(w).Encode(node); err != nil {
		http.Error(w, "failed to encode node", http.StatusInternalServerError)
	}

}