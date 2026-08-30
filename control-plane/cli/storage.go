package main

import (
	"fmt"
	"os"
)

// Handles: tinyaws storage node list
func runStorage(args []string) {
	if len(args) < 2 || args[0] != "node" || args[1] != "list" {
		fmt.Println("usage: tinyaws storage node list")
		os.Exit(1)
	}
	// reuse existing node list with storage role filter
	nodes, err := fetchNodes("storage")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, n := range nodes {
		fmt.Printf("%-20s status=%-10s hostname=%s\n", n.ID, n.Status, n.Hostname)
	}
}
