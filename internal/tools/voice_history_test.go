package tools

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryPath(t *testing.T) {
	got := HistoryPath("/custom/home")
	want := filepath.Join("/custom/home", ".local", "share", "harnez", "voice-input", "history.jsonl")
	if got != want {
		t.Fatalf("HistoryPath: got %q, want %q", got, want)
	}
}

func TestAppendAndListHistory(t *testing.T) {
	dir := t.TempDir()
	historyFile := filepath.Join(dir, "history.jsonl")

	t.Run("empty text is a no-op", func(t *testing.T) {
		entry, err := AppendHistory(historyFile, "", 10, time.Now())
		if err != nil {
			t.Fatalf("AppendHistory: %v", err)
		}
		if entry.ID != "" {
			t.Fatalf("expected empty entry ID, got %q", entry.ID)
		}
		entries, err := ListHistory(historyFile)
		if err != nil {
			t.Fatalf("ListHistory: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("appends entries and orders most recent first", func(t *testing.T) {
		t1 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		t2 := time.Date(2026, 8, 17, 10, 1, 0, 0, time.UTC)
		t3 := time.Date(2026, 8, 17, 10, 2, 0, 0, time.UTC)

		e1, err := AppendHistory(historyFile, "first transcript", 10, t1)
		if err != nil {
			t.Fatal(err)
		}
		e2, err := AppendHistory(historyFile, "second transcript", 10, t2)
		if err != nil {
			t.Fatal(err)
		}
		e3, err := AppendHistory(historyFile, "third transcript", 10, t3)
		if err != nil {
			t.Fatal(err)
		}

		entries, err := ListHistory(historyFile)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		if entries[0].ID != e3.ID || entries[1].ID != e2.ID || entries[2].ID != e1.ID {
			t.Fatalf("unexpected order: got [%s, %s, %s]", entries[0].ID, entries[1].ID, entries[2].ID)
		}
		if entries[0].Text != "third transcript" {
			t.Fatalf("expected 'third transcript', got %q", entries[0].Text)
		}
	})

	t.Run("enforces history limit", func(t *testing.T) {
		t4 := time.Date(2026, 8, 17, 10, 3, 0, 0, time.UTC)
		e4, err := AppendHistory(historyFile, "fourth transcript", 2, t4)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := ListHistory(historyFile)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries after limit trimming, got %d", len(entries))
		}
		if entries[0].ID != e4.ID || entries[0].Text != "fourth transcript" {
			t.Fatalf("unexpected newest entry: %+v", entries[0])
		}
		if entries[1].Text != "third transcript" {
			t.Fatalf("unexpected second entry: %+v", entries[1])
		}
	})

	t.Run("handles corrupt lines gracefully", func(t *testing.T) {
		corruptContent := "{corrupt json line\n{\"id\":\"valid1\",\"time\":\"2026-08-17T10:00:00Z\",\"text\":\"valid\"}\n"
		if err := os.WriteFile(historyFile, []byte(corruptContent), 0600); err != nil {
			t.Fatal(err)
		}
		entries, err := ListHistory(historyFile)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].ID != "valid1" {
			t.Fatalf("expected 1 valid entry, got %d", len(entries))
		}
	})
}

func TestFindHistoryEntry(t *testing.T) {
	dir := t.TempDir()
	historyFile := filepath.Join(dir, "history.jsonl")
	now := time.Now()

	e1, err := AppendHistory(historyFile, "find me", 10, now)
	if err != nil {
		t.Fatal(err)
	}

	found, err := FindHistoryEntry(historyFile, e1.ID)
	if err != nil {
		t.Fatalf("FindHistoryEntry: %v", err)
	}
	if found.ID != e1.ID || found.Text != "find me" {
		t.Fatalf("unexpected entry: %+v", found)
	}

	if _, err := FindHistoryEntry(historyFile, "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestClearHistory(t *testing.T) {
	dir := t.TempDir()
	historyFile := filepath.Join(dir, "history.jsonl")

	if err := ClearHistory(historyFile); err != nil {
		t.Fatalf("ClearHistory on missing file should succeed: %v", err)
	}

	if _, err := AppendHistory(historyFile, "entry", 10, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ClearHistory(historyFile); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}
	entries, err := ListHistory(historyFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}
