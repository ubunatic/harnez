// Package sshutil shares non-interactive SSH and SCP connection options.
package sshutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
