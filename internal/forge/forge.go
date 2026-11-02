// Package forge parses git remote URLs into forge coordinates (host, owner, repo).
package forge

import (
	"regexp"
	"strings"
)

var (
	sshRemoteRegex   = regexp.MustCompile(`(?i)^(?:ssh://)?(?:[^@]+@)?([^:/]+)[:/]([^/]+)/([^/]+?)(?:\.git)?$`)
	httpsRemoteRegex = regexp.MustCompile(`(?i)^https?://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?$`)
)

// ParseRemoteURL extracts the host, owner and repository from an ssh or https git remote URL.
// It reports ok=false for any other shape.
func ParseRemoteURL(raw string) (host, owner, repo string, ok bool) {
	raw = strings.TrimSpace(raw)
	if m := sshRemoteRegex.FindStringSubmatch(raw); m != nil {
		return m[1], m[2], m[3], true
	}
	if m := httpsRemoteRegex.FindStringSubmatch(raw); m != nil {
		return m[1], m[2], m[3], true
	}
	return "", "", "", false
}
