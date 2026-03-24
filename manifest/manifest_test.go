package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	m := &Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Message: "test checkpoint",
		Files: []Entry{
			{
				Path:    "~/.bashrc",
				Hash:    "a1b2c3d4e5f6",
				Type:    "file",
				Mode:    "0644",
				Updated: now,
			},
			{
				Path:    "~/.config/nvim",
				Type:    "directory",
				Mode:    "0755",
				Updated: now,
			},
			{
				Path:    "~/.vimrc",
				Type:    "link",
				Target:  "~/.config/nvim/init.vim",
				Updated: now,
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")

	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Version != m.Version {
		t.Errorf("Version: got %d, want %d", loaded.Version, m.Version)
	}
	if !loaded.Created.Equal(m.Created) {
		t.Errorf("Created: got %v, want %v", loaded.Created, m.Created)
	}
	if !loaded.Updated.Equal(m.Updated) {
		t.Errorf("Updated: got %v, want %v", loaded.Updated, m.Updated)
	}
	if loaded.Message != m.Message {
		t.Errorf("Message: got %q, want %q", loaded.Message, m.Message)
	}
	if len(loaded.Files) != len(m.Files) {
		t.Fatalf("Files: got %d entries, want %d", len(loaded.Files), len(m.Files))
	}

	for i, got := range loaded.Files {
		want := m.Files[i]
		if got.Path != want.Path {
			t.Errorf("Files[%d].Path: got %q, want %q", i, got.Path, want.Path)
		}
		if got.Hash != want.Hash {
			t.Errorf("Files[%d].Hash: got %q, want %q", i, got.Hash, want.Hash)
		}
		if got.Type != want.Type {
			t.Errorf("Files[%d].Type: got %q, want %q", i, got.Type, want.Type)
		}
		if got.Mode != want.Mode {
			t.Errorf("Files[%d].Mode: got %q, want %q", i, got.Mode, want.Mode)
		}
		if got.Target != want.Target {
			t.Errorf("Files[%d].Target: got %q, want %q", i, got.Target, want.Target)
		}
		if !got.Updated.Equal(want.Updated) {
			t.Errorf("Files[%d].Updated: got %v, want %v", i, got.Updated, want.Updated)
		}
	}
}

func TestAtomicSave(t *testing.T) {
	m := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")

	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Target file must exist.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("target file missing: %v", err)
	}

	// No .tmp files should remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file remains: %s", e.Name())
		}
	}

	// Verify content is valid YAML that loads back.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("loaded version: got %d, want 1", loaded.Version)
	}
}

func TestEntryTypes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	m := &Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Files: []Entry{
			{
				Path:    "~/.bashrc",
				Hash:    "abc123",
				Type:    "file",
				Mode:    "0644",
				Updated: now,
			},
			{
				Path:    "~/.config",
				Type:    "directory",
				Mode:    "0755",
				Updated: now,
			},
			{
				Path:    "~/.vimrc",
				Type:    "link",
				Target:  "~/.config/nvim/init.vim",
				Updated: now,
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")

	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// File entry: has hash and mode.
	file := loaded.Files[0]
	if file.Hash == "" {
		t.Error("file entry should have hash")
	}
	if file.Mode == "" {
		t.Error("file entry should have mode")
	}

	// Directory entry: has mode, no hash.
	directory := loaded.Files[1]
	if directory.Hash != "" {
		t.Errorf("directory entry should have no hash, got %q", directory.Hash)
	}
	if directory.Mode == "" {
		t.Error("directory entry should have mode")
	}

	// Link entry: has target, no hash.
	link := loaded.Files[2]
	if link.Hash != "" {
		t.Errorf("link entry should have no hash, got %q", link.Hash)
	}
	if link.Target == "" {
		t.Error("link entry should have target")
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/manifest.yaml")
	if err == nil {
		t.Fatal("Load of nonexistent file should return error")
	}
}

func TestNew(t *testing.T) {
	before := time.Now().UTC()
	m := New()
	after := time.Now().UTC()

	if m.Version != 1 {
		t.Errorf("Version: got %d, want 1", m.Version)
	}
	if m.Created.Before(before) || m.Created.After(after) {
		t.Errorf("Created %v not between %v and %v", m.Created, before, after)
	}
	if m.Updated.Before(before) || m.Updated.After(after) {
		t.Errorf("Updated %v not between %v and %v", m.Updated, before, after)
	}
	if m.Message != "" {
		t.Errorf("Message: got %q, want empty", m.Message)
	}
	if m.Files == nil {
		t.Error("Files should be non-nil empty slice")
	}
	if len(m.Files) != 0 {
		t.Errorf("Files: got %d entries, want 0", len(m.Files))
	}
}
