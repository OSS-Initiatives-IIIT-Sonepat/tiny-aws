package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Parsed CLI flags for object commands.
type objectArgs struct {
	bucket string
	key    string
	data   []byte
}

// Builds flat or bucket-scoped object URL.
func objectURL(bucket, key string) string {
	if bucket != "" {
		return objectStoreURL() + "/buckets/" + bucket + "/objects/" + key
	}
	return objectStoreURL() + "/objects/" + key
}

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

// Parses key, optional --bucket, --data, --file from CLI args.
func parseObjectArgs(args []string, needData bool) objectArgs {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws object put|get <key> [--bucket name] [--data text] [--file path]")
		os.Exit(1)
	}

	parsed := objectArgs{key: args[0]}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--bucket":
			if i+1 >= len(args) {
				fmt.Println("--bucket requires a value")
				os.Exit(1)
			}
			parsed.bucket = args[i+1]
			i++
		case "--data":
			if i+1 >= len(args) {
				fmt.Println("--data requires a value")
				os.Exit(1)
			}
			parsed.data = []byte(args[i+1])
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
			parsed.data = b
			i++
		}
	}

	if needData && len(parsed.data) == 0 {
		fmt.Println("put requires --data or --file")
		os.Exit(1)
	}

	return parsed
}

// PUT object (flat or bucket-scoped).
func runObjectPut(args []string) {
	parsed := parseObjectArgs(args, true)

	req, err := http.NewRequest(http.MethodPut, objectURL(parsed.bucket, parsed.key), bytes.NewReader(parsed.data))
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
		fmt.Fprintf(os.Stderr, "put failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	if parsed.bucket != "" {
		fmt.Printf("uploaded %q to bucket %q (%d bytes)\n", parsed.key, parsed.bucket, len(parsed.data))
	} else {
		fmt.Printf("uploaded object %q (%d bytes)\n", parsed.key, len(parsed.data))
	}
}

// GET object (flat or bucket-scoped).
func runObjectGet(args []string) {
	parsed := parseObjectArgs(args, false)

	resp, err := httpGet(objectURL(parsed.bucket, parsed.key))
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

	_, _ = os.Stdout.Write(body)
}
