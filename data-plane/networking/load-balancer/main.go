package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Target is a backend agent with its health status.
type Target struct {
	URL     string `json:"url"`
	Healthy bool   `json:"healthy"`
}

var (
	targets   []Target
	targetsMu sync.RWMutex
	counter   uint64
)

// registryURL returns REGISTRY_URL env or default.
func registryURL() string {
	if v := os.Getenv("REGISTRY_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:9000"
}

// E3: polls registry every 10s to refresh healthy compute node list.
func syncTargets() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		refresh()
		<-ticker.C
	}
}

func refresh() {
	resp, err := http.Get(registryURL() + "/nodes?role=compute")
	if err != nil {
		log.Printf("lb: registry unreachable: %v", err)
		return
	}
	defer resp.Body.Close()

	var nodes map[string]struct {
		Hostname string `json:"hostname"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		log.Printf("lb: decode error: %v", err)
		return
	}

	// E4: health-check each node's agent; keep only responsive ones
	updated := []Target{}
	for _, node := range nodes {
		if node.Status != "healthy" {
			continue
		}
		agentURL := fmt.Sprintf("http://%s:8080", node.Hostname)
		t := Target{URL: agentURL, Healthy: checkHealth(agentURL)}
		updated = append(updated, t)
	}

	targetsMu.Lock()
	targets = updated
	targetsMu.Unlock()

	log.Printf("lb: targets refreshed: %d healthy", len(updated))
}

// E4: GETs /health on the agent; true if 200.
func checkHealth(agentURL string) bool {
	resp, err := http.Get(agentURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// E2: picks next target round-robin; returns nil if none healthy.
func nextTarget() *Target {
	targetsMu.RLock()
	defer targetsMu.RUnlock()

	healthy := []Target{}
	for _, t := range targets {
		if t.Healthy {
			healthy = append(healthy, t)
		}
	}
	if len(healthy) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&counter, 1) % uint64(len(healthy))
	t := healthy[idx]
	return &t
}

func main() {
	listenAddr := os.Getenv("LB_ADDR")
	if listenAddr == "" {
		listenAddr = ":8088"
	}

	// initial sync then background refresh
	go syncTargets()

	// /health — lb health
	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		targetsMu.RLock()
		n := len(targets)
		targetsMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"healthy","service":"lb","targets":%d}`, n)
	})

	// /targets — list current targets
	http.HandleFunc("GET /targets", func(w http.ResponseWriter, r *http.Request) {
		targetsMu.RLock()
		out, _ := json.Marshal(targets)
		targetsMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})

	// catch-all: proxy to next healthy agent
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := nextTarget()
		if t == nil {
			http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
			return
		}
		u, _ := url.Parse(t.URL)
		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("lb: proxy error to %s: %v", t.URL, err)
			http.Error(w, "backend error", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	})

	log.Printf("load balancer listening on %s", listenAddr)

	// E5: list backends endpoint used by CLI
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

// listTargets is a helper for the /targets response (used by CLI via GET /targets).
func listTargets() ([]Target, error) {
	resp, err := http.Get("http://127.0.0.1:8088/targets")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out []Target
	json.Unmarshal(body, &out)
	return out, nil
}
