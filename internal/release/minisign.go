package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveMinisignKey finds the appropriate minisign secret key path for a project.
func ResolveMinisignKey(dir, explicitKey string) (string, error) {
	if explicitKey != "" {
		expanded := expandPath(explicitKey)
		if _, err := os.Stat(expanded); err != nil {
			return "", fmt.Errorf("specified minisign key not found: %s", expanded)
		}
		return expanded, nil
	}

	projectName := filepath.Base(dir)
	if projectName == "." || projectName == "/" || projectName == "" {
		if abs, err := filepath.Abs(dir); err == nil {
			projectName = filepath.Base(abs)
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		// 1. ~/.minisign/<project>.key (project-specific in user home)
		k1 := filepath.Join(home, ".minisign", projectName+".key")
		if _, err := os.Stat(k1); err == nil {
			return k1, nil
		}
	}

	// 2. Local repo keys
	for _, localCandidate := range []string{
		filepath.Join(dir, ".minisign.key"),
		filepath.Join(dir, "minisign.key"),
		filepath.Join(dir, projectName+".key"),
	} {
		if _, err := os.Stat(localCandidate); err == nil {
			return localCandidate, nil
		}
	}

	// 3. ~/.minisign/minisign.key (global fallback)
	if home != "" {
		k2 := filepath.Join(home, ".minisign", "minisign.key")
		if _, err := os.Stat(k2); err == nil {
			return k2, nil
		}
	}

	return "", fmt.Errorf("no minisign key found for project %q (checked ~/.minisign/%s.key, ~/.minisign/minisign.key). Specify with --sign-key", projectName, projectName)
}

// SignFile generates a .minisig signature for targetFile using minisign.
func SignFile(keyPath, targetFile, comment string) (string, error) {
	sigFile := targetFile + ".minisig"

	// Remove old signature if present
	_ = os.Remove(sigFile)

	args := []string{"-S", "-s", keyPath, "-m", targetFile}
	if comment != "" {
		args = append(args, "-t", comment)
	}

	cmd := exec.Command("minisign", args...)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("minisign signing failed: %w\nOutput: %s", err, string(out))
	}

	if _, err := os.Stat(sigFile); err != nil {
		return "", fmt.Errorf("signature file %s was not created: %w", sigFile, err)
	}

	return sigFile, nil
}

// TestMinisignKey checks if the key is usable (non-interactive, valid).
func TestMinisignKey(keyPath string) error {
	tmpFile, err := os.CreateTemp("", "harnez-minisign-test-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("harnez-key-test"); err != nil {
		return err
	}
	_ = tmpFile.Close()

	sigFile, err := SignFile(keyPath, tmpFile.Name(), "test")
	if err != nil {
		return err
	}
	defer os.Remove(sigFile)

	return nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
