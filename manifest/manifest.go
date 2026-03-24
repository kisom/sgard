package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Entry represents a single tracked file, directory, or symlink.
type Entry struct {
	Path    string    `yaml:"path"`
	Hash    string    `yaml:"hash,omitempty"`
	Type    string    `yaml:"type"`
	Mode    string    `yaml:"mode,omitempty"`
	Target  string    `yaml:"target,omitempty"`
	Updated time.Time `yaml:"updated"`
}

// Manifest is the top-level manifest describing all tracked entries.
type Manifest struct {
	Version int       `yaml:"version"`
	Created time.Time `yaml:"created"`
	Updated time.Time `yaml:"updated"`
	Message string    `yaml:"message,omitempty"`
	Files   []Entry   `yaml:"files"`
}

// New creates a new empty manifest with Version 1 and timestamps set to now.
func New() *Manifest {
	now := time.Now().UTC()
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
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
