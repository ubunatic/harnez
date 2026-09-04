package index

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIssuesTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001-first-bug.md"), "# 001 — First bug\n\n**Status**: Open\n**Priority**: P2 (Medium)\n")
	writeFile(t, filepath.Join(dir, "archive", "002-old-bug.md"), "# 002 — Old bug\n\n**Status**: Closed — resolved in abc123\n")

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}

	want := `| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [001-first-bug.md](001-first-bug.md) | First bug | Open |
| 002 | [archive/002-old-bug.md](archive/002-old-bug.md) | Old bug | Closed — resolved in abc123 |
`
	if table != want {
		t.Errorf("IssuesTable mismatch:\ngot:\n%s\nwant:\n%s", table, want)
	}
}

func TestIssuesTableMissingStatus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "003-no-status.md"), "# 003 — No status field\n\nBody text.\n")

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}
	if !strings.Contains(table, "| 003 | [003-no-status.md](003-no-status.md) | No status field | Unknown |") {
		t.Errorf("expected Unknown status fallback, got:\n%s", table)
	}
}

func TestIssuesTableReservedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001-open-task.md"), "# 001 — Open task\n\n**Status**: Open\n")
	writeFile(t, filepath.Join(dir, "002-reserved.md"), "# 002 — Reserved\n\n**Status**: Draft\n\n---\n\nReserved placeholder ticket.\n")

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}
	want := `| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [001-open-task.md](001-open-task.md) | Open task | Open |
| 002 | [002-reserved.md](002-reserved.md) | Reserved | Draft |
`
	if table != want {
		t.Errorf("IssuesTable mismatch with reserved ticket:\ngot:\n%s\nwant:\n%s", table, want)
	}
}

func TestUpdateIssuesReadmeIdempotent(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	writeFile(t, filepath.Join(issuesDir, "001-first-bug.md"), "# 001 — First bug\n\n**Status**: Open\n")

	readme := filepath.Join(issuesDir, "README.md")
	writeFile(t, readme, "# Issues\n\nIntro text.\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n| 001 | [stale.md](stale.md) | stale title | Stale |\n")

	changed, err := UpdateIssuesReadme(readme, issuesDir)
	if err != nil {
		t.Fatalf("UpdateIssuesReadme: %v", err)
	}
	if !changed {
		t.Fatal("expected first run to report changed=true")
	}

	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Intro text.") {
		t.Error("preamble before the table should be preserved")
	}
	if !strings.Contains(string(got), "| 001 | [001-first-bug.md](001-first-bug.md) | First bug | Open |") {
		t.Errorf("expected regenerated row, got:\n%s", got)
	}
	if strings.Contains(string(got), "stale.md") {
		t.Error("stale row should have been replaced")
	}

	// Idempotency: a second run against unchanged sources must report no change.
	changed, err = UpdateIssuesReadme(readme, issuesDir)
	if err != nil {
		t.Fatalf("UpdateIssuesReadme (2nd run): %v", err)
	}
	if changed {
		t.Error("expected second run to report changed=false (idempotent)")
	}
}

func TestUpdateIssuesReadmePreservesTrailingProse(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	writeFile(t, filepath.Join(issuesDir, "001-first-bug.md"), "# 001 — First bug\n\n**Status**: Open\n")

	readme := filepath.Join(issuesDir, "README.md")
	trailing := "\n## Recommended work queue\n\n1. Keep this hand-authored plan.\n"
	writeFile(t, readme, "# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n| 001 | [stale.md](stale.md) | stale | Stale |\n"+trailing)

	changed, err := UpdateIssuesReadme(readme, issuesDir)
	if err != nil {
		t.Fatalf("UpdateIssuesReadme: %v", err)
	}
	if !changed {
		t.Fatal("expected canonical table regeneration to report changed=true")
	}
	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(got), trailing) {
		t.Errorf("trailing prose changed or disappeared:\n%s", got)
	}
	if !strings.Contains(string(got), "[001-first-bug.md](001-first-bug.md)") {
		t.Errorf("canonical table was not regenerated:\n%s", got)
	}
}

func TestUpdateIssuesReadmeRefusesCustomizedColumnsWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	writeFile(t, filepath.Join(issuesDir, "001-first-bug.md"), "# 001 — First bug\n\n**Status**: Open\n")

	readme := filepath.Join(issuesDir, "README.md")
	original := `# Issues

Tracker context that must survive.

| # | File | Title | Status | Priority |
|---|------|-------|--------|----------|
| 001 | [old.md](old.md) | Curated title | Open | P1 |

## WebApp UI track

1. Preserve this project plan exactly.
`
	writeFile(t, readme, original)

	changed, err := UpdateIssuesReadme(readme, issuesDir)
	if err == nil {
		t.Fatal("expected customized table schema to be refused")
	}
	if changed {
		t.Fatal("refused update must report changed=false")
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
}

