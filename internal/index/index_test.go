package index

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/issues"
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
	writeFile(t, filepath.Join(dir, "003-backlog.md"), "# 003 — Backlog\n\n**Status**: Backlog — later\n")

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}

	want := `| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [001-first-bug.md](001-first-bug.md) | First bug | Open |
`
	if table != want {
		t.Errorf("IssuesTable mismatch:\ngot:\n%s\nwant:\n%s", table, want)
	}
}

func TestIssueIndexFilesPartitionStatusesAndArchive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001-open.md"), "# 001 — Open\n\n**Status**: Blocked — waiting\n")
	writeFile(t, filepath.Join(dir, "002-draft.md"), "# 002 — Draft\n\n**Status**: Draft\n")
	writeFile(t, filepath.Join(dir, "003-backlog.md"), "# 003 — Backlog\n\n**Status**: Backlog — later\n")
	writeFile(t, filepath.Join(dir, "archive", "004-closed.md"), "# 004 — Closed\n\n**Status**: Closed — resolved\n")

	indexes, err := IssueIndexFiles(dir)
	if err != nil {
		t.Fatalf("IssueIndexFiles: %v", err)
	}
	for name, want := range map[string][]string{
		"README.md":  {"001-open.md", "002-draft.md"},
		"BACKLOG.md": {"003-backlog.md"},
		"CLOSED.md":  {"archive/004-closed.md"},
	} {
		for _, path := range want {
			if !strings.Contains(indexes[name], path) {
				t.Errorf("%s missing %s:\n%s", name, path, indexes[name])
			}
		}
	}
	for name, excluded := range map[string][]string{
		"README.md":  {"003-backlog.md", "archive/004-closed.md"},
		"BACKLOG.md": {"001-open.md", "002-draft.md", "archive/004-closed.md"},
		"CLOSED.md":  {"001-open.md", "002-draft.md", "003-backlog.md"},
	} {
		for _, path := range excluded {
			if strings.Contains(indexes[name], path) {
				t.Errorf("%s unexpectedly contains %s:\n%s", name, path, indexes[name])
			}
		}
	}
}

func TestIssueIndexFilesHaveTopNoticeAndManagedBlock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001-open.md"), "# 001 — Open\n\n**Status**: Open\n")
	indexes, err := IssueIndexFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]string{
		"README.md":  "issues-active-index",
		"BACKLOG.md": "issues-backlog-index",
		"CLOSED.md":  "issues-closed-index",
	} {
		content := indexes[name]
		if !strings.HasPrefix(content, "<!-- managed by harnez index:") {
			t.Errorf("%s missing top notice: %q", name, content)
		}
		if !strings.Contains(content, "<!-- harnez:begin "+id+" -->") || !strings.Contains(content, "<!-- harnez:end "+id+" -->") {
			t.Errorf("%s missing generated block %q: %s", name, id, content)
		}
	}
}

func TestIssueIndexFilesOmitsUnknownStatus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001-unknown.md"), "# 001 — Unknown\n\n**Status**: Pending QA\n")
	indexes, err := IssueIndexFiles(dir)
	if err != nil {
		t.Fatalf("IssueIndexFiles: %v", err)
	}
	for name, content := range indexes {
		if strings.Contains(content, "001-unknown.md") {
			t.Errorf("%s unexpectedly lists unknown status ticket:\n%s", name, content)
		}
	}
}

func TestIssuesTableMissingStatus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "003-no-status.md"), "# 003 — No status field\n\nBody text.\n")

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}
	if strings.Contains(table, "003-no-status.md") {
		t.Errorf("unknown-status ticket must not appear in the active table, got:\n%s", table)
	}
}

func TestStatusCountsIncludesBacklog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001-open.md"), "# 001 — Open\n\n**Status**: Open\n")
	writeFile(t, filepath.Join(dir, "002-closed.md"), "# 002 — Closed\n\n**Status**: Closed — done\n")
	writeFile(t, filepath.Join(dir, "003-draft.md"), "# 003 — Draft\n\n**Status**: Draft\n")
	writeFile(t, filepath.Join(dir, "004-backlog.md"), "# 004 — Backlog\n\n**Status**: Backlog\n")
	writeFile(t, filepath.Join(dir, "005-unknown.md"), "# 005 — Unknown\n\n**Status**: In Review\n")

	open, closed, draft, backlog, unknown, err := StatusCounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if open != 2 || closed != 1 || draft != 1 || backlog != 1 || unknown != 0 {
		t.Fatalf("StatusCounts = open:%d closed:%d draft:%d backlog:%d unknown:%d", open, closed, draft, backlog, unknown)
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

// TestIssuesTablePipeInTitleRoundTripsThroughLint covers issue 239: a
// ticket title containing a literal "|" must be escaped on write so the
// generated table row still parses as exactly one row, and must not
// produce spurious Lint diagnostics (e.g. a false "unindexed ticket file"
// from the title's pipe being read back as a column separator).
func TestIssuesTablePipeInTitleRoundTripsThroughLint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "233-pipe-title.md"), "# 233 — docs/practices|lang|other source\n\n**Status**: Open\n")

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}
	if !strings.Contains(table, `docs/practices\|lang\|other source`) {
		t.Fatalf("expected escaped pipes in generated table, got:\n%s", table)
	}

	writeFile(t, filepath.Join(dir, "README.md"), "# Issues\n\n"+table)

	report, err := issues.Lint(dir)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if len(report.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for pipe-in-title round trip, got %d: %+v", len(report.Diagnostics), report.Diagnostics)
	}
}

