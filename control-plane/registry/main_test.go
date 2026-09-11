package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// setupHandlerTest initializes in-memory stores and global state for handler tests.
func setupHandlerTest(t *testing.T) {
	t.Helper()
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	store = s
	iamDB = s.DB()
	instanceStore = NewInstanceStore(s.DB())
	serviceStore = NewServiceStore(s.DB())
	nodes = make(map[string]Node)
	instances = nil
}

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("status = %q", body["status"])
	}
	if body["service"] != "registry" {
		t.Errorf("service = %q", body["service"])
	}
}

func TestHandleRegister(t *testing.T) {
	setupHandlerTest(t)

	node := Node{ID: "node-1", Hostname: "host-a", Addr: "10.0.0.1", CPUCount: 4, Role: "compute"}
	body, _ := json.Marshal(node)
	req := httptest.NewRequest("POST", "/nodes/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var got Node
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Status != "healthy" {
		t.Errorf("registered node status = %q, want %q", got.Status, "healthy")
	}

	// verify in-memory
	if _, ok := nodes["node-1"]; !ok {
		t.Error("node not found in global map")
	}
}

func TestHandleRegister_DefaultRole(t *testing.T) {
	setupHandlerTest(t)

	node := Node{ID: "node-2", Hostname: "host-b", CPUCount: 2}
	body, _ := json.Marshal(node)
	req := httptest.NewRequest("POST", "/nodes/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if nodes["node-2"].Role != "compute" {
		t.Errorf("default role = %q, want %q", nodes["node-2"].Role, "compute")
	}
}

func TestHandleHeartbeat(t *testing.T) {
	setupHandlerTest(t)

	// register node first
	nodes["node-1"] = Node{
		ID: "node-1", Hostname: "h", CPUCount: 1, Role: "compute",
		Status: "healthy", LastSeen: time.Now().Add(-1 * time.Minute),
	}
	store.Save(nodes["node-1"])

	hb := Heartbeat{ID: "node-1"}
	body, _ := json.Marshal(hb)
	req := httptest.NewRequest("POST", "/nodes/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleHeartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// LastSeen should be updated
	if time.Since(nodes["node-1"].LastSeen) > 2*time.Second {
		t.Error("LastSeen not updated")
	}
}

func TestHandleHeartbeat_UnknownNode(t *testing.T) {
	setupHandlerTest(t)

	hb := Heartbeat{ID: "unknown"}
	body, _ := json.Marshal(hb)
	req := httptest.NewRequest("POST", "/nodes/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleHeartbeat(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleNodes_ListAll(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{ID: "node-1", Hostname: "h1", CPUCount: 2, Role: "compute", Status: "healthy"}
	nodes["node-2"] = Node{ID: "node-2", Hostname: "h2", CPUCount: 4, Role: "storage", Status: "healthy"}

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	handleNodes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var got map[string]Node
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(got))
	}
}

func TestHandleNodes_FilterByRole(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{ID: "node-1", Hostname: "h1", CPUCount: 2, Role: "compute", Status: "healthy"}
	nodes["node-2"] = Node{ID: "node-2", Hostname: "h2", CPUCount: 4, Role: "storage", Status: "healthy"}

	req := httptest.NewRequest("GET", "/nodes?role=compute", nil)
	w := httptest.NewRecorder()
	handleNodes(w, req)

	var got map[string]Node
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 1 {
		t.Errorf("expected 1 compute node, got %d", len(got))
	}
	if got["node-1"].Role != "compute" {
		t.Errorf("filtered node role = %q", got["node-1"].Role)
	}
}

func TestHandleNodeByID_Delete(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{ID: "node-1", Hostname: "h", CPUCount: 1, Role: "compute", Status: "healthy"}
	store.Save(nodes["node-1"])

	req := httptest.NewRequest("DELETE", "/nodes/node-1", nil)
	req.SetPathValue("id", "node-1")
	w := httptest.NewRecorder()
	handleNodeByID(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if _, ok := nodes["node-1"]; ok {
		t.Error("node should be deleted from map")
	}
}

func TestHandleNodeByID_NotFound(t *testing.T) {
	setupHandlerTest(t)

	req := httptest.NewRequest("DELETE", "/nodes/missing", nil)
	req.SetPathValue("id", "missing")
	w := httptest.NewRecorder()
	handleNodeByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
