package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleServiceGet_Found(t *testing.T) {
	setupHandlerTest(t)
	svc := serviceStore.Create("n1", "i-1", "http://store/app.zip", 3000, 100)

	req := httptest.NewRequest("GET", "/services/"+svc.ID, nil)
	req.SetPathValue("id", svc.ID)
	w := httptest.NewRecorder()
	handleServiceGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got Service
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Port != 3000 {
		t.Errorf("port = %d", got.Port)
	}
}

func TestHandleServiceGet_NotFound(t *testing.T) {
	setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/services/svc-999", nil)
	req.SetPathValue("id", "svc-999")
	w := httptest.NewRecorder()
	handleServiceGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleServicePatch(t *testing.T) {
	setupHandlerTest(t)
	svc := serviceStore.Create("n1", "i-1", "", 3000, 100)

	body := `{"status":"crashed"}`
	req := httptest.NewRequest("PATCH", "/services/"+svc.ID, nil)
	req.SetPathValue("id", svc.ID)
	req.Body = io.NopCloser(strings.NewReader(body))
	w := httptest.NewRecorder()
	handleServicePatch(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	// verify via DB
	svcs, _ := serviceStore.LoadAll()
	if svcs[0].Status != "crashed" {
		t.Errorf("status = %q", svcs[0].Status)
	}
}

func TestHandleServicePatch_EmptyStatus(t *testing.T) {
	setupHandlerTest(t)
	svc := serviceStore.Create("n1", "i-1", "", 3000, 100)

	req := httptest.NewRequest("PATCH", "/services/"+svc.ID, nil)
	req.SetPathValue("id", svc.ID)
	req.Body = io.NopCloser(strings.NewReader(`{"status":""}`))
	w := httptest.NewRecorder()
	handleServicePatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
