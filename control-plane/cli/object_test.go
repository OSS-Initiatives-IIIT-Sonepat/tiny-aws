package main

import (
	"testing"
)

func TestObjectURL_Flat(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("OBJECT_STORE_URL", "http://localhost:7001")

	got := objectURL("", "mykey")
	want := "http://localhost:7001/objects/mykey"
	if got != want {
		t.Errorf("objectURL('','mykey') = %q, want %q", got, want)
	}
}

func TestObjectURL_Bucket(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("OBJECT_STORE_URL", "http://localhost:7001")

	got := objectURL("mybucket", "mykey")
	want := "http://localhost:7001/buckets/mybucket/objects/mykey"
	if got != want {
		t.Errorf("objectURL('mybucket','mykey') = %q, want %q", got, want)
	}
}

func TestObjectURL_ViaGateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "http://gw:8000")

	got := objectURL("b", "k")
	want := "http://gw:8000/v1/buckets/b/objects/k"
	if got != want {
		t.Errorf("objectURL via gateway = %q, want %q", got, want)
	}
}

func TestParseObjectArgs_KeyOnly(t *testing.T) {
	got := parseObjectArgs([]string{"mykey"}, false)
	if got.key != "mykey" {
		t.Errorf("key = %q", got.key)
	}
	if got.bucket != "" {
		t.Errorf("bucket = %q, want empty", got.bucket)
	}
}

func TestParseObjectArgs_WithBucket(t *testing.T) {
	got := parseObjectArgs([]string{"mykey", "--bucket", "b1"}, false)
	if got.key != "mykey" || got.bucket != "b1" {
		t.Errorf("key=%q bucket=%q", got.key, got.bucket)
	}
}

func TestParseObjectArgs_WithData(t *testing.T) {
	got := parseObjectArgs([]string{"mykey", "--data", "hello"}, true)
	if string(got.data) != "hello" {
		t.Errorf("data = %q", got.data)
	}
}

func TestParseObjectArgs_BucketAndData(t *testing.T) {
	got := parseObjectArgs([]string{"k", "--bucket", "b", "--data", "d"}, true)
	if got.key != "k" || got.bucket != "b" || string(got.data) != "d" {
		t.Errorf("got key=%q bucket=%q data=%q", got.key, got.bucket, got.data)
	}
}
