package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// Function holds lambda function metadata.
type Function struct {
	Name      string `json:"name"`
	Runtime   string `json:"runtime"`  // python3 | node20
	Handler   string `json:"handler"`  // file.function
	Bucket    string `json:"bucket"`   // object-store bucket
	Key       string `json:"key"`      // object key (zip)
	CreatedAt string `json:"created_at"`
}

// InvokeResult is returned from a sync invocation.
type InvokeResult struct {
	StatusCode int    `json:"status_code"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

var (
	db      *sql.DB
	jobSeq  uint64
)

func main() {
	dbPath := getenv("LAMBDA_DB", "lambda.db")
	listenAddr := getenv("LAMBDA_ADDR", ":9007")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS functions (
			name TEXT PRIMARY KEY,
			runtime TEXT NOT NULL,
			handler TEXT NOT NULL,
			bucket TEXT NOT NULL,
			key TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("GET /health", handleHealth)
	http.HandleFunc("POST /functions", handleCreate)
	http.HandleFunc("GET /functions", handleList)
	http.HandleFunc("GET /functions/{name}", handleGet)
	http.HandleFunc("POST /functions/{name}/invoke", handleInvoke)

	log.Printf("lambda service listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"healthy","service":"lambda"}`)
}

// POST /functions — create/update function metadata.
func handleCreate(w http.ResponseWriter, r *http.Request) {
	var fn Function
	if err := json.NewDecoder(r.Body).Decode(&fn); err != nil || fn.Name == "" || fn.Runtime == "" {
		http.Error(w, "name and runtime required", http.StatusBadRequest)
		return
	}
	if fn.Handler == "" {
		fn.Handler = "handler.handler"
	}
	fn.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO functions VALUES (?,?,?,?,?,?) ON CONFLICT(name) DO UPDATE SET
		 runtime=excluded.runtime, handler=excluded.handler, bucket=excluded.bucket,
		 key=excluded.key, created_at=excluded.created_at`,
		fn.Name, fn.Runtime, fn.Handler, fn.Bucket, fn.Key, fn.CreatedAt,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /functions - %s (%s)", fn.Name, fn.Runtime)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fn)
}

// GET /functions — list all functions.
func handleList(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT name, runtime, handler, bucket, key, created_at FROM functions`)
	defer rows.Close()
	out := []Function{}
	for rows.Next() {
		var fn Function
		rows.Scan(&fn.Name, &fn.Runtime, &fn.Handler, &fn.Bucket, &fn.Key, &fn.CreatedAt)
		out = append(out, fn)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// GET /functions/{name}
func handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var fn Function
	err := db.QueryRow(`SELECT name, runtime, handler, bucket, key, created_at FROM functions WHERE name=?`, name).
		Scan(&fn.Name, &fn.Runtime, &fn.Handler, &fn.Bucket, &fn.Key, &fn.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fn)
}

// G6: POST /functions/{name}/invoke — submit a lambda job via scheduler and wait.
func handleInvoke(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var fn Function
	err := db.QueryRow(`SELECT name, runtime, handler, bucket, key, created_at FROM functions WHERE name=?`, name).
		Scan(&fn.Name, &fn.Runtime, &fn.Handler, &fn.Bucket, &fn.Key, &fn.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "function not found", http.StatusNotFound)
		return
	}

	// read optional event payload
	event, _ := io.ReadAll(r.Body)

	schedulerURL := getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
	objectStoreURL := getenv("OBJECT_STORE_URL", "http://127.0.0.1:7001")

	// build download URL for function zip
	codeURL := fmt.Sprintf("%s/buckets/%s/objects/%s", objectStoreURL, fn.Bucket, fn.Key)

	// G7: job command encodes runtime, handler, event via env vars passed as shell prefix
	cmd := buildInvokeCommand(fn.Runtime, fn.Handler, codeURL, string(event))

	// G7: mark job as lambda type via command prefix
	seq := atomic.AddUint64(&jobSeq, 1)
	_ = seq

	payload, _ := json.Marshal(map[string]string{"command": cmd})
	resp, err := http.Post(schedulerURL+"/jobs", "application/json", bytes.NewReader(payload))
	if err != nil {
		http.Error(w, "scheduler error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var job struct {
		JobID string `json:"job_id"`
	}
	json.NewDecoder(resp.Body).Decode(&job)

	// poll for result (max 60s)
	result := pollJobResult(schedulerURL, job.JobID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// buildInvokeCommand builds the shell command for a lambda invocation.
func buildInvokeCommand(runtime, handler, codeURL, event string) string {
	// ponytail: inline shell command; add proper sandbox when security matters
	switch runtime {
	case "python3":
		return fmt.Sprintf(
			`python3 -c "import urllib.request,zipfile,os,json,importlib,sys; `+
				`d='/tmp/tinyaws-lambda'; os.makedirs(d,exist_ok=True); `+
				`urllib.request.urlretrieve('%s',d+'/fn.zip'); `+
				`zipfile.ZipFile(d+'/fn.zip').extractall(d); `+
				`sys.path.insert(0,d); mod,fn=('%s').rsplit('.',1); `+
				`m=importlib.import_module(mod); `+
				`r=m.__dict__[fn](%s); print(json.dumps(r))"`,
			codeURL, handler, jsonOrNull(event),
		)
	case "node20":
		return fmt.Sprintf(
			`node -e "const https=require('https'),fs=require('fs'),path=require('path'),`+
				`{execSync}=require('child_process'); `+
				`const d='/tmp/tinyaws-lambda'; fs.mkdirSync(d,{recursive:true}); `+
				`execSync('curl -s -o '+d+'/fn.zip %s'); `+
				`execSync('cd '+d+' && unzip -o fn.zip'); `+
				`const [m,f]='%s'.split('.'); `+
				`const mod=require(path.join(d,m)); `+
				`Promise.resolve(mod[f](%s)).then(r=>console.log(JSON.stringify(r)))"`,
			codeURL, handler, jsonOrNull(event),
		)
	default:
		return fmt.Sprintf("echo 'unsupported runtime: %s'", runtime)
	}
}

// jsonOrNull returns the event JSON or "null".
func jsonOrNull(event string) string {
	if event == "" {
		return "null"
	}
	return "'" + event + "'"
}

// pollJobResult waits up to 60s for job completion.
func pollJobResult(schedulerURL, jobID string) InvokeResult {
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", schedulerURL, jobID))
		if err != nil {
			continue
		}
		var job struct {
			Status string `json:"status"`
			Stdout string `json:"stdout"`
			Stderr string `json:"stderr"`
			Exit   *int   `json:"exit_code"`
		}
		json.NewDecoder(resp.Body).Decode(&job)
		resp.Body.Close()

		if job.Status == "done" {
			return InvokeResult{StatusCode: 200, Output: job.Stdout}
		}
		if job.Status == "failed" {
			return InvokeResult{StatusCode: 500, Output: job.Stdout, Error: job.Stderr}
		}
	}
	return InvokeResult{StatusCode: 504, Error: "invocation timed out"}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
