package garden

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kisom/sgard/manifest"
)

func TestEntryApplies_NoTargeting(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc"}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("entry with no targeting should always apply")
	}
}

func TestEntryApplies_OnlyMatch(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Only: []string{"os:linux"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match os:linux")
	}
}

func TestEntryApplies_OnlyNoMatch(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Only: []string{"os:darwin"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("os:darwin should not match os:linux machine")
	}
}

func TestEntryApplies_OnlyHostname(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Only: []string{"vade"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match hostname vade")
	}
}

func TestEntryApplies_OnlyTag(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Only: []string{"tag:work"}}

	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "tag:work"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match tag:work")
	}

	ok, err = EntryApplies(entry, []string{"vade", "os:linux"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("should not match without tag:work")
	}
}

func TestEntryApplies_NeverMatch(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Never: []string{"arch:arm64"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:arm64"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("should be excluded by never:arch:arm64")
	}
}

func TestEntryApplies_NeverNoMatch(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Never: []string{"arch:arm64"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("arch:amd64 machine should not be excluded by never:arch:arm64")
	}
}

func TestEntryApplies_BothError(t *testing.T) {
	entry := &manifest.Entry{
		Path:  "~/.bashrc",
		Only:  []string{"os:linux"},
		Never: []string{"arch:arm64"},
	}
	_, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err == nil {
		t.Fatal("should error when both only and never are set")
	}
}

func TestEntryApplies_CaseInsensitive(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Only: []string{"OS:Linux"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("matching should be case-insensitive")
	}
}

func TestEntryApplies_OnlyMultiple(t *testing.T) {
	entry := &manifest.Entry{Path: "~/.bashrc", Only: []string{"os:darwin", "os:linux"}}
	ok, err := EntryApplies(entry, []string{"vade", "os:linux", "arch:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match if any label in only matches")
	}
}

func TestIdentity(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	labels := g.Identity()

	// Should contain os and arch.
	found := make(map[string]bool)
	for _, l := range labels {
		found[l] = true
	}

	osLabel := "os:" + runtime.GOOS
	archLabel := "arch:" + runtime.GOARCH
	if !found[osLabel] {
		t.Errorf("identity should contain %s", osLabel)
	}
	if !found[archLabel] {
		t.Errorf("identity should contain %s", archLabel)
	}

	// Should contain a hostname (non-empty, no dots).
	hostname := labels[0]
	if hostname == "" || strings.Contains(hostname, ".") || strings.Contains(hostname, ":") {
		t.Errorf("first label should be short hostname, got %q", hostname)
	}
}

func TestTags(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	g, err := Init(repoDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// No tags initially.
	if tags := g.LoadTags(); len(tags) != 0 {
		t.Fatalf("expected no tags, got %v", tags)
	}

	// Add tags.
	if err := g.SaveTag("work"); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	if err := g.SaveTag("desktop"); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}

	tags := g.LoadTags()
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", tags)
	}

	// Duplicate add is idempotent.
	if err := g.SaveTag("work"); err != nil {
		t.Fatalf("SaveTag duplicate: %v", err)
	}
	if tags := g.LoadTags(); len(tags) != 2 {
		t.Fatalf("expected 2 tags after duplicate add, got %v", tags)
	}

	// Remove.
	if err := g.RemoveTag("work"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}
	tags = g.LoadTags()
	if len(tags) != 1 || tags[0] != "desktop" {
		t.Fatalf("expected [desktop], got %v", tags)
	}

	// Tags appear in identity.
	labels := g.Identity()
	found := false
	for _, l := range labels {
		if l == "tag:desktop" {
			found = true
		}
	}
	if !found {
		t.Errorf("identity should contain tag:desktop, got %v", labels)
	}
}

func TestInitCreatesGitignoreWithTags(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")

	if _, err := Init(repoDir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(data), "tags") {
		t.Error(".gitignore should contain 'tags'")
	}
}
