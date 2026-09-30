package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

func TestMain(m *testing.M) {
	realHome, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	root, err := os.MkdirTemp("", "harnez-usage-tests-")
	if err != nil {
		panic(err)
	}
	for key, value := range map[string]string{
		"HOME":                  filepath.Join(root, "home"),
		"XDG_DATA_HOME":         filepath.Join(root, "data"),
		"XDG_CACHE_HOME":        filepath.Join(root, "cache"),
		"XDG_STATE_HOME":        filepath.Join(root, "state"),
		"XDG_CONFIG_HOME":       filepath.Join(root, "config"),
		"HARNEZ_TEST_REAL_HOME": realHome,
	} {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "home"), 0o700); err != nil {
		panic(err)
	}
	defaultDB, err := telemetry.DefaultDBPath()
	if err != nil {
		panic(err)
	}
	if !strings.HasPrefix(filepath.Clean(defaultDB), filepath.Clean(root)+string(filepath.Separator)) {
		panic("usage tests would use storage outside isolated temporary root: " + defaultDB)
	}
	code := m.Run()
	if err := os.RemoveAll(root); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func TestUsageStorageGuardRejectsHostLocations(t *testing.T) {
	realHome := os.Getenv("HARNEZ_TEST_REAL_HOME")
	if realHome == "" {
		t.Fatal("test guard did not capture the host home directory")
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(realHome, ".local", "share"))
	if _, err := telemetry.DefaultDBPath(); err == nil {
		t.Fatal("DefaultDBPath accepted the host telemetry path during isolated tests")
	}
	t.Setenv("HOME", realHome)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := telemetry.DefaultDBPath(); err == nil {
		t.Fatal("DefaultDBPath accepted the host home during isolated tests")
	}
}
