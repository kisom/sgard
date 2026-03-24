package garden

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/kisom/sgard/manifest"
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

	// .gitignore should exist and exclude blobs/
	gitignore, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Errorf(".gitignore not found: %v", err)
	} else if string(gitignore) != "blobs/\n" {
		t.Errorf(".gitignore content = %q, want %q", gitignore, "blobs/\n")
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
	testFile := filepath.Join(testDir, "inside.txt")
	if err := os.WriteFile(testFile, []byte("inside"), 0o644); err != nil {
		t.Fatalf("writing file inside dir: %v", err)
	}

	if err := g.Add([]string{testDir}); err != nil {
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
	expectedPath := toTildePath(testFile)
	if entry.Path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, entry.Path)
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

func TestCheckpointDetectsChanges(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash

	// Modify the file.
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modifying test file: %v", err)
	}

	if err := g.Checkpoint("test checkpoint"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash == origHash {
		t.Error("checkpoint did not update hash for modified file")
	}
	if g.manifest.Message != "test checkpoint" {
		t.Errorf("expected message 'test checkpoint', got %q", g.manifest.Message)
	}

	// Verify new blob exists in store.
	if !g.store.Exists(g.manifest.Files[0].Hash) {
		t.Error("new blob not found in store")
	}

	// Verify manifest persisted.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	if g2.manifest.Files[0].Hash == origHash {
		t.Error("persisted manifest still has old hash")
	}
}

func TestCheckpointUnchangedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("same"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	origHash := g.manifest.Files[0].Hash
	origUpdated := g.manifest.Files[0].Updated

	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	if g.manifest.Files[0].Hash != origHash {
		t.Error("hash should not change for unmodified file")
	}
	if !g.manifest.Files[0].Updated.Equal(origUpdated) {
		t.Error("entry timestamp should not change for unmodified file")
	}
}

func TestCheckpointMissingFileSkipped(t *testing.T) {
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
		t.Fatalf("Add: %v", err)
	}

	// Remove the file before checkpoint.
	_ = os.Remove(testFile)

	// Checkpoint should not fail.
	if err := g.Checkpoint(""); err != nil {
		t.Fatalf("Checkpoint should not fail for missing file: %v", err)
	}
}

func TestStatusReportsCorrectly(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create and add two files.
	okFile := filepath.Join(root, "okfile")
	if err := os.WriteFile(okFile, []byte("unchanged"), 0o644); err != nil {
		t.Fatalf("writing ok file: %v", err)
	}
	modFile := filepath.Join(root, "modfile")
	if err := os.WriteFile(modFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("writing mod file: %v", err)
	}
	missingFile := filepath.Join(root, "missingfile")
	if err := os.WriteFile(missingFile, []byte("will vanish"), 0o644); err != nil {
		t.Fatalf("writing missing file: %v", err)
	}

	if err := g.Add([]string{okFile, modFile, missingFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Modify one file, remove another.
	if err := os.WriteFile(modFile, []byte("changed"), 0o644); err != nil {
		t.Fatalf("modifying file: %v", err)
	}
	_ = os.Remove(missingFile)

	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}

	stateMap := make(map[string]string)
	for _, s := range statuses {
		stateMap[s.Path] = s.State
	}

	okPath := toTildePath(okFile)
	modPath := toTildePath(modFile)
	missingPath := toTildePath(missingFile)

	if stateMap[okPath] != "ok" {
		t.Errorf("okfile: expected ok, got %s", stateMap[okPath])
	}
	if stateMap[modPath] != "modified" {
		t.Errorf("modfile: expected modified, got %s", stateMap[modPath])
	}
	if stateMap[missingPath] != "missing" {
		t.Errorf("missingfile: expected missing, got %s", stateMap[missingPath])
	}
}

func TestRestoreFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a file and add it.
	testFile := filepath.Join(root, "testfile")
	content := []byte("restore me\n")
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Delete the file, then restore it.
	_ = os.Remove(testFile)

	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("restored content = %q, want %q", got, content)
	}
}

