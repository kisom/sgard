package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneRemovesOrphanedBlob(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Add a file, then remove it from manifest. The blob becomes orphaned.
	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("orphan data"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	hash := g.manifest.Files[0].Hash
	if !g.BlobExists(hash) {
		t.Fatal("blob should exist before prune")
	}

	if err := g.Remove([]string{testFile}); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	removed, err := g.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d blobs, want 1", removed)
	}
	if g.BlobExists(hash) {
		t.Error("orphaned blob should be deleted after prune")
	}
}

func TestPruneKeepsReferencedBlobs(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	hash := g.manifest.Files[0].Hash

	removed, err := g.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed %d blobs, want 0 (all referenced)", removed)
	}
	if !g.BlobExists(hash) {
		t.Error("referenced blob should still exist after prune")
	}
}
