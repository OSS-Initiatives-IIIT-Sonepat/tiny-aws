package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// getenv returns env var or fallback.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fetch(url string) (any, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var v any
	json.Unmarshal(body, &v)
	return v, nil
}

func main() {
	listenAddr := getenv("METADATA_ADDR", ":9006")

	registryURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")
	schedulerURL := getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
	networkingURL := getenv("NETWORKING_URL", "http://127.0.0.1:9005")

	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"healthy","service":"metadata"}`)
	})

	// GET /resources — aggregates nodes, instances, jobs, vpcs, subnets, security-groups
	http.HandleFunc("GET /resources", func(w http.ResponseWriter, r *http.Request) {
		resources := map[string]any{}

		if nodes, err := fetch(registryURL + "/nodes"); err == nil {
			resources["nodes"] = nodes
		}
		if instances, err := fetch(registryURL + "/instances"); err == nil {
			resources["instances"] = instances
		}
		if jobs, err := fetch(schedulerURL + "/jobs"); err == nil {
			resources["jobs"] = jobs
		}
		if vpcs, err := fetch(networkingURL + "/vpcs"); err == nil {
			resources["vpcs"] = vpcs
		}
		if subnets, err := fetch(networkingURL + "/subnets"); err == nil {
			resources["subnets"] = subnets
		}
		if sgs, err := fetch(networkingURL + "/security-groups"); err == nil {
			resources["security_groups"] = sgs
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resources)
	})

	log.Printf("metadata service listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
