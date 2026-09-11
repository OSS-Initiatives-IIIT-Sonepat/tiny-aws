package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoleForKey_EnvVar(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "test-secret")
	iamDB = nil

	if role := roleForKey("test-secret"); role != "admin" {
		t.Errorf("roleForKey(env key) = %q, want %q", role, "admin")
	}
	if role := roleForKey("wrong"); role != "" {
		t.Errorf("roleForKey(wrong) = %q, want empty", role)
	}
}

func TestRoleForKey_DBKey(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()

	// insert a key directly
	iamDB.Exec(`INSERT INTO api_keys (key, role) VALUES (?, ?)`, "db-key", "readonly")

	if role := roleForKey("db-key"); role != "readonly" {
		t.Errorf("roleForKey(db-key) = %q, want %q", role, "readonly")
	}
	if role := roleForKey("missing"); role != "" {
		t.Errorf("roleForKey(missing) = %q, want empty", role)
	}
}

func TestRoleForKey_ExpiredKey(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	iamDB.Exec(`INSERT INTO api_keys (key, role, expires_at) VALUES (?, ?, ?)`, "exp-key", "admin", past)

	if role := roleForKey("exp-key"); role != "" {
		t.Errorf("roleForKey(expired) = %q, want empty", role)
	}
}

func TestRoleForKey_ValidExpiry(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()

	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	iamDB.Exec(`INSERT INTO api_keys (key, role, expires_at) VALUES (?, ?, ?)`, "valid-key", "admin", future)

	if role := roleForKey("valid-key"); role != "admin" {
		t.Errorf("roleForKey(valid future) = %q, want %q", role, "admin")
	}
}

func TestRequireAuth_NoAuthConfigured(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	iamDB = nil

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	if !requireAuth(w, req) {
		t.Error("expected true when no auth configured")
	}
}

func TestRequireAuth_HealthBypass(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	iamDB = nil

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	if !requireAuth(w, req) {
		t.Error("expected health endpoint to bypass auth")
	}
}

func TestRequireAuth_RegisterBypass(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	iamDB = nil

	req := httptest.NewRequest("POST", "/nodes/register", nil)
	w := httptest.NewRecorder()
	if !requireAuth(w, req) {
		t.Error("expected register endpoint to bypass auth")
	}
}

func TestRequireAuth_MissingToken(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	iamDB = nil

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	if requireAuth(w, req) {
		t.Error("expected false when token is missing")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "secret")
	iamDB = nil

	req := httptest.NewRequest("GET", "/nodes", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	if !requireAuth(w, req) {
		t.Error("expected true with valid token")
	}
}

func TestRequireAuth_ReadonlyCannotMutate(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()
	iamDB.Exec(`INSERT INTO api_keys (key, role) VALUES (?, ?)`, "ro-key", "readonly")

	req := httptest.NewRequest("POST", "/instances", nil)
	req.Header.Set("Authorization", "Bearer ro-key")
	w := httptest.NewRecorder()
	if requireAuth(w, req) {
		t.Error("expected false for readonly key on POST")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestRequireAuth_ReadonlyCanRead(t *testing.T) {
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()
	iamDB.Exec(`INSERT INTO api_keys (key, role) VALUES (?, ?)`, "ro-key", "readonly")

	req := httptest.NewRequest("GET", "/nodes", nil)
	req.Header.Set("Authorization", "Bearer ro-key")
	w := httptest.NewRecorder()
	if !requireAuth(w, req) {
		t.Error("expected true for readonly key on GET")
	}
}
