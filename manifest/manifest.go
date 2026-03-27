package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Entry represents a single tracked file, directory, or symlink.
type Entry struct {
	Path          string    `yaml:"path"`
	Hash          string    `yaml:"hash,omitempty"`
	PlaintextHash string    `yaml:"plaintext_hash,omitempty"`
	Encrypted     bool      `yaml:"encrypted,omitempty"`
	Locked        bool      `yaml:"locked,omitempty"`
	Type          string    `yaml:"type"`
	Mode          string    `yaml:"mode,omitempty"`
	Target        string    `yaml:"target,omitempty"`
	Updated       time.Time `yaml:"updated"`
	Only          []string  `yaml:"only,omitempty"`
	Never         []string  `yaml:"never,omitempty"`
}

// KekSlot describes a single KEK source that can unwrap the DEK.
type KekSlot struct {
	Type         string `yaml:"type"`                    // "passphrase" or "fido2"
	Argon2Time   int    `yaml:"argon2_time,omitempty"`   // passphrase only
	Argon2Memory int    `yaml:"argon2_memory,omitempty"` // passphrase only (KiB)
	Argon2Threads int   `yaml:"argon2_threads,omitempty"` // passphrase only
	CredentialID string `yaml:"credential_id,omitempty"` // fido2 only (base64)
	Salt         string `yaml:"salt"`                    // base64-encoded
	WrappedDEK   string `yaml:"wrapped_dek"`             // base64-encoded
}

// Encryption holds the encryption configuration embedded in the manifest.
type Encryption struct {
	Algorithm string              `yaml:"algorithm"`
	KekSlots  map[string]*KekSlot `yaml:"kek_slots"`
}

// Manifest is the top-level manifest describing all tracked entries.
type Manifest struct {
	Version    int         `yaml:"version"`
	Created    time.Time   `yaml:"created"`
	Updated    time.Time   `yaml:"updated"`
	Message    string      `yaml:"message,omitempty"`
	Files      []Entry     `yaml:"files"`
	Exclude    []string    `yaml:"exclude,omitempty"`
	Encryption *Encryption `yaml:"encryption,omitempty"`
}

// IsExcluded reports whether the given tilde path should be excluded from
// tracking. A path is excluded if it matches an exclude entry exactly, or
// if it falls under an excluded directory (an exclude entry that is a prefix
// followed by a path separator).
func (m *Manifest) IsExcluded(tildePath string) bool {
	for _, ex := range m.Exclude {
		if tildePath == ex {
			return true
		}
		// Directory exclusion: if the exclude entry is a prefix of the
		// path with a separator boundary, the path is under that directory.
		prefix := ex
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if strings.HasPrefix(tildePath, prefix) {
			return true
		}
	}
	return false
}

// New creates a new empty manifest with Version 1 and timestamps set to now.
func New() *Manifest {
	return NewWithTime(time.Now().UTC())
}

// NewWithTime creates a new empty manifest with the given timestamp.
func NewWithTime(now time.Time) *Manifest {
	return &Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Files:   []Entry{},
	}
}

// Load reads a manifest from the given file path.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &m, nil
}

// Save writes the manifest to the given file path using an atomic
// write: data is marshalled to a temporary file in the same directory,
// then renamed to the target path. This prevents corruption if the
// process crashes mid-write.
func (m *Manifest) Save(path string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
