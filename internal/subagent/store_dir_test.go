package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultStoreDirHonoursStoreDirEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "agents")
	t.Setenv(StoreDirEnv, want)
	if got := DefaultStoreDir(); got != want {
		t.Fatalf("DefaultStoreDir() = %q, want %q", got, want)
	}
}

func TestDefaultStoreDirFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, value := range []string{"", "   "} {
		t.Setenv(StoreDirEnv, value)
		want := filepath.Join(home, ".harnez", "agents")
		if got := DefaultStoreDir(); got != want {
			t.Fatalf("%s=%q: DefaultStoreDir() = %q, want %q", StoreDirEnv, value, got, want)
		}
	}
}

// A registry the process cannot create fails with an error that names the flag
// and env var an owner sets to pick a writable store.
func TestNewSessionStoreErrorNamesStoreDirOverrides(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	_, err := NewSessionStore(filepath.Join(parent, "agents"))
	if err == nil {
		t.Fatal("NewSessionStore succeeded in a read-only parent")
	}
	for _, want := range []string{"--store-dir", "HARNEZ_STORE_DIR"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("NewSessionStore error = %q, want %s", err.Error(), want)
		}
	}
}
