package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestSGRuleDefaultProtocol(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO security_groups VALUES ('sg-1','web','vpc-1','2026-01-01')`)

	body, _ := json.Marshal(map[string]any{
		"direction": "inbound", "action": "allow", "port": 443,
	})
	req := httptest.NewRequest("POST", "/security-groups/sg-1/rules", bytes.NewReader(body))
	req.SetPathValue("id", "sg-1")
	w := httptest.NewRecorder()
	handleSGRuleAdd(w, req)

	var rule SGRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.Protocol != "*" {
		t.Errorf("protocol = %q, want '*'", rule.Protocol)
	}
}

func TestSGRuleDefaultCIDR(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO security_groups VALUES ('sg-1','web','vpc-1','2026-01-01')`)

	body, _ := json.Marshal(map[string]any{
		"direction": "outbound", "action": "deny", "protocol": "tcp",
	})
	req := httptest.NewRequest("POST", "/security-groups/sg-1/rules", bytes.NewReader(body))
	req.SetPathValue("id", "sg-1")
	w := httptest.NewRecorder()
	handleSGRuleAdd(w, req)

	var rule SGRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.CIDR != "0.0.0.0/0" {
		t.Errorf("cidr = %q, want '0.0.0.0/0'", rule.CIDR)
	}
}

func TestSGRuleBothDefaults(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO security_groups VALUES ('sg-1','web','vpc-1','2026-01-01')`)

	body, _ := json.Marshal(map[string]any{
		"direction": "inbound", "action": "allow",
	})
	req := httptest.NewRequest("POST", "/security-groups/sg-1/rules", bytes.NewReader(body))
	req.SetPathValue("id", "sg-1")
	w := httptest.NewRecorder()
	handleSGRuleAdd(w, req)

	var rule SGRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.Protocol != "*" {
		t.Errorf("protocol = %q, want '*'", rule.Protocol)
	}
	if rule.CIDR != "0.0.0.0/0" {
		t.Errorf("cidr = %q, want '0.0.0.0/0'", rule.CIDR)
	}
}

func TestSGRuleExplicitValues(t *testing.T) {
	setupVPCTest(t)
	db.Exec(`INSERT INTO security_groups VALUES ('sg-1','web','vpc-1','2026-01-01')`)

	body, _ := json.Marshal(map[string]any{
		"direction": "inbound", "action": "allow",
		"protocol": "tcp", "port": 22, "cidr": "10.0.0.0/8",
	})
	req := httptest.NewRequest("POST", "/security-groups/sg-1/rules", bytes.NewReader(body))
	req.SetPathValue("id", "sg-1")
	w := httptest.NewRecorder()
	handleSGRuleAdd(w, req)

	var rule SGRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.Protocol != "tcp" {
		t.Errorf("protocol = %q, want 'tcp'", rule.Protocol)
	}
	if rule.CIDR != "10.0.0.0/8" {
		t.Errorf("cidr = %q, want '10.0.0.0/8'", rule.CIDR)
	}
}
