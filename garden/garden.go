// Package garden is the core business logic for sgard. It orchestrates the
// manifest and blob store to implement dotfile management operations.
package garden

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/kisom/sgard/manifest"
	"github.com/kisom/sgard/store"
)

// Garden ties a manifest and blob store together to manage dotfiles.
type Garden struct {
	manifest     *manifest.Manifest
	store        *store.Store
	root         string // repository root directory
	manifestPath string // path to manifest.yaml
	clock        clockwork.Clock
}

// Init creates a new sgard repository at root. It creates the directory
// structure and an empty manifest.
func Init(root string) (*Garden, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving repo path: %w", err)
	}

	manifestPath := filepath.Join(absRoot, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		return nil, fmt.Errorf("repository already exists at %s", absRoot)
	}

	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, fmt.Errorf("creating repo directory: %w", err)
	}

	s, err := store.New(absRoot)
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	gitignorePath := filepath.Join(absRoot, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("blobs/\n"), 0o644); err != nil {
		return nil, fmt.Errorf("creating .gitignore: %w", err)
	}

	clk := clockwork.NewRealClock()
	m := manifest.NewWithTime(clk.Now().UTC())
	if err := m.Save(manifestPath); err != nil {
		return nil, fmt.Errorf("saving initial manifest: %w", err)
	}

	return &Garden{
		manifest:     m,
		store:        s,
		root:         absRoot,
		manifestPath: manifestPath,
		clock:        clk,
	}, nil
}

// Open loads an existing sgard repository from root.
func Open(root string) (*Garden, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving repo path: %w", err)
	}

	manifestPath := filepath.Join(absRoot, "manifest.yaml")
	m, err := manifest.Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("loading manifest: %w", err)
	}

	s, err := store.New(absRoot)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	return &Garden{
		manifest:     m,
		store:        s,
		root:         absRoot,
		manifestPath: manifestPath,
		clock:        clockwork.NewRealClock(),
	}, nil
}

// SetClock replaces the clock used for timestamps. Intended for testing.
func (g *Garden) SetClock(c clockwork.Clock) {
	g.clock = c
}

// addEntry adds a single file or symlink to the manifest. The abs path must
// already be resolved and info must come from os.Lstat. If skipDup is true,
// already-tracked paths are silently skipped instead of returning an error.
func (g *Garden) addEntry(abs string, info os.FileInfo, now time.Time, skipDup bool) error {
	tilded := toTildePath(abs)

	if g.findEntry(tilded) != nil {
		if skipDup {
			return nil
		}
		return fmt.Errorf("already tracking %s", tilded)
	}

	entry := manifest.Entry{
		Path:    tilded,
		Mode:    fmt.Sprintf("%04o", info.Mode().Perm()),
		Updated: now,
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(abs)
		if err != nil {
			return fmt.Errorf("reading symlink %s: %w", abs, err)
		}
		entry.Type = "link"
		entry.Target = target

	default:
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", abs, err)
		}
		hash, err := g.store.Write(data)
		if err != nil {
			return fmt.Errorf("storing blob for %s: %w", abs, err)
		}
		entry.Type = "file"
		entry.Hash = hash
	}

	g.manifest.Files = append(g.manifest.Files, entry)
	return nil
}

// Add tracks new files, directories, or symlinks. Each path is resolved
// to an absolute path, inspected for its type, and added to the manifest.
// Regular files are hashed and stored in the blob store. Directories are
// recursively walked and all leaf files and symlinks are added individually.
func (g *Garden) Add(paths []string) error {
	now := g.clock.Now().UTC()

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		info, err := os.Lstat(abs)
		if err != nil {
			return fmt.Errorf("stat %s: %w", abs, err)
		}

		if info.IsDir() {
			// Recursively walk the directory, adding all files and symlinks.
			err := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil // skip directory entries themselves
				}
				fi, err := os.Lstat(path)
				if err != nil {
					return fmt.Errorf("stat %s: %w", path, err)
				}
				return g.addEntry(path, fi, now, true)
			})
			if err != nil {
				return fmt.Errorf("walking directory %s: %w", abs, err)
			}
		} else {
			if err := g.addEntry(abs, info, now, false); err != nil {
				return err
			}
		}
	}

	g.manifest.Updated = now
	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// FileStatus reports the state of a tracked entry relative to the filesystem.
