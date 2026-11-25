package subagent

import (
	"path/filepath"
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
