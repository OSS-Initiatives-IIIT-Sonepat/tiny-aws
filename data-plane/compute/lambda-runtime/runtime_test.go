package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGetenv_Fallback(t *testing.T) {
	t.Setenv("LAMBDA_DB", "")
	if got := getenv("LAMBDA_DB", "lambda.db"); got != "lambda.db" {
		t.Errorf("got %q, want 'lambda.db'", got)
	}
}

func TestGetenv_Override(t *testing.T) {
	t.Setenv("LAMBDA_DB", "custom.db")
	if got := getenv("LAMBDA_DB", "lambda.db"); got != "custom.db" {
		t.Errorf("got %q, want 'custom.db'", got)
	}
}

func TestDBSchemaCreation(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS functions (
			name TEXT PRIMARY KEY, runtime TEXT NOT NULL,
			handler TEXT NOT NULL, bucket TEXT NOT NULL,
			key TEXT NOT NULL, created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("schema creation failed: %v", err)
	}

	// Insert and read back to verify schema works.
	_, err = testDB.Exec(`INSERT INTO functions VALUES ('fn1','python3','h.h','b','k','2026-01-01')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var name, runtime string
	err = testDB.QueryRow(`SELECT name, runtime FROM functions WHERE name='fn1'`).Scan(&name, &runtime)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if name != "fn1" || runtime != "python3" {
		t.Errorf("got name=%q runtime=%q", name, runtime)
	}
}
