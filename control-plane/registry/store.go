package main

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type NodeStore struct {
	db *sql.DB
}

func NewNodeStore(path string) *NodeStore {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id         TEXT PRIMARY KEY,
			hostname   TEXT NOT NULL,
			cpu_count  INTEGER NOT NULL,
			role       TEXT NOT NULL,
			status     TEXT NOT NULL,
			last_seen  TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	return &NodeStore{db: db}
}

// DB exposes the underlying SQLite handle for other stores.
func (s *NodeStore) DB() *sql.DB {
	return s.db
}

func (s *NodeStore) Save(node Node) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (id, hostname, cpu_count, role, status, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   hostname = excluded.hostname,
		   cpu_count = excluded.cpu_count,
		   role = excluded.role,
		   status = excluded.status,
		   last_seen = excluded.last_seen`,
		node.ID,
		node.Hostname,
		node.CPUCount,
		node.Role,
		node.Status,
		node.LastSeen.Format(time.RFC3339),
	)
	return err
}

func (s *NodeStore) LoadAll() (map[string]Node, error) {
	rows, err := s.db.Query(`SELECT id, hostname, cpu_count, role, status, last_seen FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make(map[string]Node)

	for rows.Next() {
		var node Node
		var lastSeen string

		if err := rows.Scan(
			&node.ID,
			&node.Hostname,
			&node.CPUCount,
			&node.Role,
			&node.Status,
			&lastSeen,
		); err != nil {
			return nil, err
		}

		node.LastSeen, err = time.Parse(time.RFC3339, lastSeen)
		if err != nil {
			return nil, err
		}

		nodes[node.ID] = node
	}

	return nodes, rows.Err()
}
