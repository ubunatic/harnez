package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/disable"
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

func TestIssuesProtectedMainUnlocks(t *testing.T) {
	spec, err := disable.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, current, recorded      string
		flag, remove, other, allowed bool
	}{
		{name: "missing session", recorded: "owner"},
		{name: "stale session", current: "worker", recorded: "owner"},
		{name: "enabled", current: "owner", recorded: "owner", remove: true},
		{name: "other repository", current: "owner", recorded: "owner", other: true},
		{name: "matching session", current: "owner", recorded: "owner", allowed: true},
		{name: "flag", flag: true, allowed: true},
		{name: "default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, _ := closableFixture(t)
			for _, env := range spec.SessionEnv {
				t.Setenv(env, "")
			}
			t.Setenv(spec.SessionEnv[0], tc.current)
			if tc.recorded != "" {
				root := dir
				if tc.other {
					root = t.TempDir()
				}
				if err := spec.Write(root, disable.State{Class: "repo-rules", Item: "push-main", SessionID: tc.recorded}); err != nil {
					t.Fatal(err)
				}
				if tc.remove {
					if err := spec.Remove(root, "repo-rules", "push-main"); err != nil {
						t.Fatal(err)
					}
				}
			}
			head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
			var out, stderr bytes.Buffer
			_, _, err := runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir, Stderr: &stderr, PushMain: tc.flag})
			if !tc.allowed {
				if err == nil || !strings.Contains(err.Error(), "--push-main") || !strings.Contains(err.Error(), "/disable repo-rules push-main") {
					t.Fatalf("expected refusal with both overrides: %v", err)
				}
				if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
					t.Fatal("refusal committed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got == head {
				t.Fatal("commit missing")
			}
			want := "git.protected_main: ticket commit unlocked by /disable repo-rules push-main for session owner"
			if tc.flag {
				want = "git.protected_main: ticket commit unlocked by --push-main flag"
			}
			assertUnlockLine(t, stderr.String(), want)
		})
	}
}

// assertUnlockLine ignores other diagnostics but requires exactly one complete unlock line.
func assertUnlockLine(t *testing.T, stderr, want string) {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "ticket commit unlocked") {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("unlock lines = %q, want [%q]; stderr = %q", lines, want, stderr)
	}
}

func TestIssuesPushMainCLI(t *testing.T) {
	spec, err := disable.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range spec.SessionEnv {
		t.Setenv(env, "")
	}
	for _, verb := range []string{"close", "mv", "rebase"} {
		t.Run(verb, func(t *testing.T) {
			var dir string
			var args []string
			switch verb {
			case "close":
				dir, _ = closableFixture(t)
				args = []string{"close", "42"}
			case "mv":
				dir = duplicateRepo(t, 2)
				protectMain(t, dir)
				args = []string{"mv", "issues/096-b.md"}
			case "rebase":
				dir, _ = closableFixture(t)
				writeDuplicateFile(t, dir, ".gitattributes", "issues/README.md merge=union\n")
				repoRunGit(t, dir, "add", ".gitattributes")
				repoRunGit(t, dir, "commit", "-qm", "configure issue merge")
				args = []string{"rebase", "HEAD"}
			}
			head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
			cmd := newIssuesCmd()
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append(args, "-d", dir, "--push-main"))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("%s --push-main: %v; stdout=%q stderr=%q", verb, err, out.String(), stderr.String())
			}
			assertUnlockLine(t, stderr.String(), "git.protected_main: ticket commit unlocked by --push-main flag")
			if verb != "rebase" && repoRunGitOutput(t, dir, "rev-parse", "HEAD") == head {
				t.Fatal("issue commit did not advance HEAD")
			}
		})
	}
}
