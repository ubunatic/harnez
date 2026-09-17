package main

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

func TestIssuesMergeDriverOnlyValidatesCurrentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current")
	readme, err := os.ReadFile(filepath.Join("..", "..", "issues", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, readme, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesMergeDriver(path); err != nil {
		t.Fatal(err)
	}
	if err := runIssuesMergeDriver(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing-current-version error")
	}
}
