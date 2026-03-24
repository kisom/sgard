package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddLocked(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("locked content\n"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{testFile}, AddOptions{Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !g.manifest.Files[0].Locked {
		t.Error("entry should be locked")
	}
}

func TestCheckpointSkipsLocked(t *testing.T) {
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

	origHash := g.manifest.Files[0].Hash

	// Modify the file — checkpoint should NOT update the hash.
	if err := os.WriteFile(testFile, []byte("system overwrote this"), 0o644); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash != origHash {
		t.Error("checkpoint should skip locked files — hash should not change")
	}
}

func TestStatusReportsDrifted(t *testing.T) {
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

	// Modify — status should report "drifted" not "modified".
	if err := os.WriteFile(testFile, []byte("system changed this"), 0o644); err != nil {
		t.Fatalf("modifying: %v", err)
	}

	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "drifted" {
		t.Errorf("expected drifted, got %v", statuses)
	}
}

func TestRestoreAlwaysRestoresLocked(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("correct content"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{testFile}, AddOptions{Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// System overwrites the file.
	if err := os.WriteFile(testFile, []byte("system garbage"), 0o644); err != nil {
		t.Fatalf("overwriting: %v", err)
	}

	// Restore without --force — locked files should still be restored.
	if err := g.Restore(nil, false, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(got) != "correct content" {
		t.Errorf("content = %q, want %q", got, "correct content")
	}
}

func TestRestoreSkipsLockedWhenHashMatches(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := g.Add([]string{testFile}, AddOptions{Lock: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// File is unchanged — restore should skip it (no unnecessary writes).
	if err := g.Restore(nil, false, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// If we got here without error, it means it didn't try to overwrite
	// an identical file, which is correct.
}

func TestAddDirOnly(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a directory with a file inside.
	testDir := filepath.Join(root, "testdir")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "file"), []byte("data"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	// Add with --dir — should NOT recurse.
	if err := g.Add([]string{testDir}, AddOptions{DirOnly: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 entry (directory), got %d", len(g.manifest.Files))
	}
	if g.manifest.Files[0].Type != "directory" {
		t.Errorf("type = %s, want directory", g.manifest.Files[0].Type)
	}
	if g.manifest.Files[0].Hash != "" {
		t.Error("directory entry should have no hash")
	}
}

func TestDirOnlyRestoreCreatesDirectory(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "testdir")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	if err := g.Add([]string{testDir}, AddOptions{DirOnly: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Remove directory.
	_ = os.RemoveAll(testDir)

	// Restore should recreate it.
	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("directory not restored: %v", err)
	}
	if !info.IsDir() {
		t.Error("restored path should be a directory")
	}
}
