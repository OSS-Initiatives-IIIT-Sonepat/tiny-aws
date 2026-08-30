package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Target is a backend with its health status and optional service ID.
type Target struct {
	URL       string `json:"url"`
	Healthy   bool   `json:"healthy"`
	ServiceID string `json:"service_id,omitempty"` // set for service targets
	NodeID    string `json:"node_id,omitempty"`
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
	// fetch healthy compute nodes (hostname map)
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
		log.Printf("lb: decode nodes error: %v", err)
		return
	}

	// build node_id -> hostname map for service URL construction
	hostByNode := map[string]string{}
	for id, node := range nodes {
		hostByNode[id] = node.Hostname
	}

	updated := []Target{}

	// agent targets (for direct agent proxying)
	for id, node := range nodes {
		if node.Status != "healthy" {
			continue
		}
		agentURL := fmt.Sprintf("http://%s:8080", node.Hostname)
		if agentAddr := os.Getenv("AGENT_ADDR"); agentAddr != "" {
			// if a custom port is configured globally, use it
			agentURL = fmt.Sprintf("http://%s%s", node.Hostname, agentAddr)
		}
		t := Target{URL: agentURL, NodeID: id, Healthy: checkHealth(agentURL)}
		updated = append(updated, t)
	}

	// service targets — deployed apps registered by agents
	svcResp, err := http.Get(registryURL() + "/services?status=running")
	if err == nil {
		defer svcResp.Body.Close()
		var services []struct {
			ID     string `json:"id"`
			NodeID string `json:"node_id"`
			Port   int    `json:"port"`
			Status string `json:"status"`
		}
		if json.NewDecoder(svcResp.Body).Decode(&services) == nil {
			for _, svc := range services {
				if svc.Port == 0 {
					continue
				}
				host, ok := hostByNode[svc.NodeID]
				if !ok {
					continue
				}
				svcURL := fmt.Sprintf("http://%s:%d", host, svc.Port)
				t := Target{URL: svcURL, NodeID: svc.NodeID, ServiceID: svc.ID, Healthy: checkHealth(svcURL)}
				updated = append(updated, t)
			}
		}
	}

	targetsMu.Lock()
	targets = updated
	targetsMu.Unlock()

	healthy := 0
	for _, t := range updated {
		if t.Healthy {
			healthy++
		}
	}
	log.Printf("lb: targets refreshed: %d total, %d healthy", len(updated), healthy)
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
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
