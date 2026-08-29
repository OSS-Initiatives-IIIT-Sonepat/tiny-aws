package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Handles: tinyaws bucket create/list
func runBucket(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws bucket create|list ...")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		runBucketCreate(args[1:])
	case "list":
		runBucketList()
	default:
		fmt.Println("usage: tinyaws bucket create|list ...")
		os.Exit(1)
	}
}

// PUT /buckets/{name} - create a bucket.
func runBucketCreate(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws bucket create <name>")
		os.Exit(1)
	}

	name := args[0]
	url := objectStoreURL() + "/buckets/" + name

	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "create failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Printf("created bucket %q\n", name)
}

// GET /buckets - list all buckets.
func runBucketList() {
	resp, err := http.Get(objectStoreURL() + "/buckets")
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

	var buckets []struct {
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(body, &buckets); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	for _, b := range buckets {
		fmt.Printf("%s  created=%s\n", b.Name, b.CreatedAt)
	}
}