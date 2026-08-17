package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Voxtype's own `voxtype config set` subcommand only supports `engine`
// today (not type_delay_ms), so this reimplements the same
// comment-preserving strategy Voxtype uses internally (targeted in-place
// edits via toml_edit) as a small, testable regex-based line editor rather
// than a full TOML round-trip that would risk reformatting comments.
var (
	typeDelayActiveRe = regexp.MustCompile(`(?m)^([ \t]*)type_delay_ms([ \t]*=[ \t]*)([0-9]+)[ \t]*$`)
	outputHeaderRe    = regexp.MustCompile(`(?m)^\[output\][ \t]*$`)
)

// ReadTypeDelayMs reads the active (uncommented) `type_delay_ms` value from
// a voxtype config.toml. Returns 0, false if the key is absent (voxtype's
// own documented default).
func ReadTypeDelayMs(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("read %s: %w", path, err)
	}
	m := typeDelayActiveRe.FindSubmatch(data)
	if m == nil {
		return 0, false, nil
	}
	ms, err := strconv.Atoi(string(m[3]))
	if err != nil {
		return 0, false, fmt.Errorf("parse type_delay_ms: %w", err)
	}
	return ms, true, nil
}

// SetTypeDelayMs writes `ms` into config.toml at `path`, preserving every
// other line verbatim (comments, spacing, unrelated settings). If an active
// `type_delay_ms` line exists, only its numeric value is replaced in place.
// Otherwise a new line is inserted right after `[output]` (voxtype always
// ships this key active by default, so this is a defensive fallback, not
// the common path). Does not restart voxtype.service -- the caller is
// responsible for telling the user a restart is required.
func SetTypeDelayMs(path string, ms int) error {
	if ms < 0 {
		return fmt.Errorf("type_delay_ms must be >= 0, got %d", ms)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	replacement := []byte(fmt.Sprintf("${1}type_delay_ms${2}%d", ms))
	if typeDelayActiveRe.Match(data) {
		updated := typeDelayActiveRe.ReplaceAll(data, replacement)
		return writeConfigAtomic(path, updated)
	}
	loc := outputHeaderRe.FindIndex(data)
	if loc == nil {
		return fmt.Errorf("no [output] section found in %s; cannot add type_delay_ms", path)
	}
	insertAt := loc[1]
	line := []byte(fmt.Sprintf("\ntype_delay_ms = %d", ms))
	updated := append(append(append([]byte{}, data[:insertAt]...), line...), data[insertAt:]...)
	return writeConfigAtomic(path, updated)
}

// writeConfigAtomic writes data to path via a temp file + rename in the
// same directory, matching an existing file's permissions.
func writeConfigAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	mode := os.FileMode(0644)
	if err == nil {
		mode = info.Mode()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(mode)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}
