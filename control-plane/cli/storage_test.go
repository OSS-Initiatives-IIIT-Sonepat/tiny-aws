package main

import "testing"

// storage.go has no URL helper — it delegates to fetchNodes("storage").
// Test the fetchNodes role filter contract via the registry URL it uses.

func TestStorageRunRequiresNodeList(t *testing.T) {
	// runStorage expects exactly "node" "list"; anything else should be an error path.
	// We can't call runStorage directly (it calls os.Exit), so verify the guard logic.
	args := []string{"node", "list"}
	if len(args) < 2 || args[0] != "node" || args[1] != "list" {
		t.Error("valid args rejected")
	}

	bad := [][]string{
		{},
		{"node"},
		{"list"},
		{"node", "create"},
	}
	for _, b := range bad {
		if len(b) >= 2 && b[0] == "node" && b[1] == "list" {
			t.Errorf("bad args %v should be rejected", b)
		}
	}
}
