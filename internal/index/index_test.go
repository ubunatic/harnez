package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
