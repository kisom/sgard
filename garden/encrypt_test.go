package garden

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kisom/sgard/manifest"
)

func TestEncryptInit(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("test-passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	if g.manifest.Encryption == nil {
		t.Fatal("encryption section should be present")
	}
	if g.manifest.Encryption.Algorithm != "xchacha20-poly1305" {
		t.Errorf("algorithm = %s, want xchacha20-poly1305", g.manifest.Encryption.Algorithm)
	}
	slot, ok := g.manifest.Encryption.KekSlots["passphrase"]
	if !ok {
		t.Fatal("passphrase slot should exist")
	}
	if slot.Type != "passphrase" {
		t.Errorf("slot type = %s, want passphrase", slot.Type)
	}
	if slot.Salt == "" || slot.WrappedDEK == "" {
		t.Error("slot should have salt and wrapped DEK")
	}

	// DEK should be cached.
	if g.dek == nil {
		t.Error("DEK should be cached after EncryptInit")
	}

	// Double init should fail.
	if err := g.EncryptInit("other"); err == nil {
		t.Fatal("double EncryptInit should fail")
	}
}

func TestEncryptInitPersists(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("test-passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// Re-open and verify encryption section persisted.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if g2.manifest.Encryption == nil {
		t.Fatal("encryption section should persist after re-open")
	}
}

func TestUnlockDEK(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("correct-passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// Re-open (DEK not cached).
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Unlock with correct passphrase.
	err = g2.UnlockDEK(func() (string, error) { return "correct-passphrase", nil })
	if err != nil {
		t.Fatalf("UnlockDEK with correct passphrase: %v", err)
	}

	// Re-open and try wrong passphrase.
	g3, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	err = g3.UnlockDEK(func() (string, error) { return "wrong-passphrase", nil })
	if err == nil {
		t.Fatal("UnlockDEK with wrong passphrase should fail")
	}
}

func TestAddEncrypted(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// Add an encrypted file.
	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("secret data\n"), 0o600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	if err := g.Add([]string{secretFile}, true); err != nil {
		t.Fatalf("Add encrypted: %v", err)
	}

	// Add a plaintext file.
	plainFile := filepath.Join(root, "plain")
	if err := os.WriteFile(plainFile, []byte("plain data\n"), 0o644); err != nil {
		t.Fatalf("writing plain file: %v", err)
	}

	if err := g.Add([]string{plainFile}); err != nil {
		t.Fatalf("Add plaintext: %v", err)
	}

	if len(g.manifest.Files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(g.manifest.Files))
	}

	// Check encrypted entry.
	var secretEntry, plainEntry *manifest.Entry
	for i := range g.manifest.Files {
		if g.manifest.Files[i].Encrypted {
			secretEntry = &g.manifest.Files[i]
		} else {
			plainEntry = &g.manifest.Files[i]
		}
	}

	if secretEntry == nil {
		t.Fatal("expected an encrypted entry")
	}
	if secretEntry.PlaintextHash == "" {
		t.Error("encrypted entry should have plaintext_hash")
	}
	if secretEntry.Hash == "" {
		t.Error("encrypted entry should have hash (of ciphertext)")
	}

	if plainEntry == nil {
		t.Fatal("expected a plaintext entry")
	}
	if plainEntry.PlaintextHash != "" {
		t.Error("plaintext entry should not have plaintext_hash")
	}
	if plainEntry.Encrypted {
		t.Error("plaintext entry should not be encrypted")
	}

	// The stored blob for the encrypted file should NOT be the plaintext.
	storedData, err := g.ReadBlob(secretEntry.Hash)
	if err != nil {
		t.Fatalf("ReadBlob: %v", err)
	}
	if string(storedData) == "secret data\n" {
		t.Error("stored blob should be encrypted, not plaintext")
	}
}

func TestEncryptedRestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	content := []byte("sensitive config data\n")
	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, content, 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{secretFile}, true); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Delete and restore.
	_ = os.Remove(secretFile)

	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("restored content = %q, want %q", got, content)
	}
}

func TestEncryptedCheckpoint(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("original"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{secretFile}, true); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash
	origPtHash := g.manifest.Files[0].PlaintextHash

	// Modify file.
	if err := os.WriteFile(secretFile, []byte("modified"), 0o600); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash == origHash {
		t.Error("encrypted hash should change after modification")
	}
	if g.manifest.Files[0].PlaintextHash == origPtHash {
		t.Error("plaintext hash should change after modification")
	}
}

func TestEncryptedStatus(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("data"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{secretFile}, true); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Unchanged — should be ok.
	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "ok" {
		t.Errorf("expected ok, got %v", statuses)
	}

	// Modify — should be modified.
	if err := os.WriteFile(secretFile, []byte("changed"), 0o600); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	statuses, err = g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "modified" {
		t.Errorf("expected modified, got %v", statuses)
	}
}

func TestEncryptedDiff(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{secretFile}, true); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Unchanged — empty diff.
	d, err := g.Diff(secretFile)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d != "" {
		t.Errorf("expected empty diff for unchanged encrypted file, got:\n%s", d)
	}

	// Modify.
	if err := os.WriteFile(secretFile, []byte("modified\n"), 0o600); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	d, err = g.Diff(secretFile)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d == "" {
		t.Fatal("expected non-empty diff for modified encrypted file")
	}
}

func TestAddEncryptedRequiresDEK(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// No encryption initialized.
	testFile := filepath.Join(root, "file")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	err = g.Add([]string{testFile}, true)
	if err == nil {
		t.Fatal("Add --encrypt without DEK should fail")
	}
}
