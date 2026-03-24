// Package garden is the core business logic for sgard. It orchestrates the
// manifest and blob store to implement dotfile management operations.
package garden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kisom/sgard/manifest"
	"github.com/kisom/sgard/store"
)

// Garden ties a manifest and blob store together to manage dotfiles.
type Garden struct {
	manifest     *manifest.Manifest
	store        *store.Store
	root         string // repository root directory
	manifestPath string // path to manifest.yaml
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

	m := manifest.New()
	if err := m.Save(manifestPath); err != nil {
		return nil, fmt.Errorf("saving initial manifest: %w", err)
	}

	return &Garden{
		manifest:     m,
		store:        s,
		root:         absRoot,
		manifestPath: manifestPath,
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
	}, nil
}

// Add tracks new files, directories, or symlinks. Each path is resolved
// to an absolute path, inspected for its type, and added to the manifest.
// Regular files are hashed and stored in the blob store.
func (g *Garden) Add(paths []string) error {
	now := time.Now().UTC()

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		info, err := os.Lstat(abs)
		if err != nil {
			return fmt.Errorf("stat %s: %w", abs, err)
		}

		tilded := toTildePath(abs)

		// Check if already tracked.
		if g.findEntry(tilded) != nil {
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

		case info.IsDir():
			entry.Type = "directory"

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
	}

	g.manifest.Updated = now
	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
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
