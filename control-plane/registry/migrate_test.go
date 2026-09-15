package main

import (
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrate_FreshDB(t *testing.T) {
	// NewNodeStore creates tables + runs ALTER TABLE migrations.
	// On a fresh DB the columns already exist in CREATE TABLE,
	// so the ALTERs silently fail — that's the expected behavior.
	s := NewNodeStore(":memory:")
	if s == nil {
		t.Fatal("expected non-nil store on fresh DB")
	}

	// Verify the migrated columns are queryable.
	var dummy string
	err := s.DB().QueryRow(`SELECT addr FROM nodes LIMIT 1`).Scan(&dummy)
	// ErrNoRows is fine — column exists, table is just empty.
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("addr column missing: %v", err)
	}

	err = s.DB().QueryRow(`SELECT expires_at FROM api_keys LIMIT 1`).Scan(&dummy)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("expires_at column missing: %v", err)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	// Open the same in-memory DB path twice via the store constructor.
	// The second call re-runs the ALTER TABLE statements on a schema
	// that already has those columns — must not panic or error.
	s1 := NewNodeStore(":memory:")
	if s1 == nil {
		t.Fatal("first open failed")
	}

	// Simulate opening the same DB again (e.g. process restart).
	// In-memory DBs are unique per Open call, so we manually re-run
	// the migration on the same handle to prove idempotency.
	_, _ = s1.DB().Exec(`ALTER TABLE nodes ADD COLUMN addr TEXT NOT NULL DEFAULT ''`)
	_, _ = s1.DB().Exec(`ALTER TABLE api_keys ADD COLUMN expires_at TEXT`)

	// If we got here without panic, the migration is idempotent.
	// Verify data still works after double-migration.
	_, err := s1.DB().Exec(`INSERT INTO api_keys (key, role) VALUES ('k1', 'admin')`)
	if err != nil {
		t.Fatalf("insert after re-migration failed: %v", err)
	}

	var role string
	err = s1.DB().QueryRow(`SELECT role FROM api_keys WHERE key='k1'`).Scan(&role)
	if err != nil {
		t.Fatalf("select after re-migration failed: %v", err)
	}
	if role != "admin" {
		t.Errorf("role = %q, want admin", role)
	}
}

func TestMigrate_PreExistingDB_NoAddrColumn(t *testing.T) {
	// Simulate a DB created before the addr column existed.
	db := testDB(t)
	_, err := db.Exec(`
		CREATE TABLE nodes (
			id TEXT PRIMARY KEY, hostname TEXT NOT NULL,
			cpu_count INTEGER NOT NULL, role TEXT NOT NULL,
			status TEXT NOT NULL, last_seen TEXT NOT NULL
		);
		CREATE TABLE api_keys (key TEXT PRIMARY KEY, role TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Run the same migrations NewNodeStore runs.
	_, _ = db.Exec(`ALTER TABLE nodes ADD COLUMN addr TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE api_keys ADD COLUMN expires_at TEXT`)

	// Verify the new columns exist.
	_, err = db.Exec(`INSERT INTO nodes (id, hostname, addr, cpu_count, role, status, last_seen) VALUES ('n1','h','10.0.0.1',2,'compute','healthy','2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert with addr failed: %v", err)
	}

	var addr string
	db.QueryRow(`SELECT addr FROM nodes WHERE id='n1'`).Scan(&addr)
	if addr != "10.0.0.1" {
		t.Errorf("addr = %q", addr)
	}
}
