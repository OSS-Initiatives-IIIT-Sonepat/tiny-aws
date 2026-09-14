package main

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkJobStoreSave(b *testing.B) {
	js := NewJobStore(":memory:")

	job := Job{
		ID:        "job-bench",
		NodeID:    "node-1",
		Command:   "echo hello",
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job.ID = fmt.Sprintf("job-%d", i)
		if err := js.Save(job); err != nil {
			b.Fatal(err)
		}
	}
}
