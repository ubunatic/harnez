package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

// printUnifiedDiff is what makes `harnez index --check` show the specific
// drift inline (not just "would update <path>"), so an agent running the
// command in-session can see exactly what changed and act on it directly.
func TestPrintUnifiedDiff_ShowsChanges(t *testing.T) {
	var out bytes.Buffer
	old := []byte("| 148 | ... | old title | Open |\n")
	new_ := []byte("| 148 | ... | new title | Open |\n")

	if err := printUnifiedDiff(&out, "issues/README.md", old, new_); err != nil {
		t.Fatalf("printUnifiedDiff: %v", err)
	}

	got := out.String()
	for _, want := range []string{"-| 148 | ... | old title | Open |", "+| 148 | ... | new title | Open |", "issues/README.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff output missing %q, got:\n%s", want, got)
		}
	}
}

func TestPrintUnifiedDiff_NoChangesNoOutput(t *testing.T) {
	var out bytes.Buffer
	same := []byte("identical content\n")

	if err := printUnifiedDiff(&out, "docs/README.md", same, same); err != nil {
		t.Fatalf("printUnifiedDiff: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no diff output for identical content, got:\n%s", out.String())
	}
}

func TestRunIndex_WarnsOnMismatchedIssueHeadingToStderr(t *testing.T) {
	repoDir := indexSnapshotFixtureDir(t)
	if err := os.WriteFile(filepath.Join(repoDir, "issues", "001-first.md"),
		[]byte("# 002 — First\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	err := runIndex(&out, indexOptions{Dir: repoDir, DBPath: filepath.Join(t.TempDir(), "telemetry.sqlite"), Stderr: &stderr})
	if err != nil {
		t.Fatalf("runIndex: %v", err)
	}
	if !strings.Contains(stderr.String(), "heading #002 (file number #001)") {
		t.Errorf("missing mismatch warning on stderr: %q", stderr.String())
	}
	if strings.Contains(out.String(), "warning:") {
		t.Errorf("warning leaked to stdout: %q", out.String())
	}
}

// indexSnapshotFixtureDir writes a minimal issues/ tree runIndex can index,
// under a repo dir named "myrepo" so the derived project_name is
// deterministic regardless of the enclosing t.TempDir() path.
func indexSnapshotFixtureDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	issuesDir := filepath.Join(repoDir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "001-first.md"),
		[]byte("# 001 — First\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "README.md"),
		[]byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repoDir
}

// TestRunIndex_RecordsIssueSnapshotAcrossTwoPoints verifies that 'harnez
// index' records a telemetry snapshot whose open/closed counts change
// between two ticket-status states, and that both points are visible via
// QueryIssueSnapshots -- issue 228's read-path acceptance criterion.
func TestRunIndex_RecordsIssueSnapshotAcrossTwoPoints(t *testing.T) {
	repoDir := indexSnapshotFixtureDir(t)
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")

	var out bytes.Buffer
	if err := runIndex(&out, indexOptions{Dir: repoDir, DBPath: dbPath}); err != nil {
		t.Fatalf("runIndex (1): %v", err)
	}

	// Close the one open ticket so the next index run sees a different
	// open/closed split.
	issuePath := filepath.Join(repoDir, "issues", "001-first.md")
	if err := os.WriteFile(issuePath, []byte("# 001 — First\n\n**Status**: Closed — resolved in abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIndex(&out, indexOptions{Dir: repoDir, DBPath: dbPath}); err != nil {
		t.Fatalf("runIndex (2): %v", err)
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	defer db.Close()

	snapshots, err := db.QueryIssueSnapshots("myrepo")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 recorded snapshots (distinct counts), got %d: %+v", len(snapshots), snapshots)
	}
	if snapshots[0].OpenCount != 1 || snapshots[0].ClosedCount != 0 {
		t.Errorf("expected first snapshot open=1 closed=0, got %+v", snapshots[0])
	}
	if snapshots[1].OpenCount != 0 || snapshots[1].ClosedCount != 1 {
		t.Errorf("expected second snapshot open=0 closed=1, got %+v", snapshots[1])
	}
}

// TestRunIndex_NoOpRunsDoNotGrowHistory verifies that repeated 'harnez
// index' runs against an unchanged issues/ directory record at most one
// history entry rather than growing unboundedly -- issue 228's idempotency
// acceptance criterion.
func TestRunIndex_NoOpRunsDoNotGrowHistory(t *testing.T) {
	repoDir := indexSnapshotFixtureDir(t)
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")

	var out bytes.Buffer
	for i := 0; i < 3; i++ {
		if err := runIndex(&out, indexOptions{Dir: repoDir, DBPath: dbPath}); err != nil {
			t.Fatalf("runIndex (run %d): %v", i, err)
		}
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("telemetry.Open: %v", err)
	}
	defer db.Close()

	snapshots, err := db.QueryIssueSnapshots("myrepo")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected exactly 1 snapshot after 3 no-op runs, got %d: %+v", len(snapshots), snapshots)
	}
}

func TestRunIndex_RefusesCustomizedIssuesTableWithoutWriting(t *testing.T) {
	repoDir := t.TempDir()
	issuesDir := filepath.Join(repoDir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "001-first.md"),
		[]byte("# 001 — First\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(issuesDir, "README.md")
	original := `# Issues

| # | File | Title | Status | Priority |
|---|------|-------|--------|----------|
| 001 | [old.md](old.md) | Curated title | Open | P1 |

## Recommended work queue

1. Keep this hand-authored plan.
`
	if err := os.WriteFile(readme, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := runIndex(&out, indexOptions{Dir: repoDir, DBPath: filepath.Join(t.TempDir(), "telemetry.sqlite")})
	if err == nil {
		t.Fatal("expected harnez index path to refuse a customized issues table")
	}
	if !strings.Contains(err.Error(), "refusing to replace customized issues table header") {
		t.Fatalf("unexpected error: %v", err)
	}
	got, readErr := os.ReadFile(readme)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != original {
		t.Errorf("README changed despite refusal:\ngot:\n%s\nwant:\n%s", got, original)
	}
	if out.Len() != 0 {
		t.Errorf("refused index should not report an update, got %q", out.String())
	}
}