func TestRestorePermissions(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "secret")
	if err := os.WriteFile(testFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	_ = os.Remove(testFile)

	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("stat restored file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestRestoreSymlink(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	if err := g.Add([]string{link}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	_ = os.Remove(link)

	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if got != target {
		t.Errorf("symlink target = %q, want %q", got, target)
	}
}

func TestRestoreCreatesParentDirs(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create nested file.
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}
	if err := os.WriteFile(nested, []byte("deep"), 0o644); err != nil {
		t.Fatalf("writing nested file: %v", err)
	}

	if err := g.Add([]string{nested}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Remove the entire directory tree.
	_ = os.RemoveAll(filepath.Join(root, "a"))

	if err := g.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(nested)
	if err != nil {
		t.Fatalf("reading restored nested file: %v", err)
	}
	if string(got) != "deep" {
		t.Errorf("content = %q, want %q", got, "deep")
	}
}

func TestRestoreSelectivePaths(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	file1 := filepath.Join(root, "file1")
	file2 := filepath.Join(root, "file2")
	if err := os.WriteFile(file1, []byte("one"), 0o644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("two"), 0o644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	if err := g.Add([]string{file1, file2}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	_ = os.Remove(file1)
	_ = os.Remove(file2)

	// Restore only file1.
	if err := g.Restore([]string{file1}, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, err := os.Stat(file1); err != nil {
		t.Error("file1 should have been restored")
	}
	if _, err := os.Stat(file2); err == nil {
		t.Error("file2 should NOT have been restored")
	}
}

func TestRestoreConfirmSkips(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Overwrite with newer content (file mtime will be >= manifest updated).
	if err := os.WriteFile(testFile, []byte("newer on disk"), 0o644); err != nil {
		t.Fatalf("modifying test file: %v", err)
	}

	// Confirm returns false — should skip the file.
	alwaysNo := func(path string) bool { return false }
	if err := g.Restore(nil, false, alwaysNo); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != "newer on disk" {
		t.Error("file should not have been overwritten when confirm returns false")
	}
}

func TestGetManifest(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	m := g.GetManifest()
	if m == nil {
		t.Fatal("GetManifest returned nil")
	}
	if len(m.Files) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m.Files))
	}
}

func TestBlobExists(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, []byte("blob exists test"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	hash := g.GetManifest().Files[0].Hash
	if !g.BlobExists(hash) {
		t.Error("BlobExists returned false for a stored blob")
	}
	if g.BlobExists("0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("BlobExists returned true for a fake hash")
	}
}

func TestReadBlob(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	content := []byte("read blob test content")
	testFile := filepath.Join(root, "testfile")
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := g.Add([]string{testFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	hash := g.GetManifest().Files[0].Hash
	got, err := g.ReadBlob(hash)
	if err != nil {
		t.Fatalf("ReadBlob: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("ReadBlob content = %q, want %q", got, content)
	}
}

func TestWriteBlob(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	data := []byte("write blob test data")
	hash, err := g.WriteBlob(data)
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}

	// Verify the hash is correct SHA-256.
	sum := sha256.Sum256(data)
	wantHash := hex.EncodeToString(sum[:])
	if hash != wantHash {
		t.Errorf("WriteBlob hash = %s, want %s", hash, wantHash)
	}

	if !g.BlobExists(hash) {
		t.Error("BlobExists returned false after WriteBlob")
	}
}

func TestReplaceManifest(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a new manifest with a custom entry.
	newManifest := manifest.New()
	newManifest.Files = append(newManifest.Files, manifest.Entry{
		Path: "~/replaced-file",
		Type: "file",
		Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Mode: "0644",
	})

	if err := g.ReplaceManifest(newManifest); err != nil {
		t.Fatalf("ReplaceManifest: %v", err)
	}

	// Verify in-memory manifest was updated.
	m := g.GetManifest()
	if len(m.Files) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.Files))
	}
	if m.Files[0].Path != "~/replaced-file" {
		t.Errorf("expected path ~/replaced-file, got %s", m.Files[0].Path)
	}

	// Verify persistence by re-opening.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	m2 := g2.GetManifest()
	if len(m2.Files) != 1 {
		t.Fatalf("persisted manifest has %d entries, want 1", len(m2.Files))
	}
	if m2.Files[0].Path != "~/replaced-file" {
		t.Errorf("persisted entry path = %s, want ~/replaced-file", m2.Files[0].Path)
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
