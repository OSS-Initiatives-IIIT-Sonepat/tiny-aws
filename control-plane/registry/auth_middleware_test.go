package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test authMiddleware wraps a handler, checks auth, and calls through.
func TestAuthMiddleware_CallsThrough(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	iamDB = nil

	called := false
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	wrapped := authMiddleware(inner)
	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	wrapped(w, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthMiddleware_BlocksUnauthorized(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "my-secret")
	iamDB = nil

	called := false
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
	}

	wrapped := authMiddleware(inner)
	req := httptest.NewRequest("GET", "/nodes", nil)
	// no Authorization header
	w := httptest.NewRecorder()
	wrapped(w, req)

	if called {
		t.Error("inner handler should NOT be called without auth")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_AllowsValidToken(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "my-secret")
	iamDB = nil

	called := false
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	wrapped := authMiddleware(inner)
	req := httptest.NewRequest("POST", "/instances", nil)
	req.Header.Set("Authorization", "Bearer my-secret")
	w := httptest.NewRecorder()
	wrapped(w, req)

	if !called {
		t.Error("inner handler was not called with valid token")
	}
}
