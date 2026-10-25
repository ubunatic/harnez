package modelcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "harnez-modelcatalog-home-")
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
			_ = os.Unsetenv(strings.SplitN(pair, "=", 2)[0])
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
