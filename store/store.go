// Package store implements a content-addressable blob store keyed by SHA-256 hash.
//
// Blobs are stored in a two-level directory structure under a blobs/ subdirectory:
//
//	blobs/<first 2 hex chars>/<next 2 hex chars>/<full 64-char hash>
//
// The store only handles raw bytes. It does not know about files, paths, or
// permissions — that is the garden package's job.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// validHash reports whether s is a 64-character lowercase hex string.
func validHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// Store is a content-addressable blob store rooted at a directory on disk.
type Store struct {
	root string
}

// New creates a Store rooted at root. It ensures the blobs/ subdirectory
// exists, creating it (and any parents) if needed.
func New(root string) (*Store, error) {
	blobsDir := filepath.Join(root, "blobs")
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create blobs directory: %w", err)
	}
	return &Store{root: root}, nil
}

// Write computes the SHA-256 hash of data, writes the blob to disk, and
// returns the hex-encoded hash. If a blob with the same hash already exists,
// this is a no-op (deduplication). Writes are atomic: data is written to a
// temporary file first, then renamed into place.
func (s *Store) Write(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	p := s.blobPath(hash)

	// Deduplication: if the blob already exists, nothing to do.
	if _, err := os.Stat(p); err == nil {
		return hash, nil
	}

	// Ensure the parent directory exists.
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("store: create blob directory: %w", err)
	}

	// Write to a temp file in the same directory, then rename for atomicity.
	tmp, err := os.CreateTemp(dir, ".blob-*")
	if err != nil {
		return "", fmt.Errorf("store: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("store: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("store: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("store: rename blob into place: %w", err)
	}

	return hash, nil
}

// Read returns the blob contents for the given hash. It returns an error if
// the hash is malformed or the blob does not exist.
func (s *Store) Read(hash string) ([]byte, error) {
	if !validHash(hash) {
		return nil, fmt.Errorf("store: invalid hash %q", hash)
	}

	data, err := os.ReadFile(s.blobPath(hash))
	if err != nil {
		return nil, fmt.Errorf("store: read blob %s: %w", hash, err)
	}
	return data, nil
}

// Exists reports whether a blob with the given hash is present in the store.
// It returns false for malformed hashes.
func (s *Store) Exists(hash string) bool {
	if !validHash(hash) {
		return false
	}
	_, err := os.Stat(s.blobPath(hash))
	return err == nil
}

// Delete removes the blob file for the given hash. It returns an error if the
// hash is malformed or the blob does not exist.
func (s *Store) Delete(hash string) error {
	if !validHash(hash) {
		return fmt.Errorf("store: invalid hash %q", hash)
	}

	if err := os.Remove(s.blobPath(hash)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("store: blob %s does not exist: %w", hash, err)
		}
		return fmt.Errorf("store: delete blob %s: %w", hash, err)
	}
	return nil
}

// blobPath returns the filesystem path for a blob with the given hash.
// Layout: blobs/<first 2 hex chars>/<next 2 hex chars>/<full 64-char hash>
func (s *Store) blobPath(hash string) string {
	return filepath.Join(s.root, "blobs", hash[:2], hash[2:4], hash)
}
