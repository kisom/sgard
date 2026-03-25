package server

import (
	"testing"
	"time"

	"github.com/kisom/sgard/manifest"
)

func TestManifestRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m := &manifest.Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Message: "test checkpoint",
		Files: []manifest.Entry{
			{Path: "~/.bashrc", Hash: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234", Type: "file", Mode: "0644", Updated: now},
			{Path: "~/.config/nvim", Type: "directory", Mode: "0755", Updated: now},
			{Path: "~/.vimrc", Type: "link", Target: "~/.config/nvim/init.vim", Updated: now},
		},
	}

	proto := ManifestToProto(m)
	back := ProtoToManifest(proto)

	if back.Version != m.Version {
		t.Errorf("Version: got %d, want %d", back.Version, m.Version)
	}
	if !back.Created.Equal(m.Created) {
		t.Errorf("Created: got %v, want %v", back.Created, m.Created)
	}
	if !back.Updated.Equal(m.Updated) {
		t.Errorf("Updated: got %v, want %v", back.Updated, m.Updated)
	}
	if back.Message != m.Message {
		t.Errorf("Message: got %q, want %q", back.Message, m.Message)
	}
	if len(back.Files) != len(m.Files) {
		t.Fatalf("Files count: got %d, want %d", len(back.Files), len(m.Files))
	}
	for i, want := range m.Files {
		got := back.Files[i]
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

func TestEmptyManifestRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 30, 0, 0, time.UTC)
	m := &manifest.Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Files:   []manifest.Entry{},
	}

	proto := ManifestToProto(m)
	back := ProtoToManifest(proto)

	if back.Version != m.Version {
		t.Errorf("Version: got %d, want %d", back.Version, m.Version)
	}
	if !back.Created.Equal(m.Created) {
		t.Errorf("Created: got %v, want %v", back.Created, m.Created)
	}
	if !back.Updated.Equal(m.Updated) {
		t.Errorf("Updated: got %v, want %v", back.Updated, m.Updated)
	}
	if back.Message != "" {
		t.Errorf("Message: got %q, want empty", back.Message)
	}
	if len(back.Files) != 0 {
		t.Errorf("Files count: got %d, want 0", len(back.Files))
	}
}

func TestTargetingRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)

	onlyEntry := manifest.Entry{
		Path:    "~/.bashrc.linux",
		Type:    "file",
		Hash:    "abcd",
		Only:    []string{"os:linux", "tag:work"},
		Updated: now,
	}

	proto := EntryToProto(onlyEntry)
	back := ProtoToEntry(proto)

	if len(back.Only) != 2 || back.Only[0] != "os:linux" || back.Only[1] != "tag:work" {
		t.Errorf("Only round-trip: got %v, want [os:linux tag:work]", back.Only)
	}
	if len(back.Never) != 0 {
		t.Errorf("Never should be empty, got %v", back.Never)
	}

	neverEntry := manifest.Entry{
		Path:    "~/.config/heavy",
		Type:    "file",
		Hash:    "efgh",
		Never:   []string{"arch:arm64"},
		Updated: now,
	}

	proto2 := EntryToProto(neverEntry)
	back2 := ProtoToEntry(proto2)

	if len(back2.Never) != 1 || back2.Never[0] != "arch:arm64" {
		t.Errorf("Never round-trip: got %v, want [arch:arm64]", back2.Never)
	}
	if len(back2.Only) != 0 {
		t.Errorf("Only should be empty, got %v", back2.Only)
	}
}

func TestEntryEmptyOptionalFieldsRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	e := manifest.Entry{
		Path:    "~/.profile",
		Type:    "file",
		Updated: now,
		// Hash, Mode, Target intentionally empty
	}

	proto := EntryToProto(e)
	back := ProtoToEntry(proto)

	if back.Path != e.Path {
		t.Errorf("Path: got %q, want %q", back.Path, e.Path)
	}
	if back.Hash != "" {
		t.Errorf("Hash: got %q, want empty", back.Hash)
	}
	if back.Type != e.Type {
		t.Errorf("Type: got %q, want %q", back.Type, e.Type)
	}
	if back.Mode != "" {
		t.Errorf("Mode: got %q, want empty", back.Mode)
	}
	if back.Target != "" {
		t.Errorf("Target: got %q, want empty", back.Target)
	}
	if !back.Updated.Equal(e.Updated) {
		t.Errorf("Updated: got %v, want %v", back.Updated, e.Updated)
	}
}
