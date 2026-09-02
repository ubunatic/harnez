package feedback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAndLoad(t *testing.T) {
	feedbackDir := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "myproj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	e := Entry{
		ID:          NewID(projectDir, "bad instruction in AGENTS.md", now),
		Time:        now,
		Project:     "myproj",
		Description: "bad instruction in AGENTS.md",
		Severity:    "instruction",
		Status:      StatusNew,
	}
	if err := Append(feedbackDir, projectDir, e); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	entries, err := Load(feedbackDir, projectDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].ID != e.ID || entries[0].Description != e.Description {
		t.Errorf("expected loaded entry to match appended entry, got %+v", entries[0])
	}
	if entries[0].Status != StatusNew {
		t.Errorf("expected status %q, got %q", StatusNew, entries[0].Status)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	feedbackDir := t.TempDir()
	entries, err := Load(feedbackDir, filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("expected no error for missing log, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries, got %+v", entries)
	}
}

func TestLoadFoldsRepeatedIDLastWriteWins(t *testing.T) {
	feedbackDir := t.TempDir()
	projectDir := t.TempDir()

	first := Entry{ID: "abc123", Time: time.Now(), Description: "found a bug", Status: StatusNew}
	if err := Append(feedbackDir, projectDir, first); err != nil {
		t.Fatalf("Append 1: %v", err)
	}

	second := Entry{ID: "abc123", Time: time.Now(), Description: "found a bug", Status: StatusPromoted, TicketPath: "issues/999-found-a-bug.md"}
	if err := Append(feedbackDir, projectDir, second); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	entries, err := Load(feedbackDir, projectDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected repeated ID to fold into 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Status != StatusPromoted {
		t.Errorf("expected last-write-wins status %q, got %q", StatusPromoted, entries[0].Status)
	}
	if entries[0].TicketPath != "issues/999-found-a-bug.md" {
		t.Errorf("expected ticket path to be updated, got %q", entries[0].TicketPath)
	}
}

func TestAppendIsolatesDifferentProjects(t *testing.T) {
	feedbackDir := t.TempDir()
	projA := filepath.Join(t.TempDir(), "a")
	projB := filepath.Join(t.TempDir(), "b")

	if err := Append(feedbackDir, projA, Entry{ID: "1", Description: "in A", Status: StatusNew}); err != nil {
		t.Fatalf("Append A: %v", err)
	}
	if err := Append(feedbackDir, projB, Entry{ID: "2", Description: "in B", Status: StatusNew}); err != nil {
		t.Fatalf("Append B: %v", err)
	}

	aEntries, err := Load(feedbackDir, projA)
	if err != nil {
		t.Fatalf("Load A: %v", err)
	}
	if len(aEntries) != 1 || aEntries[0].Description != "in A" {
		t.Errorf("expected project A's log to only contain its own entry, got %+v", aEntries)
	}

	bEntries, err := Load(feedbackDir, projB)
	if err != nil {
		t.Fatalf("Load B: %v", err)
	}
	if len(bEntries) != 1 || bEntries[0].Description != "in B" {
		t.Errorf("expected project B's log to only contain its own entry, got %+v", bEntries)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Fix the `harnez apply` idempotency bug!": "fix-the-harnez-apply-idempotency-bug",
		"":          "feedback",
		"   ---   ": "feedback",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextTicketNumber(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"003-foo.md", "010-bar.md", "002-baz.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# 003 — Foo\n\n**Status**: Open\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	got, err := NextTicketNumber(dir)
	if err != nil {
		t.Fatalf("NextTicketNumber failed: %v", err)
	}
	if got != "011" {
		t.Errorf("expected next number 011, got %q", got)
	}
}

func TestNextTicketNumberEmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := NextTicketNumber(dir)
	if err != nil {
		t.Fatalf("NextTicketNumber failed: %v", err)
	}
	if got != "001" {
		t.Errorf("expected 001 for an empty issues dir, got %q", got)
	}
}

func TestPromoteWritesTicketFile(t *testing.T) {
	issuesDir := filepath.Join(t.TempDir(), "issues")
	e := Entry{
		ID:          "deadbeef",
		Time:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Description: "harnez apply silently drops a managed section on rerun",
		Severity:    "bug",
		SessionID:   "sess-1",
	}

	path, err := Promote(issuesDir, e)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if filepath.Dir(path) != issuesDir {
		t.Errorf("expected ticket under %s, got %s", issuesDir, path)
	}
	if filepath.Base(path) != "001-harnez-apply-silently-drops-a-managed-section-on-rerun.md" {
		t.Errorf("unexpected ticket filename: %s", filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ticket file: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"# 001 — harnez apply silently drops a managed section on rerun",
		"**Status**: Draft",
		"**Category**: Bug",
		"harnez feedback entry deadbeef",
		"session sess-1",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected ticket content to contain %q, got:\n%s", want, content)
		}
	}
}