// TestIssuesTablePunctuationInTitleRoundTripsThroughLint covers issue 346: a
// ticket title containing colons, parens, and slashes must round-trip cleanly
// between IssuesTable and issues.Lint without producing false-positive
// unindexed file diagnostics or broken links.
func TestIssuesTablePunctuationInTitleRoundTripsThroughLint(t *testing.T) {
	dir := t.TempDir()
	ticketPath := filepath.Join(dir, "040-treemap-theming-theme-1-current-vs-theme-2-quad-halfblock-sub-cell-rendering.md")
	ticketContent := `# 040 — Treemap theming: theme 1 (current) vs. theme 2 (quad/halfblock sub-cell rendering)

**Status**: Open
**Priority**: P3 (Low)

---

## 1. Problem
`
	writeFile(t, ticketPath, ticketContent)

	table, err := IssuesTable(dir)
	if err != nil {
		t.Fatalf("IssuesTable: %v", err)
	}

	writeFile(t, filepath.Join(dir, "README.md"), "# Issues\n\n"+table)

	report, err := issues.Lint(dir)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if len(report.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for punctuation-in-title round trip, got %d: %+v", len(report.Diagnostics), report.Diagnostics)
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

func TestRenderIssuesReadmeMigratesLegacyTableAndReplacesEditedBlock(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	writeFile(t, filepath.Join(issuesDir, "001-first-bug.md"), "# 001 — First bug\n\n**Status**: Open\n")
	readme := "# Issues\n\nIntro text.\n\n[Backlog](BACKLOG.md) · [Closed](CLOSED.md)\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n| 001 | [edited.md](edited.md) | hand edit | Open |\n\n## Hand-written\n\nKeep these bytes.\n"
	got, err := RenderIssuesReadme(readme, issuesDir, filepath.Join(issuesDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "<!-- managed by harnez index:") {
		t.Fatalf("missing first-line notice: %q", got)
	}
	if !strings.Contains(got, "<!-- harnez:begin issues-active-index -->\n| # | File | Title | Status |\n") || !strings.Contains(got, "<!-- harnez:end issues-active-index -->") {
		t.Fatalf("legacy table was not wrapped: %s", got)
	}
	if !strings.Contains(got, "| 001 | [001-first-bug.md](001-first-bug.md) | First bug | Open |") {
		t.Fatalf("table was not regenerated: %s", got)
	}
	if !strings.HasSuffix(got, "## Hand-written\n\nKeep these bytes.\n") {
		t.Fatalf("bytes outside generated table changed: %q", got)
	}

	edited := strings.Replace(got, "First bug", "hand edited inside marker", 1)
	withConflict := strings.Replace(edited, "hand edited inside marker", "<<<<<<< ours\nhand edited inside marker\n=======\ntheirs\n>>>>>>> theirs", 1)
	regenerated, err := RenderIssuesReadme(withConflict, issuesDir, filepath.Join(issuesDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(regenerated, "<<<<<<<") || !strings.Contains(regenerated, "First bug") {
		t.Fatalf("marked generated content was not replaced cleanly: %s", regenerated)
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

func TestHasConcurrentEditsOutsideManagedBlock(t *testing.T) {
	base := "preface\n<!-- harnez:begin studies -->\nold table\n<!-- harnez:end studies -->\ntail\n"
	oursGenerated := "preface\n<!-- harnez:begin studies -->\nours table\n<!-- harnez:end studies -->\ntail\n"
	theirsGenerated := "preface\n<!-- harnez:begin studies -->\ntheirs table\n<!-- harnez:end studies -->\ntail\n"
	if HasConcurrentEditsOutsideManagedBlock(base, oursGenerated, theirsGenerated, "studies") {
		t.Fatal("generated block changes should not be considered hand-written conflicts")
	}
	oursHandwritten := strings.Replace(base, "preface", "ours preface", 1)
	theirsHandwritten := strings.Replace(base, "preface", "theirs preface", 1)
	if !HasConcurrentEditsOutsideManagedBlock(base, oursHandwritten, theirsHandwritten, "studies") {
		t.Fatal("divergent hand-written changes should require normal Git conflict handling")
	}
	if HasConcurrentEditsOutsideManagedBlock(base, oursHandwritten, oursHandwritten, "studies") {
		t.Fatal("identical hand-written changes should not be considered conflicting")
	}
}

func TestRenderDocsReadmeReplacesConflictedMarkedSectionAndPreservesOutsideBytes(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	writeFile(t, filepath.Join(docsDir, "studies", "fresh.md"), "# Study: Fresh\n\n**Scope**: regenerated row\n")
	path := filepath.Join(docsDir, "README.md")
	prefix := docsStudiesNotice + "\n# Docs\r\n\r\nHand-written preface.\r\n\r\n**`docs/studies/`** — case studies\r\n\r\n<!-- harnez:begin docs-studies-index -->\r\n"
	conflicted := prefix + "<<<<<<< ours\r\n| stale | hand edited |\r\n=======\r\n| theirs | conflict |\r\n>>>>>>> theirs\r\n<!-- harnez:end docs-studies-index -->\r\n\r\n**Hand-written table**\r\n\r\nKeep exactly.\r\n"
	got, err := RenderDocsReadme(conflicted, docsDir, path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<<<<<<<") || !strings.Contains(got, "| [studies/fresh.md](studies/fresh.md) | regenerated row |") {
		t.Fatalf("generated conflict was not resolved from source: %s", got)
	}
	if !strings.Contains(got, "# Docs\r\n\r\nHand-written preface.\r\n") || !strings.HasSuffix(got, "\r\n\r\n**Hand-written table**\r\n\r\nKeep exactly.\r\n") {
		t.Fatalf("bytes outside generated section changed: %q", got)
	}
}

func TestUpdateDocsReadmeAppendsMissingStudiesSection(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	readme := filepath.Join(docsDir, "README.md")
	writeFile(t, readme, "# Docs\n\nProject docs.\n")
	writeFile(t, filepath.Join(docsDir, "studies", "example.md"), "# Study: Example\n\n**Scope**: A generated row\n")

	changed, err := UpdateDocsReadme(readme, docsDir)
	if err != nil {
		t.Fatalf("UpdateDocsReadme: %v", err)
	}
	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !strings.Contains(string(got), "**`docs/studies/`** — case studies") || !strings.Contains(string(got), "| [studies/example.md](studies/example.md) | A generated row |") {
		t.Fatalf("missing appended studies section or index: changed=%v\n%s", changed, got)
	}
}

func TestUpdateDocsReadmeCreatesMissingStudiesDirectory(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	readme := filepath.Join(docsDir, "README.md")
	writeFile(t, readme, "# Docs\n\nProject docs.\n")

	changed, err := UpdateDocsReadme(readme, docsDir)
	if err != nil {
		t.Fatalf("UpdateDocsReadme: %v", err)
	}
	if !changed {
		t.Fatal("expected missing section to be appended")
	}
	if info, err := os.Stat(filepath.Join(docsDir, "studies")); err != nil || !info.IsDir() {
		t.Fatalf("studies directory not created: info=%v err=%v", info, err)
	}
}

func TestUpdateDocsReadmeStudiesTableAtEOF(t *testing.T) {
	for _, finalNewline := range []bool{false, true} {
		name := "without final newline"
		ending := ""
		if finalNewline {
			name = "with final newline"
			ending = "\n"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			docsDir := filepath.Join(dir, "docs")
			study := "# Study: Example\n\n**Scope**: Generated topic\n"
			writeFile(t, filepath.Join(docsDir, "studies", "example.md"), study)
			readme := filepath.Join(docsDir, "README.md")
			content := "# Docs\n\n**`docs/studies/`** — case studies\n\n| File | Topic |\n|------|-------|\n| [studies/stale.md](studies/stale.md) | stale |" + ending
			writeFile(t, readme, content)

			changed, err := UpdateDocsReadme(readme, docsDir)
			if err != nil {
				t.Fatalf("UpdateDocsReadme: %v", err)
			}
			got, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			want := docsStudiesNotice + "\n# Docs\n\n**`docs/studies/`** — case studies\n\n<!-- harnez:begin docs-studies-index -->\n| File | Topic |\n|------|-------|\n| [studies/example.md](studies/example.md) | Generated topic |\n<!-- harnez:end docs-studies-index -->" + ending
			if !changed || string(got) != want {
				t.Fatalf("UpdateDocsReadme() = changed %v, content %q; want changed true, content %q", changed, got, want)
			}
		})
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
	if elapsed > 2*time.Second*raceFactor {
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
