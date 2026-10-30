package usage

import (
	"os"
	"os/exec"
	"path/filepath"
)

// resolveExecutable looks for name on PATH, then in common per-user bin dirs.
func resolveExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}
	originalErr := err

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", originalErr
	}
	for _, dir := range []string{".local/bin", "go/bin", ".cargo/bin"} {
		candidate := filepath.Join(home, dir, name)
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			continue
		}
		absolute, absErr := filepath.Abs(candidate)
		if absErr == nil {
			return absolute, nil
		}
	}
	return "", originalErr
}
