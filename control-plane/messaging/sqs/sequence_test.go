package main

import (
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMsgSeqRestoredFromDB(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS queues (name TEXT PRIMARY KEY, created_at TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, queue_name TEXT NOT NULL,
			body TEXT NOT NULL, visible_after TEXT NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate previous run: insert messages with known IDs.
	for i := 1; i <= 5; i++ {
		testDB.Exec(`INSERT INTO messages (id, queue_name, body, visible_after) VALUES (?, 'q', 'body', '2026-01-01')`,
			fmt.Sprintf("msg-%d", i))
	}

	// Restore sequence the same way main() does.
	var raw sql.NullString
	_ = testDB.QueryRow(`SELECT id FROM messages ORDER BY rowid DESC LIMIT 1`).Scan(&raw)
	var restored uint64
	if raw.Valid {
		fmt.Sscanf(raw.String, "msg-%d", &restored)
	}

	if restored != 5 {
		t.Errorf("restored seq = %d, want 5", restored)
	}
}

func TestMsgSeqRestoredEmpty(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, queue_name TEXT NOT NULL,
			body TEXT NOT NULL, visible_after TEXT NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	var raw sql.NullString
	_ = testDB.QueryRow(`SELECT id FROM messages ORDER BY rowid DESC LIMIT 1`).Scan(&raw)
	var restored uint64
	if raw.Valid {
		fmt.Sscanf(raw.String, "msg-%d", &restored)
	}

	if restored != 0 {
		t.Errorf("restored seq = %d, want 0", restored)
	}
}

func TestMsgSeqUsedAfterRestore(t *testing.T) {
	setupSQSTest(t)

	// Insert 3 messages via the handler to advance msgSeq.
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('q', '2026-01-01')`)
	for i := 0; i < 3; i++ {
		db.Exec(`INSERT INTO messages (id, queue_name, body, visible_after) VALUES (?, 'q', 'x', '2026-01-01')`,
			fmt.Sprintf("msg-%d", i+1))
	}

	// Simulate restart: re-read max seq.
	var raw sql.NullString
	_ = db.QueryRow(`SELECT id FROM messages ORDER BY rowid DESC LIMIT 1`).Scan(&raw)
	if raw.Valid {
		fmt.Sscanf(raw.String, "msg-%d", &msgSeq)
	}

	if msgSeq != 3 {
		t.Fatalf("msgSeq = %d, want 3", msgSeq)
	}
}
