// Package runstate resolves Harnez paths inside an externally confined Taia
// runner. The directory checks are defense in depth, not a same-UID boundary.
package runstate

import (
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/sessioncontract"
)

// RunnerDirEnv returns the runner session directory environment variable name
// declared by the embedded session isolation contract.
func RunnerDirEnv() string {
	return sessioncontract.SessionDirEnv()
}

// RunnerDir returns the active runner confinement directory, or "" on the host.
func RunnerDir() string {
	env := RunnerDirEnv()
	if env == "" {
		return ""
	}
	return os.Getenv(env)
}

// InRunnerMode reports whether the process is running inside a Taia runner
// session directory declared by the session isolation contract.
func InRunnerMode() bool {
	return RunnerDir() != ""
}

// SessionStateDir returns the runner-local session state directory, or
// legacyDir when Harnez is not running under a Taia runner.
func SessionStateDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.SessionState })
}

// DefaultSessionStateDir computes the runner-local session state directory path
// without creating directories on disk, falling back to legacyDir on host.
func DefaultSessionStateDir(legacyDir string) string {
	contract, err := sessioncontract.Load()
	if err != nil {
		return legacyDir
	}
	root := os.Getenv(contract.SessionDirEnv)
	if root == "" {
		return legacyDir
	}
	parts := append([]string{root}, contract.RunnerPaths.SessionState...)
	return filepath.Join(parts...)
}

// ProcessRecordDir returns the runner-local process record directory, or
// legacyDir when Harnez is not running under a Taia runner.
func ProcessRecordDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.ProcessRecords })
}

func privatePath(envName, root string, parts ...string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", fmt.Errorf("%s must be an absolute, clean path", envName)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", envName, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return "", fmt.Errorf("%s must be a real directory with mode 0700", envName)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		return "", fmt.Errorf("%s must not traverse symlinks", envName)
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
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.UsageState })
}

// TelemetrySpoolDir returns the private runner spool or the host directory.
func TelemetrySpoolDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.TelemetrySpool })
}

// AGYMeterDir returns the runner-local meter directory or the host directory.
func AGYMeterDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.AGYMeter })
}

// LogDir returns the runner-local debug log directory or the host directory.
func LogDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.Logs })
}

// FeedbackDir returns the runner-local feedback directory or the host directory.
func FeedbackDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.Feedback })
}

// BenchDir returns the runner-local benchmark directory or the host directory.
func BenchDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.Bench })
}

// PolicyDir returns the runner-local policy overlay directory or the host directory.
func PolicyDir(legacyDir string) (string, error) {
	return scopedContractDir(legacyDir, func(p sessioncontract.RunnerPathsContract) []string { return p.Policy })
}

func scopedContractDir(legacyDir string, selectParts func(sessioncontract.RunnerPathsContract) []string) (string, error) {
	contract, err := sessioncontract.Load()
	if err != nil {
		return "", err
	}
	root := os.Getenv(contract.SessionDirEnv)
	if root == "" {
		return legacyDir, nil
	}
	return privatePath(contract.SessionDirEnv, root, selectParts(contract.RunnerPaths)...)
}
