package garden

import (
	"fmt"
	"os"
	"strings"
)

// FileInfo holds detailed information about a single tracked entry.
type FileInfo struct {
	Path          string   // tilde path from manifest
	Type          string   // "file", "link", or "directory"
	State         string   // "ok", "modified", "drifted", "missing", "skipped"
	Mode          string   // octal file mode from manifest
	Hash          string   // blob hash from manifest (files only)
	PlaintextHash string   // plaintext hash (encrypted files only)
	CurrentHash   string   // SHA-256 of current file on disk (files only, empty if missing)
	Encrypted     bool
	Locked        bool
	Updated       string   // manifest timestamp (RFC 3339)
	DiskModTime   string   // filesystem modification time (RFC 3339, empty if missing)
	Target        string   // symlink target (links only)
	CurrentTarget string   // current symlink target on disk (links only, empty if missing)
	Only          []string // targeting: only these labels
	Never         []string // targeting: never these labels
	BlobStored    bool     // whether the blob exists in the store
}

// Info returns detailed information about a tracked file.
func (g *Garden) Info(path string) (*FileInfo, error) {
	abs, err := resolvePath(path)
	if err != nil {
		return nil, err
	}
	tilded := toTildePath(abs)

	entry := g.findEntry(tilded)
	if entry == nil {
		// Also try the path as given (it might already be a tilde path).
		entry = g.findEntry(path)
		if entry == nil {
			return nil, fmt.Errorf("not tracked: %s", path)
		}
	}

	fi := &FileInfo{
		Path:          entry.Path,
		Type:          entry.Type,
		Mode:          entry.Mode,
		Hash:          entry.Hash,
		PlaintextHash: entry.PlaintextHash,
		Encrypted:     entry.Encrypted,
		Locked:        entry.Locked,
		Target:        entry.Target,
		Only:          entry.Only,
		Never:         entry.Never,
	}

	if !entry.Updated.IsZero() {
		fi.Updated = entry.Updated.Format("2006-01-02 15:04:05 UTC")
	}

	// Check blob existence for files.
	if entry.Type == "file" && entry.Hash != "" {
		fi.BlobStored = g.store.Exists(entry.Hash)
	}

	// Determine state and filesystem info.
	labels := g.Identity()
	applies, err := EntryApplies(entry, labels)
	if err != nil {
		return nil, err
	}
	if !applies {
		fi.State = "skipped"
		return fi, nil
	}

	entryAbs, err := ExpandTildePath(entry.Path)
	if err != nil {
		return nil, fmt.Errorf("expanding path %s: %w", entry.Path, err)
	}

	info, err := os.Lstat(entryAbs)
	if os.IsNotExist(err) {
		fi.State = "missing"
		return fi, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", entryAbs, err)
	}

	fi.DiskModTime = info.ModTime().UTC().Format("2006-01-02 15:04:05 UTC")

	switch entry.Type {
	case "file":
		hash, err := HashFile(entryAbs)
		if err != nil {
			return nil, fmt.Errorf("hashing %s: %w", entryAbs, err)
		}
		fi.CurrentHash = hash

		compareHash := entry.Hash
		if entry.Encrypted && entry.PlaintextHash != "" {
			compareHash = entry.PlaintextHash
		}
		if hash != compareHash {
			if entry.Locked {
				fi.State = "drifted"
			} else {
				fi.State = "modified"
			}
		} else {
			fi.State = "ok"
		}

	case "link":
		target, err := os.Readlink(entryAbs)
		if err != nil {
			return nil, fmt.Errorf("reading symlink %s: %w", entryAbs, err)
		}
		fi.CurrentTarget = target
		if target != entry.Target {
			fi.State = "modified"
		} else {
			fi.State = "ok"
		}

	case "directory":
		fi.State = "ok"
	}

	return fi, nil
}

// resolvePath resolves a user-provided path to an absolute path, handling
// tilde expansion and relative paths.
func resolvePath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		return ExpandTildePath(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// If it looks like a tilde path already, just expand it.
	if strings.HasPrefix(path, home) {
		return path, nil
	}
	abs, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, "/") {
		path = abs + "/" + path
	}
	return path, nil
}
