package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesStructure(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// manifest.yaml should exist
	if _, err := os.Stat(filepath.Join(repoDir, "manifest.yaml")); err != nil {
		t.Errorf("manifest.yaml not found: %v", err)
	}

	// blobs/ directory should exist
	if _, err := os.Stat(filepath.Join(repoDir, "blobs")); err != nil {
		t.Errorf("blobs/ not found: %v", err)
	}

	if g.manifest.Version != 1 {
		t.Errorf("expected version 1, got %d", g.manifest.Version)
	}

	if len(g.manifest.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(g.manifest.Files))
	}
}

func TestInitRejectsExisting(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	if _, err := Init(repoDir); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	if _, err := Init(repoDir); err == nil {
		t.Fatal("second Init should fail on existing repo")
	}
}

func TestOpenLoadsRepo(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	if _, err := Init(repoDir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	g, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if g.manifest.Version != 1 {
		t.Errorf("expected version 1, got %d", g.manifest.Version)
	}
}

func TestAddFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a file to add.
	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(g.manifest.Files))
	}

	entry := g.manifest.Files[0]
	if entry.Type != "file" {
		t.Errorf("expected type file, got %s", entry.Type)
	}
	if entry.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if entry.Mode != "0644" {
		t.Errorf("expected mode 0644, got %s", entry.Mode)
	}

	// Verify the blob was stored.
	if !g.store.Exists(entry.Hash) {
		t.Error("blob not found in store")
	}

	// Verify manifest was persisted.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	if len(g2.manifest.Files) != 1 {
		t.Errorf("persisted manifest has %d files, want 1", len(g2.manifest.Files))
	}
}

func TestAddDirectory(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "testdir")
	if err := os.Mkdir(testDir, 0o755); err != nil {
		t.Fatalf("creating test dir: %v", err)
	}

	if err := g.Add([]string{testDir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry := g.manifest.Files[0]
	if entry.Type != "directory" {
		t.Errorf("expected type directory, got %s", entry.Type)
	}
	if entry.Hash != "" {
		t.Error("directories should have no hash")
	}
}

func TestAddSymlink(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a target and a symlink to it.
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("target content"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	if err := g.Add([]string{link}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry := g.manifest.Files[0]
	if entry.Type != "link" {
		t.Errorf("expected type link, got %s", entry.Type)
	}
	if entry.Target != target {
		t.Errorf("expected target %s, got %s", target, entry.Target)
	}
	if entry.Hash != "" {
		t.Error("symlinks should have no hash")
	}
}

func TestAddDuplicateRejected(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	if err := g.Add([]string{testFile}); err == nil {
		t.Fatal("second Add of same path should fail")
	}
}

func TestHashFile(t *testing.T) {
	root := t.TempDir()
	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	hash, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	// SHA-256 of "hello"
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("expected %s, got %s", expected, hash)
	}
}

func TestExpandTildePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"~/.config/nvim", filepath.Join(home, ".config/nvim")},
		{"/tmp/foo", "/tmp/foo"},
	}

	for _, tt := range tests {
		got, err := ExpandTildePath(tt.input)
		if err != nil {
			t.Errorf("ExpandTildePath(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandTildePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
