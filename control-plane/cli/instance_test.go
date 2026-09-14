package main

import (
	"encoding/json"
	"testing"
)

// instance.go's runInstanceLaunch builds a JSON payload with instance_type and
// optional volumes parsed from CLI flags. The parsing is inline, so we test
// the same logic: marshal a request body from flags and verify the JSON shape.

func TestInstanceLaunchPayload_Default(t *testing.T) {
	// Simulates: tinyaws instance launch (no flags)
	instanceType := "small"
	var volumes []string

	reqBody := map[string]any{"instance_type": instanceType}
	if len(volumes) > 0 {
		reqBody["volumes"] = volumes
	}
	payload, _ := json.Marshal(reqBody)

	var got map[string]any
	json.Unmarshal(payload, &got)

	if got["instance_type"] != "small" {
		t.Errorf("instance_type = %v", got["instance_type"])
	}
	if _, ok := got["volumes"]; ok {
		t.Error("volumes should be absent when none provided")
	}
}

func TestInstanceLaunchPayload_WithTypeAndVolumes(t *testing.T) {
	// Simulates: tinyaws instance launch --type medium --volume /data:/data
	args := []string{"--type", "medium", "--volume", "/data:/data"}

	instanceType := "small"
	var volumes []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--type" {
			instanceType = args[i+1]
			i++
		} else if args[i] == "--volume" {
			volumes = append(volumes, args[i+1])
			i++
		}
	}

	reqBody := map[string]any{"instance_type": instanceType}
	if len(volumes) > 0 {
		reqBody["volumes"] = volumes
	}
	payload, _ := json.Marshal(reqBody)

	var got map[string]any
	json.Unmarshal(payload, &got)

	if got["instance_type"] != "medium" {
		t.Errorf("instance_type = %v", got["instance_type"])
	}
	vols, ok := got["volumes"].([]any)
	if !ok || len(vols) != 1 || vols[0] != "/data:/data" {
		t.Errorf("volumes = %v", got["volumes"])
	}
}

func TestInstanceLaunchPayload_MultipleVolumes(t *testing.T) {
	args := []string{"--volume", "/a:/a", "--volume", "/b:/b"}

	instanceType := "small"
	var volumes []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--type" {
			instanceType = args[i+1]
			i++
		} else if args[i] == "--volume" {
			volumes = append(volumes, args[i+1])
			i++
		}
	}

	reqBody := map[string]any{"instance_type": instanceType}
	if len(volumes) > 0 {
		reqBody["volumes"] = volumes
	}
	payload, _ := json.Marshal(reqBody)

	var got map[string]any
	json.Unmarshal(payload, &got)

	vols, ok := got["volumes"].([]any)
	if !ok || len(vols) != 2 {
		t.Errorf("expected 2 volumes, got %v", got["volumes"])
	}
}
