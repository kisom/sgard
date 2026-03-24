package garden

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffUnchangedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	d, err := g.Diff(testFile)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if d != "" {
		t.Errorf("expected empty diff for unchanged file, got:\n%s", d)
	}
}

func TestDiffModifiedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Modify the file on disk.
	if err := os.WriteFile(testFile, []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("modifying test file: %v", err)
	}

	d, err := g.Diff(testFile)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if d == "" {
		t.Fatal("expected non-empty diff for modified file")
	}

	if !strings.Contains(d, "original") {
		t.Errorf("diff should contain old content 'original', got:\n%s", d)
	}
	if !strings.Contains(d, "modified") {
		t.Errorf("diff should contain new content 'modified', got:\n%s", d)
	}
	if !strings.Contains(d, "---") || !strings.Contains(d, "+++") {
		t.Errorf("diff should contain --- and +++ headers, got:\n%s", d)
	}
}

func TestDiffUntrackedPath(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err = g.Diff(filepath.Join(root, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for untracked path")
	}
}
