package garden

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

// TestEncryptionE2E exercises the full encryption lifecycle:
// encrypt init → add encrypted + plaintext files → checkpoint → modify →
// status → restore → verify → push/pull simulation via Garden accessors.
func TestEncryptionE2E(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	// 1. Init repo and encryption.
	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	g.SetClock(fakeClock)

	if err := g.EncryptInit("test-passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	// 2. Add a mix of encrypted and plaintext files.
	sshConfig := filepath.Join(root, "ssh_config")
	bashrc := filepath.Join(root, "bashrc")
	awsCreds := filepath.Join(root, "aws_credentials")

	if err := os.WriteFile(sshConfig, []byte("Host *\n  AddKeysToAgent yes\n"), 0o600); err != nil {
		t.Fatalf("writing ssh_config: %v", err)
	}
	if err := os.WriteFile(bashrc, []byte("export PS1='$ '\n"), 0o644); err != nil {
		t.Fatalf("writing bashrc: %v", err)
	}
	if err := os.WriteFile(awsCreds, []byte("[default]\naws_access_key_id=AKIA...\n"), 0o600); err != nil {
		t.Fatalf("writing aws_credentials: %v", err)
	}

	// Encrypted files.
	if err := g.Add([]string{sshConfig, awsCreds}, AddOptions{Encrypt: true}); err != nil {
		t.Fatalf("Add encrypted: %v", err)
	}
	// Plaintext file.
	if err := g.Add([]string{bashrc}); err != nil {
		t.Fatalf("Add plaintext: %v", err)
	}

	if len(g.manifest.Files) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(g.manifest.Files))
	}

	// Verify encrypted blobs are not plaintext.
	for _, e := range g.manifest.Files {
		if e.Encrypted {
			blob, err := g.ReadBlob(e.Hash)
			if err != nil {
				t.Fatalf("ReadBlob %s: %v", e.Path, err)
			}
			// The blob should NOT contain the plaintext.
			if e.Path == toTildePath(sshConfig) && string(blob) == "Host *\n  AddKeysToAgent yes\n" {
				t.Error("ssh_config blob should be encrypted")
			}
		}
	}

	// 3. Checkpoint.
	fakeClock.Advance(time.Hour)
	if err := g.Checkpoint("encrypted checkpoint"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	// 4. Modify an encrypted file.
	if err := os.WriteFile(sshConfig, []byte("Host *\n  ForwardAgent yes\n"), 0o600); err != nil {
		t.Fatalf("modifying ssh_config: %v", err)
	}

	// 5. Status — should detect modification without DEK.
	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	stateMap := make(map[string]string)
	for _, s := range statuses {
		stateMap[s.Path] = s.State
	}

	sshPath := toTildePath(sshConfig)
	bashrcPath := toTildePath(bashrc)
	awsPath := toTildePath(awsCreds)

	if stateMap[sshPath] != "modified" {
		t.Errorf("ssh_config should be modified, got %s", stateMap[sshPath])
	}
	if stateMap[bashrcPath] != "ok" {
		t.Errorf("bashrc should be ok, got %s", stateMap[bashrcPath])
	}
	if stateMap[awsPath] != "ok" {
		t.Errorf("aws_credentials should be ok, got %s", stateMap[awsPath])
	}

	// 6. Re-checkpoint after modification.
	fakeClock.Advance(time.Hour)
	if err := g.Checkpoint("after modification"); err != nil {
		t.Fatalf("Checkpoint after mod: %v", err)
	}

	// 7. Delete all files, then restore.
	_ = os.Remove(sshConfig)
	_ = os.Remove(bashrc)
	_ = os.Remove(awsCreds)

	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// 8. Verify restored contents.
	got, err := os.ReadFile(sshConfig)
	if err != nil {
		t.Fatalf("reading restored ssh_config: %v", err)
	}
	if string(got) != "Host *\n  ForwardAgent yes\n" {
		t.Errorf("ssh_config content = %q, want modified version", got)
	}

	got, err = os.ReadFile(bashrc)
	if err != nil {
		t.Fatalf("reading restored bashrc: %v", err)
	}
	if string(got) != "export PS1='$ '\n" {
		t.Errorf("bashrc content = %q", got)
	}

	got, err = os.ReadFile(awsCreds)
	if err != nil {
		t.Fatalf("reading restored aws_credentials: %v", err)
	}
	if string(got) != "[default]\naws_access_key_id=AKIA...\n" {
		t.Errorf("aws_credentials content = %q", got)
	}

	// 9. Verify blob integrity.
	results, err := g.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("verify failed for %s: %s", r.Path, r.Detail)
		}
	}

	// 10. Re-open repo, unlock via passphrase, verify diff works on encrypted file.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}

	if err := g2.UnlockDEK(func() (string, error) { return "test-passphrase", nil }); err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}

	// Modify ssh_config again for diff.
	if err := os.WriteFile(sshConfig, []byte("Host *\n  ForwardAgent no\n"), 0o600); err != nil {
		t.Fatalf("modifying ssh_config: %v", err)
	}

	d, err := g2.Diff(sshConfig)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d == "" {
		t.Error("expected non-empty diff for modified encrypted file")
	}

	// 11. Slot management.
	slots := g2.ListSlots()
	if len(slots) != 1 {
		t.Errorf("expected 1 slot, got %d", len(slots))
	}
	if slots["passphrase"] != "passphrase" {
		t.Errorf("expected passphrase slot, got %v", slots)
	}

	// Cannot remove the last slot.
	if err := g2.RemoveSlot("passphrase"); err == nil {
		t.Fatal("should not be able to remove last slot")
	}

	// Change passphrase.
	if err := g2.ChangePassphrase("new-passphrase"); err != nil {
		t.Fatalf("ChangePassphrase: %v", err)
	}

	// Re-open and unlock with new passphrase.
	g3, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open after passphrase change: %v", err)
	}

	if err := g3.UnlockDEK(func() (string, error) { return "new-passphrase", nil }); err != nil {
		t.Fatalf("UnlockDEK with new passphrase: %v", err)
	}

	// Old passphrase should fail.
	g4, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	if err := g4.UnlockDEK(func() (string, error) { return "test-passphrase", nil }); err == nil {
		t.Fatal("old passphrase should fail after change")
	}
}
