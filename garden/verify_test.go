package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyAllOK(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	results, err := g.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].OK {
		t.Errorf("expected OK, got %s", results[0].Detail)
	}
	if results[0].Detail != "ok" {
		t.Errorf("expected detail 'ok', got %q", results[0].Detail)
	}
}

func TestVerifyHashMismatch(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Corrupt the blob on disk.
	hash := g.manifest.Files[0].Hash
	blobPath := filepath.Join(repoDir, "blobs", hash[:2], hash[2:4], hash)
	if err := os.WriteFile(blobPath, []byte("corrupted data"), 0o644); err != nil {
		t.Fatalf("corrupting blob: %v", err)
	}

	results, err := g.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].OK {
		t.Error("expected not OK for corrupted blob")
	}
	if results[0].Detail != "hash mismatch" {
		t.Errorf("expected detail 'hash mismatch', got %q", results[0].Detail)
	}
}

func TestVerifyBlobMissing(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Delete the blob from disk.
	hash := g.manifest.Files[0].Hash
	blobPath := filepath.Join(repoDir, "blobs", hash[:2], hash[2:4], hash)
	if err := os.Remove(blobPath); err != nil {
		t.Fatalf("removing blob: %v", err)
	}

	results, err := g.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].OK {
		t.Error("expected not OK for missing blob")
	}
	if results[0].Detail != "blob missing" {
		t.Errorf("expected detail 'blob missing', got %q", results[0].Detail)
	}
}
