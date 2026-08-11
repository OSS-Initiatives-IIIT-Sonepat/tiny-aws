package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// Node struct represents a node in the rust cluster.
type Node struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	CPUCount int    `json:"cpu_count"`
}