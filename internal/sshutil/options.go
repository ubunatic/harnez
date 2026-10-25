// Package sshutil shares non-interactive SSH and SCP connection options.
package sshutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var hostPattern = regexp.MustCompile(`^[a-zA-Z0-9_.@:-]+$`)

// ValidHost accepts SSH aliases and user@host destinations while rejecting
// option-like values and characters that can alter command parsing.
func ValidHost(host string) bool {
	return host != "" && !strings.HasPrefix(host, "-") && hostPattern.MatchString(host)
}

var installPathPattern = regexp.MustCompile(`^(~/|/)([A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+$`)

// ValidInstallPath accepts home-relative or absolute paths safe to place in
// remote shell commands. Components may not be empty, parent traversals, or
// inside temporary trees.
func ValidInstallPath(path string) bool {
	if !installPathPattern.MatchString(path) {
		return false
	}
	absolute := strings.HasPrefix(path, "/")
	components := strings.Split(strings.TrimPrefix(path, "~/"), "/")
	if absolute {
		components = strings.Split(strings.TrimPrefix(path, "/"), "/")
	}
	for _, component := range components {
		if component == ".." || component == "" {
			return false
		}
	}
	if absolute && (components[0] == "tmp" || (len(components) > 1 && components[0] == "var" && components[1] == "tmp")) {
		return false
	}
	return true
}

// Options explicitly loads the user's config when present, including in a
// systemd sandbox where SSH's implicit config discovery can skip it.
func Options(timeout time.Duration) []string {
	var args []string
	if home, err := os.UserHomeDir(); err == nil {
		config := filepath.Join(home, ".ssh", "config")
		if info, err := os.Stat(config); err == nil && info.Mode().IsRegular() {
			args = append(args, "-F", config)
		}
	}
	return append(args, "-o", "BatchMode=yes", "-o", ConnectTimeoutOption(timeout))
}

// ConnectTimeoutOption rounds a caller's timeout up to SSH's whole seconds.
func ConnectTimeoutOption(timeout time.Duration) string {
	seconds := timeout / time.Second
	if timeout%time.Second > 0 {
		seconds++
	}
	return fmt.Sprintf("ConnectTimeout=%d", seconds)
}
