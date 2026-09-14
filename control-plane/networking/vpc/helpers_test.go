package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_StatusAndContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 201, map[string]string{"id": "vpc-1"})

	if w.Code != 201 {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestWriteJSON_Body(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 200, VPC{ID: "vpc-1", Name: "main", CIDR: "10.0.0.0/16"})

	var got VPC
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "vpc-1" || got.Name != "main" || got.CIDR != "10.0.0.0/16" {
		t.Errorf("got %+v", got)
	}
}

func TestWriteJSON_Slice(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 200, []string{"a", "b"})

	var got []string
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestGetenv_Default(t *testing.T) {
	t.Setenv("NETWORKING_DB", "")
	if got := getenv("NETWORKING_DB", "fallback.db"); got != "fallback.db" {
		t.Errorf("got %q", got)
	}
}

func TestGetenv_Override(t *testing.T) {
	t.Setenv("NETWORKING_DB", "custom.db")
	if got := getenv("NETWORKING_DB", "fallback.db"); got != "custom.db" {
		t.Errorf("got %q", got)
	}
}

func TestNextID_Prefix(t *testing.T) {
	id := nextID("vpc")
	if len(id) < 5 || id[:4] != "vpc-" {
		t.Errorf("nextID = %q, want vpc-N", id)
	}
}
