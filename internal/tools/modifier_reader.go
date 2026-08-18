package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ModifierReader reads instantaneous modifier key state from a shared memory/file state.
type ModifierReader struct {
	path string
}

// NewModifierReader creates a new reader for the specified modifier state file path.
// If path is empty, DefaultModifierStatePath ("/run/harnez/modifiers") is used,
// falling back to $XDG_RUNTIME_DIR/harnez/modifiers if /run/harnez is not accessible.
func NewModifierReader(path string) *ModifierReader {
	if path == "" {
		path = ResolveModifierStatePath("")
	}
	return &ModifierReader{path: path}
}

// ResolveModifierStatePath finds the active or default modifier state file path.
func ResolveModifierStatePath(envRuntimeDir string) string {
	// First preference: standard system path /run/harnez/modifiers
	if _, err := os.Stat(DefaultModifierStatePath); err == nil {
		return DefaultModifierStatePath
	}

	// Fallback to user runtime directory
	runtimeDir := envRuntimeDir
	if runtimeDir == "" {
		runtimeDir = os.Getenv("XDG_RUNTIME_DIR")
	}
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	userPath := filepath.Join(runtimeDir, "harnez", "modifiers")
	if _, err := os.Stat(userPath); err == nil {
		return userPath
	}

	// Default fallback to standard path
	return DefaultModifierStatePath
}

// ReadMask reads the single-byte instantaneous modifier mask.
// If the state file does not exist or cannot be read, it returns 0 (no modifiers) and nil error
// to avoid breaking injection workflows when the daemon is not running.
func (r *ModifierReader) ReadMask() (ModifierMask, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read modifier state %s: %w", r.path, err)
	}
	if len(data) == 0 {
		return 0, nil
	}
	return ModifierMask(data[0]), nil
}

// AreModifiersActive returns true if any physical modifier keys are currently depressed.
func (r *ModifierReader) AreModifiersActive() (bool, error) {
	mask, err := r.ReadMask()
	if err != nil {
		return false, err
	}
	return mask.AnyActive(), nil
}

// WaitModifiersReleased blocks until all physical modifier keys are released,
// or until the context is cancelled / timeout expires.
// Polling interval is 10ms for sub-frame latency.
func (r *ModifierReader) WaitModifiersReleased(ctx context.Context, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		active, err := r.AreModifiersActive()
		if err != nil || !active {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Global default modifier reader instance for convenience.
var defaultReader = NewModifierReader("")

// AreModifiersActive returns whether any modifiers are active using the default reader.
func AreModifiersActive() (bool, error) {
	return defaultReader.AreModifiersActive()
}

// WaitModifiersReleased waits for modifier release using the default reader.
func WaitModifiersReleased(ctx context.Context, timeout time.Duration) error {
	return defaultReader.WaitModifiersReleased(ctx, timeout)
}
