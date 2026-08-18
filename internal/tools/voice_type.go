package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// BuildDotoolCommands renders the dotool script-language command stream for
// typing `text`, mirroring Voxtype's own src/output/dotool.rs
// `build_commands` (typedelay/typehold + one `type` line per input line,
// since dotool's `type` command takes the rest of its line literally and
// cannot itself contain a newline).
func BuildDotoolCommands(text string, typeDelayMs int) string {
	var b strings.Builder
	if typeDelayMs > 0 {
		fmt.Fprintf(&b, "typedelay %d\n", typeDelayMs)
		fmt.Fprintf(&b, "typehold %d\n", typeDelayMs)
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		fmt.Fprintf(&b, "type %s\n", line)
		if i < len(lines)-1 {
			b.WriteString("key enter\n")
		}
	}
	return b.String()
}

// dotoolPipePath resolves the fifo dotoold reads from, matching
// DotoolOutput::daemon_pipe_path in Voxtype's Rust source.
func dotoolPipePath(getenv func(string) string) string {
	if p := getenv("DOTOOL_PIPE"); p != "" {
		return p
	}
	return "/tmp/dotool-pipe"
}

// dotoolDaemonReady detects whether dotoold is actually running and
// accepting input: the fifo must exist, be a real named pipe, and a
// non-blocking write-open must succeed (which only happens when some
// process -- dotoold -- already has the other end open for reading). This
// is the same technique Voxtype's Rust source uses for the identical
// purpose, translated to Go. A crashed daemon leaves the fifo on disk; the
// kernel returns ENXIO from the non-blocking open in that case, so this
// correctly falls through to the cold `dotool` path.
func dotoolDaemonReady(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return false
	}
	fd, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	_ = fd.Close()
	return true
}

// TypeText invokes the dotool/dotoold typing primitive from issue 020/021,
// unchanged: prefer the persistent dotoold daemon (dotoolc, sub-10ms) when
// it's actually accepting input, otherwise fall back to a cold `dotool`
// invocation (slower, ~700-800ms uinput registration, but works standalone
// without the daemon running). This performs NO window-focus capture or
// restoration -- it types into whatever currently has focus, exactly like
// Voxtype's own output drivers. Callers (a future GNOME Shell extension, or
// a human testing from a terminal) are responsible for ensuring the right
// window is focused first.
func TypeText(ctx context.Context, d Dependencies, text string) error {
	if text == "" {
		return nil
	}

	// Gate on active physical modifier keys: wait up to 5s for user to release modifiers (e.g. Ctrl, Alt, Super)
	// before injecting keystrokes to prevent unintentional hotkey collisions.
	if reader := NewModifierReader(""); reader != nil {
		_ = reader.WaitModifiersReleased(ctx, 5*time.Second)
	}
	typeDelayMs := 0
	if home := d.Getenv("HOME"); home != "" {
		if ms, ok, err := ReadTypeDelayMs(voxtypeConfigPath(home)); err == nil && ok {
			typeDelayMs = ms
		}
	}
	commands := BuildDotoolCommands(text, typeDelayMs)

	if dotoolDaemonReady(dotoolPipePath(d.Getenv)) {
		if err := d.RunStdin(ctx, commands, "dotoolc"); err == nil {
			return nil
		}
		// Fall through to the cold path rather than failing outright,
		// matching Voxtype's own never-hard-fail-on-the-fast-path
		// philosophy for this driver.
	}
	if _, err := d.LookPath("dotool"); err != nil {
		return fmt.Errorf("dotool not found on PATH: %w", err)
	}
	if err := d.RunStdin(ctx, commands, "dotool"); err != nil {
		return fmt.Errorf("dotool: %w", err)
	}
	return nil
}

// CopyText copies text to the Wayland clipboard via wl-copy, the same
// clipboard driver Voxtype's own fallback chain uses.
func CopyText(ctx context.Context, d Dependencies, text string) error {
	if _, err := d.LookPath("wl-copy"); err != nil {
		return fmt.Errorf("wl-copy not found on PATH: %w", err)
	}
	if err := d.RunStdin(ctx, text, "wl-copy"); err != nil {
		return fmt.Errorf("wl-copy: %w", err)
	}
	return nil
}

// voxtypeConfigPath is the well-known location of Voxtype's user config.
func voxtypeConfigPath(home string) string {
	return filepath.Join(home, ".config", "voxtype", "config.toml")
}
