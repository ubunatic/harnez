package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome expands leading ~/ into the user's home directory.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// ContractHome replaces home directory prefix with ~/.
func ContractHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home+"/") {
		return "~/" + path[len(home)+1:]
	}
	return path
}

// EnsureSymlink ensures a symlink exists at linkPath pointing to target.
// It will resolve target to a relative path from the linkPath's parent directory if possible.
// Returns changed = true if the symlink was created or updated.
func EnsureSymlink(linkPath, target string) (changed bool, err error) {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return false, err
	}
	if rel, err := filepath.Rel(filepath.Dir(linkPath), target); err == nil {
		target = rel
	}
	existing, err := os.Readlink(linkPath)
	if err == nil && existing == target {
		return false, nil
	}
	os.Remove(linkPath) //nolint:errcheck
	if err := os.Symlink(target, linkPath); err != nil {
		return false, err
	}
	return true, nil
}

// WriteIfChanged writes data to dst only if the content differs.
// Replaces symlinks with real files unconditionally.
// Returns changed = true if the file was written.
func WriteIfChanged(dst string, data []byte) (changed bool, err error) {
	if fi, err := os.Lstat(dst); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			os.Remove(dst) //nolint:errcheck
		} else if existing, err := os.ReadFile(dst); err == nil && bytes.Equal(existing, data) {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return false, err
	}
	return true, nil
}

// Copy copies a file from src to dst, writing only if content differs.
func Copy(src, dst string) (changed bool, err error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return false, err
	}
	return WriteIfChanged(dst, data)
}
