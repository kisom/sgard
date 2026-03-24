package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListEmpty(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	entries := g.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestListAfterAdd(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create two files and add them.
	file1 := filepath.Join(root, "file1")
	if err := os.WriteFile(file1, []byte("one"), 0o644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	file2 := filepath.Join(root, "file2")
	if err := os.WriteFile(file2, []byte("two"), 0o644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	if err := g.Add([]string{file1, file2}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entries := g.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify entries have the correct paths (in tilde form).
	paths := make(map[string]bool)
	for _, e := range entries {
		paths[e.Path] = true
	}

	want1 := toTildePath(file1)
	want2 := toTildePath(file2)

	if !paths[want1] {
		t.Errorf("missing entry for %s", want1)
	}
	if !paths[want2] {
		t.Errorf("missing entry for %s", want2)
	}
}
