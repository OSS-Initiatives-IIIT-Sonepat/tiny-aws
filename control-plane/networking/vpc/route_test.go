package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteCreate_MissingFields(t *testing.T) {
	setupVPCTest(t)

	cases := []struct {
		name    string
		payload map[string]string
	}{
		{"missing all", map[string]string{}},
		{"missing destination", map[string]string{"subnet_id": "s-1", "target": "igw-1"}},
		{"missing target", map[string]string{"subnet_id": "s-1", "destination": "0.0.0.0/0"}},
		{"missing subnet_id", map[string]string{"destination": "0.0.0.0/0", "target": "igw-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/route-tables", bytes.NewReader(body))
			w := httptest.NewRecorder()
			handleRouteCreate(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestRouteCreateAndList(t *testing.T) {
	setupVPCTest(t)

	body, _ := json.Marshal(map[string]string{
		"subnet_id": "s-1", "destination": "0.0.0.0/0", "target": "igw-1",
	})
	req := httptest.NewRequest("POST", "/route-tables", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleRouteCreate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d", w.Code)
	}

	var created RouteTable
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.SubnetID != "s-1" || created.Dest != "0.0.0.0/0" || created.Target != "igw-1" {
		t.Errorf("created = %+v", created)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	// list all
	req = httptest.NewRequest("GET", "/route-tables", nil)
	w = httptest.NewRecorder()
	handleRouteList(w, req)
	var routes []RouteTable
	json.Unmarshal(w.Body.Bytes(), &routes)
	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	}
}

func TestRouteList_FilterBySubnet(t *testing.T) {
	setupVPCTest(t)

	db.Exec(`INSERT INTO route_tables VALUES ('rt-1','s-1','0.0.0.0/0','igw-1','2026-01-01')`)
	db.Exec(`INSERT INTO route_tables VALUES ('rt-2','s-1','10.0.0.0/8','local','2026-01-01')`)
	db.Exec(`INSERT INTO route_tables VALUES ('rt-3','s-2','0.0.0.0/0','igw-2','2026-01-01')`)

	// filter by s-1
	req := httptest.NewRequest("GET", "/route-tables?subnet_id=s-1", nil)
	w := httptest.NewRecorder()
	handleRouteList(w, req)
	var routes []RouteTable
	json.Unmarshal(w.Body.Bytes(), &routes)
	if len(routes) != 2 {
		t.Errorf("expected 2 routes for s-1, got %d", len(routes))
	}

	// filter by s-2
	req = httptest.NewRequest("GET", "/route-tables?subnet_id=s-2", nil)
	w = httptest.NewRecorder()
	handleRouteList(w, req)
	json.Unmarshal(w.Body.Bytes(), &routes)
	if len(routes) != 1 {
		t.Errorf("expected 1 route for s-2, got %d", len(routes))
	}

	// no filter — all 3
	req = httptest.NewRequest("GET", "/route-tables", nil)
	w = httptest.NewRecorder()
	handleRouteList(w, req)
	json.Unmarshal(w.Body.Bytes(), &routes)
	if len(routes) != 3 {
		t.Errorf("expected 3 routes total, got %d", len(routes))
	}
}
