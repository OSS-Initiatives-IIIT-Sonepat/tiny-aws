package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var auditDB *sql.DB

// proxy returns a reverse-proxy handler that strips /v1 prefix and forwards to backend.
func proxy(backend string) http.Handler {
	target, _ := url.Parse(backend)
	rp := httputil.NewSingleHostReverseProxy(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// strip /v1 so /v1/nodes -> /nodes on backend
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/v1")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		r.URL.RawPath = ""
		rp.ServeHTTP(w, r)
	})
}

// auditMiddleware logs every API call to SQLite — the CloudTrail equivalent.
func auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auditDB == nil {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now().UTC()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Milliseconds()

		// extract API key identity (masked)
		identity := ""
		if auth := r.Header.Get("Authorization"); auth != "" {
			key := strings.TrimPrefix(auth, "Bearer ")
			if len(key) > 8 {
				identity = key[:8] + "..."
			} else {
				identity = key
			}
		}

		auditDB.Exec(
			`INSERT INTO audit_log (timestamp, method, path, query, source_ip, identity, status, latency_ms)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			start.Format(time.RFC3339Nano), r.Method, r.URL.Path, r.URL.RawQuery,
			r.RemoteAddr, identity, rec.status, elapsed,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// handleAuditLog returns recent audit log entries.
// GET /v1/audit?limit=50
func handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if auditDB == nil {
		http.Error(w, "audit log not enabled", http.StatusServiceUnavailable)
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	rows, err := auditDB.Query(
		`SELECT timestamp, method, path, query, source_ip, identity, status, latency_ms
		 FROM audit_log ORDER BY rowid DESC LIMIT ?`, limit)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type entry struct {
		Timestamp string `json:"timestamp"`
		Method    string `json:"method"`
		Path      string `json:"path"`
		Query     string `json:"query,omitempty"`
		SourceIP  string `json:"source_ip"`
		Identity  string `json:"identity,omitempty"`
		Status    int    `json:"status"`
		LatencyMs int64  `json:"latency_ms"`
	}
	var entries []entry
	for rows.Next() {
		var e entry
		rows.Scan(&e.Timestamp, &e.Method, &e.Path, &e.Query, &e.SourceIP, &e.Identity, &e.Status, &e.LatencyMs)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func main() {
	registryURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")
	schedulerURL := getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
	objectStoreURL := getenv("OBJECT_STORE_URL", "http://127.0.0.1:7001")
	listenAddr := getenv("API_GATEWAY_ADDR", ":8000")

	// init audit log DB
	auditDBPath := getenv("AUDIT_DB", "audit.db")
	var err error
	auditDB, err = sql.Open("sqlite", auditDBPath)
	if err != nil {
		log.Printf("audit log disabled: %v", err)
		auditDB = nil
	} else {
		_, err = auditDB.Exec(`
			CREATE TABLE IF NOT EXISTS audit_log (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				timestamp  TEXT NOT NULL,
				method     TEXT NOT NULL,
				path       TEXT NOT NULL,
				query      TEXT NOT NULL DEFAULT '',
				source_ip  TEXT NOT NULL DEFAULT '',
				identity   TEXT NOT NULL DEFAULT '',
				status     INTEGER NOT NULL DEFAULT 0,
				latency_ms INTEGER NOT NULL DEFAULT 0
			);
		`)
		if err != nil {
			log.Printf("audit log disabled: %v", err)
			auditDB = nil
		} else {
			log.Println("audit log enabled (CloudTrail-style)")
		}
	}

	mux := http.NewServeMux()

	// audit log query endpoint
	mux.HandleFunc("GET /v1/audit", handleAuditLog)

	// /v1/health/* — forward health checks to the right backend
	mux.Handle("/v1/health/registry", proxy(registryURL))
	mux.Handle("/v1/health/scheduler", proxy(schedulerURL))
	mux.Handle("/v1/health/store", proxy(objectStoreURL))

	// registry
	for _, p := range []string{"/v1/nodes", "/v1/instances", "/v1/iam", "/v1/services"} {
		mux.Handle(p+"/", proxy(registryURL))
		mux.Handle(p, proxy(registryURL))
	}

	// scheduler
	for _, p := range []string{"/v1/jobs", "/v1/schedule"} {
		mux.Handle(p+"/", proxy(schedulerURL))
		mux.Handle(p, proxy(schedulerURL))
	}

	// object-store
	for _, p := range []string{"/v1/objects", "/v1/buckets"} {
		mux.Handle(p+"/", proxy(objectStoreURL))
		mux.Handle(p, proxy(objectStoreURL))
	}

	log.Printf("api gateway listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, auditMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

// getenv returns the env var value or fallback.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
