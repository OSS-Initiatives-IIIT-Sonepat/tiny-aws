package main

import (
	"testing"
)

func TestGetenv(t *testing.T) {
	t.Setenv("TEST_CLI_KEY", "")
	if got := getenv("TEST_CLI_KEY", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}

	t.Setenv("TEST_CLI_KEY", "custom")
	if got := getenv("TEST_CLI_KEY", "default"); got != "custom" {
		t.Errorf("got %q, want %q", got, "custom")
	}
}

func TestAPIBase(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	if got := apiBase(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	t.Setenv("TINYAWS_API_URL", "http://gw:8000")
	if got := apiBase(); got != "http://gw:8000" {
		t.Errorf("got %q", got)
	}
}

func TestRegistryURL_Direct(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", "")
	if got := registryURL(); got != "http://127.0.0.1:9000" {
		t.Errorf("default = %q", got)
	}
}

func TestRegistryURL_Custom(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", "http://reg:9000")
	if got := registryURL(); got != "http://reg:9000" {
		t.Errorf("got %q", got)
	}
}

func TestRegistryURL_Gateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "http://gw:8000")
	if got := registryURL(); got != "http://gw:8000/v1" {
		t.Errorf("via gateway = %q", got)
	}
}

func TestSchedulerURL_Direct(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("SCHEDULER_URL", "")
	if got := schedulerURL(); got != "http://127.0.0.1:9001" {
		t.Errorf("default = %q", got)
	}
}

func TestSchedulerURL_Gateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "http://gw:8000")
	if got := schedulerURL(); got != "http://gw:8000/v1" {
		t.Errorf("via gateway = %q", got)
	}
}

func TestObjectStoreURL_Direct(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("OBJECT_STORE_URL", "")
	if got := objectStoreURL(); got != "http://127.0.0.1:7001" {
		t.Errorf("default = %q", got)
	}
}

func TestObjectStoreURL_Gateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "http://gw:8000")
	if got := objectStoreURL(); got != "http://gw:8000/v1" {
		t.Errorf("via gateway = %q", got)
	}
}

func TestNetworkingURL(t *testing.T) {
	t.Setenv("NETWORKING_URL", "")
	if got := networkingURL(); got != "http://127.0.0.1:9005" {
		t.Errorf("default = %q", got)
	}
	t.Setenv("NETWORKING_URL", "http://net:9005")
	if got := networkingURL(); got != "http://net:9005" {
		t.Errorf("custom = %q", got)
	}
}
