package garden

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Diff returns a unified-style diff between the stored blob and the current
// on-disk content for the file at path. If the file is unchanged, it returns
// an empty string. Only regular files (type "file") can be diffed.
func (g *Garden) Diff(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	tilded := toTildePath(abs)

	entry := g.findEntry(tilded)
	if entry == nil {
		return "", fmt.Errorf("not tracked: %s", tilded)
	}

	if entry.Type != "file" {
		return "", fmt.Errorf("cannot diff entry of type %q (only files)", entry.Type)
	}

	stored, err := g.store.Read(entry.Hash)
	if err != nil {
		return "", fmt.Errorf("reading stored blob: %w", err)
	}

	current, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("reading current file: %w", err)
	}

	if bytes.Equal(stored, current) {
		return "", nil
	}

	oldLines := splitLines(string(stored))
	newLines := splitLines(string(current))

	return simpleDiff(tilded+" (stored)", tilded+" (current)", oldLines, newLines), nil
}

// splitLines splits s into lines, preserving the trailing empty element if s
// ends with a newline so that the diff output is accurate.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.SplitAfter(s, "\n")
}

// simpleDiff produces a minimal unified-style diff header followed by removed
// and added lines. It walks both slices in lockstep, emitting unchanged lines
// as context and changed lines with -/+ prefixes.
func simpleDiff(oldName, newName string, oldLines, newLines []string) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "--- %s\n", oldName)
	fmt.Fprintf(&buf, "+++ %s\n", newName)

	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			fmt.Fprintf(&buf, " %s", oldLines[i])
			i++
			j++
		} else {
			fmt.Fprintf(&buf, "-%s", oldLines[i])
			fmt.Fprintf(&buf, "+%s", newLines[j])
			i++
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		fmt.Fprintf(&buf, "-%s", oldLines[i])
	}
	for ; j < len(newLines); j++ {
		fmt.Fprintf(&buf, "+%s", newLines[j])
	}

	return buf.String()
}
