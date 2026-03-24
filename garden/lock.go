package garden

import (
	"fmt"
	"path/filepath"
)

// Lock marks existing tracked entries as locked (repo-authoritative).
func (g *Garden) Lock(paths []string) error {
	return g.setLocked(paths, true)
}

// Unlock removes the locked flag from existing tracked entries.
func (g *Garden) Unlock(paths []string) error {
	return g.setLocked(paths, false)
}

func (g *Garden) setLocked(paths []string, locked bool) error {
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		tilded := toTildePath(abs)
		entry := g.findEntry(tilded)
		if entry == nil {
			return fmt.Errorf("not tracked: %s", tilded)
		}

		entry.Locked = locked
	}

	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}
