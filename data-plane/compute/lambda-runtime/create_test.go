package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test handleCreate upsert: create a function, update it, verify fields changed.
func TestHandleCreateUpsert(t *testing.T) {
	setupLambdaTest(t)

	// create
	fn := Function{Name: "greet", Runtime: "python3", Handler: "app.handler", Bucket: "b1", Key: "v1.zip"}
	body, _ := json.Marshal(fn)
	req := httptest.NewRequest("POST", "/functions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleCreate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", w.Code)
	}

	// update same name with different runtime and key
	fn2 := Function{Name: "greet", Runtime: "node20", Handler: "index.handler", Bucket: "b1", Key: "v2.zip"}
	body2, _ := json.Marshal(fn2)
	req2 := httptest.NewRequest("POST", "/functions", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	handleCreate(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("upsert status = %d, want 201", w2.Code)
	}

	// verify via GET
	getReq := httptest.NewRequest("GET", "/functions/greet", nil)
	getReq.SetPathValue("name", "greet")
	gw := httptest.NewRecorder()
	handleGet(gw, getReq)

	var got Function
	json.Unmarshal(gw.Body.Bytes(), &got)
	if got.Runtime != "node20" {
		t.Errorf("runtime = %q, want node20", got.Runtime)
	}
	if got.Handler != "index.handler" {
		t.Errorf("handler = %q, want index.handler", got.Handler)
	}
	if got.Key != "v2.zip" {
		t.Errorf("key = %q, want v2.zip", got.Key)
	}

	// verify only one function exists
	listReq := httptest.NewRequest("GET", "/functions", nil)
	lw := httptest.NewRecorder()
	handleList(lw, listReq)
	var fns []Function
	json.Unmarshal(lw.Body.Bytes(), &fns)
	if len(fns) != 1 {
		t.Errorf("expected 1 function after upsert, got %d", len(fns))
	}
}
