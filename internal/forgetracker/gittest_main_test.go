package forgetracker

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"ubunatic.com/harnez/internal/gittest"
)

// TestMain hides the host's git config from these tests (issue 1120) and stubs
// the fj fallback CLI so unit tests never execute real /usr/bin/fj.
func TestMain(m *testing.M) {
	FJLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	FJRun = func(c Command) ([]byte, error) {
		return nil, fmt.Errorf("test did not stub forgetracker fj CLI: %s", c)
	}
	cleanup := gittest.MustIsolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
