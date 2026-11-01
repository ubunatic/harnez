package maintenance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuesRebaseDryRunListsCollisions1017(t *testing.T) {
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
	gitTestRun(t, dir, "add", "issues/README.md", ".gitattributes")
	gitTestRun(t, dir, "commit", "-m", "base")
	base := gitTestRun(t, dir, "rev-parse", "HEAD")

	writeTestTicket(t, dir, 1010, "local-issue")
	commitIssueIndex(t, dir, "local 1010")

	gitTestRun(t, dir, "branch", "upstream", base)
	gitTestRun(t, dir, "switch", "upstream")
	writeTestTicket(t, dir, 1010, "upstream-issue")
	commitIssueIndex(t, dir, "upstream 1010")

	gitTestRun(t, dir, "switch", "main")
	gitTestRun(t, dir, "config", "merge.harnez-issues-index.driver", "true")

	var out bytes.Buffer
	err := runIssuesRebase(&out, dir, "upstream", true)
	if err != nil {
		t.Fatalf("runIssuesRebase failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "1010") || !strings.Contains(got, "local-issue") || !strings.Contains(got, "upstream-issue") || !strings.Contains(got, "1011") {
		t.Fatalf("dry run missing collision details (wanted list of ours, theirs, new number);\nGot:\n%s", got)
	}
}

func TestIssuesRebaseRealRunRegeneratesIndexes1017(t *testing.T) {
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
	gitTestRun(t, dir, "add", "issues/README.md", ".gitattributes")
	gitTestRun(t, dir, "commit", "-m", "base")
	base := gitTestRun(t, dir, "rev-parse", "HEAD")

	writeTestTicket(t, dir, 1010, "local-issue")
	commitIssueIndex(t, dir, "local 1010")

	gitTestRun(t, dir, "branch", "upstream", base)
	gitTestRun(t, dir, "switch", "upstream")
	writeTestTicket(t, dir, 1010, "upstream-issue")
	commitIssueIndex(t, dir, "upstream 1010")

	gitTestRun(t, dir, "switch", "main")
	gitTestRun(t, dir, "config", "merge.harnez-issues-index.driver", "true")

	var out bytes.Buffer
	err := runIssuesRebase(&out, dir, "upstream", false)
	if err != nil {
		t.Fatalf("runIssuesRebase real run failed: %v", err)
	}

	if err := runIssuesLintCached(&out, dir); err != nil {
		t.Fatalf("real run did not leave valid index (lint failed): %v\nOutput: %s", err, out.String())
	}
}
