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

func setupVPCTest(t *testing.T) {
	t.Helper()
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vpcs (id TEXT PRIMARY KEY, name TEXT, cidr TEXT, created_at TEXT);
		CREATE TABLE IF NOT EXISTS subnets (id TEXT PRIMARY KEY, vpc_id TEXT, name TEXT, cidr TEXT, created_at TEXT);
		CREATE TABLE IF NOT EXISTS route_tables (id TEXT PRIMARY KEY, subnet_id TEXT, destination TEXT, target TEXT, created_at TEXT);
		CREATE TABLE IF NOT EXISTS security_groups (id TEXT PRIMARY KEY, name TEXT, vpc_id TEXT, created_at TEXT);
		CREATE TABLE IF NOT EXISTS sg_rules (id TEXT PRIMARY KEY, sg_id TEXT, direction TEXT, action TEXT, protocol TEXT, port INTEGER, cidr TEXT);
		CREATE TABLE IF NOT EXISTS instance_subnets (instance_id TEXT PRIMARY KEY, subnet_id TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	seq = 0
	t.Cleanup(func() { db.Close() })
}

func TestVPCHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestVPCCreate(t *testing.T) {
	setupVPCTest(t)
	body, _ := json.Marshal(map[string]string{"name": "main-vpc", "cidr": "10.0.0.0/16"})
	req := httptest.NewRequest("POST", "/vpcs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleVPCCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}
	var vpc VPC
	json.Unmarshal(w.Body.Bytes(), &vpc)
	if vpc.Name != "main-vpc" || vpc.CIDR != "10.0.0.0/16" {
		t.Errorf("vpc = %+v", vpc)
	}
}

func TestVPCCreate_MissingFields(t *testing.T) {
	setupVPCTest(t)
	body, _ := json.Marshal(map[string]string{"name": "x"})
	req := httptest.NewRequest("POST", "/vpcs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleVPCCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestVPCList(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO vpcs VALUES ('vpc-1','test','10.0.0.0/16','2026-01-01')`)

	req := httptest.NewRequest("GET", "/vpcs", nil)
	w := httptest.NewRecorder()
	handleVPCList(w, req)

	var vpcs []VPC
	json.Unmarshal(w.Body.Bytes(), &vpcs)
	if len(vpcs) != 1 {
		t.Errorf("expected 1, got %d", len(vpcs))
	}
}

func TestSubnetCreate(t *testing.T) {
	setupVPCTest(t)
	body, _ := json.Marshal(map[string]string{"vpc_id": "vpc-1", "cidr": "10.0.1.0/24", "name": "sub-a"})
	req := httptest.NewRequest("POST", "/subnets", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleSubnetCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSubnetList(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO subnets VALUES ('s-1','vpc-1','sub-a','10.0.1.0/24','2026-01-01')`)
	db.Exec(`INSERT INTO subnets VALUES ('s-2','vpc-2','sub-b','10.0.2.0/24','2026-01-01')`)

	// list all
	req := httptest.NewRequest("GET", "/subnets", nil)
	w := httptest.NewRecorder()
	handleSubnetList(w, req)
	var all []Subnet
	json.Unmarshal(w.Body.Bytes(), &all)
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}

	// filter by vpc_id
	req = httptest.NewRequest("GET", "/subnets?vpc_id=vpc-1", nil)
	w = httptest.NewRecorder()
	handleSubnetList(w, req)
	var filtered []Subnet
	json.Unmarshal(w.Body.Bytes(), &filtered)
	if len(filtered) != 1 {
		t.Errorf("expected 1 for vpc-1, got %d", len(filtered))
	}
}

func TestSGCreateAndList(t *testing.T) {
	setupVPCTest(t)
	body, _ := json.Marshal(map[string]string{"name": "web-sg", "vpc_id": "vpc-1"})
	req := httptest.NewRequest("POST", "/security-groups", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleSGCreate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/security-groups", nil)
	w = httptest.NewRecorder()
	handleSGList(w, req)
	var sgs []SecurityGroup
	json.Unmarshal(w.Body.Bytes(), &sgs)
	if len(sgs) != 1 {
		t.Errorf("expected 1, got %d", len(sgs))
	}
}

func TestSGRuleAddAndList(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO security_groups VALUES ('sg-1','web','vpc-1','2026-01-01')`)

	body, _ := json.Marshal(map[string]any{
		"direction": "inbound", "action": "allow",
		"protocol": "tcp", "port": 80, "cidr": "0.0.0.0/0",
	})
	req := httptest.NewRequest("POST", "/security-groups/sg-1/rules", bytes.NewReader(body))
	req.SetPathValue("id", "sg-1")
	w := httptest.NewRecorder()
	handleSGRuleAdd(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("rule add status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/security-groups/sg-1/rules", nil)
	req.SetPathValue("id", "sg-1")
	w = httptest.NewRecorder()
	handleSGRuleList(w, req)
	var rules []SGRule
	json.Unmarshal(w.Body.Bytes(), &rules)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Port != 80 || rules[0].Action != "allow" {
		t.Errorf("rule = %+v", rules[0])
	}
}

func TestInstanceSubnet(t *testing.T) {
	setupVPCTest(t)

	body, _ := json.Marshal(map[string]string{"subnet_id": "s-1"})
	req := httptest.NewRequest("PUT", "/instances/i-1/subnet", bytes.NewReader(body))
	req.SetPathValue("id", "i-1")
	w := httptest.NewRecorder()
	handleInstanceSubnet(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("assign status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/instances/i-1/subnet", nil)
	req.SetPathValue("id", "i-1")
	w = httptest.NewRecorder()
	handleInstanceSubnetGet(w, req)
	var got map[string]string
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["subnet_id"] != "s-1" {
		t.Errorf("subnet_id = %q", got["subnet_id"])
	}
}

func TestNextID(t *testing.T) {
	seq = 0
	id1 := nextID("vpc")
	id2 := nextID("vpc")
	if id1 != "vpc-1" || id2 != "vpc-2" {
		t.Errorf("ids = %q, %q", id1, id2)
	}
}
