package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestSnsPublish_HitsMockServer(t *testing.T) {
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

	snsPublish(srv.URL, "job-status", `{"job_id":"j-1","status":"done"}`)

	mu.Lock()
	defer mu.Unlock()

	if gotPath != "/topics/job-status/publish" {
		t.Errorf("path = %q, want /topics/job-status/publish", gotPath)
	}
	if gotBody["message"] == nil {
		t.Fatal("message field missing from body")
	}
}

func TestSnsPublish_ServerDown(t *testing.T) {
	// should not panic; just logs
	snsPublish("http://127.0.0.1:1", "test", "msg")
}
