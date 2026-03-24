package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveTrackedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create and add a file.
	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file after add, got %d", len(g.manifest.Files))
	}

	// Remove it.
	if err := g.Remove([]string{testFile}); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if len(g.manifest.Files) != 0 {
		t.Errorf("expected 0 files after remove, got %d", len(g.manifest.Files))
	}

	// Verify the manifest was persisted.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	if len(g2.manifest.Files) != 0 {
		t.Errorf("persisted manifest has %d files, want 0", len(g2.manifest.Files))
	}
}

func TestRemoveUntrackedPathErrors(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Try removing a path that was never added.
	bogus := filepath.Join(root, "nonexistent")
	if err := g.Remove([]string{bogus}); err == nil {
		t.Fatal("Remove of untracked path should return an error")
	}
}
