package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddRecursesDirectory(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a directory tree with nested files.
	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.conf"), []byte("aaa"), 0o644); err != nil {
		t.Fatalf("writing a.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.conf"), []byte("bbb"), 0o644); err != nil {
		t.Fatalf("writing b.conf: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(g.manifest.Files))
	}

	for _, e := range g.manifest.Files {
		if e.Type == "directory" {
			t.Errorf("should not have directory type entries, got %+v", e)
		}
		if e.Type != "file" {
			t.Errorf("expected type file, got %s for %s", e.Type, e.Path)
		}
		if e.Hash == "" {
			t.Errorf("expected non-empty hash for %s", e.Path)
		}
	}
}

func TestAddRecursesSkipsDuplicates(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	// Second add of the same directory should not error or create duplicates.
	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("second Add should not error: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Errorf("expected 1 entry, got %d", len(g.manifest.Files))
	}
}

func TestMirrorUpAddsNew(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file after Add, got %d", len(g.manifest.Files))
	}

	// Create a new file inside the directory.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	if err := g.MirrorUp([]string{dir}); err != nil {
		t.Fatalf("MirrorUp: %v", err)
	}

	if len(g.manifest.Files) != 2 {
		t.Fatalf("expected 2 files after MirrorUp, got %d", len(g.manifest.Files))
	}
}

func TestMirrorUpRemovesDeleted(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("writing keep file: %v", err)
	}
	deleteFile := filepath.Join(dir, "delete.txt")
	if err := os.WriteFile(deleteFile, []byte("delete"), 0o644); err != nil {
		t.Fatalf("writing delete file: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(g.manifest.Files))
	}

	// Delete one file from disk.
	_ = os.Remove(deleteFile)

	if err := g.MirrorUp([]string{dir}); err != nil {
		t.Fatalf("MirrorUp: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file after MirrorUp, got %d", len(g.manifest.Files))
	}

	if g.manifest.Files[0].Path != toTildePath(filepath.Join(dir, "keep.txt")) {
		t.Errorf("remaining entry should be keep.txt, got %s", g.manifest.Files[0].Path)
	}
}

func TestMirrorUpRehashesChanged(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	f := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(f, []byte("original"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash

	// Modify the file.
	if err := os.WriteFile(f, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modifying file: %v", err)
	}

	if err := g.MirrorUp([]string{dir}); err != nil {
		t.Fatalf("MirrorUp: %v", err)
	}

	if g.manifest.Files[0].Hash == origHash {
		t.Error("MirrorUp did not update hash for modified file")
	}
}

func TestMirrorDownRestoresAndCleans(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("tracked"), 0o644); err != nil {
		t.Fatalf("writing tracked file: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	// Modify the tracked file and create an untracked file.
	if err := os.WriteFile(tracked, []byte("overwritten"), 0o644); err != nil {
		t.Fatalf("modifying tracked file: %v", err)
	}
	extra := filepath.Join(dir, "extra.txt")
	if err := os.WriteFile(extra, []byte("extra"), 0o644); err != nil {
		t.Fatalf("writing extra file: %v", err)
	}

	if err := g.MirrorDown([]string{dir}, true, nil); err != nil {
		t.Fatalf("MirrorDown: %v", err)
	}

	// Tracked file should be restored to original content.
	got, err := os.ReadFile(tracked)
	if err != nil {
		t.Fatalf("reading tracked file: %v", err)
	}
	if string(got) != "tracked" {
		t.Errorf("tracked file content = %q, want %q", got, "tracked")
	}

	// Extra file should be deleted.
	if _, err := os.Stat(extra); err == nil {
		t.Error("extra file should have been deleted by MirrorDown with force")
	}
}

func TestMirrorDownConfirmSkips(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	dir := filepath.Join(root, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("tracked"), 0o644); err != nil {
		t.Fatalf("writing tracked file: %v", err)
	}

	if err := g.Add([]string{dir}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	// Create an untracked file.
	extra := filepath.Join(dir, "extra.txt")
	if err := os.WriteFile(extra, []byte("extra"), 0o644); err != nil {
		t.Fatalf("writing extra file: %v", err)
	}

	// Confirm returns false — should NOT delete.
	alwaysNo := func(path string) bool { return false }
	if err := g.MirrorDown([]string{dir}, false, alwaysNo); err != nil {
		t.Fatalf("MirrorDown: %v", err)
	}

	if _, err := os.Stat(extra); err != nil {
		t.Error("extra file should NOT have been deleted when confirm returns false")
	}
}
