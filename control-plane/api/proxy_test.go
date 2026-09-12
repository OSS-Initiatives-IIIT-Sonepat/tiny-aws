package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxy_StripsV1Prefix(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	handler := proxy(backend.URL)
	req := httptest.NewRequest("GET", "/v1/nodes", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotPath != "/nodes" {
		t.Errorf("backend saw path %q, want /nodes", gotPath)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestProxy_V1RootBecomesSlash(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)
	req := httptest.NewRequest("GET", "/v1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotPath != "/" {
		t.Errorf("backend saw path %q, want /", gotPath)
	}
}

func TestProxy_ForwardsToCorrectBackend(t *testing.T) {
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend-a"))
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend-b"))
	}))
	defer backendB.Close()

	handlerA := proxy(backendA.URL)
	handlerB := proxy(backendB.URL)

	// Request to handler A should hit backend A
	reqA := httptest.NewRequest("GET", "/v1/nodes", nil)
	wA := httptest.NewRecorder()
	handlerA.ServeHTTP(wA, reqA)
	bodyA, _ := io.ReadAll(wA.Body)
	if string(bodyA) != "backend-a" {
		t.Errorf("handlerA body = %q, want backend-a", bodyA)
	}

	// Request to handler B should hit backend B
	reqB := httptest.NewRequest("GET", "/v1/jobs", nil)
	wB := httptest.NewRecorder()
	handlerB.ServeHTTP(wB, reqB)
	bodyB, _ := io.ReadAll(wB.Body)
	if string(bodyB) != "backend-b" {
		t.Errorf("handlerB body = %q, want backend-b", bodyB)
	}
}

func TestProxy_PreservesQueryString(t *testing.T) {
	var gotQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)
	req := httptest.NewRequest("GET", "/v1/nodes?role=compute", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotQuery != "role=compute" {
		t.Errorf("query = %q, want role=compute", gotQuery)
	}
}
