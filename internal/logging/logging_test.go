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

func TestRunnerRejectsMalformedRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DEBUG", "1")
	for _, kind := range []string{"missing", "permissive", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			switch kind {
			case "missing":
				root = filepath.Join(root, "missing")
			case "permissive":
				if err := os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
			}
			t.Setenv("TAIA_SESSION_DIR", root)
			Debugf("invalid root")
			if _, err := os.Stat(filepath.Join(home, ".harnez", "debug.log")); !os.IsNotExist(err) {
				t.Fatalf("shared log: %v", err)
			}
		})
	}
}
