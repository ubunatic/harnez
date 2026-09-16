package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
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
		"issues/100-open-vram.md":             "# 100 — VRAM Load Panel\n\n**Status**: Open\n\n---\n\nDiscusses vram usage.\n",
		"issues/101-closed-gtt.md":            "# 101 — GTT Memory Cleanup\n\n**Status:** Closed — resolved in abc123\n\n---\n\nHandles gtt allocation.\n",
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

func testRunFind(w io.Writer, dir string, args ...string) error {
	return runFind(w, dir, args, false, false, "")
}

func TestRunFind_ExactTSVOutput(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := testRunFind(&out, dir, "issues", "vram"); err != nil {
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
	if err := testRunFind(&out, dir, "issues", "File"); err != nil {
		t.Fatalf("runFind: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected issues/README.md to never be searched, got:\n%s", out.String())
	}
}

func TestRunFind_ZeroMatchesExitsCleanly(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := testRunFind(&out, dir, "issues", "zzznotfound"); err != nil {
		t.Fatalf("runFind: unexpected error: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected no output on zero matches, got %q", out.String())
	}
}

func TestRunFind_StatusFilterAndTextCombo(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := testRunFind(&out, dir, "issues", "status:closed", "vram|gtt"); err != nil {
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
	err := testRunFind(&out, dir, "docs", "vram")
	if err == nil {
		t.Fatal("expected error for unsupported entity")
	}
}

func TestRunFind_EmptyQueryIsUsageError(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	err := testRunFind(&out, dir, "issues", "   ")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestRunFind_DefaultListingTakesNewestTen(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 12; n++ {
		name := fmt.Sprintf("%03d-ticket.md", n)
		content := fmt.Sprintf("# %03d — Ticket %d\n\n**Status**: Open\n\n---\n\nbody\n", n, n)
		if err := os.WriteFile(filepath.Join(dir, "issues", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	if err := runFindWithOptions(&out, &errOut, dir, []string{"issues"}, false, false, "", 10, false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 10 || !strings.HasPrefix(lines[0], "003\t") || !strings.HasPrefix(lines[9], "012\t") {
		t.Fatalf("default listing = %q, want tickets 003 through 012", out.String())
	}
	wantNotice := "# 12 matches, showing last 10 (use --all to show all)\n"
	if errOut.String() != wantNotice {
		t.Errorf("truncation notice = %q, want %q", errOut.String(), wantNotice)
	}
}

func TestRunFind_TextSearchKeepsBestMatches(t *testing.T) {
	dir := findFixtureDir(t)
	var out, errOut bytes.Buffer
	if err := runFindWithOptions(&out, &errOut, dir, []string{"issues", "vram"}, false, false, "", 1, false); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "050\t") {
		t.Fatalf("ranked limit selected %q, want best title match 050", out.String())
	}
	wantNotice := "# 2 matches, showing top 1 (use --all to show all)\n"
	if errOut.String() != wantNotice {
		t.Errorf("truncation notice = %q, want %q", errOut.String(), wantNotice)
	}
}

func TestRunFind_BareQueryCanBeUncapped(t *testing.T) {
	dir := findFixtureDir(t)
	var out, errOut bytes.Buffer
	if err := runFindWithOptions(&out, &errOut, dir, []string{"issues"}, false, false, "", 1, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out.String(), "\n"); got != 3 {
		t.Fatalf("--all output lines = %d, want 3", got)
	}
	if errOut.String() != "" {
		t.Fatalf("expected no truncation notice when --all is set, got %q", errOut.String())
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
		if err := testRunFind(&out, dir, args...); err == nil {
			t.Errorf("runFind(%v): expected usage error, got nil", args)
		}
	}
}

func TestRunFind_IssuesNext(t *testing.T) {
	dir := findFixtureDir(t)
	// Current max issue in fixture is 101, so next is 102
	var out bytes.Buffer
	if err := runFind(&out, dir, []string{"issues", "next"}, false, false, ""); err != nil {
		t.Fatalf("runFind next: %v", err)
	}
	if out.String() != "102\n" {
		t.Errorf("runFind issues next = %q, want %q", out.String(), "102\n")
	}

	// Flag form: --next
	out.Reset()
	if err := runFind(&out, dir, []string{"issues"}, true, false, ""); err != nil {
		t.Fatalf("runFind --next: %v", err)
	}
	if out.String() != "102\n" {
		t.Errorf("runFind issues --next = %q, want %q", out.String(), "102\n")
	}

	// JSON format
	out.Reset()
	if err := runFind(&out, dir, []string{"issues", "next"}, false, true, ""); err != nil {
		t.Fatalf("runFind next --json: %v", err)
	}
	wantJSON := `{"number":"102","reserved":false}` + "\n"
	if out.String() != wantJSON {
		t.Errorf("runFind issues next --json = %q, want %q", out.String(), wantJSON)
	}
}

// TestRunFind_IssuesNextIsReadOnly confirms `find issues next` never creates
// a ticket file -- reservation/creation moved to `harnez issues new` (see
// TestRunIssuesNew_* in issues_test.go). Repeated calls must keep returning
// the same next-free number since nothing is allocated.
func TestRunFind_IssuesNextIsReadOnly(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer
	if err := runFind(&out, dir, []string{"issues", "next"}, false, false, ""); err != nil {
		t.Fatalf("runFind next: %v", err)
	}
	if out.String() != "102\n" {
		t.Errorf("got %q, want %q", out.String(), "102\n")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "issues"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "102") {
			t.Errorf("find issues next must not create a file, found %q", e.Name())
		}
	}

	out.Reset()
	if err := runFind(&out, dir, []string{"issues", "next"}, false, false, ""); err != nil {
		t.Fatalf("runFind next (2nd call): %v", err)
	}
	if out.String() != "102\n" {
		t.Errorf("second call got %q, want %q (no allocation should have occurred)", out.String(), "102\n")
	}
}

// TestRunFindHistory_TableShowsBothRecordedPoints seeds two distinct
// issue_status_snapshots rows directly via telemetry.InsertIssueSnapshot
// and verifies runFindHistory's table output surfaces both, oldest first
// -- issue 228's read-path acceptance criterion.
func TestRunFindHistory_TableShowsBothRecordedPoints(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.InsertIssueSnapshot(telemetry.IssueStatusSnapshot{
		CreatedAt: t1, ProjectName: "harnez", OpenCount: 10, ClosedCount: 5,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertIssueSnapshot(telemetry.IssueStatusSnapshot{
		CreatedAt: t2, ProjectName: "harnez", OpenCount: 8, ClosedCount: 8,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runFindHistory(&out, findHistoryOptions{DBPath: dbPath}); err != nil {
		t.Fatalf("runFindHistory: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "10") || !strings.Contains(got, "5") {
		t.Errorf("expected first snapshot's counts in output, got:\n%s", got)
	}
	if !strings.Contains(got, "8") {
		t.Errorf("expected second snapshot's counts in output, got:\n%s", got)
	}
	firstIdx := strings.Index(got, "2026-01-01")
	secondIdx := strings.Index(got, "2026-02-01")
	if firstIdx == -1 || secondIdx == -1 || firstIdx > secondIdx {
		t.Errorf("expected oldest-first ordering (2026-01-01 before 2026-02-01), got:\n%s", got)
	}
}

// TestRunFindHistory_ProjectFilter verifies --project narrows results to
// one project_name, mirroring issue 227's harnez stats --project idiom.
func TestRunFindHistory_ProjectFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	if _, err := db.InsertIssueSnapshot(telemetry.IssueStatusSnapshot{ProjectName: "harnez", OpenCount: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertIssueSnapshot(telemetry.IssueStatusSnapshot{ProjectName: "voxi", OpenCount: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runFindHistory(&out, findHistoryOptions{DBPath: dbPath, Project: "harnez"}); err != nil {
		t.Fatalf("runFindHistory: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "harnez") {
		t.Errorf("expected harnez row in filtered output, got:\n%s", got)
	}
	if strings.Contains(got, "voxi") {
		t.Errorf("expected voxi row to be filtered out, got:\n%s", got)
	}
}
