package garden

import (
	"fmt"
	"path/filepath"
)

// Exclude adds the given paths to the manifest's exclusion list. Excluded
// paths are skipped during Add and MirrorUp directory walks. If any of the
// paths are already tracked, they are removed from the manifest.
func (g *Garden) Exclude(paths []string) error {
	existing := make(map[string]bool, len(g.manifest.Exclude))
	for _, e := range g.manifest.Exclude {
		existing[e] = true
	}

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		tilded := toTildePath(abs)

		if existing[tilded] {
			continue
		}

		g.manifest.Exclude = append(g.manifest.Exclude, tilded)
		existing[tilded] = true

		// Remove any already-tracked entries that match this exclusion.
		g.removeExcludedEntries(tilded)
	}

	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// Include removes the given paths from the manifest's exclusion list,
// allowing them to be tracked again.
func (g *Garden) Include(paths []string) error {
	remove := make(map[string]bool, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}
		remove[toTildePath(abs)] = true
	}

	filtered := g.manifest.Exclude[:0]
	for _, e := range g.manifest.Exclude {
		if !remove[e] {
			filtered = append(filtered, e)
		}
	}
	g.manifest.Exclude = filtered

	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// removeExcludedEntries drops manifest entries that match the given
// exclusion path (exact match or under an excluded directory).
func (g *Garden) removeExcludedEntries(tildePath string) {
	kept := g.manifest.Files[:0]
	for _, e := range g.manifest.Files {
		if !g.manifest.IsExcluded(e.Path) {
			kept = append(kept, e)
		}
	}
	g.manifest.Files = kept
}
