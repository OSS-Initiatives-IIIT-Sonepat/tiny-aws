package main

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// JobStore wraps SQLite persistence for scheduler jobs.
type JobStore struct {
	db *sql.DB
}

// Opens scheduler.db and creates the jobs table if missing.
func NewJobStore(path string) *JobStore {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id          TEXT PRIMARY KEY,
			node_id     TEXT NOT NULL,
			instance_id TEXT NOT NULL DEFAULT '',
			command     TEXT NOT NULL,
			status      TEXT NOT NULL,
			exit_code   INTEGER,
			stdout      TEXT NOT NULL DEFAULT '',
			stderr      TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			finished_at TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = db.Exec(`ALTER TABLE jobs ADD COLUMN instance_id TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE jobs ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0`)

	return &JobStore{db: db}
}

// Saves or updates one job row.
func (s *JobStore) Save(job Job) error {
	var exitCode any
	if job.ExitCode != nil {
		exitCode = *job.ExitCode
	}

	var finishedAt any
	if job.FinishedAt != nil {
		finishedAt = job.FinishedAt.Format(time.RFC3339)
	}

	_, err := s.db.Exec(
		`INSERT INTO jobs (id, node_id, instance_id, command, status, retry_count, exit_code, stdout, stderr, created_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   node_id = excluded.node_id,
		   instance_id = excluded.instance_id,
		   command = excluded.command,
		   status = excluded.status,
		   retry_count = excluded.retry_count,
		   exit_code = excluded.exit_code,
		   stdout = excluded.stdout,
		   stderr = excluded.stderr,
		   created_at = excluded.created_at,
		   finished_at = excluded.finished_at`,
		job.ID,
		job.NodeID,
		job.InstanceID,
		job.Command,
		job.Status,
		job.RetryCount,
		exitCode,
		job.Stdout,
		job.Stderr,
		job.CreatedAt.Format(time.RFC3339),
		finishedAt,
	)
	return err
}

// Loads all jobs from DB and returns the highest job-N sequence number.
func (s *JobStore) LoadAll() (map[string]Job, uint64, error) {
	rows, err := s.db.Query(`
		SELECT id, node_id, instance_id, command, status, retry_count, exit_code, stdout, stderr, created_at, finished_at
		FROM jobs`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	jobs := make(map[string]Job)
	var maxSeq uint64

	for rows.Next() {
		var job Job
		var created string
		var finished sql.NullString
		var exitCode sql.NullInt64

		if err := rows.Scan(
			&job.ID,
			&job.NodeID,
			&job.InstanceID,
			&job.Command,
			&job.Status,
			&job.RetryCount,
			&exitCode,
			&job.Stdout,
			&job.Stderr,
			&created,
			&finished,
		); err != nil {
			return nil, 0, err
		}

		job.CreatedAt, err = time.Parse(time.RFC3339, created)
		if err != nil {
			return nil, 0, err
		}

		if exitCode.Valid {
			code := int(exitCode.Int64)
			job.ExitCode = &code
		}

		if finished.Valid {
			t, err := time.Parse(time.RFC3339, finished.String)
			if err != nil {
				return nil, 0, err
			}
			job.FinishedAt = &t
		}

		jobs[job.ID] = job

		if seq, ok := parseJobSeq(job.ID); ok && seq > maxSeq {
			maxSeq = seq
		}
	}

	return jobs, maxSeq, rows.Err()
}

// Parses "job-12" → 12 so we don't reuse IDs after restart.
func parseJobSeq(id string) (uint64, bool) {
	if !strings.HasPrefix(id, "job-") {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(id, "job-"), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}