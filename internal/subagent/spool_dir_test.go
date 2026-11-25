package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpoolDirHonoursEnvAndDefaultsUnderHome(t *testing.T) {
	want := filepath.Join(t.TempDir(), "spool")
	t.Setenv("HARNEZ_SPOOL_DIR", want)
	if got := SpoolDir(); got != want {
		t.Fatalf("SpoolDir() with HARNEZ_SPOOL_DIR = %q, want %q", got, want)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNEZ_SPOOL_DIR", "")
	if got, want := SpoolDir(), filepath.Join(home, ".harnez", "spool"); got != want {
		t.Fatalf("SpoolDir() unset = %q, want %q", got, want)
	}
}

// Reading pointers must not create the store a stale pointer names.
func TestSpoolReadDoesNotCreateMissingStore(t *testing.T) {
	t.Setenv("HARNEZ_SPOOL_DIR", t.TempDir())
	storeDir := filepath.Join(t.TempDir(), "gone")
	store, err := NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: "gone1", Name: "gone", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(storeDir); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSpoolEntries(map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(storeDir); !os.IsNotExist(err) {
		t.Fatalf("ReadSpoolEntries recreated the missing store: stat err = %v", err)
	}
}
