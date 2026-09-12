package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHttpGet_AuthHeader(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "my-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != "Bearer my-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer my-token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := httpGet(server.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestHttpGet_NoAuth(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no auth header, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := httpGet(server.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestHttpPost_ContentType(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	body, _ := json.Marshal(map[string]string{"key": "val"})
	resp, err := httpPost(server.URL, "application/json", nil)
	_ = body
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestFetchNodes_MockRegistry(t *testing.T) {
	nodes := map[string]nodeRecord{
		"node-1": {ID: "node-1", Hostname: "h1", CPUCount: 4, Role: "compute", Status: "healthy"},
		"node-2": {ID: "node-2", Hostname: "h2", CPUCount: 2, Role: "storage", Status: "healthy"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodes)
	}))
	defer server.Close()

	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", server.URL)
	t.Setenv("TINYAWS_API_KEY", "")

	got, err := fetchNodes("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(got))
	}
}

func TestFetchNodes_RegistryDown(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", "http://127.0.0.1:1")
	t.Setenv("TINYAWS_API_KEY", "")

	_, err := fetchNodes("")
	if err == nil {
		t.Error("expected error for unreachable registry")
	}
}
