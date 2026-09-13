package main

import "testing"

func TestLbURL_Default(t *testing.T) {
	t.Setenv("LB_URL", "")
	if got := lbURL(); got != "http://127.0.0.1:8088" {
		t.Errorf("default = %q", got)
	}
}

func TestLbURL_Custom(t *testing.T) {
	t.Setenv("LB_URL", "http://lb:8088")
	if got := lbURL(); got != "http://lb:8088" {
		t.Errorf("custom = %q", got)
	}
}
