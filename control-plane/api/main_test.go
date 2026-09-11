package main

import (
	"testing"
)

func TestGetenv_Default(t *testing.T) {
	t.Setenv("TEST_KEY", "")
	if got := getenv("TEST_KEY", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestGetenv_Set(t *testing.T) {
	t.Setenv("TEST_KEY", "value")
	if got := getenv("TEST_KEY", "fallback"); got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

func TestProxy_StripPrefix(t *testing.T) {
	// proxy() strips /v1 — verify the handler creation doesn't panic
	h := proxy("http://127.0.0.1:9000")
	if h == nil {
		t.Fatal("proxy returned nil handler")
	}
}
