package main

import "os"

// Reads env var or returns fallback default.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Registry base URL.
func registryURL() string {
	return getenv("REGISTRY_URL", "http://127.0.0.1:9000")
}

// Scheduler base URL.
func schedulerURL() string {
	return getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
}

// Object store base URL.
func objectStoreURL() string {
	return getenv("OBJECT_STORE_URL", "http://127.0.0.1:7001")
}
