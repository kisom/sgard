package garden

import "fmt"

// SetTargeting updates the Only/Never fields on an existing manifest entry.
// If clear is true, both fields are reset to nil.
func (g *Garden) SetTargeting(path string, only, never []string, clear bool) error {
	abs, err := ExpandTildePath(path)
	if err != nil {
		return fmt.Errorf("expanding path: %w", err)
	}
	tilded := toTildePath(abs)

	entry := g.findEntry(tilded)
	if entry == nil {
		return fmt.Errorf("not tracking %s", tilded)
	}

	if clear {
		entry.Only = nil
		entry.Never = nil
	} else {
		if len(only) > 0 {
			entry.Only = only
			entry.Never = nil
		}
		if len(never) > 0 {
			entry.Never = never
			entry.Only = nil
		}
	}

	return g.manifest.Save(g.manifestPath)
}
