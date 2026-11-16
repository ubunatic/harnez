package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"ubunatic.com/harnez/internal/runstate"
	"ubunatic.com/harnez/internal/sessioncontract"
)

func hasBwrapDiagLine(lower string) bool {
	for _, line := range strings.Split(lower, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "bwrap:") {
			return true
		}
	}
	return false
}

// DiagnoseBwrapError translates bubblewrap namespace and mount-propagation
// errors into actionable host or Taia-profile prerequisite diagnostics.
func DiagnoseBwrapError(output string, taiaConfined bool) string {
	lower := strings.ToLower(output)
	if !hasBwrapDiagLine(lower) &&
		!strings.Contains(lower, "setting up uid map") &&
		!strings.Contains(lower, "failed to make / slave") &&
		!strings.Contains(lower, "create a new namespace") &&
		!strings.Contains(lower, "creating new namespace failed") {
		return ""
	}
	var hints []string
	if strings.Contains(lower, "failed to make / slave") {
		hints = append(hints, "mount propagation blocked (Failed to make / slave: Operation not permitted)")
	}
	if strings.Contains(lower, "setting up uid map") {
		hints = append(hints, "unprivileged user namespace UID mapping denied (setting up uid map: Permission denied)")
	}
	if strings.Contains(lower, "create a new namespace") || strings.Contains(lower, "creating new namespace failed") {
		hints = append(hints, "user/mount namespace creation failed")
	}
	detail := strings.Join(hints, "; ")
	if detail == "" {
		detail = "bubblewrap failed"
	}
	contract, err := sessioncontract.Load()
	if err != nil {
		return detail
	}
	if taiaConfined {
		return fmt.Sprintf("%s: %s; %s", contract.Bwrap.ConfinedFailurePrefix, detail, contract.Bwrap.ConfinedFailureGuidance)
	}
	return fmt.Sprintf("%s: %s; %s", contract.Bwrap.HostFailurePrefix, detail, contract.Bwrap.HostFailureGuidance)
}

// ProbeBwrapSupport verifies that bubblewrap can start a minimal read-only
// namespace on this host, failing closed with a clear diagnostic on unsupported
// platforms or missing Taia --allow-bwrap grants.
func ProbeBwrapSupport(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("bwrap requires Linux (found %s)", runtime.GOOS)
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return fmt.Errorf("bwrap prerequisite missing from PATH: %w", err)
	}
	if sessDir := runstate.RunnerDir(); sessDir != "" {
		if _, err := validateBwrapSessionBind(sessDir); err != nil {
			return err
		}
	}
	cmd := exec.CommandContext(ctx, bwrap, "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--", "true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if diag := DiagnoseBwrapError(string(out), runstate.InRunnerMode()); diag != "" {
			return fmt.Errorf("%s: %w", diag, err)
		}
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" {
			return fmt.Errorf("bwrap prerequisite probe failed (%s): %w", trimmed, err)
		}
		return fmt.Errorf("bwrap prerequisite probe failed: %w", err)
	}
	return nil
}

func validateBwrapSessionBind(sessDir string) (string, error) {
	envName := runstate.RunnerDirEnv()
	if !filepath.IsAbs(sessDir) || filepath.Clean(sessDir) != sessDir {
		return "", fmt.Errorf("%s must be a clean absolute path before entering bwrap", envName)
	}
	info, err := os.Lstat(sessDir)
	if err != nil {
		return "", fmt.Errorf("inspect %s before entering bwrap: %w", envName, err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return "", fmt.Errorf("%s must be a real directory with mode 0700 before entering bwrap", envName)
	}
	// bwrap --bind takes a path rather than a dirfd, so validate uid/mode and
	// symlink-free resolution immediately before spawn to bound swap risk.
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Geteuid() {
		return "", fmt.Errorf("%s must be owned by current user before entering bwrap", envName)
	}
	resolved, err := filepath.EvalSymlinks(sessDir)
	if err != nil || resolved != sessDir {
		return "", fmt.Errorf("%s must not traverse symlinks before entering bwrap", envName)
	}
	return sessDir, nil
}