type FileStatus struct {
	Path  string // tilde path from manifest
	State string // "ok", "modified", "missing"
}

// Checkpoint re-hashes all tracked files, stores any changed blobs, and
// updates the manifest timestamps. The optional message is recorded in
// the manifest.
func (g *Garden) Checkpoint(message string) error {
	now := g.clock.Now().UTC()

	for i := range g.manifest.Files {
		entry := &g.manifest.Files[i]

		abs, err := ExpandTildePath(entry.Path)
		if err != nil {
			return fmt.Errorf("expanding path %s: %w", entry.Path, err)
		}

		info, err := os.Lstat(abs)
		if err != nil {
			// File is missing — leave the manifest entry as-is so status
			// can report it. Don't fail the whole checkpoint.
			continue
		}

		entry.Mode = fmt.Sprintf("%04o", info.Mode().Perm())

		switch entry.Type {
		case "file":
			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Errorf("reading %s: %w", abs, err)
			}
			hash, err := g.store.Write(data)
			if err != nil {
				return fmt.Errorf("storing blob for %s: %w", abs, err)
			}
			if hash != entry.Hash {
				entry.Hash = hash
				entry.Updated = now
			}

		case "link":
			target, err := os.Readlink(abs)
			if err != nil {
				return fmt.Errorf("reading symlink %s: %w", abs, err)
			}
			if target != entry.Target {
				entry.Target = target
				entry.Updated = now
			}

		case "directory":
			// Nothing to hash; just update mode (already done above).
		}
	}

	g.manifest.Updated = now
	g.manifest.Message = message
	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// Status compares each tracked entry against the current filesystem state
// and returns a status for each.
func (g *Garden) Status() ([]FileStatus, error) {
	var results []FileStatus

	for i := range g.manifest.Files {
		entry := &g.manifest.Files[i]

		abs, err := ExpandTildePath(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("expanding path %s: %w", entry.Path, err)
		}

		_, err = os.Lstat(abs)
		if os.IsNotExist(err) {
			results = append(results, FileStatus{Path: entry.Path, State: "missing"})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", abs, err)
		}

		switch entry.Type {
		case "file":
			hash, err := HashFile(abs)
			if err != nil {
				return nil, fmt.Errorf("hashing %s: %w", abs, err)
			}
			if hash != entry.Hash {
				results = append(results, FileStatus{Path: entry.Path, State: "modified"})
			} else {
				results = append(results, FileStatus{Path: entry.Path, State: "ok"})
			}

		case "link":
			target, err := os.Readlink(abs)
			if err != nil {
				return nil, fmt.Errorf("reading symlink %s: %w", abs, err)
			}
			if target != entry.Target {
				results = append(results, FileStatus{Path: entry.Path, State: "modified"})
			} else {
				results = append(results, FileStatus{Path: entry.Path, State: "ok"})
			}

		case "directory":
			results = append(results, FileStatus{Path: entry.Path, State: "ok"})
		}
	}

	return results, nil
}

