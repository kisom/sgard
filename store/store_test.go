package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("hello, sgard")
	hash, err := s.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read(hash)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("Read returned %q, want %q", got, data)
	}
}

func TestHashCorrectness(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("known test data")
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])

	got, err := s.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got != want {
		t.Errorf("Write returned hash %q, want %q", got, want)
	}
}

func TestDeduplication(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("duplicate me")
	hash1, err := s.Write(data)
	if err != nil {
		t.Fatalf("first Write: %v", err)
	}

	hash2, err := s.Write(data)
	if err != nil {
		t.Fatalf("second Write: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("hashes differ: %q vs %q", hash1, hash2)
	}

	// Verify only one blob file exists on disk at the expected path.
	p := s.blobPath(hash1)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat blob: %v", err)
	}
	if info.IsDir() {
		t.Fatal("blob path is a directory, not a file")
	}

	// Count files in the leaf directory — should be exactly one.
	dir := filepath.Dir(p)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var count int
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 blob file in %s, found %d", dir, count)
	}
}

func TestExists(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("existence check")
	hash, err := s.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !s.Exists(hash) {
		t.Error("Exists returned false for written blob")
	}

	fake := "0000000000000000000000000000000000000000000000000000000000000000"
	if s.Exists(fake) {
		t.Error("Exists returned true for nonexistent hash")
	}
}

func TestExistsInvalidHash(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if s.Exists("not-a-valid-hash") {
		t.Error("Exists returned true for invalid hash")
	}
}

func TestReadNonexistent(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fake := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err = s.Read(fake)
	if err == nil {
		t.Error("Read of nonexistent blob should return an error")
	}
}

func TestReadInvalidHash(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = s.Read("bad")
	if err == nil {
		t.Error("Read with invalid hash should return an error")
	}
}

func TestDelete(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("delete me")
	hash, err := s.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := s.Delete(hash); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if s.Exists(hash) {
		t.Error("Exists returned true after Delete")
	}

	if _, err := s.Read(hash); err == nil {
		t.Error("Read succeeded after Delete, expected error")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fake := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := s.Delete(fake); err == nil {
		t.Error("Delete of nonexistent blob should return an error")
	}
}

func TestDeleteInvalidHash(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := s.Delete("xyz"); err == nil {
		t.Error("Delete with invalid hash should return an error")
	}
}

func TestWriteCreatesSubdirectories(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("subdir test")
	hash, err := s.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify the two-level directory structure exists.
	level1 := filepath.Join(root, "blobs", hash[:2])
	level2 := filepath.Join(level1, hash[2:4])

	if info, err := os.Stat(level1); err != nil || !info.IsDir() {
		t.Errorf("expected directory at %s", level1)
	}
	if info, err := os.Stat(level2); err != nil || !info.IsDir() {
		t.Errorf("expected directory at %s", level2)
	}

	// And the blob file itself exists in level2.
	blobFile := filepath.Join(level2, hash)
	if _, err := os.Stat(blobFile); err != nil {
		t.Errorf("expected blob file at %s: %v", blobFile, err)
	}
}
