package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// runAuth handles: tinyaws auth set-key <key> [--role admin|readonly] | auth whoami
func runAuth(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws auth set-key <key> [--role admin|readonly]")
		fmt.Println("       tinyaws auth whoami")
		os.Exit(1)
	}
	switch args[0] {
	case "set-key":
		authSetKey(args[1:])
	case "whoami":
		authWhoami()
	default:
		fmt.Printf("unknown auth command: %s\n", args[0])
		os.Exit(1)
	}
}

// authSetKey POSTs a key/role to /iam/keys on the registry.
func authSetKey(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws auth set-key <key> [--role admin|readonly] [--expires 2026-12-31T00:00:00Z]")
		os.Exit(1)
	}
	key := args[0]
	role := "admin"
	expires := ""
	for i := 1; i < len(args)-1; i++ {
		switch args[i] {
		case "--role":
			role = args[i+1]
		case "--expires":
			expires = args[i+1]
		}
	}
	payload := map[string]string{"key": key, "role": role}
	if expires != "" {
		payload["expires_at"] = expires
	}
	body, _ := json.Marshal(payload)
	resp, err := httpPost(registryURL()+"/iam/keys", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(resp.Body)
		fmt.Printf("error %d: %s\n", resp.StatusCode, msg)
		os.Exit(1)
	}
	if expires != "" {
		fmt.Printf("key set (role=%s expires=%s)\n", role, expires)
	} else {
		fmt.Printf("key set (role=%s)\n", role)
	}
}

// authWhoami shows the role of the current TINYAWS_API_KEY by querying /iam/keys.
func authWhoami() {
	currentKey := os.Getenv("TINYAWS_API_KEY")
	if currentKey == "" {
		fmt.Println("no TINYAWS_API_KEY set — unauthenticated")
		return
	}
	resp, err := httpGet(registryURL() + "/iam/keys")
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		fmt.Printf("error %d: %s\n", resp.StatusCode, msg)
		os.Exit(1)
	}
	var keys []struct {
		Key  string `json:"key"`
		Role string `json:"role"`
	}
	json.NewDecoder(resp.Body).Decode(&keys)
	for _, k := range keys {
		if k.Key == currentKey {
			fmt.Printf("key: %s  role: %s\n", currentKey, k.Role)
			return
		}
	}
	// env key not in DB — it may be the implicit admin env key
	fmt.Printf("key: %s  role: admin (env key)\n", currentKey)
}