// Restore writes tracked files back to their original locations. If paths is
// empty, all entries are restored. If force is true, existing files are
// overwritten without prompting. Otherwise, confirm is called for files where
// the on-disk version is newer than or equal to the manifest timestamp;
// if confirm returns false, that file is skipped.
func (g *Garden) Restore(paths []string, force bool, confirm func(path string) bool) error {
	entries := g.manifest.Files
	if len(paths) > 0 {
		entries = g.filterEntries(paths)
		if len(entries) == 0 {
			return fmt.Errorf("no matching tracked entries")
		}
	}

	for i := range entries {
		entry := &entries[i]

		abs, err := ExpandTildePath(entry.Path)
		if err != nil {
			return fmt.Errorf("expanding path %s: %w", entry.Path, err)
		}

		// Check if the file exists and whether we need confirmation.
		if !force {
			if info, err := os.Lstat(abs); err == nil {
				// File exists. If on-disk mtime >= manifest updated, ask.
				// Truncate to seconds because filesystem mtime granularity
				// varies across platforms.
				diskTime := info.ModTime().Truncate(time.Second)
				entryTime := entry.Updated.Truncate(time.Second)
				if !diskTime.Before(entryTime) {
					if confirm == nil || !confirm(entry.Path) {
						continue
					}
				}
			}
		}

		// Create parent directories.
		dir := filepath.Dir(abs)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}

		switch entry.Type {
		case "file":
			if err := g.restoreFile(abs, entry); err != nil {
				return err
			}

		case "link":
			if err := restoreLink(abs, entry); err != nil {
				return err
			}

		case "directory":
			mode, err := parseMode(entry.Mode)
			if err != nil {
				return fmt.Errorf("parsing mode for %s: %w", entry.Path, err)
			}
			if err := os.MkdirAll(abs, mode); err != nil {
				return fmt.Errorf("creating directory %s: %w", abs, err)
			}
		}
	}

	return nil
}

func (g *Garden) restoreFile(abs string, entry *manifest.Entry) error {
	data, err := g.store.Read(entry.Hash)
	if err != nil {
		return fmt.Errorf("reading blob for %s: %w", entry.Path, err)
	}

	mode, err := parseMode(entry.Mode)
	if err != nil {
		return fmt.Errorf("parsing mode for %s: %w", entry.Path, err)
	}

	if err := os.WriteFile(abs, data, mode); err != nil {
		return fmt.Errorf("writing %s: %w", abs, err)
	}

	return nil
}

func restoreLink(abs string, entry *manifest.Entry) error {
	// Remove existing file/link at the target path so we can create the symlink.
	_ = os.Remove(abs)

	if err := os.Symlink(entry.Target, abs); err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", abs, entry.Target, err)
	}
	return nil
}

// filterEntries returns manifest entries whose tilde paths match any of the
// given paths (resolved to tilde form).
func (g *Garden) filterEntries(paths []string) []manifest.Entry {
	wanted := make(map[string]bool)
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			wanted[p] = true
			continue
		}
		wanted[toTildePath(abs)] = true
	}

	var result []manifest.Entry
	for _, e := range g.manifest.Files {
		if wanted[e.Path] {
			result = append(result, e)
		}
	}
	return result
}

// parseMode converts a mode string like "0644" to an os.FileMode.
func parseMode(s string) (os.FileMode, error) {
	if s == "" {
		return 0o644, nil
	}
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode %q: %w", s, err)
	}
	return os.FileMode(v), nil
}

// findEntry returns the entry for the given tilde path, or nil if not found.
func (g *Garden) findEntry(tildePath string) *manifest.Entry {
	for i := range g.manifest.Files {
		if g.manifest.Files[i].Path == tildePath {
			return &g.manifest.Files[i]
		}
	}
	return nil
}

// toTildePath converts an absolute path to a ~/... path if it falls under
// the user's home directory. Otherwise returns the path unchanged.
func toTildePath(abs string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return abs
	}
	// Ensure trailing separator for prefix matching.
	homePrefix := home + string(filepath.Separator)
	if abs == home {
		return "~"
	}
	if strings.HasPrefix(abs, homePrefix) {
		return "~/" + abs[len(homePrefix):]
	}
	return abs
}

// ExpandTildePath converts a ~/... path to an absolute path using the
// current user's home directory. Non-tilde paths are returned unchanged.
func ExpandTildePath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding ~: %w", err)
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
