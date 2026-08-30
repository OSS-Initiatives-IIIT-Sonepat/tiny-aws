package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// lbURL returns LB_URL env or default.
func lbURL() string {
	return getenv("LB_URL", "http://127.0.0.1:8088")
}

// Handles: tinyaws lb create | lb list
func runLB(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws lb create | tinyaws lb list")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		// lb is a standalone process; "create" just confirms it's reachable
		resp, err := httpGet(lbURL() + "/health")
		if err != nil {
			fmt.Fprintf(os.Stderr, "lb unreachable: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	case "list":
		resp, err := httpGet(lbURL() + "/targets")
		if err != nil {
			fmt.Fprintf(os.Stderr, "lb unreachable: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var targets []struct {
			URL     string `json:"url"`
			Healthy bool   `json:"healthy"`
		}
		if err := json.Unmarshal(body, &targets); err != nil {
			fmt.Println(string(body))
			return
		}
		for _, t := range targets {
			fmt.Printf("%-30s healthy=%v\n", t.URL, t.Healthy)
		}
	default:
		fmt.Printf("unknown lb command: %s\n", args[0])
		os.Exit(1)
	}
}
