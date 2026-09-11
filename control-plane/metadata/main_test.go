package main

import (
	"testing"
)

func TestMetadataGetenv_Default(t *testing.T) {
	t.Setenv("TEST_KEY", "")
	if got := getenv("TEST_KEY", "fallback"); got != "fallback" {
		t.Errorf("got %q", got)
	}
}

func TestMetadataGetenv_Set(t *testing.T) {
	t.Setenv("TEST_KEY", "val")
	if got := getenv("TEST_KEY", "fallback"); got != "val" {
		t.Errorf("got %q", got)
	}
}

func TestFetch_Unreachable(t *testing.T) {
	_, err := fetch("http://127.0.0.1:1/nope")
	if err == nil {
		t.Error("expected error for unreachable URL")
	}
}
