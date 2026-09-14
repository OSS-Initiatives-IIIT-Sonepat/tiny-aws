package main

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkNodeStoreSave(b *testing.B) {
	s := NewNodeStore(":memory:")

	node := Node{
		ID:       "node-bench",
		Hostname: "host-bench",
		Addr:     "10.0.0.1",
		CPUCount: 4,
		Role:     "compute",
		Status:   "healthy",
		LastSeen: time.Now().UTC(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.ID = fmt.Sprintf("node-%d", i)
		if err := s.Save(node); err != nil {
			b.Fatal(err)
		}
	}
}
