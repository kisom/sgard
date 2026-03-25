package garden

import (
	"fmt"
	"strings"

	"github.com/kisom/sgard/manifest"
)

// EntryApplies reports whether the given entry should be active on a
// machine with the given labels. Returns an error if both Only and
// Never are set on the same entry.
func EntryApplies(entry *manifest.Entry, labels []string) (bool, error) {
	if len(entry.Only) > 0 && len(entry.Never) > 0 {
		return false, fmt.Errorf("entry %s has both only and never set", entry.Path)
	}

	if len(entry.Only) > 0 {
		for _, matcher := range entry.Only {
			if matchesLabel(matcher, labels) {
				return true, nil
			}
		}
		return false, nil
	}

	if len(entry.Never) > 0 {
		for _, matcher := range entry.Never {
			if matchesLabel(matcher, labels) {
				return false, nil
			}
		}
	}

	return true, nil
}

// matchesLabel checks if a matcher string matches any label in the set.
// Matching is case-insensitive.
func matchesLabel(matcher string, labels []string) bool {
	matcher = strings.ToLower(matcher)
	for _, label := range labels {
		if strings.ToLower(label) == matcher {
			return true
		}
	}
	return false
}
