package garden

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// VerifyResult reports the integrity of a single tracked blob.
type VerifyResult struct {
	Path   string // tilde path from manifest
	OK     bool
	Detail string // e.g. "ok", "hash mismatch", "blob missing"
}

// Verify checks every file entry in the manifest against the blob store.
// It confirms that the blob exists and that its content still matches
// the recorded hash. Directories and symlinks are skipped because they
// have no blobs.
func (g *Garden) Verify() ([]VerifyResult, error) {
	var results []VerifyResult

	for _, entry := range g.manifest.Files {
		if entry.Type != "file" {
			continue
		}

		if !g.store.Exists(entry.Hash) {
			results = append(results, VerifyResult{
				Path:   entry.Path,
				OK:     false,
				Detail: "blob missing",
			})
			continue
		}

		data, err := g.store.Read(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("reading blob for %s: %w", entry.Path, err)
		}

		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])

		if got != entry.Hash {
			results = append(results, VerifyResult{
				Path:   entry.Path,
				OK:     false,
				Detail: "hash mismatch",
			})
		} else {
			results = append(results, VerifyResult{
				Path:   entry.Path,
				OK:     true,
				Detail: "ok",
			})
		}
	}

	return results, nil
}
