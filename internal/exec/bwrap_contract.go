package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DiagnoseBwrapError translates bubblewrap namespace and mount-propagation
// errors into actionable host or Taia-profile prerequisite diagnostics.
func DiagnoseBwrapError(output string, taiaConfined bool) string {
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "bwrap:") &&
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
	if taiaConfined {
		return fmt.Sprintf("bwrap prerequisite failure under Taia confinement: %s; re-launch taia with --allow-bwrap (or a profile including it such as --allow develop)", detail)
	}
	return fmt.Sprintf("bwrap prerequisite failure on host: %s; host kernel/AppArmor userns configuration does not permit unprivileged bubblewrap", detail)
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
	if sessDir := os.Getenv("TAIA_SESSION_DIR"); sessDir != "" {
		if _, err := validateBwrapSessionBind(sessDir); err != nil {
			return err
		}
	}
	cmd := exec.CommandContext(ctx, bwrap, "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--", "true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if diag := DiagnoseBwrapError(string(out), os.Getenv("TAIA_SESSION_DIR") != ""); diag != "" {
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
	if !filepath.IsAbs(sessDir) || filepath.Clean(sessDir) != sessDir {
		return "", fmt.Errorf("TAIA_SESSION_DIR must be a clean absolute path before entering bwrap")
	}
	info, err := os.Lstat(sessDir)
	if err != nil {
		return "", fmt.Errorf("inspect TAIA_SESSION_DIR before entering bwrap: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return "", fmt.Errorf("TAIA_SESSION_DIR must be a real directory with mode 0700 before entering bwrap")
	}
	resolved, err := filepath.EvalSymlinks(sessDir)
	if err != nil || resolved != sessDir {
		return "", fmt.Errorf("TAIA_SESSION_DIR must not traverse symlinks before entering bwrap")
	}
	return sessDir, nil
}
