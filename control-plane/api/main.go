package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// proxy returns a reverse-proxy handler that strips /v1 prefix and forwards to backend.
func proxy(backend string) http.Handler {
	target, _ := url.Parse(backend)
	rp := httputil.NewSingleHostReverseProxy(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// strip /v1 so /v1/nodes -> /nodes on backend
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/v1")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		r.URL.RawPath = ""
		rp.ServeHTTP(w, r)
	})
}

func main() {
	registryURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")
	schedulerURL := getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
	objectStoreURL := getenv("OBJECT_STORE_URL", "http://127.0.0.1:7001")
	listenAddr := getenv("API_GATEWAY_ADDR", ":8000")

	// /v1/health/* — forward health checks to the right backend
	http.Handle("/v1/health/registry", proxy(registryURL))
	http.Handle("/v1/health/scheduler", proxy(schedulerURL))
	http.Handle("/v1/health/store", proxy(objectStoreURL))

	// registry
	for _, p := range []string{"/v1/nodes", "/v1/instances", "/v1/iam", "/v1/services"} {
		http.Handle(p+"/", proxy(registryURL))
		http.Handle(p, proxy(registryURL))
	}

	// scheduler
	for _, p := range []string{"/v1/jobs", "/v1/schedule"} {
		http.Handle(p+"/", proxy(schedulerURL))
		http.Handle(p, proxy(schedulerURL))
	}

	// object-store
	for _, p := range []string{"/v1/objects", "/v1/buckets"} {
		http.Handle(p+"/", proxy(objectStoreURL))
		http.Handle(p, proxy(objectStoreURL))
	}

	log.Printf("api gateway listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}

// getenv returns the env var value or fallback.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
