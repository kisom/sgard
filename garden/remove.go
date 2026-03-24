package garden

import (
	"fmt"
	"path/filepath"
)

// Remove stops tracking the given paths. Each path is resolved to absolute
// form, converted to a tilde path, and removed from the manifest. An error
// is returned if any path is not currently tracked.
func (g *Garden) Remove(paths []string) error {
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", p, err)
		}

		tilded := toTildePath(abs)

		if g.findEntry(tilded) == nil {
			return fmt.Errorf("not tracking %s", tilded)
		}

		filtered := g.manifest.Files[:0]
		for _, e := range g.manifest.Files {
			if e.Path != tilded {
				filtered = append(filtered, e)
			}
		}
		g.manifest.Files = filtered
	}

	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}
