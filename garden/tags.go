package garden

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadTags reads the tags from <repo>/tags, one per line.
func (g *Garden) LoadTags() []string {
	data, err := os.ReadFile(filepath.Join(g.root, "tags"))
	if err != nil {
		return nil
	}

	var tags []string
	for _, line := range strings.Split(string(data), "\n") {
		tag := strings.TrimSpace(line)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// SaveTag adds a tag to <repo>/tags if not already present.
func (g *Garden) SaveTag(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}

	tags := g.LoadTags()
	for _, existing := range tags {
		if existing == tag {
			return nil // already present
		}
	}

	tags = append(tags, tag)
	return g.writeTags(tags)
}

// RemoveTag removes a tag from <repo>/tags.
func (g *Garden) RemoveTag(tag string) error {
	tag = strings.TrimSpace(tag)
	tags := g.LoadTags()

	var filtered []string
	for _, t := range tags {
		if t != tag {
			filtered = append(filtered, t)
		}
	}

	return g.writeTags(filtered)
}

func (g *Garden) writeTags(tags []string) error {
	content := strings.Join(tags, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(filepath.Join(g.root, "tags"), []byte(content), 0o644)
}
