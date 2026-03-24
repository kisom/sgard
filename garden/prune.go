package garden

import "fmt"

// Prune removes orphaned blobs that are not referenced by any manifest entry.
// Returns the number of blobs removed.
func (g *Garden) Prune() (int, error) {
	referenced := make(map[string]bool)
	for _, e := range g.manifest.Files {
		if e.Type == "file" && e.Hash != "" {
			referenced[e.Hash] = true
		}
	}

	allBlobs, err := g.store.List()
	if err != nil {
		return 0, fmt.Errorf("listing blobs: %w", err)
	}

	removed := 0
	for _, hash := range allBlobs {
		if !referenced[hash] {
			if err := g.store.Delete(hash); err != nil {
				return removed, fmt.Errorf("deleting blob %s: %w", hash, err)
			}
			removed++
		}
	}

	return removed, nil
}
