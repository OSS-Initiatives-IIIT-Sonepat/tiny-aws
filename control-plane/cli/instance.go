package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Handles: tinyaws instance launch|list|terminate|info ...
func runInstance(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws instance launch|list|terminate|info ...")
		os.Exit(1)
	}

	switch args[0] {
	case "launch":
		runInstanceLaunch()
	case "list":
		runInstanceList()
	case "terminate":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws instance terminate <id>")
			os.Exit(1)
		}
		runInstanceTerminate(args[1])
	case "info":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws instance info <id>")
			os.Exit(1)
		}
		runInstanceInfo(args[1])
	default:
		fmt.Println("usage: tinyaws instance launch|list|terminate|info ...")
		os.Exit(1)
	}
}

// POST /instances — launch a fake EC2 instance.
func runInstanceLaunch() {
	resp, err := httpPost(registryURL()+"/instances", "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "launch failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Println(string(body))
}

// GET /instances — list all instances.
func runInstanceList() {
	resp, err := httpGet(registryURL() + "/instances")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "list failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	for _, inst := range list {
		fmt.Printf("%s node=%s status=%s\n", inst["id"], inst["node_id"], inst["status"])
	}
}

// DELETE /instances/{id} — terminate instance.
func runInstanceTerminate(id string) {
	req, err := http.NewRequest(http.MethodDelete, registryURL()+"/instances/"+id, nil)
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
		fmt.Fprintf(os.Stderr, "terminate failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Println("terminated", id)
}

// GET /instances/{id} — show instance node, status, and workspace path.
func runInstanceInfo(id string) {
	resp, err := httpGet(registryURL() + "/instances/" + id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "info failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var inst map[string]any
	if err := json.Unmarshal(body, &inst); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	// workspace path mirrors what the agent creates: $TEMP/tinyaws/<id>
	fmt.Printf("id:        %s\n", inst["id"])
	fmt.Printf("node:      %s\n", inst["node_id"])
	fmt.Printf("status:    %s\n", inst["status"])
	fmt.Printf("workspace: %s/tinyaws/%s\n", os.TempDir(), inst["id"])
}
