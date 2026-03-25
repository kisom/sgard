package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInfoTrackedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a file to track.
	filePath := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := g.Add([]string{filePath}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	fi, err := g.Info(filePath)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if fi.Type != "file" {
		t.Errorf("Type = %q, want %q", fi.Type, "file")
	}
	if fi.State != "ok" {
		t.Errorf("State = %q, want %q", fi.State, "ok")
	}
	if fi.Hash == "" {
		t.Error("Hash is empty")
	}
	if fi.CurrentHash == "" {
		t.Error("CurrentHash is empty")
	}
	if fi.Hash != fi.CurrentHash {
		t.Errorf("Hash = %q != CurrentHash = %q", fi.Hash, fi.CurrentHash)
	}
	if fi.Updated == "" {
		t.Error("Updated is empty")
	}
	if fi.DiskModTime == "" {
		t.Error("DiskModTime is empty")
	}
	if !fi.BlobStored {
		t.Error("BlobStored = false, want true")
	}
	if fi.Mode != "0644" {
		t.Errorf("Mode = %q, want %q", fi.Mode, "0644")
	}
}

func TestInfoModifiedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	filePath := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := g.Add([]string{filePath}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Modify the file.
	if err := os.WriteFile(filePath, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fi, err := g.Info(filePath)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if fi.State != "modified" {
		t.Errorf("State = %q, want %q", fi.State, "modified")
	}
	if fi.CurrentHash == fi.Hash {
		t.Error("CurrentHash should differ from Hash after modification")
	}
}

func TestInfoMissingFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	filePath := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := g.Add([]string{filePath}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Remove the file.
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	fi, err := g.Info(filePath)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if fi.State != "missing" {
		t.Errorf("State = %q, want %q", fi.State, "missing")
	}
	if fi.DiskModTime != "" {
		t.Errorf("DiskModTime = %q, want empty for missing file", fi.DiskModTime)
	}
}

func TestInfoUntracked(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	filePath := filepath.Join(root, "nope.txt")
	if err := os.WriteFile(filePath, []byte("nope\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = g.Info(filePath)
	if err == nil {
		t.Fatal("Info should fail for untracked file")
	}
}

func TestInfoSymlink(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("target\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if err := g.Add([]string{linkPath}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	fi, err := g.Info(linkPath)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if fi.Type != "link" {
		t.Errorf("Type = %q, want %q", fi.Type, "link")
	}
	if fi.State != "ok" {
		t.Errorf("State = %q, want %q", fi.State, "ok")
	}
	if fi.Target != target {
		t.Errorf("Target = %q, want %q", fi.Target, target)
	}
}
