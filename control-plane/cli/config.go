package main

import "os"

// Reads env var or returns fallback default.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// apiBase returns the API gateway base when TINYAWS_API_URL is set, else "".
func apiBase() string {
	return os.Getenv("TINYAWS_API_URL")
}

// Registry base URL. Uses API gateway /v1 prefix when TINYAWS_API_URL is set.
func registryURL() string {
	if b := apiBase(); b != "" {
		return b + "/v1"
	}
	return getenv("REGISTRY_URL", "http://127.0.0.1:9000")
}

// Scheduler base URL. Uses API gateway /v1 prefix when TINYAWS_API_URL is set.
func schedulerURL() string {
	if b := apiBase(); b != "" {
		return b + "/v1"
	}
	return getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
}

// Object store base URL. Uses API gateway /v1 prefix when TINYAWS_API_URL is set.
func objectStoreURL() string {
	if b := apiBase(); b != "" {
		return b + "/v1"
	}
	return getenv("OBJECT_STORE_URL", "http://127.0.0.1:7001")
}
