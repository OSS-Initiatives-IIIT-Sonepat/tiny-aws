package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSchedulerAuthMiddleware_NoKey(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	called := false
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/jobs", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("expected handler to be called when no key is configured")
	}
}

func TestSchedulerAuthMiddleware_WithKey(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// missing token
	req := httptest.NewRequest("GET", "/schedule", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing token: status = %d, want 401", w.Code)
	}

	// valid token
	req = httptest.NewRequest("GET", "/schedule", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("valid token: status = %d, want 200", w.Code)
	}
}

func TestSchedulerAuthMiddleware_HealthBypass(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health bypass: status = %d", w.Code)
	}
}

func TestSchedulerAuthMiddleware_AgentPollBypass(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// agent polls GET /jobs with node_id
	req := httptest.NewRequest("GET", "/jobs?node_id=n1&status=pending", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("agent poll bypass: status = %d", w.Code)
	}
}
