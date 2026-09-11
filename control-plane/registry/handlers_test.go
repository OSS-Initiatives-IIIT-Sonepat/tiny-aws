package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleInstanceGet_Found(t *testing.T) {
	setupHandlerTest(t)
	inst := instanceStore.Create("node-1", "small")
	instances = append(instances, inst)

	req := httptest.NewRequest("GET", "/instances/i-1", nil)
	req.SetPathValue("id", "i-1")
	w := httptest.NewRecorder()
	handleInstanceGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got Instance
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.InstanceType != "small" {
		t.Errorf("type = %q", got.InstanceType)
	}
}

func TestHandleInstanceGet_NotFound(t *testing.T) {
	setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/instances/i-999", nil)
	req.SetPathValue("id", "i-999")
	w := httptest.NewRecorder()
	handleInstanceGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestListInstances_FilterByStatus(t *testing.T) {
	setupHandlerTest(t)
	inst1 := instanceStore.Create("node-1", "small")
	inst2 := instanceStore.Create("node-1", "small")
	instanceStore.SetStatus(inst2.ID, "terminated")
	inst2.Status = "terminated"
	instances = append(instances, inst1, inst2)

	req := httptest.NewRequest("GET", "/instances?status=provisioning", nil)
	w := httptest.NewRecorder()
	listInstances(w, req)

	var got []Instance
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 1 {
		t.Errorf("expected 1 provisioning, got %d", len(got))
	}
}

func TestHandleInstancePatch(t *testing.T) {
	setupHandlerTest(t)
	inst := instanceStore.Create("node-1", "small")
	instances = append(instances, inst)

	body, _ := json.Marshal(map[string]string{"status": "running"})
	req := httptest.NewRequest("PATCH", "/instances/"+inst.ID, bytes.NewReader(body))
	req.SetPathValue("id", inst.ID)
	w := httptest.NewRecorder()
	handleInstancePatch(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if instances[0].Status != "running" {
		t.Errorf("in-memory status = %q", instances[0].Status)
	}
}

func TestHandleServiceCreate(t *testing.T) {
	setupHandlerTest(t)

	body, _ := json.Marshal(map[string]any{
		"node_id": "node-1", "instance_id": "i-1",
		"port": 3000, "pid": 1234, "deploy_url": "http://store/app.zip",
	})
	req := httptest.NewRequest("POST", "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleServiceCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var svc Service
	json.Unmarshal(w.Body.Bytes(), &svc)
	if svc.Port != 3000 || svc.PID != 1234 {
		t.Errorf("svc = %+v", svc)
	}
}

func TestHandleServiceList_FilterByStatus(t *testing.T) {
	setupHandlerTest(t)
	serviceStore.Create("n1", "i-1", "", 3000, 100)
	svc2 := serviceStore.Create("n1", "i-2", "", 3001, 200)
	serviceStore.UpdateStatus(svc2.ID, "stopped")

	req := httptest.NewRequest("GET", "/services?status=running", nil)
	w := httptest.NewRecorder()
	handleServiceList(w, req)

	var svcs []Service
	json.Unmarshal(w.Body.Bytes(), &svcs)
	if len(svcs) != 1 {
		t.Errorf("expected 1 running, got %d", len(svcs))
	}
}

func TestHandleServiceDelete(t *testing.T) {
	setupHandlerTest(t)
	svc := serviceStore.Create("n1", "i-1", "", 3000, 100)

	req := httptest.NewRequest("DELETE", "/services/"+svc.ID, nil)
	req.SetPathValue("id", svc.ID)
	w := httptest.NewRecorder()
	handleServiceDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	// verify status
	svcs, _ := serviceStore.LoadAll()
	if svcs[0].Status != "stopped" {
		t.Errorf("status = %q, want stopped", svcs[0].Status)
	}
}
