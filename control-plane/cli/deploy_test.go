package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipDirectory(t *testing.T) {
	dir := t.TempDir()

	// create a couple of files
	os.WriteFile(filepath.Join(dir, "main.sh"), []byte("#!/bin/bash\necho hi\n"), 0644)
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0755)
	os.WriteFile(filepath.Join(sub, "data.txt"), []byte("hello"), 0644)

	zipPath, err := zipDirectory(dir)
	if err != nil {
		t.Fatalf("zipDirectory: %v", err)
	}
	defer os.Remove(zipPath)

	// zip file must exist and be non-empty
	info, err := os.Stat(zipPath)
	if err != nil {
		t.Fatalf("stat zip: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("zip file is empty")
	}

	// open and verify entries
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}
	if !names["main.sh"] {
		t.Error("missing main.sh in zip")
	}
	if !names["sub/data.txt"] {
		t.Error("missing sub/data.txt in zip")
	}
	if len(names) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(names), names)
	}
}

func TestZipDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	zipPath, err := zipDirectory(dir)
	if err != nil {
		t.Fatalf("zipDirectory on empty dir: %v", err)
	}
	defer os.Remove(zipPath)

	info, err := os.Stat(zipPath)
	if err != nil {
		t.Fatalf("stat zip: %v", err)
	}
	// zip exists, just very small (header only)
	if info.Size() == 0 {
		t.Fatal("zip file is empty")
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 0 {
		t.Errorf("expected 0 entries for empty dir, got %d", len(r.File))
	}
}
