package main

import (
	"testing"
	"time"
)

func setupExpiryTest(t *testing.T) {
	t.Helper()
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()
}

func TestRoleForKey_FutureExpiry(t *testing.T) {
	setupExpiryTest(t)

	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	iamDB.Exec(`INSERT INTO api_keys (key, role, expires_at) VALUES (?, ?, ?)`, "k-future", "admin", future)

	role := roleForKey("k-future")
	if role != "admin" {
		t.Errorf("role = %q, want admin", role)
	}
}

func TestRoleForKey_PastExpiry(t *testing.T) {
	setupExpiryTest(t)

	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	iamDB.Exec(`INSERT INTO api_keys (key, role, expires_at) VALUES (?, ?, ?)`, "k-expired", "admin", past)

	role := roleForKey("k-expired")
	if role != "" {
		t.Errorf("expired key returned role = %q, want empty", role)
	}
}

func TestRoleForKey_NullExpiry(t *testing.T) {
	setupExpiryTest(t)

	iamDB.Exec(`INSERT INTO api_keys (key, role, expires_at) VALUES (?, ?, NULL)`, "k-forever", "readonly")

	role := roleForKey("k-forever")
	if role != "readonly" {
		t.Errorf("role = %q, want readonly", role)
	}
}

func TestRoleForKey_UnknownKey(t *testing.T) {
	setupExpiryTest(t)

	role := roleForKey("does-not-exist")
	if role != "" {
		t.Errorf("unknown key returned role = %q, want empty", role)
	}
}
