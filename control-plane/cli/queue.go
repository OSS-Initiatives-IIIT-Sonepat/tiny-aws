package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// sqsURL returns SQS_URL env or default.
func sqsURL() string {
	return getenv("SQS_URL", "http://127.0.0.1:9003")
}

// Handles: tinyaws queue create <name> | send <name> <body> | receive <name>
func runQueue(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws queue create <name>")
		fmt.Println("       tinyaws queue send <name> <message>")
		fmt.Println("       tinyaws queue receive <name>")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws queue create <name>")
			os.Exit(1)
		}
		queueCreate(args[1])
	case "send":
		if len(args) < 3 {
			fmt.Println("usage: tinyaws queue send <name> <message>")
			os.Exit(1)
		}
		queueSend(args[1], args[2])
	case "receive":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws queue receive <name>")
			os.Exit(1)
		}
		queueReceive(args[1])
	default:
		fmt.Printf("unknown queue command: %s\n", args[0])
		os.Exit(1)
	}
}

// POST /queues/{name} — create queue.
func queueCreate(name string) {
	resp, err := httpPost(sqsURL()+"/queues/"+name, "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "create failed %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}
	fmt.Println("queue created:", name)
}

// POST /queues/{name}/messages — send message.
func queueSend(name, message string) {
	payload, _ := json.Marshal(map[string]string{"body": message})
	resp, err := httpPost(sqsURL()+"/queues/"+name+"/messages", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "send failed %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}
	var result map[string]string
	json.Unmarshal(body, &result)
	fmt.Println("sent:", result["id"])
}

// GET /queues/{name}/messages — receive next message.
func queueReceive(name string) {
	resp, err := httpGet(sqsURL() + "/queues/" + name + "/messages")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "receive failed %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	// null means empty queue
	if string(body) == "null\n" || string(body) == "null" {
		fmt.Println("(empty)")
		return
	}

	var msg struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		fmt.Println(string(body))
		return
	}

	fmt.Printf("id: %s\nbody: %s\n", msg.ID, msg.Body)

	// ack automatically
	req, _ := http.NewRequest(http.MethodDelete, sqsURL()+"/queues/"+name+"/messages/"+msg.ID, nil)
	if req != nil {
		resp, _ := httpDo(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}
