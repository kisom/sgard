package garden

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckpointSkipsNonMatching(t *testing.T) {
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

	// Add with only:os:fakeos — won't match this machine.
	if err := g.Add([]string{testFile}, AddOptions{Only: []string{"os:fakeos"}}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash

	// Modify file.
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	// Checkpoint should skip this entry.
	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash != origHash {
		t.Error("checkpoint should skip non-matching entry")
	}
}

func TestCheckpointProcessesMatching(t *testing.T) {
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

	// Add with only matching current OS.
	if err := g.Add([]string{testFile}, AddOptions{Only: []string{"os:" + runtime.GOOS}}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash

	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash == origHash {
		t.Error("checkpoint should process matching entry")
	}
}

func TestStatusReportsSkipped(t *testing.T) {
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

	if err := g.Add([]string{testFile}, AddOptions{Only: []string{"os:fakeos"}}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "skipped" {
		t.Errorf("expected skipped, got %v", statuses)
	}
}

func TestRestoreSkipsNonMatching(t *testing.T) {
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

	if err := g.Add([]string{testFile}, AddOptions{Only: []string{"os:fakeos"}}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Delete file and try to restore — should skip.
	_ = os.Remove(testFile)
	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// File should NOT have been restored.
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("restore should skip non-matching entry — file should not exist")
	}
}

func TestAddWithTargeting(t *testing.T) {
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

	if err := g.Add([]string{testFile}, AddOptions{
		Only: []string{"os:linux", "tag:work"},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry := g.manifest.Files[0]
	if len(entry.Only) != 2 {
		t.Fatalf("expected 2 only labels, got %d", len(entry.Only))
	}
	if entry.Only[0] != "os:linux" || entry.Only[1] != "tag:work" {
		t.Errorf("only = %v, want [os:linux tag:work]", entry.Only)
	}
}

func TestAddWithNever(t *testing.T) {
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

	if err := g.Add([]string{testFile}, AddOptions{
		Never: []string{"arch:arm64"},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry := g.manifest.Files[0]
	if len(entry.Never) != 1 || entry.Never[0] != "arch:arm64" {
		t.Errorf("never = %v, want [arch:arm64]", entry.Never)
	}
}
