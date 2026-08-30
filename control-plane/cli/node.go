package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Node shape returned by registry GET /nodes.
type nodeRecord struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	CPUCount int    `json:"cpu_count"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

// Handles: tinyaws node list [--role compute|storage].
func runNode(args []string) {
	if len(args) < 1 || args[0] != "list" {
		fmt.Println("usage: tinyaws node list [--role compute|storage] [--healthy-only]")
		os.Exit(1)
	}

	role := ""
	healthyOnly := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--role":
			if i+1 >= len(args) {
				fmt.Println("--role requires a value")
				os.Exit(1)
			}
			role = args[i+1]
			i++
		case "--healthy-only":
			healthyOnly = true
		}
	}

	nodes, err := fetchNodes(role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, n := range nodes {
		if healthyOnly && n.Status != "healthy" {
			continue
		}
		fmt.Printf("%-20s role=%-8s status=%-10s hostname=%s cpus=%d\n",
			n.ID, n.Role, n.Status, n.Hostname, n.CPUCount)
	}
}

// GET /nodes from registry, optionally filtered by role.
func fetchNodes(role string) ([]nodeRecord, error) {
	url := registryURL() + "/nodes"
	if role != "" {
		url += "?role=" + role
	}

	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("registry returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]nodeRecord
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	out := make([]nodeRecord, 0, len(raw))
	for _, n := range raw {
		out = append(out, n)
	}
	return out, nil
}
