package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// Node struct represents a node in the rust cluster.
type Node struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	CPUCount int    `json:"cpu_count"`
}

func main () {
	
	// register end points
	http.HandleFunc("/nodes", handleNodes)


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