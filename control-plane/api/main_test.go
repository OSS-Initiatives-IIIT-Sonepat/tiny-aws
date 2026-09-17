package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGetenv_Default(t *testing.T) {
	t.Setenv("TEST_KEY", "")
	if got := getenv("TEST_KEY", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestGetenv_Set(t *testing.T) {
	t.Setenv("TEST_KEY", "value")
	if got := getenv("TEST_KEY", "fallback"); got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

func TestProxy_StripPrefix(t *testing.T) {
	// proxy() strips /v1 — verify the handler creation doesn't panic
	h := proxy("http://127.0.0.1:9000")
	if h == nil {
		t.Fatal("proxy returned nil handler")
	}
}

func setupAuditTest(t *testing.T) {
	t.Helper()
	var err error
	auditDB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = auditDB.Exec(`
		CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL, method TEXT NOT NULL, path TEXT NOT NULL,
			query TEXT NOT NULL DEFAULT '', source_ip TEXT NOT NULL DEFAULT '',
			identity TEXT NOT NULL DEFAULT '', status INTEGER NOT NULL DEFAULT 0,
			latency_ms INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { auditDB.Close(); auditDB = nil })
}

func TestAuditMiddleware_LogsRequest(t *testing.T) {
	setupAuditTest(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auditMiddleware(inner)

	req := httptest.NewRequest("GET", "/v1/nodes?role=compute", nil)
	req.Header.Set("Authorization", "Bearer test-key-12345")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var count int
	auditDB.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 audit entry, got %d", count)
	}

	var method, path, query, identity string
	var status int
	auditDB.QueryRow(`SELECT method, path, query, identity, status FROM audit_log`).
		Scan(&method, &path, &query, &identity, &status)
	if method != "GET" {
		t.Errorf("method = %q", method)
	}
	if path != "/v1/nodes" {
		t.Errorf("path = %q", path)
	}
	if query != "role=compute" {
		t.Errorf("query = %q", query)
	}
	if identity != "test-key..." {
		t.Errorf("identity = %q, want test-key...", identity)
	}
	if status != 200 {
		t.Errorf("status = %d", status)
	}
}

func TestAuditMiddleware_NoAuth(t *testing.T) {
	setupAuditTest(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler := auditMiddleware(inner)

	req := httptest.NewRequest("POST", "/v1/jobs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var identity string
	var status int
	auditDB.QueryRow(`SELECT identity, status FROM audit_log`).Scan(&identity, &status)
	if identity != "" {
		t.Errorf("identity = %q, want empty", identity)
	}
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestHandleAuditLog(t *testing.T) {
	setupAuditTest(t)

	// insert some entries
	auditDB.Exec(`INSERT INTO audit_log (timestamp, method, path, query, source_ip, identity, status, latency_ms) VALUES ('2026-01-01T00:00:00Z', 'GET', '/nodes', '', '127.0.0.1', '', 200, 5)`)
	auditDB.Exec(`INSERT INTO audit_log (timestamp, method, path, query, source_ip, identity, status, latency_ms) VALUES ('2026-01-01T00:00:01Z', 'POST', '/jobs', '', '127.0.0.1', 'key...', 201, 10)`)

	req := httptest.NewRequest("GET", "/v1/audit?limit=10", nil)
	w := httptest.NewRecorder()
	handleAuditLog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var entries []map[string]any
	json.Unmarshal(w.Body.Bytes(), &entries)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	// most recent first
	if entries[0]["method"] != "POST" {
		t.Errorf("first entry method = %v, want POST (most recent)", entries[0]["method"])
	}
}

func TestHandleAuditLog_Empty(t *testing.T) {
	setupAuditTest(t)

	req := httptest.NewRequest("GET", "/v1/audit", nil)
	w := httptest.NewRecorder()
	handleAuditLog(w, req)

	var entries []map[string]any
	json.Unmarshal(w.Body.Bytes(), &entries)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
