package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func setupLambdaTest(t *testing.T) {
	t.Helper()
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS functions (
			name TEXT PRIMARY KEY, runtime TEXT NOT NULL,
			handler TEXT NOT NULL, bucket TEXT NOT NULL,
			key TEXT NOT NULL, created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
}

func TestLambdaHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestLambdaCreateFunction(t *testing.T) {
	setupLambdaTest(t)

	fn := Function{Name: "hello", Runtime: "python3", Handler: "app.handler", Bucket: "funcs", Key: "hello.zip"}
	body, _ := json.Marshal(fn)
	req := httptest.NewRequest("POST", "/functions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var got Function
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Name != "hello" || got.Runtime != "python3" {
		t.Errorf("got = %+v", got)
	}
}

func TestLambdaCreateFunction_DefaultHandler(t *testing.T) {
	setupLambdaTest(t)

	body, _ := json.Marshal(map[string]string{"name": "fn1", "runtime": "node20", "bucket": "b", "key": "k"})
	req := httptest.NewRequest("POST", "/functions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleCreate(w, req)

	var got Function
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Handler != "handler.handler" {
		t.Errorf("default handler = %q", got.Handler)
	}
}

func TestLambdaCreateFunction_MissingFields(t *testing.T) {
	setupLambdaTest(t)

	body, _ := json.Marshal(map[string]string{"name": "x"})
	req := httptest.NewRequest("POST", "/functions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestLambdaListFunctions(t *testing.T) {
	setupLambdaTest(t)
	db.Exec(`INSERT INTO functions VALUES ('fn1','python3','h','b','k','2026-01-01')`)
	db.Exec(`INSERT INTO functions VALUES ('fn2','node20','h','b','k','2026-01-01')`)

	req := httptest.NewRequest("GET", "/functions", nil)
	w := httptest.NewRecorder()
	handleList(w, req)

	var fns []Function
	json.Unmarshal(w.Body.Bytes(), &fns)
	if len(fns) != 2 {
		t.Errorf("expected 2, got %d", len(fns))
	}
}

func TestLambdaGetFunction(t *testing.T) {
	setupLambdaTest(t)
	db.Exec(`INSERT INTO functions VALUES ('fn1','python3','h.h','b','k','2026-01-01')`)

	req := httptest.NewRequest("GET", "/functions/fn1", nil)
	req.SetPathValue("name", "fn1")
	w := httptest.NewRecorder()
	handleGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var fn Function
	json.Unmarshal(w.Body.Bytes(), &fn)
	if fn.Name != "fn1" {
		t.Errorf("name = %q", fn.Name)
	}
}

func TestLambdaGetFunction_NotFound(t *testing.T) {
	setupLambdaTest(t)

	req := httptest.NewRequest("GET", "/functions/missing", nil)
	req.SetPathValue("name", "missing")
	w := httptest.NewRecorder()
	handleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestBuildInvokeCommand_Python(t *testing.T) {
	cmd := buildInvokeCommand("python3")
	if cmd == "" {
		t.Fatal("expected non-empty command")
	}
	if len(cmd) < 50 {
		t.Errorf("python command too short: %q", cmd)
	}
}

func TestBuildInvokeCommand_Node(t *testing.T) {
	cmd := buildInvokeCommand("node20")
	if cmd == "" {
		t.Fatal("expected non-empty command")
	}
}

func TestBuildInvokeCommand_Unknown(t *testing.T) {
	cmd := buildInvokeCommand("ruby3")
	if cmd != "echo unsupported-runtime" {
		t.Errorf("unknown runtime cmd = %q", cmd)
	}
}
