package garden

import "github.com/kisom/sgard/manifest"

// List returns all tracked entries from the manifest.
func (g *Garden) List() []manifest.Entry {
	return g.manifest.Files
}
