package main

import "testing"

func TestSqsURL_Default(t *testing.T) {
	t.Setenv("SQS_URL", "")
	if got := sqsURL(); got != "http://127.0.0.1:9003" {
		t.Errorf("default = %q", got)
	}
}

func TestSqsURL_Custom(t *testing.T) {
	t.Setenv("SQS_URL", "http://sqs:9003")
	if got := sqsURL(); got != "http://sqs:9003" {
		t.Errorf("custom = %q", got)
	}
}
