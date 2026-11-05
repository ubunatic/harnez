package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/gittest"
)

func TestMain(m *testing.M) {
	if os.Getenv("TEST_983_CHILD") == "1" {
		os.Exit(m.Run())
	}
	home, err := os.MkdirTemp("", "harnez-subagent-home-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	_ = os.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	_ = os.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	_ = os.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	_ = os.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "runtime"))
	_ = os.MkdirAll(filepath.Join(home, "runtime"), 0o700)
	for _, pair := range os.Environ() {
		if strings.HasPrefix(pair, "HARNEZ_") {
			name := strings.SplitN(pair, "=", 2)[0]
			if name == "HARNEZ_PTY_RESIZE_CHILD" {
				continue
			}
			_ = os.Unsetenv(name)
		}
	}
	gitCleanup := gittest.MustIsolate() // hide host git config (issue 1120)
	code := m.Run()
	gitCleanup()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
