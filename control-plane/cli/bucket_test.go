package main

import (
	"testing"
)

// bucket.go has no exported pure functions beyond URL construction
// (which is tested via config_test.go and object_test.go).
// These tests verify the bucket URL patterns used by runBucketCreate and runBucketList.

func TestBucketCreateURL(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("OBJECT_STORE_URL", "http://localhost:7001")

	got := objectStoreURL() + "/buckets/" + "my-bucket"
	want := "http://localhost:7001/buckets/my-bucket"
	if got != want {
		t.Errorf("bucket create URL = %q, want %q", got, want)
	}
}

func TestBucketListURL(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("OBJECT_STORE_URL", "http://localhost:7001")

	got := objectStoreURL() + "/buckets"
	want := "http://localhost:7001/buckets"
	if got != want {
		t.Errorf("bucket list URL = %q, want %q", got, want)
	}
}

func TestBucketCreateURL_ViaGateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "http://gw:8000")

	got := objectStoreURL() + "/buckets/" + "b"
	want := "http://gw:8000/v1/buckets/b"
	if got != want {
		t.Errorf("bucket URL via gateway = %q, want %q", got, want)
	}
}
