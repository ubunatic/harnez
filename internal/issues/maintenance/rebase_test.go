package maintenance

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	indexpkg "ubunatic.com/harnez/internal/index"
)

func gitTestRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeTestTicket(t *testing.T, dir string, n int, slug string) {
	t.Helper()
	path := filepath.Join(dir, "issues", fmt.Sprintf("%03d-%s.md", n, slug))
	content := fmt.Sprintf("# %03d — %s\n\n**Status**: Open\n", n, slug)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitIssueIndex(t *testing.T, dir, message string) {
	t.Helper()
	if _, err := indexpkg.UpdateIssuesReadme(filepath.Join(dir, "issues", "README.md"), filepath.Join(dir, "issues")); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "issues")
	gitTestRun(t, dir, "commit", "-m", message)
}

func TestIssuesRebaseRepairsIndependentTicketCollisions(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o777)
			}
			return nil
		})
	})
	gitTestRun(t, dir, "init", "-b", "main")
	gitTestRun(t, dir, "config", "user.name", "Test")
	gitTestRun(t, dir, "config", "user.email", "test@example.invalid")
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issues", "README.md"), []byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(issuesAttributesLineForTest+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unstaged.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "issues/README.md", ".gitattributes", "staged.txt", "unstaged.txt")
	gitTestRun(t, dir, "commit", "-m", "base")
	base := gitTestRun(t, dir, "rev-parse", "HEAD")

	writeTestTicket(t, dir, 270, "local-a")
	commitIssueIndex(t, dir, "local 270")
	writeTestTicket(t, dir, 271, "local-b")
	commitIssueIndex(t, dir, "local 271")

	gitTestRun(t, dir, "branch", "upstream", base)
	gitTestRun(t, dir, "switch", "upstream")
	for n := 270; n <= 276; n++ {
		writeTestTicket(t, dir, n, fmt.Sprintf("canonical-%d", n))
	}
	commitIssueIndex(t, dir, "canonical 270 through 276")
	gitTestRun(t, dir, "switch", "main")
	gitTestRun(t, dir, "config", "merge.harnez-issues-index.driver", "true")
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged user change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(dir, "unstaged.txt"), []byte("unstaged user change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantStaged := gitTestRun(t, dir, "diff", "--cached", "--", "staged.txt")
	wantUnstaged := gitTestRun(t, dir, "diff", "--", "unstaged.txt")

	var out bytes.Buffer
	if err := runIssuesRebase(&out, dir, "upstream", false); err != nil {
		t.Fatalf("runIssuesRebase: %v\n%s", err, out.String())
	}
	for _, path := range []string{"issues/277-local-a.md", "issues/278-local-b.md"} {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatalf("missing repaired %s: %v", path, err)
		}
		want := "# " + filepath.Base(path)[:3] + " —"
		if !strings.HasPrefix(string(data), want) {
			t.Errorf("%s header = %q, want prefix %q", path, strings.SplitN(string(data), "\n", 2)[0], want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "issues", "270-local-a.md")); !os.IsNotExist(err) {
		t.Errorf("old local ticket survived: %v", err)
	}
	if err := runIssuesLint(&out, dir); err != nil {
		t.Fatalf("lint after repair: %v", err)
	}
	if got := gitTestRun(t, dir, "diff", "--cached", "--", "staged.txt"); got != wantStaged {
		t.Errorf("unrelated staged change was altered:\ngot:\n%s\nwant:\n%s", got, wantStaged)
	}
	if got := gitTestRun(t, dir, "diff", "--", "unstaged.txt"); got != wantUnstaged {
		t.Errorf("unrelated unstaged change was altered:\ngot:\n%s\nwant:\n%s", got, wantUnstaged)
	}
}

