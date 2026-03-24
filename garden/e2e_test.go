package garden

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

// TestE2E exercises the full lifecycle: init → add → checkpoint → modify →
// status → restore → verify.
func TestE2E(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	// Use a fake clock so timestamps are deterministic.
	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	// 1. Init
	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	g.SetClock(fakeClock)

	// 2. Create and add files.
	bashrc := filepath.Join(root, "bashrc")
	gitconfig := filepath.Join(root, "gitconfig")
	if err := os.WriteFile(bashrc, []byte("export PS1='$ '\n"), 0o644); err != nil {
		t.Fatalf("writing bashrc: %v", err)
	}
	if err := os.WriteFile(gitconfig, []byte("[user]\n\tname = test\n"), 0o644); err != nil {
		t.Fatalf("writing gitconfig: %v", err)
	}

	if err := g.Add([]string{bashrc, gitconfig}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(g.manifest.Files))
	}

	// 3. Checkpoint.
	fakeClock.Advance(time.Hour)
	if err := g.Checkpoint("initial checkpoint"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Message != "initial checkpoint" {
		t.Errorf("expected message 'initial checkpoint', got %q", g.manifest.Message)
	}

	// 4. Modify a file.
	if err := os.WriteFile(bashrc, []byte("export PS1='> '\n"), 0o644); err != nil {
		t.Fatalf("modifying bashrc: %v", err)
	}

	// 5. Status — should detect modification.
	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	stateMap := make(map[string]string)
	for _, s := range statuses {
		stateMap[s.Path] = s.State
	}

	bashrcPath := toTildePath(bashrc)
	gitconfigPath := toTildePath(gitconfig)

	if stateMap[bashrcPath] != "modified" {
		t.Errorf("bashrc should be modified, got %s", stateMap[bashrcPath])
	}
	if stateMap[gitconfigPath] != "ok" {
		t.Errorf("gitconfig should be ok, got %s", stateMap[gitconfigPath])
	}

	// 6. Restore — force to overwrite modified file.
	if err := g.Restore([]string{bashrc}, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(bashrc)
	if err != nil {
		t.Fatalf("reading restored bashrc: %v", err)
	}
	if string(got) != "export PS1='$ '\n" {
		t.Errorf("bashrc not restored correctly, got %q", got)
	}

	// 7. Status after restore — should be ok.
	statuses, err = g.Status()
	if err != nil {
		t.Fatalf("Status after restore: %v", err)
	}
	for _, s := range statuses {
		if s.State != "ok" {
			t.Errorf("after restore, %s should be ok, got %s", s.Path, s.State)
		}
	}

	// 8. Verify — all blobs should be intact.
	results, err := g.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("verify failed for %s: %s", r.Path, r.Detail)
		}
	}
}
