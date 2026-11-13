package subagent

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/xdgpath"
)

//go:embed tool-call-limit.ts
var piToolCallLimitSource []byte

// piToolCallLimitExtPath writes the embedded tool-call-limit extension atomically
// to a content-hashed path under the Harnez state dir so concurrent processes or
// stale files never drift from the running binary.
func piToolCallLimitExtPath() (string, error) {
	sum := sha256.Sum256(piToolCallLimitSource)
	name := "tool-call-limit-" + hex.EncodeToString(sum[:6]) + ".ts"
	dir := filepath.Join(xdgpath.StateHome(), "harnez", "pi")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create pi extension state dir: %w", err)
	}
	target := filepath.Join(dir, name)
	if existing, err := os.ReadFile(target); err == nil && len(existing) == len(piToolCallLimitSource) {
		return target, nil
	}
	tmp, err := os.CreateTemp(dir, name+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp pi extension: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(piToolCallLimitSource); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write pi extension: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("chmod pi extension: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close pi extension: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("rename pi extension: %w", err)
	}
	return target, nil
}
