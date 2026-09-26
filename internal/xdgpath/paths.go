// Package xdgpath resolves Harnez's XDG data and cache locations.
package xdgpath

import (
	"os"
	"path/filepath"
)

func DataHome() string  { return home("XDG_DATA_HOME", ".local/share") }
func CacheHome() string { return home("XDG_CACHE_HOME", ".cache") }

func home(variable, fallback string) string {
	if value := os.Getenv(variable); value != "" && filepath.IsAbs(value) {
		return value
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, filepath.FromSlash(fallback))
}
