package main

import "testing"

func TestGetenv_Fallback(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "")
	if got := getenv("TEST_GETENV_KEY", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}

func TestGetenv_EnvSet(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "custom")
	if got := getenv("TEST_GETENV_KEY", "default"); got != "custom" {
		t.Errorf("got %q, want %q", got, "custom")
	}
}

func TestGetenv_EmptyFallback(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "")
	if got := getenv("TEST_GETENV_KEY", ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
