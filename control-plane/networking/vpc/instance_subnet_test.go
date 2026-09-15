package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test instance subnet assignment and retrieval, including not-found.
func TestInstanceSubnet_AssignAndRetrieve(t *testing.T) {
	setupVPCTest(t)

	// assign subnet
	body, _ := json.Marshal(map[string]string{"subnet_id": "subnet-10"})
	req := httptest.NewRequest("PUT", "/instances/i-42/subnet", bytes.NewReader(body))
	req.SetPathValue("id", "i-42")
	w := httptest.NewRecorder()
	handleInstanceSubnet(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("assign status = %d, want 204", w.Code)
	}

	// retrieve
	getReq := httptest.NewRequest("GET", "/instances/i-42/subnet", nil)
	getReq.SetPathValue("id", "i-42")
	gw := httptest.NewRecorder()
	handleInstanceSubnetGet(gw, getReq)

	if gw.Code != http.StatusOK {
		t.Fatalf("get status = %d", gw.Code)
	}
	var got map[string]string
	json.Unmarshal(gw.Body.Bytes(), &got)
	if got["instance_id"] != "i-42" {
		t.Errorf("instance_id = %q", got["instance_id"])
	}
	if got["subnet_id"] != "subnet-10" {
		t.Errorf("subnet_id = %q", got["subnet_id"])
	}
}

func TestInstanceSubnet_NotFound(t *testing.T) {
	setupVPCTest(t)

	req := httptest.NewRequest("GET", "/instances/i-999/subnet", nil)
	req.SetPathValue("id", "i-999")
	w := httptest.NewRecorder()
	handleInstanceSubnetGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestInstanceSubnet_Reassign(t *testing.T) {
	setupVPCTest(t)

	// first assignment
	body1, _ := json.Marshal(map[string]string{"subnet_id": "s-1"})
	req1 := httptest.NewRequest("PUT", "/instances/i-5/subnet", bytes.NewReader(body1))
	req1.SetPathValue("id", "i-5")
	w1 := httptest.NewRecorder()
	handleInstanceSubnet(w1, req1)

	// reassign to different subnet
	body2, _ := json.Marshal(map[string]string{"subnet_id": "s-2"})
	req2 := httptest.NewRequest("PUT", "/instances/i-5/subnet", bytes.NewReader(body2))
	req2.SetPathValue("id", "i-5")
	w2 := httptest.NewRecorder()
	handleInstanceSubnet(w2, req2)

	// verify latest
	getReq := httptest.NewRequest("GET", "/instances/i-5/subnet", nil)
	getReq.SetPathValue("id", "i-5")
	gw := httptest.NewRecorder()
	handleInstanceSubnetGet(gw, getReq)

	var got map[string]string
	json.Unmarshal(gw.Body.Bytes(), &got)
	if got["subnet_id"] != "s-2" {
		t.Errorf("subnet_id = %q, want s-2 after reassign", got["subnet_id"])
	}
}
