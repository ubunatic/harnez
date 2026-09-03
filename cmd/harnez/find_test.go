package main

import (
	"bytes"
	"io"
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

func testRunFind(w io.Writer, dir string, args ...string) error {
	return runFind(w, dir, args, false, false, "", false)
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
	if err := runFind(&out, dir, []string{"issues", "next"}, false, false, "", false); err != nil {
		t.Fatalf("runFind next: %v", err)
	}
	if out.String() != "102\n" {
		t.Errorf("runFind issues next = %q, want %q", out.String(), "102\n")
	}

	// Flag form: --next
	out.Reset()
	if err := runFind(&out, dir, []string{"issues"}, true, false, "", false); err != nil {
		t.Fatalf("runFind --next: %v", err)
	}
	if out.String() != "102\n" {
		t.Errorf("runFind issues --next = %q, want %q", out.String(), "102\n")
	}

	// JSON format
	out.Reset()
	if err := runFind(&out, dir, []string{"issues", "next"}, false, false, "", true); err != nil {
		t.Fatalf("runFind next --json: %v", err)
	}
	wantJSON := `{"number":"102","reserved":false}` + "\n"
	if out.String() != wantJSON {
		t.Errorf("runFind issues next --json = %q, want %q", out.String(), wantJSON)
	}
}

func TestRunFind_IssuesNextReserve(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer

	// Reserve without title
	if err := runFind(&out, dir, []string{"issues", "next"}, false, true, "", false); err != nil {
		t.Fatalf("runFind issues next --reserve: %v", err)
	}
	if out.String() != "102\tissues/102-reserved.md\n" {
		t.Errorf("got %q, want %q", out.String(), "102\tissues/102-reserved.md\n")
	}
	reservedFile := filepath.Join(dir, "issues", "102-reserved.md")
	if _, err := os.Stat(reservedFile); err != nil {
		t.Fatalf("expected reserved file %s to exist: %v", reservedFile, err)
	}

	// Second reservation with title
	out.Reset()
	if err := runFind(&out, dir, []string{"issues", "next"}, false, true, "New Feature", false); err != nil {
		t.Fatalf("runFind issues next --reserve 'New Feature': %v", err)
	}
	if out.String() != "103\tissues/103-new-feature.md\n" {
		t.Errorf("got %q, want %q", out.String(), "103\tissues/103-new-feature.md\n")
	}
	titledFile := filepath.Join(dir, "issues", "103-new-feature.md")
	if _, err := os.Stat(titledFile); err != nil {
		t.Fatalf("expected reserved file %s to exist: %v", titledFile, err)
	}

	// Third reservation with JSON
	out.Reset()
	if err := runFind(&out, dir, []string{"issues", "next"}, false, true, "JSON Feature", true); err != nil {
		t.Fatalf("runFind issues next --reserve --json: %v", err)
	}
	wantJSON := `{"number":"104","reserved":true,"file":"104-json-feature.md","path":"issues/104-json-feature.md"}` + "\n"
	if out.String() != wantJSON {
		t.Errorf("runFind issues next --reserve --json = %q, want %q", out.String(), wantJSON)
	}
}

// TestRunFind_IssuesNextReservePrintsExactFilename reproduces issue 202: a
// caller that hand-derives a slug from the same title used for --reserve can
// land on a different filename than the one Reserve() actually created
// (e.g. "Add Doubled-Res. Sparklines!" slugifies differently depending on
// how the caller handles punctuation). Requiring callers to write to the
// filename printed by --reserve, instead of re-deriving it, removes that
// divergence opportunity at the source.
func TestRunFind_IssuesNextReservePrintsExactFilename(t *testing.T) {
	dir := findFixtureDir(t)
	var out bytes.Buffer

	title := "Add Doubled-Res. Sparklines!"
	if err := runFind(&out, dir, []string{"issues", "next"}, false, true, title, false); err != nil {
		t.Fatalf("runFind issues next --reserve %q: %v", title, err)
	}

	got := out.String()
	parts := bytes.SplitN([]byte(got), []byte("\t"), 2)
	if len(parts) != 2 {
		t.Fatalf("expected NUMBER<TAB>PATH output, got %q", got)
	}
	num := string(parts[0])
	printedPath := string(bytes.TrimSuffix(parts[1], []byte("\n")))

	// A plausible hand-derived slug that differs from the one Reserve()
	// actually produced (e.g. dropping the trailing period differently, or
	// collapsing "doubled-res" vs "doubled-resolution").
	handDerivedPath := filepath.Join("issues", num+"-add-doubled-resolution-sparklines.md")
	if printedPath == handDerivedPath {
		t.Fatalf("test setup invalid: hand-derived path %q should differ from printed path", handDerivedPath)
	}

	// The path harnez actually printed must exist and be the one Reserve()
	// created; a caller writing ticket content there never diverges.
	if _, err := os.Stat(filepath.Join(dir, printedPath)); err != nil {
		t.Fatalf("printed reserve path %q does not exist on disk: %v", printedPath, err)
	}
	// And the hand-derived guess must NOT exist -- proving that following
	// the printed path (rather than re-deriving a slug) is what avoids the
	// orphaned-placeholder bug.
	if _, err := os.Stat(filepath.Join(dir, handDerivedPath)); err == nil {
		t.Fatalf("hand-derived path %q unexpectedly exists; test no longer demonstrates divergence", handDerivedPath)
	}
}

