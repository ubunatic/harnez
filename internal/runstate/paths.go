// Package runstate resolves Harnez paths inside an externally confined Taia
// runner. The directory checks are defense in depth, not a same-UID boundary.
package runstate

import (
	"fmt"
	"os"
	"path/filepath"
)

const runnerDirEnv = "TAIA_SESSION_DIR"

// SessionStateDir returns the runner-local session state directory, or
// legacyDir when Harnez is not running under a Taia runner.
func SessionStateDir(legacyDir string) (string, error) {
	return scopedDir(legacyDir, "state", "sessions")
}

// ProcessRecordDir returns the runner-local process record directory, or
// legacyDir when Harnez is not running under a Taia runner.
func ProcessRecordDir(legacyDir string) (string, error) {
	return scopedDir(legacyDir, "runtime", "procs")
}

func privatePath(root string, parts ...string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", fmt.Errorf("%s must be an absolute, clean path", runnerDirEnv)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", runnerDirEnv, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return "", fmt.Errorf("%s must be a real directory with mode 0700", runnerDirEnv)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		return "", fmt.Errorf("%s must not traverse symlinks", runnerDirEnv)
	}

	path := root
	for _, part := range parts {
		path = filepath.Join(path, part)
		if err := os.Mkdir(path, 0o700); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("create runner state directory %s: %w", path, err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", fmt.Errorf("inspect runner state directory %s: %w", path, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
			return "", fmt.Errorf("runner state path %s must be a real directory with mode 0700", path)
		}
	}
	return path, nil
}

// UsageStateDir returns the private runner usage directory or the legacy directory.
func UsageStateDir(legacyDir string) (string, error) {
	return scopedDir(legacyDir, "state", "usage")
}

// TelemetrySpoolDir returns the private runner spool or the host directory.
func TelemetrySpoolDir(legacyDir string) (string, error) {
	return scopedDir(legacyDir, "state", "telemetry-spool")
}

// AGYMeterDir returns the runner-local meter directory or the host directory.
func AGYMeterDir(legacyDir string) (string, error) {
	return scopedDir(legacyDir, "state", "agymeter")
}

// LogDir returns the runner-local debug log directory or the host directory.
func LogDir(legacyDir string) (string, error) { return scopedDir(legacyDir, "state", "logs") }

// FeedbackDir returns the runner-local feedback directory or the host directory.
func FeedbackDir(legacyDir string) (string, error) { return scopedDir(legacyDir, "state", "feedback") }

// BenchDir returns the runner-local benchmark directory or the host directory.
func BenchDir(legacyDir string) (string, error) { return scopedDir(legacyDir, "state", "bench") }

// PolicyDir returns the runner-local policy overlay directory or the host directory.
func PolicyDir(legacyDir string) (string, error) { return scopedDir(legacyDir, "state", "policy") }

func scopedDir(legacyDir string, parts ...string) (string, error) {
	root := os.Getenv(runnerDirEnv)
	if root == "" {
		return legacyDir, nil
	}
	return privatePath(root, parts...)
}
