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

// Trigger wires an event source (S3 PUT, SQS message) to a lambda function.
type Trigger struct {
	ID           string `json:"id"`
	FunctionName string `json:"function_name"`
	EventSource  string `json:"event_source"`  // "s3:ObjectCreated" or "sqs"
	SourceName   string `json:"source_name"`   // bucket name or queue name
	CreatedAt    string `json:"created_at"`
}

// InvokeResult is returned from a sync invocation.
type InvokeResult struct {
	StatusCode int    `json:"status_code"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

var (
	db       *sql.DB
	trigSeq  uint64
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
		CREATE TABLE IF NOT EXISTS triggers (
			id            TEXT PRIMARY KEY,
			function_name TEXT NOT NULL,
			event_source  TEXT NOT NULL,
			source_name   TEXT NOT NULL,
			created_at    TEXT NOT NULL
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
	http.HandleFunc("POST /triggers", handleCreateTrigger)
	http.HandleFunc("GET /triggers", handleListTriggers)
	http.HandleFunc("DELETE /triggers/{id}", handleDeleteTrigger)
	http.HandleFunc("POST /trigger-notify", handleTriggerNotify)

	// subscribe to SNS s3:ObjectCreated topic so object store PUTs fire triggers
	go subscribeToS3Events(listenAddr)

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

	// G7: job command encodes runtime, handler, event via actual env vars on the job
	// Pass handler, code URL, and event as env_vars on the scheduler job —
	// no shell interpolation, no injection risk.
	envVars := map[string]string{
		"TINYAWS_HANDLER":  fn.Handler,
		"TINYAWS_CODE_URL": codeURL,
		"TINYAWS_EVENT":    string(event),
	}
	cmd := buildInvokeCommand(fn.Runtime)

	payload, _ := json.Marshal(map[string]any{"command": cmd, "env_vars": envVars})
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

// buildInvokeCommand returns a fixed shell template that reads handler/code/event
// from env vars — no interpolation, no injection risk.
func buildInvokeCommand(runtime string) string {
	switch runtime {
	case "python3":
		return `python3 -c "` +
			`import urllib.request,zipfile,os,json,importlib,sys; ` +
			`d='/tmp/tinyaws-lambda'; os.makedirs(d,exist_ok=True); ` +
			`urllib.request.urlretrieve(os.environ['TINYAWS_CODE_URL'],d+'/fn.zip'); ` +
			`zipfile.ZipFile(d+'/fn.zip').extractall(d); ` +
			`sys.path.insert(0,d); mod,fn=os.environ['TINYAWS_HANDLER'].rsplit('.',1); ` +
			`m=importlib.import_module(mod); ` +
			`ev=json.loads(os.environ.get('TINYAWS_EVENT','null')); ` +
			`r=m.__dict__[fn](ev); print(json.dumps(r))"`
	case "node20":
		return `node -e "` +
			`const fs=require('fs'),path=require('path'),{execSync}=require('child_process'); ` +
			`const d='/tmp/tinyaws-lambda'; fs.mkdirSync(d,{recursive:true}); ` +
			`execSync('curl -s -o '+d+'/fn.zip '+process.env.TINYAWS_CODE_URL); ` +
			`execSync('cd '+d+' && unzip -o fn.zip'); ` +
			`const [m,f]=process.env.TINYAWS_HANDLER.split('.'); ` +
			`const mod=require(path.join(d,m)); ` +
			`const ev=JSON.parse(process.env.TINYAWS_EVENT||'null'); ` +
			`Promise.resolve(mod[f](ev)).then(r=>console.log(JSON.stringify(r)))"`
	default:
		return `echo unsupported-runtime`
	}
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

