package main

import (
	"testing"
)

func TestLambdaURL_Default(t *testing.T) {
	t.Setenv("LAMBDA_URL", "")
	got := lambdaURL()
	if got != "http://127.0.0.1:9007" {
		t.Errorf("lambdaURL() = %q, want default", got)
	}
}

func TestLambdaURL_Custom(t *testing.T) {
	t.Setenv("LAMBDA_URL", "http://custom:1234")
	got := lambdaURL()
	if got != "http://custom:1234" {
		t.Errorf("lambdaURL() = %q, want custom", got)
	}
}
