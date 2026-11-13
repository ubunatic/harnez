package usage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestTerminalSizeDoesNotSpawnStty guards issue 1231: `stty -F /dev/tty` raises
// SIGTTOU in a background process group, so `harnez usage` run by an agent
// (under `harnez exec`) was stopped and killed with exit 125.
func TestTerminalSizeDoesNotSpawnStty(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "stty-ran")
	script := "#!/bin/sh\ntouch '" + marker + "'\necho '30 100'\n"
	if err := os.WriteFile(filepath.Join(dir, "stty"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COLUMNS", "")
	t.Setenv("LINES", "")

	cols, rows := terminalSize(&bytes.Buffer{})
	if cols < minTerminalWidth || rows < minTerminalHeight {
		t.Fatalf("terminalSize = %dx%d, below minimum", cols, rows)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("terminalSize spawned stty; use TIOCGWINSZ on /dev/tty instead")
	}
}
