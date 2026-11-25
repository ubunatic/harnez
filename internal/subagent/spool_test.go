package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSpoolPointerWrittenOnCreateAndRemovedOnDelete(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	storeDir := t.TempDir()
	store, err := NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: "abc123", Name: "abc", Status: "running"}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(spool, "abc123.json"))
	if err != nil {
		t.Fatalf("pointer not written: %v", err)
	}
	var p spoolPointer
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.V != SpoolVersion || p.SessionID != "abc123" {
		t.Fatalf("pointer header = v%d id %q, want v%d abc123", p.V, p.SessionID, SpoolVersion)
	}
	if p.Store != storeDir {
		t.Fatalf("pointer store = %s, want %s", p.Store, storeDir)
	}
	if len(p.Entry) == 0 {
		t.Fatal("pointer carries no entry copy")
	}
	info, err := os.Stat(filepath.Join(spool, "abc123.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("pointer mode = %v, want 0600", info.Mode().Perm())
	}

	if err := store.Delete("abc123"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(spool, "abc123.json")); !os.IsNotExist(err) {
		t.Fatalf("pointer survives delete: %v", err)
	}
}

func TestSpoolFailureDoesNotFailRegistryWrite(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARNEZ_SPOOL_DIR", filepath.Join(blocker, "spool"))
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: "def456", Name: "def", Status: "running"}); err != nil {
		t.Fatalf("Create failed because the spool is unavailable: %v", err)
	}
	if _, err := store.Get("def456"); err != nil {
		t.Fatalf("registry entry missing after spool failure: %v", err)
	}
}

func TestSpoolIDCannotEscapeSpoolDir(t *testing.T) {
	for _, id := range []string{"", "../x", "a/b", `a\b`, ".hidden"} {
		if safeSpoolID(id) {
			t.Fatalf("safeSpoolID(%q) = true, want false", id)
		}
	}
}
