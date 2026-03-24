package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockExistingEntry(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	// Add without lock.
	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if g.manifest.Files[0].Locked {
		t.Fatal("should not be locked initially")
	}

	// Lock it.
	if err := g.Lock([]string{testFile}); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if !g.manifest.Files[0].Locked {
		t.Error("should be locked after Lock()")
	}

	// Verify persisted.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !g2.manifest.Files[0].Locked {
		t.Error("locked state should persist")
	}
}

func TestUnlockExistingEntry(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{testFile}, AddOptions{Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !g.manifest.Files[0].Locked {
		t.Fatal("should be locked")
	}

	if err := g.Unlock([]string{testFile}); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	if g.manifest.Files[0].Locked {
		t.Error("should not be locked after Unlock()")
	}
}

func TestLockUntrackedErrors(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "nottracked")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Lock([]string{testFile}); err == nil {
		t.Fatal("Lock on untracked path should error")
	}
}

func TestLockChangesCheckpointBehavior(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	// Add unlocked, checkpoint picks up changes.
	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash

	if err := os.WriteFile(testFile, []byte("changed"), 0o644); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash == origHash {
		t.Fatal("unlocked file: checkpoint should update hash")
	}

	newHash := g.manifest.Files[0].Hash

	// Now lock it and modify again — checkpoint should NOT update.
	if err := g.Lock([]string{testFile}); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("system overwrote"), 0o644); err != nil {
		t.Fatalf("overwriting: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash != newHash {
		t.Error("locked file: checkpoint should not update hash")
	}
}

func TestUnlockChangesStatusBehavior(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{testFile}, AddOptions{Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("changed"), 0o644); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	// Locked: should be "drifted".
	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if statuses[0].State != "drifted" {
		t.Errorf("locked: expected drifted, got %s", statuses[0].State)
	}

	// Unlock: should now be "modified".
	if err := g.Unlock([]string{testFile}); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	statuses, err = g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if statuses[0].State != "modified" {
		t.Errorf("unlocked: expected modified, got %s", statuses[0].State)
	}
}