// POST /triggers — create a trigger wiring an event source to a function.
func handleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	var trig Trigger
	if err := json.NewDecoder(r.Body).Decode(&trig); err != nil || trig.FunctionName == "" || trig.EventSource == "" {
		http.Error(w, "function_name and event_source required", http.StatusBadRequest)
		return
	}
	seq := atomic.AddUint64(&trigSeq, 1)
	trig.ID = fmt.Sprintf("trig-%d", seq)
	trig.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO triggers (id, function_name, event_source, source_name, created_at) VALUES (?,?,?,?,?)`,
		trig.ID, trig.FunctionName, trig.EventSource, trig.SourceName, trig.CreatedAt,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /triggers - %s -> %s on %s:%s", trig.ID, trig.FunctionName, trig.EventSource, trig.SourceName)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(trig)
}

// GET /triggers — list all triggers.
func handleListTriggers(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT id, function_name, event_source, source_name, created_at FROM triggers`)
	defer rows.Close()
	out := []Trigger{}
	for rows.Next() {
		var t Trigger
		rows.Scan(&t.ID, &t.FunctionName, &t.EventSource, &t.SourceName, &t.CreatedAt)
		out = append(out, t)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// DELETE /triggers/{id} — remove a trigger.
func handleDeleteTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	db.Exec(`DELETE FROM triggers WHERE id = ?`, id)
	log.Printf("DELETE /triggers/%s", id)
	w.WriteHeader(http.StatusNoContent)
}

// POST /trigger-notify — called by SNS when an s3:ObjectCreated event fires.
// Looks up matching triggers and invokes functions asynchronously.
func handleTriggerNotify(w http.ResponseWriter, r *http.Request) {
	var notification struct {
		Topic   string `json:"topic"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// parse the inner message (JSON from object store)
	var event struct {
		Event  string `json:"event"`
		Bucket string `json:"bucket"`
		Key    string `json:"key"`
		Size   int    `json:"size"`
	}
	if err := json.Unmarshal([]byte(notification.Message), &event); err != nil {
		log.Printf("trigger-notify: bad message: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// find matching triggers
	rows, err := db.Query(
		`SELECT function_name FROM triggers WHERE event_source = 's3:ObjectCreated' AND (source_name = ? OR source_name = '')`,
		event.Bucket,
	)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	defer rows.Close()

	eventJSON, _ := json.Marshal(event)
	for rows.Next() {
		var fnName string
		rows.Scan(&fnName)
		log.Printf("trigger: s3:ObjectCreated on %s/%s -> invoking %s", event.Bucket, event.Key, fnName)
		go triggerInvoke(fnName, string(eventJSON))
	}
	w.WriteHeader(http.StatusOK)
}

// triggerInvoke invokes a function by name with the given event payload.
func triggerInvoke(fnName, eventPayload string) {
	lambdaURL := fmt.Sprintf("http://127.0.0.1%s", getenv("LAMBDA_ADDR", ":9007"))
	resp, err := http.Post(
		fmt.Sprintf("%s/functions/%s/invoke", lambdaURL, fnName),
		"application/json",
		bytes.NewReader([]byte(eventPayload)),
	)
	if err != nil {
		log.Printf("trigger invoke %s failed: %v", fnName, err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("trigger invoke %s result: %s", fnName, string(body))
}

// subscribeToS3Events registers this lambda service as an SNS subscriber
// for the s3:ObjectCreated topic so the object store PUTs trigger functions.
func subscribeToS3Events(listenAddr string) {
	snsURL := getenv("SNS_URL", "")
	if snsURL == "" {
		log.Println("SNS_URL not set — lambda triggers disabled")
		return
	}

	// wait for SNS to be ready
	time.Sleep(2 * time.Second)

	// create the topic (idempotent)
	http.Post(snsURL+"/topics/s3:ObjectCreated", "application/json", nil)

	// subscribe our /trigger-notify endpoint
	// figure out our own address for SNS to call back
	lambdaAddr := getenv("LAMBDA_ADVERTISE_ADDR", "")
	if lambdaAddr == "" {
		lambdaAddr = fmt.Sprintf("http://127.0.0.1%s", listenAddr)
	}
	endpoint := lambdaAddr + "/trigger-notify"

	payload, _ := json.Marshal(map[string]string{"endpoint": endpoint})
	resp, err := http.Post(
		snsURL+"/topics/s3:ObjectCreated/subscribe",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Printf("lambda: failed to subscribe to s3:ObjectCreated: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("lambda: subscribed to s3:ObjectCreated topic at %s", endpoint)
}
