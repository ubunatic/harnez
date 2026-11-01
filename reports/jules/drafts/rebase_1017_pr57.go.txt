package maintenance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuesRebaseDryRunPrintsCollisionPlan(t *testing.T) {
	dir := t.TempDir()
	gitTestRun(t, dir, "init", "-b", "main")
	gitTestRun(t, dir, "config", "user.name", "Test")
	gitTestRun(t, dir, "config", "user.email", "test@example.invalid")

	issuesDir := filepath.Join(dir, "issues")
	if err := os.Mkdir(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "README.md"), []byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(issuesAttributesLineForTest+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "issues/README.md", ".gitattributes")
	gitTestRun(t, dir, "commit", "-m", "base")
	base := gitTestRun(t, dir, "rev-parse", "HEAD")

	// Upstream
	gitTestRun(t, dir, "branch", "upstream", base)
	gitTestRun(t, dir, "switch", "upstream")
	writeTestTicket(t, dir, 1012, "upstream-ticket")
	commitIssueIndex(t, dir, "upstream 1012")

	// Local
	gitTestRun(t, dir, "switch", "main")
	writeTestTicket(t, dir, 1012, "local-ticket")
	commitIssueIndex(t, dir, "local 1012")

	gitTestRun(t, dir, "config", "merge.harnez-issues-index.driver", "true")

	var out bytes.Buffer
	err := runIssuesRebase(&out, dir, "upstream", true)
	if err != nil {
		t.Fatalf("runIssuesRebase --dry-run failed: %v", err)
	}

	got := out.String()
	wantOurs := "issues/1012-local-ticket.md"
	wantTheirs := "issues/1012-upstream-ticket.md"

	if !strings.Contains(got, wantOurs) || !strings.Contains(got, wantTheirs) || !strings.Contains(got, "1013") {
		t.Errorf("dry run output did not list collision plan (ours, theirs, new number).\ngot:\n%s\nwant output containing %q, %q, and \"1013\"", got, wantOurs, wantTheirs)
	}
}
