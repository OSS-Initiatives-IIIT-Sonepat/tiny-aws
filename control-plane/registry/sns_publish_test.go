package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestSnsPublish_SendsCorrectRequest(t *testing.T) {
	var mu sync.Mutex
	var gotPath string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	snsPublish(srv.URL, "instance-launch", `{"instance_id":"i-1","node_id":"n-1"}`)

	mu.Lock()
	defer mu.Unlock()

	if gotPath != "/topics/instance-launch/publish" {
		t.Errorf("path = %q, want /topics/instance-launch/publish", gotPath)
	}
	msg, ok := gotBody["message"].(string)
	if !ok || msg == "" {
		t.Fatal("message field missing or empty")
	}
}

func TestSnsPublish_Unreachable(t *testing.T) {
	// must not panic on unreachable server
	snsPublish("http://127.0.0.1:1", "topic", "msg")
}