func TestStudyTopic(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "explicit override wins",
			content: "# Study: Something\n\n<!-- harnez:topic: Curated one-liner -->\n\n**Scope**: Should be ignored\n",
			want:    "Curated one-liner",
		},
		{
			name:    "scope field, single line",
			content: "# Case Study: Something\n\n**Date**: 2026-01-01\n**Scope**: A single line scope\n**Status**: Done\n",
			want:    "A single line scope",
		},
		{
			name:    "scope field wraps across lines without hard breaks",
			content: "# Study: X\n\n**Scope**: First line of the scope\nsecond line continues it\nthird line too.\n\n**Status**: Done\n",
			want:    "First line of the scope second line continues it third line too.",
		},
		{
			name:    "falls back to title, stripping kind and date prefixes",
			content: "# Case Study: 2026-08-16 — Migration and Unification\n\nNo scope field here.\n",
			want:    "Migration and Unification",
		},
		{
			name:    "plain title with no metadata block at all",
			content: "# Worktrees — learnings & TODOs\n\nObservations from running agents.\n",
			want:    "Worktrees — learnings & TODOs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StudyTopic(tt.content)
			if got != tt.want {
				t.Errorf("StudyTopic() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateDocsReadmeStudiesOnly(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	writeFile(t, filepath.Join(docsDir, "studies", "2026-01-01-a.md"), "# Study: A\n\n**Scope**: About A\n")
	writeFile(t, filepath.Join(docsDir, "studies", "2026-01-02-b.md"), "# Study: B\n\n**Scope**: About B\n")

	readme := filepath.Join(docsDir, "README.md")
	orig := `# Docs

Some intro.

**` + "`docs/studies/`" + `** — case studies

| File | Topic |
|------|-------|
| [studies/stale.md](studies/stale.md) | stale topic |

**` + "`docs/feedback/`" + `** — retrospectives

| File | Topic |
|------|-------|
| [feedback/x.md](feedback/x.md) | untouched |
`
	writeFile(t, readme, orig)

	changed, err := UpdateDocsReadme(readme, docsDir)
	if err != nil {
		t.Fatalf("UpdateDocsReadme: %v", err)
	}
	if !changed {
		t.Fatal("expected first run to report changed=true")
	}

	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	gotStr := string(got)

	if !strings.Contains(gotStr, "| [studies/2026-01-01-a.md](studies/2026-01-01-a.md) | About A |") {
		t.Errorf("missing regenerated study row for a.md, got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "| [studies/2026-01-02-b.md](studies/2026-01-02-b.md) | About B |") {
		t.Errorf("missing regenerated study row for b.md, got:\n%s", gotStr)
	}
	if strings.Contains(gotStr, "studies/stale.md") {
		t.Error("stale studies row should have been replaced")
	}
	if !strings.Contains(gotStr, "| [feedback/x.md](feedback/x.md) | untouched |") {
		t.Error("the unrelated feedback table must be left untouched")
	}

	changed, err = UpdateDocsReadme(readme, docsDir)
	if err != nil {
		t.Fatalf("UpdateDocsReadme (2nd run): %v", err)
	}
	if changed {
		t.Error("expected second run to report changed=false (idempotent)")
	}
}

// TestUpdateIssuesReadme_LockContentionFailsFastNotHang verifies the
// non-blocking, bounded-retry flock issue 232 adds around
// UpdateIssuesReadme's read-modify-write: when another holder already has
// the exclusive lock on README.md's sidecar ".lock" file, UpdateIssuesReadme
// must return an error within its bounded retry budget (~250ms) rather than
// blocking indefinitely, and it must leave README.md untouched.
func TestUpdateIssuesReadme_LockContentionFailsFastNotHang(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	writeFile(t, filepath.Join(issuesDir, "001-first-bug.md"), "# 001 — First bug\n\n**Status**: Open\n")

	readme := filepath.Join(issuesDir, "README.md")
	original := "# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"
	writeFile(t, readme, original)

	// Simulate a concurrent holder: open+lock the same sidecar lock file
	// UpdateIssuesReadme will try to acquire.
	lockPath := readme + ".lock"
	holder, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("hold flock: %v", err)
	}
	defer func() {
		syscall.Flock(int(holder.Fd()), syscall.LOCK_UN)
		holder.Close()
	}()

	start := time.Now()
	_, err = UpdateIssuesReadme(readme, issuesDir)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected UpdateIssuesReadme to fail while the lock is held by another process")
	}
	if elapsed > 2*time.Second {
		t.Errorf("UpdateIssuesReadme took %s to fail -- expected a bounded retry, not a long/indefinite block", elapsed)
	}

	got, readErr := os.ReadFile(readme)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != original {
		t.Errorf("README.md was modified despite failing to acquire the lock")
	}
}
