package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Handles: tinyaws object put/get ...
func runObject(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws object put|get ...")
		os.Exit(1)
	}

	switch args[0] {
	case "put":
		runObjectPut(args[1:])
	case "get":
		runObjectGet(args[1:])
	default:
		fmt.Println("usage: tinyaws object put|get ...")
		os.Exit(1)
	}
}

// PUT /objects/{key} with --data or --file body.
func runObjectPut(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws object put <key> [--data text] [--file path]")
		os.Exit(1)
	}

	key := args[0]
	data := []byte{}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--data":
			if i+1 >= len(args) {
				fmt.Println("--data requires a value")
				os.Exit(1)
			}
			data = []byte(args[i+1])
			i++
		case "--file":
			if i+1 >= len(args) {
				fmt.Println("--file requires a path")
				os.Exit(1)
			}
			b, err := os.ReadFile(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "read file: %v\n", err)
				os.Exit(1)
			}
			data = b
			i++
		}
	}

	url := objectStoreURL() + "/objects/" + key
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
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
		fmt.Fprintf(os.Stderr, "put failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Printf("uploaded object %q (%d bytes)\n", key, len(data))
}

// GET /objects/{key} and print to stdout.
func runObjectGet(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws object get <key>")
		os.Exit(1)
	}

	key := args[0]
	url := objectStoreURL() + "/objects/" + key

	resp, err := http.Get(url)
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
		fmt.Fprintf(os.Stderr, "get failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	os.Stdout.Write(body)
}
