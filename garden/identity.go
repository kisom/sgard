package garden

import (
	"os"
	"runtime"
	"strings"
)

// Identity returns the machine's label set: short hostname, os:<GOOS>,
// arch:<GOARCH>, and tag:<name> for each tag in <repo>/tags.
func (g *Garden) Identity() []string {
	labels := []string{
		shortHostname(),
		"os:" + runtime.GOOS,
		"arch:" + runtime.GOARCH,
	}

	tags := g.LoadTags()
	for _, tag := range tags {
		labels = append(labels, "tag:"+tag)
	}

	return labels
}

// shortHostname returns the hostname before the first dot, lowercased.
func shortHostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	host = strings.ToLower(host)
	if idx := strings.IndexByte(host, '.'); idx >= 0 {
		host = host[:idx]
	}
	return host
}
