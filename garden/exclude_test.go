package garden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExcludeAddsToManifest(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	secretFile := filepath.Join(root, "secret.key")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret: %v", err)
	}

	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}

	if len(g.manifest.Exclude) != 1 {
		t.Fatalf("expected 1 exclusion, got %d", len(g.manifest.Exclude))
	}

	expected := toTildePath(secretFile)
	if g.manifest.Exclude[0] != expected {
		t.Errorf("exclude[0] = %q, want %q", g.manifest.Exclude[0], expected)
	}

	// Verify persistence.
	g2, err := Open(repoDir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	if len(g2.manifest.Exclude) != 1 {
		t.Errorf("persisted excludes = %d, want 1", len(g2.manifest.Exclude))
	}
}

func TestExcludeDeduplicates(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	secretFile := filepath.Join(root, "secret.key")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret: %v", err)
	}

	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("first Exclude: %v", err)
	}
	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("second Exclude: %v", err)
	}

	if len(g.manifest.Exclude) != 1 {
		t.Errorf("expected 1 exclusion after dedup, got %d", len(g.manifest.Exclude))
	}
}

func TestExcludeRemovesTrackedEntry(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	secretFile := filepath.Join(root, "secret.key")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret: %v", err)
	}

	// Add the file first.
	if err := g.Add([]string{secretFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(g.manifest.Files))
	}

	// Now exclude it — should remove from tracked files.
	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}

	if len(g.manifest.Files) != 0 {
		t.Errorf("expected 0 files after exclude, got %d", len(g.manifest.Files))
	}
}

func TestIncludeRemovesFromExcludeList(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	secretFile := filepath.Join(root, "secret.key")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret: %v", err)
	}

	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}
	if len(g.manifest.Exclude) != 1 {
		t.Fatalf("expected 1 exclusion, got %d", len(g.manifest.Exclude))
	}

	if err := g.Include([]string{secretFile}); err != nil {
		t.Fatalf("Include: %v", err)
	}
	if len(g.manifest.Exclude) != 0 {
		t.Errorf("expected 0 exclusions after include, got %d", len(g.manifest.Exclude))
	}
}

func TestAddSkipsExcludedFile(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "config")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	normalFile := filepath.Join(testDir, "settings.yaml")
	secretFile := filepath.Join(testDir, "credentials.key")
	if err := os.WriteFile(normalFile, []byte("settings"), 0o644); err != nil {
		t.Fatalf("writing normal file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	// Exclude the secret file before adding the directory.
	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}

	if err := g.Add([]string{testDir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(g.manifest.Files))
	}

	expectedPath := toTildePath(normalFile)
	if g.manifest.Files[0].Path != expectedPath {
		t.Errorf("tracked file = %q, want %q", g.manifest.Files[0].Path, expectedPath)
	}
}

func TestAddSkipsExcludedDirectory(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "config")
	subDir := filepath.Join(testDir, "secrets")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	normalFile := filepath.Join(testDir, "settings.yaml")
	secretFile := filepath.Join(subDir, "token.key")
	if err := os.WriteFile(normalFile, []byte("settings"), 0o644); err != nil {
		t.Fatalf("writing normal file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("token"), 0o600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	// Exclude the entire secrets subdirectory.
	if err := g.Exclude([]string{subDir}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}

	if err := g.Add([]string{testDir}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(g.manifest.Files))
	}

	expectedPath := toTildePath(normalFile)
	if g.manifest.Files[0].Path != expectedPath {
		t.Errorf("tracked file = %q, want %q", g.manifest.Files[0].Path, expectedPath)
	}
}

func TestMirrorUpSkipsExcluded(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "config")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	normalFile := filepath.Join(testDir, "settings.yaml")
	secretFile := filepath.Join(testDir, "credentials.key")
	if err := os.WriteFile(normalFile, []byte("settings"), 0o644); err != nil {
		t.Fatalf("writing normal file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	// Exclude the secret file.
	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}

	if err := g.MirrorUp([]string{testDir}); err != nil {
		t.Fatalf("MirrorUp: %v", err)
	}

	// Only the normal file should be tracked.
	if len(g.manifest.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(g.manifest.Files))
	}

	expectedPath := toTildePath(normalFile)
	if g.manifest.Files[0].Path != expectedPath {
		t.Errorf("tracked file = %q, want %q", g.manifest.Files[0].Path, expectedPath)
	}
}

func TestMirrorDownLeavesExcludedAlone(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	testDir := filepath.Join(root, "config")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	normalFile := filepath.Join(testDir, "settings.yaml")
	secretFile := filepath.Join(testDir, "credentials.key")
	if err := os.WriteFile(normalFile, []byte("settings"), 0o644); err != nil {
		t.Fatalf("writing normal file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	// Add only the normal file.
	if err := g.Add([]string{normalFile}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Exclude the secret file.
	if err := g.Exclude([]string{secretFile}); err != nil {
		t.Fatalf("Exclude: %v", err)
	}

	// MirrorDown with force — excluded file should NOT be deleted.
	if err := g.MirrorDown([]string{testDir}, true, nil); err != nil {
		t.Fatalf("MirrorDown: %v", err)
	}

	if _, err := os.Stat(secretFile); err != nil {
		t.Error("excluded file should not have been deleted by MirrorDown")
	}
}

func TestIsExcludedDirectoryPrefix(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Exclude a directory.
	g.manifest.Exclude = []string{"~/config/secrets"}

	if !g.manifest.IsExcluded("~/config/secrets") {
		t.Error("exact match should be excluded")
	}
	if !g.manifest.IsExcluded("~/config/secrets/token.key") {
		t.Error("file under excluded dir should be excluded")
	}
	if !g.manifest.IsExcluded("~/config/secrets/nested/deep.key") {
		t.Error("deeply nested file under excluded dir should be excluded")
	}
	if g.manifest.IsExcluded("~/config/secrets-backup/file.key") {
		t.Error("path with similar prefix but different dir should not be excluded")
	}
	if g.manifest.IsExcluded("~/config/other.yaml") {
		t.Error("unrelated path should not be excluded")
	}
}
