package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// lambdaURL returns LAMBDA_URL env or default.
func lambdaURL() string {
	return getenv("LAMBDA_URL", "http://127.0.0.1:9007")
}

// Handles: tinyaws lambda create <name> --runtime python3|node20 [--handler file.fn] [--file code.zip]
//          tinyaws lambda list
//          tinyaws lambda invoke <name> [--event '{"key":"val"}']
func runLambda(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws lambda create <name> --runtime python3|node20 [--handler h] [--bucket b] [--key k]")
		fmt.Println("       tinyaws lambda list")
		fmt.Println("       tinyaws lambda invoke <name> [--event '{...}']")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		lambdaCreate(args[1:])
	case "list":
		lambdaList()
	case "invoke":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws lambda invoke <name> [--event '{...}']")
			os.Exit(1)
		}
		lambdaInvoke(args[1], args[2:])
	default:
		fmt.Printf("unknown lambda command: %s\n", args[0])
		os.Exit(1)
	}
}

// lambdaCreate POSTs function metadata; optionally uploads code zip if --file given.
func lambdaCreate(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws lambda create <name> --runtime python3|node20")
		os.Exit(1)
	}
	name := args[0]
	runtime, handler, bucket, key := "python3", "handler.handler", "lambdas", name+".zip"

	for i := 1; i < len(args)-1; i++ {
		switch args[i] {
		case "--runtime":
			runtime = args[i+1]
			i++
		case "--handler":
			handler = args[i+1]
			i++
		case "--bucket":
			bucket = args[i+1]
			i++
		case "--key":
			key = args[i+1]
			i++
		case "--file":
			// G2: upload zip to object store
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "read file: %v\n", err)
				os.Exit(1)
			}
			ensureBucket(bucket)
			uploadObject(bucket, key, data)
			fmt.Printf("uploaded %s -> %s/%s\n", args[i+1], bucket, key)
			i++
		}
	}

	payload, _ := json.Marshal(map[string]string{
		"name": name, "runtime": runtime, "handler": handler,
		"bucket": bucket, "key": key,
	})
	resp, err := httpPost(lambdaURL()+"/functions", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func lambdaList() {
	resp, err := httpGet(lambdaURL() + "/functions")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var list []map[string]any
	json.Unmarshal(body, &list)
	for _, fn := range list {
		fmt.Printf("%-20s runtime=%-8s handler=%s\n", fn["name"], fn["runtime"], fn["handler"])
	}
}

func lambdaInvoke(name string, args []string) {
	event := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--event" {
			event = args[i+1]
		}
	}
	resp, err := httpPost(lambdaURL()+"/functions/"+name+"/invoke", "application/json",
		bytes.NewReader([]byte(event)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
