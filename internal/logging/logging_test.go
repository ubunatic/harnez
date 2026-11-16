package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDebugRunnerAndInvalidRoot(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("DEBUG", "1")
	t.Setenv("TAIA_SESSION_DIR", root)
	Debugf("runner message")
	if _, err := os.Stat(filepath.Join(root, "state", "logs", "debug.log")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", "relative")
	Debugf("must not fall back")
	if _, err := os.Stat(filepath.Join(home, ".harnez", "debug.log")); !os.IsNotExist(err) {
		t.Fatalf("host log: %v", err)
	}
}
