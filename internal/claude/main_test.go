package claude

import (
	"fmt"
	"os"
	"testing"
)

func loadTestConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}
	cfg.JevCompactionEnabled = false
	return cfg
}

func LoadTestConfig(t *testing.T) *Config {
	return loadTestConfig(t)
}

// TestMain points HOME at a temp dir so no test can touch the user's real
// home: CleanAll removes ~/.local/bin/harnez-agy and ~/.harnez/shims (issue 553).
func TestMain(m *testing.M) {
	oldHome, hadHome := os.LookupEnv("HOME")
	tmpHome, err := os.MkdirTemp("", "harnez-claude-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp HOME: %v", err))
	}
	if err := os.Setenv("HOME", tmpHome); err != nil {
		panic(fmt.Sprintf("failed to set HOME: %v", err))
	}

	code := m.Run()

	os.RemoveAll(tmpHome)
	if hadHome {
		os.Setenv("HOME", oldHome)
	} else {
		os.Unsetenv("HOME")
	}

	os.Exit(code)
}
