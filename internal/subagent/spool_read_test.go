package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// spoolFor returns the spool entry for id, failing when it is absent.
func spoolFor(t *testing.T, entries []SpoolEntry, id string) SpoolEntry {
	t.Helper()
	for _, e := range entries {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no spool entry for %s in %+v", id, entries)
	return SpoolEntry{}
}

func TestSpoolEntryVerifiedWhenRegistryConfirms(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	storeDir := t.TempDir()
	store, err := NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: "ver1", Name: "ver", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update("ver1", func(s *Session) error { s.Status = "completed"; return nil }); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	e := spoolFor(t, entries, "ver1")
	if e.State != SpoolVerified || e.Session == nil || e.Session.Status != "completed" {
		t.Fatalf("entry = state %s status %v, want verified completed", e.State, e.Session)
	}
}

func TestSpoolEntryStaleWhenStoreGone(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	storeDir := filepath.Join(t.TempDir(), "gone")
	store, err := NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: "st1", Name: "st", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(storeDir); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	e := spoolFor(t, entries, "st1")
	if e.State != SpoolStale || e.Session == nil || e.Session.Name != "st" {
		t.Fatalf("entry = state %s session %v, want stale with copied name st", e.State, e.Session)
	}
}

func TestSpoolForgedPointerIsUnverified(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	realStore := t.TempDir()
	realStoreSess, err := NewSessionStore(realStore)
	if err != nil {
		t.Fatal(err)
	}
	if err := realStoreSess.Create(&Session{ID: "other", Name: "other", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	// The pointer claims session "victim" but names a store holding "other".
	if err := writeSpoolPointer(realStore, "victim", []byte(`{"id":"victim","name":"victim"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	e := spoolFor(t, entries, "victim")
	if e.State != SpoolStale && e.State != SpoolUnverified {
		t.Fatalf("forged pointer state = %s, want unverified or stale", e.State)
	}
	if e.State == SpoolVerified {
		t.Fatal("forged pointer was verified")
	}
}

func TestSpoolUnknownVersionShownWithoutMetadata(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	body := `{"v":9,"session_id":"future","store":"/x","entry":{"id":"future","name":"f"}}`
	if err := os.WriteFile(filepath.Join(spool, "future.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	e := spoolFor(t, entries, "future")
	if e.State != SpoolUnverified || e.Session != nil {
		t.Fatalf("unknown v: state %s session %v, want unverified without metadata", e.State, e.Session)
	}
}

func TestSpoolRegistryEntryWins(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	if err := writeSpoolPointer(t.TempDir(), "dup", []byte(`{"id":"dup"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{"dup": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.ID == "dup" {
			t.Fatal("pointer shown although the registry holds the same session")
		}
	}
}

func TestSpoolSymlinkedStoreIsNotFollowed(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	realStore := t.TempDir()
	realSess, err := NewSessionStore(realStore)
	if err != nil {
		t.Fatal(err)
	}
	if err := realSess.Create(&Session{ID: "sym", Name: "sym", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realStore, link); err != nil {
		t.Fatal(err)
	}
	if err := writeSpoolPointer(link, "sym", []byte(`{"id":"sym"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if e := spoolFor(t, entries, "sym"); e.State == SpoolVerified {
		t.Fatal("pointer through a symlinked store was verified")
	}
}

func TestSpoolOversizedPointerIsUnverifiedHint(t *testing.T) {
	spool := t.TempDir()
	t.Setenv("HARNEZ_SPOOL_DIR", spool)
	big := `{"v":1,"session_id":"big","entry":"` + strings.Repeat("x", maxSpoolPointerBytes) + `"}`
	if err := os.WriteFile(filepath.Join(spool, "big.json"), []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadSpoolEntries(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if e := spoolFor(t, entries, "big"); e.State != SpoolUnverified || e.Session != nil {
		t.Fatalf("oversized pointer state = %s session %v, want unverified without metadata", e.State, e.Session)
	}
}
