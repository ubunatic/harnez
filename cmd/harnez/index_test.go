package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

func TestRunIndexCheckDetectsAndRestoresAllIssueIndexes(t *testing.T) {
	root := t.TempDir()
	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, status := range map[int]string{1: "Open", 2: "Backlog — later", 3: "Closed — resolved"} {
		path := filepath.Join(issuesDir, fmt.Sprintf("%03d-ticket.md", n))
		content := fmt.Sprintf("# %03d — Ticket\n\n**Status**: %s\n", n, status)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	originals := map[string][]byte{
		"README.md":  []byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"),
		"BACKLOG.md": []byte("stale backlog\n"),
		"CLOSED.md":  []byte("stale closed\n"),
	}
	for name, content := range originals {
		if err := os.WriteFile(filepath.Join(issuesDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := runIndexCheck(&out, filepath.Join(issuesDir, "README.md"), issuesDir, "", "", true, false); err == nil {
		t.Fatal("runIndexCheck accepted index drift")
	}
	for name, want := range originals {
		got, err := os.ReadFile(filepath.Join(issuesDir, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("--check changed %s: got %q, err=%v", name, got, err)
		}
	}
	for _, name := range []string{"README.md", "BACKLOG.md", "CLOSED.md"} {
		if !strings.Contains(out.String(), filepath.Join(issuesDir, name)) {
			t.Errorf("diff output missing %s: %s", name, out.String())
		}
	}
}

func TestRunIndexCheckDetectsAndRestoresDocsStudiesIndex(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	studiesDir := filepath.Join(docsDir, "studies")
	if err := os.MkdirAll(studiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(studiesDir, "fresh.md"), []byte("# Fresh\n\n**Scope**: new topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readmePath := filepath.Join(docsDir, "README.md")
	original := "# Docs\n\n**`docs/studies/`** — case studies\n\n| File | Topic |\n|------|-------|\n| [studies/stale.md](studies/stale.md) | stale |\n"
	if err := os.WriteFile(readmePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runIndexCheck(&out, "", "", readmePath, docsDir, false, true)
	if err == nil || !strings.Contains(out.String(), readmePath) {
		t.Fatalf("runIndexCheck error = %v, output %q; want docs drift", err, out.String())
	}
	got, err := os.ReadFile(readmePath)
	if err != nil || string(got) != original {
		t.Fatalf("--check changed docs README: got %q, err=%v", got, err)
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
	writeIndexRepoConfig(t, repoDir, "harnez", false)
	return repoDir
}

func writeIndexRepoConfig(t *testing.T, dir, tracker string, studies bool) {
	t.Helper()
	config := fmt.Sprintf(`version: 1
issues:
  tracker: %s
  merge_driver: %t
docs:
  studies: %t
  copies: []
rules:
  quota: false
make:
  container_test: false
`, tracker, tracker == "harnez", studies)
	path := filepath.Join(dir, ".harnez", "repo.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunIndexSkipsForeignIssueTracker(t *testing.T) {
	repoDir := t.TempDir()
	issuesDir := filepath.Join(repoDir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(issuesDir, "README.md")
	original := []byte("# Foreign tracker\n\nOwner-maintained index.\n")
	if err := os.WriteFile(readme, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "001-ticket.md"), []byte("# 001 — Ticket\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeIndexRepoConfig(t, repoDir, "none", false)

	var out bytes.Buffer
	if err := runIndex(&out, indexOptions{Dir: repoDir}); err != nil {
		t.Fatalf("runIndex: %v", err)
	}
	got, err := os.ReadFile(readme)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("foreign issue index changed: got=%q err=%v", got, err)
	}
	if !strings.Contains(out.String(), "nothing to do") {
		t.Fatalf("missing no-op note: %q", out.String())
	}
}

func TestRunIndexGatesIssuesAndStudiesIndependently(t *testing.T) {
	t.Run("studies only", func(t *testing.T) {
		repoDir := t.TempDir()
		issuesDir := filepath.Join(repoDir, "issues")
		studiesDir := filepath.Join(repoDir, "docs", "studies")
		if err := os.MkdirAll(issuesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(studiesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		issuesReadme := []byte("# Foreign tracker\n\nKeep me.\n")
		if err := os.WriteFile(filepath.Join(issuesDir, "README.md"), issuesReadme, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(issuesDir, "001-ticket.md"), []byte("# 001 — Ticket\n\n**Status**: Open\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(studiesDir, "fresh.md"), []byte("# Fresh study\n\n**Scope**: new topic\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		docsReadme := filepath.Join(repoDir, "docs", "README.md")
		if err := os.WriteFile(docsReadme, []byte("# Docs\n\n**`docs/studies/`** — case studies\n\n| File | Topic |\n|------|-------|\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeIndexRepoConfig(t, repoDir, "none", true)

		var out bytes.Buffer
		if err := runIndex(&out, indexOptions{Dir: repoDir}); err != nil {
			t.Fatalf("runIndex: %v", err)
		}
		gotIssues, err := os.ReadFile(filepath.Join(issuesDir, "README.md"))
		if err != nil || !bytes.Equal(gotIssues, issuesReadme) {
			t.Errorf("issues index changed with tracker none: got=%q err=%v", gotIssues, err)
		}
		gotDocs, err := os.ReadFile(docsReadme)
		if err != nil || !strings.Contains(string(gotDocs), "studies/fresh.md") {
			t.Errorf("studies index was not updated: got=%q err=%v", gotDocs, err)
		}
	})

	t.Run("issues only", func(t *testing.T) {
		repoDir := indexSnapshotFixtureDir(t)
		docsReadme := filepath.Join(repoDir, "docs", "README.md")
		if err := os.MkdirAll(filepath.Dir(docsReadme), 0o755); err != nil {
			t.Fatal(err)
		}
		docsOriginal := []byte("# Owner docs index\n\nDo not rewrite.\n")
		if err := os.WriteFile(docsReadme, docsOriginal, 0o644); err != nil {
			t.Fatal(err)
		}

		var out bytes.Buffer
		if err := runIndex(&out, indexOptions{Dir: repoDir, DBPath: filepath.Join(t.TempDir(), "telemetry.sqlite")}); err != nil {
			t.Fatalf("runIndex: %v", err)
		}
		gotDocs, err := os.ReadFile(docsReadme)
		if err != nil || !bytes.Equal(gotDocs, docsOriginal) {
			t.Errorf("docs index changed with studies disabled: got=%q err=%v", gotDocs, err)
		}
		gotIssues, err := os.ReadFile(filepath.Join(repoDir, "issues", "README.md"))
		if err != nil || !strings.Contains(string(gotIssues), "First") {
			t.Errorf("issue index was not updated: got=%q err=%v", gotIssues, err)
		}
	})
}

func TestRunIndexWithoutEnabledIndexesPrintsNote(t *testing.T) {
	var out bytes.Buffer
	if err := runIndex(&out, indexOptions{Dir: t.TempDir()}); err != nil {
		t.Fatalf("runIndex: %v", err)
	}
	if !strings.Contains(out.String(), "nothing to do") {
		t.Fatalf("missing no-op note: %q", out.String())
	}
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
	writeIndexRepoConfig(t, repoDir, "harnez", false)

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

func TestRunIndexStageStagesOnlyRewrittenIndex(t *testing.T) {
	dir := newStageIndexFixture(t, true)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "studies"), 0o755); err != nil {
		t.Fatal(err)
	}
	study := filepath.Join(dir, "docs", "studies", "new-study.md")
	if err := os.WriteFile(study, []byte("# New Study\n\n**Scope**: Fresh topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIndexTestRun(t, dir, "add", "docs/studies/new-study.md")

	var out bytes.Buffer
	if err := runIndex(&out, indexOptions{Dir: dir, Stage: true}); err != nil {
		t.Fatalf("runIndex --stage: %v", err)
	}
	got := gitIndexTestRun(t, dir, "diff", "--cached", "--name-only")
	want := "docs/README.md\ndocs/studies/new-study.md\n"
	if got != want {
		t.Fatalf("staged paths = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "staged docs/README.md") {
		t.Errorf("output did not report staged index: %q", out.String())
	}
}

func TestRunIndexStagePreservesOtherStagedAndUnstagedChanges(t *testing.T) {
	dir := newStageIndexFixture(t, true)
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIndexTestRun(t, dir, "add", "docs/README.md", "tracked.txt")
	gitIndexTestRun(t, dir, "commit", "-m", "baseline")

	if err := os.WriteFile(filepath.Join(dir, "docs", "studies", "new-study.md"), []byte("# New Study\n\n**Scope**: Fresh topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIndexTestRun(t, dir, "add", "docs/studies/new-study.md", "staged.txt")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runIndex(&bytes.Buffer{}, indexOptions{Dir: dir, Stage: true}); err != nil {
		t.Fatalf("runIndex --stage: %v", err)
	}
	gotCached := gitIndexTestRun(t, dir, "diff", "--cached", "--name-only")
	wantCached := "docs/README.md\ndocs/studies/new-study.md\nstaged.txt\n"
	if gotCached != wantCached {
		t.Fatalf("cached paths = %q, want %q", gotCached, wantCached)
	}
	gotUnstaged := gitIndexTestRun(t, dir, "diff", "--name-only")
	if gotUnstaged != "tracked.txt\n" {
		t.Fatalf("unstaged paths = %q, want tracked.txt", gotUnstaged)
	}
}

func TestRunIndexStageNoChangesStagesNothing(t *testing.T) {
	dir := newStageIndexFixture(t, true)
	if err := os.WriteFile(filepath.Join(dir, "docs", "studies", "current.md"), []byte("# Current\n\n**Scope**: Existing topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIndex(&bytes.Buffer{}, indexOptions{Dir: dir}); err != nil {
		t.Fatalf("initial runIndex: %v", err)
	}
	var out bytes.Buffer
	if err := runIndex(&out, indexOptions{Dir: dir, Stage: true}); err != nil {
		t.Fatalf("no-op runIndex --stage: %v", err)
	}
	if !strings.Contains(out.String(), "no index files changed; nothing staged") {
		t.Fatalf("no-op output = %q", out.String())
	}
	if got := gitIndexTestRun(t, dir, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("no-op staged paths = %q", got)
	}
}

func TestRunIndexStageRequiresGitRepository(t *testing.T) {
	dir := newStageIndexFixture(t, false)
	var out bytes.Buffer
	err := runIndex(&out, indexOptions{Dir: dir, Stage: true})
	if err == nil || !strings.Contains(err.Error(), "is not inside a Git repository") {
		t.Fatalf("runIndex --stage error = %v, want clear Git repository error", err)
	}
}

func newStageIndexFixture(t *testing.T, initGit bool) string {
	t.Helper()
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	studies := filepath.Join(docs, "studies")
	if err := os.MkdirAll(studies, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "<!-- managed by harnez index: only the studies table is generated; evergreen documentation and other tables are hand-written -->\n# Docs\n\n**`docs/studies/`** — case studies\n\n<!-- harnez:begin docs-studies-index -->\n| File | Topic |\n|------|-------|\n<!-- harnez:end docs-studies-index -->\n"
	if err := os.WriteFile(filepath.Join(docs, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	writeIndexRepoConfig(t, dir, "none", true)
	if initGit {
		gitIndexTestRun(t, dir, "init", "-q")
		gitIndexTestRun(t, dir, "config", "user.name", "Harnez Test")
		gitIndexTestRun(t, dir, "config", "user.email", "harnez-test@example.invalid")
	}
	return dir
}

func gitIndexTestRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
