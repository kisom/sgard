package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedLockedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	testFile := filepath.Join(root, "secret")
	if err := os.WriteFile(testFile, []byte("locked secret"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	// Add as both encrypted and locked.
	if err := g.Add([]string{testFile}, AddOptions{Encrypt: true, Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry := g.manifest.Files[0]
	if !entry.Encrypted {
		t.Error("should be encrypted")
	}
	if !entry.Locked {
		t.Error("should be locked")
	}
	if entry.PlaintextHash == "" {
		t.Error("should have plaintext hash")
	}

	origHash := entry.Hash

	// Modify the file — checkpoint should skip (locked).
	if err := os.WriteFile(testFile, []byte("system overwrote"), 0o600); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash != origHash {
		t.Error("checkpoint should skip locked file even if encrypted")
	}

	// Status should report drifted.
	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "drifted" {
		t.Errorf("expected drifted, got %v", statuses)
	}

	// Restore should decrypt and overwrite without prompting.
	if err := g.Restore(nil, false, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "locked secret" {
		t.Errorf("content = %q, want %q", got, "locked secret")
	}
}

func TestDirOnlyLocked(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "lockdir")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Add as dir-only and locked.
	if err := g.Add([]string{testDir}, AddOptions{DirOnly: true, Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry := g.manifest.Files[0]
	if entry.Type != "directory" {
		t.Errorf("type = %s, want directory", entry.Type)
	}
	if !entry.Locked {
		t.Error("should be locked")
	}

	// Remove the directory.
	if err := os.RemoveAll(testDir); err != nil {
		t.Fatalf("removing: %v", err)
	}

	// Restore should recreate it.
	if err := g.Restore(nil, false, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("directory not restored: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
}

func TestLockUnlockEncryptedToggle(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	testFile := filepath.Join(root, "secret")
	if err := os.WriteFile(testFile, []byte("data"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	// Add encrypted but not locked.
	if err := g.Add([]string{testFile}, AddOptions{Encrypt: true}); err != nil {
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
		t.Error("should be locked")
	}
	if !g.manifest.Files[0].Encrypted {
		t.Error("should still be encrypted")
	}

	// Modify — checkpoint should skip.
	origHash := g.manifest.Files[0].Hash
	if err := os.WriteFile(testFile, []byte("changed"), 0o600); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash != origHash {
		t.Error("checkpoint should skip locked encrypted file")
	}

	// Unlock — checkpoint should now pick up changes.
	if err := g.Unlock([]string{testFile}); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash == origHash {
		t.Error("unlocked: checkpoint should update encrypted file hash")
	}
}
