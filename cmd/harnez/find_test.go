package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// findFixtureDir writes a small temporary tracker fixture (issue 158's
// verification plan requires an on-disk fixture, not just an in-memory
// one) covering active, archived, and suffix-status tickets, plus a file
// that must be excluded from the entity scan.
func findFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	archiveDir := filepath.Join(issuesDir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"issues/100-open-vram.md": "# 100 — VRAM Load Panel\n\n**Status**: Open\n\n---\n\nDiscusses vram usage.\n",
		"issues/101-closed-gtt.md": "# 101 — GTT Memory Cleanup\n\n**Status:** Closed — resolved in abc123\n\n---\n\nHandles gtt allocation.\n",
		"issues/archive/050-archived-vram.md": "# 050 — Archived VRAM Ticket\n\n**Status:** Closed\n\n---\n\nvram content here too.\n",
		// Must never appear in results: it's the tracker index, not a ticket.
		"issues/README.md": "| # | File | Title | Status |\n|---|------|-------|--------|\n| 100 | [100-open-vram.md](100-open-vram.md) | VRAM Load Panel | Open |\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunFind_ExactTSVOutput(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := runFind(&out, dir, []string{"issues", "vram"}); err != nil {
		t.Fatalf("runFind: %v", err)
	}
	want := "050\tClosed\tArchived VRAM Ticket\tissues/archive/050-archived-vram.md\n" +
		"100\tOpen\tVRAM Load Panel\tissues/100-open-vram.md\n"
	if out.String() != want {
		t.Errorf("output mismatch:\ngot:\n%q\nwant:\n%q", out.String(), want)
	}
}

func TestRunFind_ExcludesReadme(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := runFind(&out, dir, []string{"issues", "File"}); err != nil {
		t.Fatalf("runFind: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected issues/README.md to never be searched, got:\n%s", out.String())
	}
}

func TestRunFind_ZeroMatchesExitsCleanly(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := runFind(&out, dir, []string{"issues", "zzznotfound"}); err != nil {
		t.Fatalf("runFind: unexpected error: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected no output on zero matches, got %q", out.String())
	}
}

func TestRunFind_StatusFilterAndTextCombo(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := runFind(&out, dir, []string{"issues", "status:closed", "vram|gtt"}); err != nil {
		t.Fatalf("runFind: %v", err)
	}
	want := "050\tClosed\tArchived VRAM Ticket\tissues/archive/050-archived-vram.md\n" +
		"101\tClosed — resolved in abc123\tGTT Memory Cleanup\tissues/101-closed-gtt.md\n"
	if out.String() != want {
		t.Errorf("output mismatch:\ngot:\n%q\nwant:\n%q", out.String(), want)
	}
}

func TestRunFind_UnknownEntityIsUsageError(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	err := runFind(&out, dir, []string{"docs", "vram"})
	if err == nil {
		t.Fatal("expected error for unsupported entity")
	}
}

func TestRunFind_EmptyQueryIsUsageError(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	err := runFind(&out, dir, []string{"issues", "   "})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestRunFind_MalformedQueryIsUsageError(t *testing.T) {
	dir := findFixtureDir(t)
	cases := [][]string{
		{"issues", "vram|"},
		{"issues", "(vram)"},
		{"issues", "status:bogus"},
		{"issues", "foo:bar"},
	}
	for _, args := range cases {
		var out bytes.Buffer
		if err := runFind(&out, dir, args); err == nil {
			t.Errorf("runFind(%v): expected usage error, got nil", args)
		}
	}
}
