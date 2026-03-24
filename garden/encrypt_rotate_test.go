package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateDEK(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	passphrase := "test-passphrase"
	if err := g.EncryptInit(passphrase); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// Add an encrypted file and a plaintext file.
	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("secret data"), 0o600); err != nil {
		t.Fatalf("writing secret: %v", err)
	}
	plainFile := filepath.Join(root, "plain")
	if err := os.WriteFile(plainFile, []byte("plain data"), 0o644); err != nil {
		t.Fatalf("writing plain: %v", err)
	}

	if err := g.Add([]string{secretFile}, AddOptions{Encrypt: true}); err != nil {
		t.Fatalf("Add encrypted: %v", err)
	}
	if err := g.Add([]string{plainFile}); err != nil {
		t.Fatalf("Add plain: %v", err)
	}

	// Record pre-rotation state.
	var origEncHash, origEncPtHash, origPlainHash string
	for _, e := range g.manifest.Files {
		if e.Encrypted {
			origEncHash = e.Hash
			origEncPtHash = e.PlaintextHash
		} else {
			origPlainHash = e.Hash
		}
	}

	oldDEK := make([]byte, len(g.dek))
	copy(oldDEK, g.dek)

	// Rotate.
	prompt := func() (string, error) { return passphrase, nil }
	if err := g.RotateDEK(prompt); err != nil {
		t.Fatalf("RotateDEK: %v", err)
	}

	// DEK should have changed.
	if string(g.dek) == string(oldDEK) {
		t.Error("DEK should change after rotation")
	}

	// Check manifest entries.
	for _, e := range g.manifest.Files {
		if e.Encrypted {
			// Ciphertext hash should change (new nonce + new key).
			if e.Hash == origEncHash {
				t.Error("encrypted entry hash should change after rotation")
			}
			// Plaintext hash should NOT change.
			if e.PlaintextHash != origEncPtHash {
				t.Errorf("plaintext hash changed: %s → %s", origEncPtHash, e.PlaintextHash)
			}
		} else {
			// Plaintext entry should be untouched.
			if e.Hash != origPlainHash {
				t.Errorf("plaintext entry hash changed: %s → %s", origPlainHash, e.Hash)
			}
		}
	}

	// Verify the new blob decrypts correctly.
	_ = os.Remove(secretFile)
	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore after rotation: %v", err)
	}
	got, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(got) != "secret data" {
		t.Errorf("restored content = %q, want %q", got, "secret data")
	}
}

func TestRotateDEK_UnlockWithNewPassphrase(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	passphrase := "original"
	if err := g.EncryptInit(passphrase); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("data"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if err := g.Add([]string{secretFile}, AddOptions{Encrypt: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Rotate with the same passphrase.
	prompt := func() (string, error) { return passphrase, nil }
	if err := g.RotateDEK(prompt); err != nil {
		t.Fatalf("RotateDEK: %v", err)
	}

	// Re-open and verify unlock still works with the same passphrase.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := g2.UnlockDEK(prompt); err != nil {
		t.Fatalf("UnlockDEK after rotation: %v", err)
	}

	// Verify restore works.
	_ = os.Remove(secretFile)
	if err := g2.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore after re-open: %v", err)
	}
	got, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}
}

func TestRotateDEK_WithFIDO2(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	passphrase := "passphrase"
	if err := g.EncryptInit(passphrase); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// Add a FIDO2 slot.
	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "testkey"); err != nil {
		t.Fatalf("AddFIDO2Slot: %v", err)
	}

	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, []byte("fido2 data"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if err := g.Add([]string{secretFile}, AddOptions{Encrypt: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Rotate with both passphrase and FIDO2 device.
	prompt := func() (string, error) { return passphrase, nil }
	if err := g.RotateDEK(prompt, device); err != nil {
		t.Fatalf("RotateDEK: %v", err)
	}

	// Both slots should still exist.
	slots := g.ListSlots()
	if _, ok := slots["passphrase"]; !ok {
		t.Error("passphrase slot should still exist after rotation")
	}
	if _, ok := slots["fido2/testkey"]; !ok {
		t.Error("fido2/testkey slot should still exist after rotation")
	}

	// Unlock via FIDO2 should work.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := g2.UnlockDEK(nil, device); err != nil {
		t.Fatalf("UnlockDEK via FIDO2 after rotation: %v", err)
	}

	// Verify decryption.
	_ = os.Remove(secretFile)
	if err := g2.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	got, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "fido2 data" {
		t.Errorf("got %q, want %q", got, "fido2 data")
	}
}

func TestRotateDEK_RequiresUnlock(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("pass"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// Re-open without unlocking.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	err = g2.RotateDEK(func() (string, error) { return "pass", nil })
	if err == nil {
		t.Fatal("RotateDEK without unlock should fail")
	}
}
