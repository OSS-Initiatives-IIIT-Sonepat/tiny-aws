package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Instance struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// registryURL returns REGISTRY_URL env or default.
func registryURL() string {
	if v := os.Getenv("REGISTRY_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:9000"
}

// workspacePath mirrors the agent convention: $TEMP/tinyaws/<instance-id>
func workspacePath(instanceID string) string {
	return filepath.Join(os.TempDir(), "tinyaws", instanceID)
}

// reconcileLoop polls registry and removes workspace dirs for terminated instances.
func reconcileLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	cleaned := make(map[string]bool)

	for range ticker.C {
		resp, err := http.Get(registryURL() + "/instances")
		if err != nil {
			log.Printf("reconcile: registry unreachable: %v", err)
			continue
		}

		var instances []Instance
		if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
			resp.Body.Close()
			log.Printf("reconcile: decode error: %v", err)
			continue
		}
		resp.Body.Close()

		for _, inst := range instances {
			if inst.Status != "terminated" || cleaned[inst.ID] {
				continue
			}
			path := workspacePath(inst.ID)
			if err := os.RemoveAll(path); err != nil {
				log.Printf("reconcile: remove workspace %s: %v", path, err)
			} else {
				log.Printf("reconcile: cleaned workspace %s", path)
				cleaned[inst.ID] = true
			}
		}
	}
}

func main() {
	listenAddr := os.Getenv("CONTROLLER_ADDR")
	if listenAddr == "" {
		listenAddr = ":9002"
	}

	go reconcileLoop()

	// health endpoint
	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"healthy","service":"controller"}`)
	})

	log.Printf("controller listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
