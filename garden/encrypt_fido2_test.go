package garden

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// mockFIDO2 simulates a FIDO2 device for testing.
type mockFIDO2 struct {
	deviceSecret []byte // fixed secret for HMAC derivation
	credentials  map[string]bool
	available    bool
}

func newMockFIDO2() *mockFIDO2 {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return &mockFIDO2{
		deviceSecret: secret,
		credentials:  make(map[string]bool),
		available:    true,
	}
}

func (m *mockFIDO2) Register(salt []byte) ([]byte, []byte, error) {
	// Generate a random credential ID.
	credID := make([]byte, 32)
	_, _ = rand.Read(credID)
	m.credentials[string(credID)] = true

	// Derive HMAC-secret.
	mac := hmac.New(sha256.New, m.deviceSecret)
	mac.Write(salt)
	return credID, mac.Sum(nil), nil
}

func (m *mockFIDO2) Derive(credentialID []byte, salt []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, m.deviceSecret)
	mac.Write(salt)
	return mac.Sum(nil), nil
}

func (m *mockFIDO2) Available() bool {
	return m.available
}

func (m *mockFIDO2) MatchesCredential(credentialID []byte) bool {
	return m.credentials[string(credentialID)]
}

func TestAddFIDO2Slot(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "test-key"); err != nil {
		t.Fatalf("AddFIDO2Slot: %v", err)
	}

	slot, ok := g.manifest.Encryption.KekSlots["fido2/test-key"]
	if !ok {
		t.Fatal("fido2/test-key slot should exist")
	}
	if slot.Type != "fido2" {
		t.Errorf("slot type = %s, want fido2", slot.Type)
	}
	if slot.CredentialID == "" {
		t.Error("slot should have credential_id")
	}
	if slot.Salt == "" || slot.WrappedDEK == "" {
		t.Error("slot should have salt and wrapped DEK")
	}
}

func TestAddFIDO2SlotDuplicateRejected(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "mykey"); err != nil {
		t.Fatalf("first AddFIDO2Slot: %v", err)
	}

	if err := g.AddFIDO2Slot(device, "mykey"); err == nil {
		t.Fatal("duplicate AddFIDO2Slot should fail")
	}
}

func TestUnlockViaFIDO2(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "test-key"); err != nil {
		t.Fatalf("AddFIDO2Slot: %v", err)
	}

	// Re-open (DEK not cached).
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Unlock via FIDO2 — should succeed without passphrase prompt.
	err = g2.UnlockDEK(nil, device)
	if err != nil {
		t.Fatalf("UnlockDEK via FIDO2: %v", err)
	}

	if g2.dek == nil {
		t.Error("DEK should be cached after FIDO2 unlock")
	}
}

func TestFIDO2FallbackToPassphrase(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "test-key"); err != nil {
		t.Fatalf("AddFIDO2Slot: %v", err)
	}

	// Re-open.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// FIDO2 device is "unavailable" — should fall back to passphrase.
	unavailable := newMockFIDO2()
	unavailable.available = false

	err = g2.UnlockDEK(
		func() (string, error) { return "passphrase", nil },
		unavailable,
	)
	if err != nil {
		t.Fatalf("UnlockDEK fallback to passphrase: %v", err)
	}
}

func TestFIDO2SlotPersists(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "test-key"); err != nil {
		t.Fatalf("AddFIDO2Slot: %v", err)
	}

	// Re-open and verify slot persisted.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if _, ok := g2.manifest.Encryption.KekSlots["fido2/test-key"]; !ok {
		t.Fatal("FIDO2 slot should persist after re-open")
	}
}

func TestEncryptedRoundTripWithFIDO2(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := g.EncryptInit("passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	device := newMockFIDO2()
	if err := g.AddFIDO2Slot(device, "test-key"); err != nil {
		t.Fatalf("AddFIDO2Slot: %v", err)
	}

	// Add an encrypted file.
	content := []byte("fido2-protected secret\n")
	secretFile := filepath.Join(root, "secret")
	if err := os.WriteFile(secretFile, content, 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{secretFile}, true); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Re-open, unlock via FIDO2, restore.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := g2.UnlockDEK(nil, device); err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}

	_ = os.Remove(secretFile)
	if err := g2.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("reading restored: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}