func TestIssuesLintChecksBacklogAndClosedIndexes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	for n, status := range map[int]string{1: "Open", 2: "Backlog — later", 3: "Closed — resolved"} {
		path := filepath.Join(dir, "issues", fmt.Sprintf("%03d-ticket.md", n))
		content := fmt.Sprintf("# %03d — Ticket\n\n**Status**: %s\n", n, status)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := indexpkg.UpdateIssuesReadme(filepath.Join(dir, "issues", "README.md"), filepath.Join(dir, "issues")); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err != nil {
		t.Fatalf("runIssuesLint on generated indexes: %v", err)
	}
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(filepath.Join(docsDir, "studies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "studies", "study.md"), []byte("# Study\n\n**Scope**: Fresh topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docsReadme := filepath.Join(docsDir, "README.md")
	staleDocs := "<!-- managed by harnez index: only the studies table is generated; evergreen documentation and other tables are hand-written -->\n# Docs\n\n**`docs/studies/`** — case studies\n\n<!-- harnez:begin docs-studies-index -->\n| File | Topic |\n|------|-------|\n| [studies/stale.md](studies/stale.md) | stale |\n<!-- harnez:end docs-studies-index -->\n"
	if err := os.WriteFile(docsReadme, []byte(staleDocs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err == nil || !strings.Contains(err.Error(), "docs/README.md is generated studies index drift; run 'harnez index --stage'") {
		t.Fatalf("runIssuesLint error = %v, want docs/README.md drift", err)
	}
	if _, err := indexpkg.UpdateDocsReadme(docsReadme, docsDir); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err != nil {
		t.Fatalf("runIssuesLint after docs index regeneration: %v", err)
	}
	backlog := filepath.Join(dir, "issues", "BACKLOG.md")
	if err := os.WriteFile(backlog, []byte("# Backlog\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err == nil || !strings.Contains(err.Error(), "BACKLOG.md") {
		t.Fatalf("runIssuesLint error = %v, want BACKLOG.md drift", err)
	}
	unknown := filepath.Join(dir, "issues", "004-unknown.md")
	if err := os.WriteFile(unknown, []byte("# 004 — Unknown\n\n**Status**: Pending QA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := indexpkg.UpdateIssuesReadme(filepath.Join(dir, "issues", "README.md"), filepath.Join(dir, "issues")); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err == nil || !strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("runIssuesLint error = %v, want unknown status diagnostic", err)
	}
}

func TestRunIssuesLintWarnsWhenActiveCapExceeded(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 21; n++ {
		writeTestTicket(t, dir, n, fmt.Sprintf("ticket-%02d", n))
	}
	if _, err := indexpkg.UpdateIssuesReadme(filepath.Join(issuesDir, "README.md"), issuesDir); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runIssuesLint(&out, dir); err != nil {
		t.Fatalf("runIssuesLint: %v", err)
	}
	if !strings.Contains(out.String(), "warning: 21 active tickets exceeds configured cap 20") {
		t.Fatalf("missing active cap warning: %q", out.String())
	}
	out.Reset()
	if err := runIssuesStatus(&out, dir); err != nil {
		t.Fatalf("runIssuesStatus: %v", err)
	}
	if !strings.Contains(out.String(), "warning: 21 active tickets exceeds configured cap 20") {
		t.Fatalf("status missing active cap warning: %q", out.String())
	}
}

const issuesAttributesLineForTest = "issues/README.md merge=harnez-issues-index"

func TestRunIssuesLintCachedNamesUntrackedTicketWhenIndexIncludesIt(t *testing.T) {
	dir := t.TempDir()
	gitTestRun(t, dir, "init", "-b", "main")
	gitTestRun(t, dir, "config", "user.name", "Test")
	gitTestRun(t, dir, "config", "user.email", "test@example.invalid")
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issues", "README.md"), []byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestTicket(t, dir, 1, "tracked")
	commitIssueIndex(t, dir, "base")
	writeTestTicket(t, dir, 2, "untracked")
	if _, err := indexpkg.UpdateIssuesReadme(filepath.Join(dir, "issues", "README.md"), filepath.Join(dir, "issues")); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "issues/README.md")

	var out bytes.Buffer
	err := runIssuesLintCached(&out, dir)
	if err == nil || !strings.Contains(err.Error(), "issues/002-untracked.md") || !strings.Contains(err.Error(), "staged ticket set") {
		t.Fatalf("cached lint error = %v, want actionable untracked-ticket diagnostic", err)
	}
}

func TestIssuesRebaseMissingIntegrationShowsExactSetupCommand(t *testing.T) {
	dir := t.TempDir()
	gitTestRun(t, dir, "init", "-b", "main")
	err := runIssuesRebase(&bytes.Buffer{}, dir, "main", false)
	if err == nil {
		t.Fatal("expected missing-integration error")
	}
	if !strings.Contains(err.Error(), "harnez init --issues-git") {
		t.Fatalf("error does not contain exact setup command: %v", err)
	}
}

func TestIssuesMergeDriverRegeneratesCurrentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current")
	readme, err := os.ReadFile(filepath.Join("..", "..", "..", "issues", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, readme, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesMergeDriver(path, "issues/README.md"); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesMergeDriver(filepath.Join(t.TempDir(), "missing"), "issues/README.md"); err == nil {
		t.Fatal("expected missing-current-version error")
	}
}

func TestIssuesMergeDriverReplacesConflictInsideDocsBlock(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	conflicted := strings.Replace(string(original), "<!-- harnez:begin docs-studies-index -->\n", "<!-- harnez:begin docs-studies-index -->\n<<<<<<< ours\n| File | Topic |\n=======\n| conflict | conflict |\n>>>>>>> theirs\n", 1)
	path := filepath.Join(t.TempDir(), "current")
	if err := os.WriteFile(path, []byte(conflicted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesMergeDriver(path, "docs/README.md"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "<<<<<<<") || !strings.Contains(string(got), "<!-- harnez:begin docs-studies-index -->") {
		t.Fatalf("driver left generated-section conflict unresolved:\n%s", got)
	}
}

func TestIssuesMergeDriverRejectsConflictOutsideDocsBlock(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	conflicted := strings.Replace(string(original), "# Docs — Evergreen Project Docs", "# Docs — Evergreen Project Docs\n\n<<<<<<< ours\nhand-written conflict\n=======\nother version\n>>>>>>> theirs", 1)
	path := filepath.Join(t.TempDir(), "current")
	if err := os.WriteFile(path, []byte(conflicted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesMergeDriver(path, "docs/README.md"); err == nil || !strings.Contains(err.Error(), "unresolved conflict outside generated block") {
		t.Fatalf("driver error = %v, want unresolved hand-written conflict", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != conflicted {
		t.Fatalf("driver changed unresolved hand-written conflict:\n%s", got)
	}
}

func TestIssuesRebaseRefusedOnProtectedMain(t *testing.T) {
	dir := t.TempDir()
	gitTestRun(t, dir, "init", "-b", "main")
	gitTestRun(t, dir, "config", "user.name", "Test")
	gitTestRun(t, dir, "config", "user.email", "test@example.invalid")
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		"issues/README.md":  "# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n",
		".gitattributes":    issuesAttributesLineForTest + "\n",
		".harnez/repo.yaml": "version: 1\nissues:\n  tracker: harnez\n  merge_driver: true\ndocs:\n  studies: false\n  copies: []\nrules:\n  quota: false\nmake:\n  container_test: false\ngit:\n  mode: team\n  protected_main: true\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitTestRun(t, dir, "add", ".")
	gitTestRun(t, dir, "commit", "-m", "base")
	gitTestRun(t, dir, "config", "merge.harnez-issues-index.driver", "true")
	var out bytes.Buffer
	err := runIssuesRebase(&out, dir, "main", false)
	if err == nil || !strings.Contains(err.Error(), "git.protected_main") {
		t.Fatalf("expected a protected_main refusal, got %v", err)
	}
	if err := runIssuesRebase(&out, dir, "main", true); err != nil && strings.Contains(err.Error(), "git.protected_main") {
		t.Fatalf("--dry-run must not be guarded: %v", err)
	}
}
