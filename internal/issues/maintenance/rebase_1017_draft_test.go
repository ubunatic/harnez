package maintenance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuesRebaseRealRunEndsWithValidIndexesAndNoManualFix(t *testing.T) {
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
	for _, idx := range []string{"README.md", "BACKLOG.md", "CLOSED.md"} {
		if err := os.WriteFile(filepath.Join(dir, "issues", idx), []byte("# "+strings.TrimSuffix(idx, ".md")+"\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(issuesAttributesLineForTest+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "issues/README.md", "issues/BACKLOG.md", "issues/CLOSED.md", ".gitattributes")
	gitTestRun(t, dir, "commit", "-m", "base")
	base := gitTestRun(t, dir, "rev-parse", "HEAD")

	// Local branch has ticket 1012.
	writeTestTicket(t, dir, 1012, "local")
	commitIssueIndex(t, dir, "local 1012")

	// Upstream has a different ticket 1012 and also 1013.
	gitTestRun(t, dir, "branch", "upstream", base)
	gitTestRun(t, dir, "switch", "upstream")
	writeTestTicket(t, dir, 1012, "upstream")
	commitIssueIndex(t, dir, "upstream 1012")
	writeTestTicket(t, dir, 1013, "upstream-next")
	commitIssueIndex(t, dir, "upstream 1013")

	gitTestRun(t, dir, "switch", "main")
	gitTestRun(t, dir, "config", "merge.harnez-issues-index.driver", "true")

	// 1. Dry run should show collision details
	var dryRunOut bytes.Buffer
	err := runIssuesRebase(&dryRunOut, dir, "upstream", true)
	if err != nil {
		t.Fatalf("runIssuesRebase --dry-run failed: %v\n%s", err, dryRunOut.String())
	}

	gotDry := dryRunOut.String()
	if !strings.Contains(gotDry, "1012") || !strings.Contains(gotDry, "1014") {
		t.Errorf("dry run missing collision details:\ngot:\n%s\nwant:\ncollision details including original (1012) and new target (1014)", gotDry)
	}

	// 2. Real run should succeed without manual intervention
	var realRunOut bytes.Buffer
	err = runIssuesRebase(&realRunOut, dir, "upstream", false)
	if err != nil {
		t.Fatalf("runIssuesRebase failed: %v\n%s", err, realRunOut.String())
	}

	// 3. Real run should end with valid indexes (tested via lint)
	var lintOut bytes.Buffer
	err = runIssuesLint(&lintOut, dir)
	if err != nil {
		t.Fatalf("lint failed after rebase repair, indexes are invalid: %v\n%s", err, lintOut.String())
	}

	// 4. Real run should end with clean working tree
	status := gitTestRun(t, dir, "status", "--porcelain")
	if status != "" {
		t.Errorf("expected clean working directory after rebase repair, got:\n%s", status)
	}
}
