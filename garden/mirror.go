package garden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MirrorUp synchronises the manifest with the current filesystem state for
// each given directory path. New files/symlinks are added, deleted files are
// removed from the manifest, and changed files are re-hashed.
func (g *Garden) MirrorUp(paths []string) error {
	now := g.clock.Now().UTC()

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		tildePrefix := toTildePath(abs)
		// Ensure we match entries *under* the directory, not just the dir itself.
		if !strings.HasSuffix(tildePrefix, "/") {
			tildePrefix += "/"
		}

		// 1. Walk the directory and add any new files/symlinks.
		err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			fi, lstatErr := os.Lstat(path)
			if lstatErr != nil {
				return fmt.Errorf("stat %s: %w", path, lstatErr)
			}
			return g.addEntry(path, fi, now, true)
		})
		if err != nil {
			return fmt.Errorf("walking directory %s: %w", abs, err)
		}

		// 2. Remove manifest entries whose files no longer exist on disk.
		kept := g.manifest.Files[:0]
		for _, e := range g.manifest.Files {
			if strings.HasPrefix(e.Path, tildePrefix) {
				expanded, err := ExpandTildePath(e.Path)
				if err != nil {
					return fmt.Errorf("expanding path %s: %w", e.Path, err)
				}
				if _, err := os.Lstat(expanded); err != nil {
					// File no longer exists — drop entry.
					continue
				}
			}
			kept = append(kept, e)
		}
		g.manifest.Files = kept

		// 3. Re-hash remaining file entries under the prefix (like Checkpoint).
		for i := range g.manifest.Files {
			entry := &g.manifest.Files[i]
			if !strings.HasPrefix(entry.Path, tildePrefix) {
				continue
			}
			if entry.Type != "file" {
				continue
			}

			expanded, err := ExpandTildePath(entry.Path)
			if err != nil {
				return fmt.Errorf("expanding path %s: %w", entry.Path, err)
			}

			data, err := os.ReadFile(expanded)
			if err != nil {
				return fmt.Errorf("reading %s: %w", expanded, err)
			}
			hash, err := g.store.Write(data)
			if err != nil {
				return fmt.Errorf("storing blob for %s: %w", expanded, err)
			}
			if hash != entry.Hash {
				entry.Hash = hash
				entry.Updated = now
			}
		}
	}

	g.manifest.Updated = now
	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// MirrorDown synchronises the filesystem with the manifest for each given
// directory path. Tracked entries are restored and untracked files on disk
// are deleted. If force is false, confirm is called before each deletion;
// a false return skips that file.
func (g *Garden) MirrorDown(paths []string, force bool, confirm func(string) bool) error {
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		tildePrefix := toTildePath(abs)
		if !strings.HasSuffix(tildePrefix, "/") {
			tildePrefix += "/"
		}

		// 1. Collect manifest entries under this prefix.
		tracked := make(map[string]bool)
		for i := range g.manifest.Files {
			entry := &g.manifest.Files[i]
			if !strings.HasPrefix(entry.Path, tildePrefix) {
				continue
			}

			expanded, err := ExpandTildePath(entry.Path)
			if err != nil {
				return fmt.Errorf("expanding path %s: %w", entry.Path, err)
			}
			tracked[expanded] = true

			// Create parent directories.
			dir := filepath.Dir(expanded)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}

			// Restore the entry.
			switch entry.Type {
			case "file":
				if err := g.restoreFile(expanded, entry); err != nil {
					return err
				}
			case "link":
				if err := restoreLink(expanded, entry); err != nil {
					return err
				}
			}
		}

		// 2. Walk disk and delete files not in manifest.
		var emptyDirs []string
		err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				// Collect directories for potential cleanup (post-order).
				if path != abs {
					emptyDirs = append(emptyDirs, path)
				}
				return nil
			}
			if tracked[path] {
				return nil
			}
			// Untracked file/symlink on disk.
			if !force {
				if confirm == nil || !confirm(path) {
					return nil
				}
			}
			_ = os.Remove(path)
			return nil
		})
		if err != nil {
			return fmt.Errorf("walking directory %s: %w", abs, err)
		}

		// 3. Clean up empty directories (reverse order so children come first).
		for i := len(emptyDirs) - 1; i >= 0; i-- {
			// os.Remove only removes empty directories.
			_ = os.Remove(emptyDirs[i])
		}
	}

	return nil
}
