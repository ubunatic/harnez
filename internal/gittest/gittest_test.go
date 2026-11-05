package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsolateHidesGlobalHooksPath reproduces issue 1120: a global
// core.hooksPath must not reach git once Isolate ran.
func TestIsolateHidesGlobalHooksPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	hostile := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(hostile, []byte("[core]\n\thooksPath = /dev/null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", hostile)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_CONFIG_SYSTEM", "")

	cleanup, err := Isolate()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	if out, _ := exec.Command("git", "config", "--get", "core.hooksPath").Output(); len(out) != 0 {
		t.Fatalf("core.hooksPath = %q after Isolate, want unset", out)
	}
	out, err := exec.Command("git", "config", "--get", "user.email").Output()
	if err != nil || strings.TrimSpace(string(out)) != "test@harnez.invalid" {
		t.Fatalf("user.email = %q, %v; want test identity", out, err)
	}
}
