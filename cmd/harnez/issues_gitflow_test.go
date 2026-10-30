package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gitflowRepoYAML = `version: 1
issues:
  tracker: harnez
  merge_driver: true
docs:
  studies: false
  copies: []
rules:
  quota: false
make:
  container_test: false
git:
  mode: team
  protected_main: true
`

// protectMain commits a team + protected_main repo.yaml so the working tree stays clean.
func protectMain(t *testing.T, dir string) {
	t.Helper()
	writeDuplicateFile(t, dir, ".harnez/repo.yaml", gitflowRepoYAML)
	repoRunGit(t, dir, "add", ".harnez/repo.yaml")
	repoRunGit(t, dir, "commit", "-qm", "chore: protect main")
}

// closableFixture is a protected-main repo whose ticket 42 passes the close gate.
func closableFixture(t *testing.T) (dir, ticketPath string) {
	t.Helper()
	dir, ticketPath = issuesFixtureRepo(t, sampleTicket)
	commitFile(t, dir, "impl.go", "package main\n", "fix parser (issue 42)")
	protectMain(t, dir)
	return dir, ticketPath
}

func TestIssuesCloseRefusedOnProtectedMain(t *testing.T) {
	dir, ticketPath := closableFixture(t)
	before, err := os.ReadFile(ticketPath)
	if err != nil {
		t.Fatal(err)
	}
	head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	var out, stderr bytes.Buffer
	_, _, err = runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir, Stderr: &stderr})
	if err == nil || !strings.Contains(err.Error(), "git.protected_main") || !strings.Contains(err.Error(), "git switch -c") {
		t.Fatalf("expected a protected_main refusal with the switch command, got %v", err)
	}
	after, readErr := os.ReadFile(ticketPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("refused close changed the ticket:\n%s", after)
	}
	if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("refused close created a commit: HEAD %s -> %s", head, got)
	}
	if status := repoRunGitOutput(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("refused close left the tree dirty:\n%s", status)
	}
}

func TestIssuesCloseCommitsOnTopicBranch(t *testing.T) {
	dir, ticketPath := closableFixture(t)
	repoRunGit(t, dir, "switch", "-c", "sprint/1039")
	mainHead := repoRunGitOutput(t, dir, "rev-parse", "main")
	head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	var out, stderr bytes.Buffer
	if _, _, err := runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir, Stderr: &stderr}); err != nil {
		t.Fatalf("close on a topic branch: %v", err)
	}
	if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got == head {
		t.Error("HEAD did not advance on the topic branch")
	}
	if got := repoRunGitOutput(t, dir, "rev-parse", "main"); got != mainHead {
		t.Errorf("main moved: %s -> %s", mainHead, got)
	}
	if got := strings.TrimSpace(repoRunGitOutput(t, dir, "branch", "--show-current")); got != "sprint/1039" {
		t.Errorf("branch = %q", got)
	}
	data, err := os.ReadFile(ticketPath)
	if err != nil || !strings.Contains(string(data), "**Status**: Closed") {
		t.Errorf("ticket not closed: %v\n%s", err, data)
	}
}

func TestIssuesCheckSkipsGuard(t *testing.T) {
	dir, ticketPath := closableFixture(t)
	before, _ := os.ReadFile(ticketPath)
	var out, stderr bytes.Buffer
	if _, _, err := runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir, Stderr: &stderr, Check: true}); err != nil {
		t.Fatalf("--check must pass on protected main: %v", err)
	}
	if after, _ := os.ReadFile(ticketPath); !bytes.Equal(before, after) {
		t.Errorf("--check left the ticket changed:\n%s", after)
	}
}

func TestIssuesNoCommitSkipsGuard(t *testing.T) {
	dir, ticketPath := closableFixture(t)
	head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	var out, stderr bytes.Buffer
	if _, _, err := runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir, Stderr: &stderr, NoCommit: true}); err != nil {
		t.Fatalf("--no-commit must pass on protected main: %v", err)
	}
	if data, _ := os.ReadFile(ticketPath); !strings.Contains(string(data), "**Status**: Closed") {
		t.Errorf("--no-commit did not write the status:\n%s", data)
	}
	if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
		t.Error("--no-commit created a commit")
	}
}

func TestIssuesRecloseNoopPassesOnProtectedMain(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, strings.Replace(sampleTicket, "**Status**: Open", "**Status**: Closed", 1))
	protectMain(t, dir)
	var out, stderr bytes.Buffer
	result, _, err := runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir, Stderr: &stderr, NoCode: true})
	if err != nil || !result.Noop {
		t.Fatalf("closing a closed ticket is a no-op, not a refusal: noop=%v err=%v", result.Noop, err)
	}
}

func TestIssuesMvRefusedOnProtectedMain(t *testing.T) {
	dir := duplicateRepo(t, 2)
	protectMain(t, dir)
	head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	var out bytes.Buffer
	_, _, err := runIssuesMv(&out, "issues/096-b.md", "", issuesRunOptions{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "git.protected_main") {
		t.Fatalf("expected a protected_main refusal, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "issues", "096-b.md")); statErr != nil {
		t.Errorf("refused mv moved the source: %v", statErr)
	}
	if status := repoRunGitOutput(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("refused mv left the tree dirty:\n%s", status)
	}
	if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
		t.Error("refused mv created a commit")
	}
}

func TestIssuesMvCommitsOnTopicBranch(t *testing.T) {
	dir := duplicateRepo(t, 2)
	protectMain(t, dir)
	repoRunGit(t, dir, "switch", "-c", "sprint/1039")
	var out bytes.Buffer
	result, _, err := runIssuesMv(&out, "issues/096-b.md", "", issuesRunOptions{Dir: dir})
	if err != nil || !result.Committed {
		t.Fatalf("mv on a topic branch: committed=%v err=%v", result.Committed, err)
	}
}
