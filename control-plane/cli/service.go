package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Handles: tinyaws service list | stop <id> | logs <id>
func runService(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws service list")
		fmt.Println("       tinyaws service stop <id>")
		fmt.Println("       tinyaws service logs <id>")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		serviceList()
	case "stop":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws service stop <id>")
			os.Exit(1)
		}
		serviceStop(args[1])
	case "logs":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws service logs <id>")
			os.Exit(1)
		}
		serviceLogs(args[1])
	default:
		fmt.Printf("unknown service command: %s\n", args[0])
		os.Exit(1)
	}
}

// GET /services — list all running services.
func serviceList() {
	resp, err := httpGet(registryURL() + "/services")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "list failed %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}
	var svcs []map[string]any
	if err := json.Unmarshal(body, &svcs); err != nil || len(svcs) == 0 {
		fmt.Println("(no services)")
		return
	}
	fmt.Printf("%-12s %-12s %-20s %-8s %-8s\n", "ID", "INSTANCE", "NODE", "PORT", "STATUS")
	for _, s := range svcs {
		fmt.Printf("%-12s %-12s %-20s %-8v %-8s\n",
			s["id"], s["instance_id"], s["node_id"], s["port"], s["status"])
	}
}

// DELETE /services/{id} — mark service stopped (agent kills the PID).
func serviceStop(id string) {
	req, err := http.NewRequest(http.MethodDelete, registryURL()+"/services/"+id, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	resp, err := httpDo(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "stop failed %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}
	fmt.Println("service", id, "stopped")
}

// Fetches service.log from the object store (agent streams logs there).
// Log key convention: logs/<service_id>/service.log
func serviceLogs(id string) {
	// first look up the service to know its deploy bucket/key pattern
	resp, err := httpGet(registryURL() + "/services/" + id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		fmt.Fprintf(os.Stderr, "service %s not found\n", id)
		os.Exit(1)
	}

	// logs are stored in object store under logs/<id>/service.log
	logKey := "logs/" + id + "/service.log"
	logResp, err := httpGet(objectStoreURL() + "/objects/" + logKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching logs: %v\n", err)
		os.Exit(1)
	}
	defer logResp.Body.Close()
	if logResp.StatusCode == 404 {
		fmt.Println("(no logs yet — service may still be starting)")
		return
	}
	io.Copy(os.Stdout, logResp.Body)
}
