package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// networkingURL returns NETWORKING_URL env or default.
func networkingURL() string {
	return getenv("NETWORKING_URL", "http://127.0.0.1:9005")
}

// Handles: tinyaws vpc create <name> <cidr> | vpc list
func runVPC(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws vpc create <name> <cidr> | vpc list")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		if len(args) < 3 {
			fmt.Println("usage: tinyaws vpc create <name> <cidr>")
			os.Exit(1)
		}
		payload, _ := json.Marshal(map[string]string{"name": args[1], "cidr": args[2]})
		resp, err := httpPost(networkingURL()+"/vpcs", "application/json", bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	case "list":
		resp, err := httpGet(networkingURL() + "/vpcs")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var list []map[string]any
		json.Unmarshal(body, &list)
		for _, v := range list {
			fmt.Printf("%-12s %-20s cidr=%s\n", v["id"], v["name"], v["cidr"])
		}
	default:
		fmt.Printf("unknown vpc command: %s\n", args[0])
		os.Exit(1)
	}
}

// Handles: tinyaws subnet create <vpc-id> <cidr> [--name n] | subnet list [--vpc vpc-id]
func runSubnet(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws subnet create <vpc-id> <cidr> | subnet list [--vpc <id>]")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		if len(args) < 3 {
			fmt.Println("usage: tinyaws subnet create <vpc-id> <cidr>")
			os.Exit(1)
		}
		payload, _ := json.Marshal(map[string]string{"vpc_id": args[1], "cidr": args[2]})
		resp, err := httpPost(networkingURL()+"/subnets", "application/json", bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	case "list":
		url := networkingURL() + "/subnets"
		for i := 1; i < len(args)-1; i++ {
			if args[i] == "--vpc" {
				url += "?vpc_id=" + args[i+1]
			}
		}
		resp, err := httpGet(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var list []map[string]any
		json.Unmarshal(body, &list)
		for _, s := range list {
			fmt.Printf("%-15s vpc=%-12s cidr=%s\n", s["id"], s["vpc_id"], s["cidr"])
		}
	}
}

// Handles: tinyaws sg create <vpc-id> <name> | sg list | sg allow <sg-id> <direction> <proto> <port> <cidr>
func runSG(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws sg create <vpc-id> <name> | sg list | sg allow <id> in|out tcp|udp <port> <cidr>")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		if len(args) < 3 {
			fmt.Println("usage: tinyaws sg create <vpc-id> <name>")
			os.Exit(1)
		}
		payload, _ := json.Marshal(map[string]string{"vpc_id": args[1], "name": args[2]})
		resp, err := httpPost(networkingURL()+"/security-groups", "application/json", bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	case "list":
		resp, err := httpGet(networkingURL() + "/security-groups")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var list []map[string]any
		json.Unmarshal(body, &list)
		for _, sg := range list {
			fmt.Printf("%-12s %-20s vpc=%s\n", sg["id"], sg["name"], sg["vpc_id"])
		}
	case "allow", "deny":
		// sg allow <id> <direction> <proto> <port> <cidr>
		if len(args) < 6 {
			fmt.Println("usage: tinyaws sg allow|deny <id> <direction> <proto> <port> <cidr>")
			os.Exit(1)
		}
		sgID := args[1]
		payload, _ := json.Marshal(map[string]any{
			"direction": args[2], "action": args[0],
			"protocol": args[3], "port": args[4], "cidr": args[5],
		})
		resp, err := httpPost(networkingURL()+"/security-groups/"+sgID+"/rules", "application/json", bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	}
}
