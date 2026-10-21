package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	findpkg "ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/index"
	"ubunatic.com/harnez/internal/issues"
)

func TestIssuesRemoteAllocation(t *testing.T) {
	for _, mode := range []string{"flag", "env", "empty-env", "failure", "bad-output", "edit-failure"} {
		t.Run(mode, func(t *testing.T) {
			dir, _ := issuesFixtureRepo(t, sampleTicket)
			repoRunGit(t, dir, "remote", "add", "github", "git@github.com:example/tracker.git")
			t.Setenv("CLAUDE_CODE_REMOTE", "")
			if mode == "env" {
				t.Setenv("CLAUDE_CODE_REMOTE", "1")
			}
			calls := 0
			runner := func(gotDir string, args ...string) ([]byte, error) {
				calls++
				joined := strings.Join(args, " ")
				if gotDir != dir || !strings.Contains(joined, "--repo git@github.com:example/tracker.git") {
					t.Fatalf("wrong target: %s %v", gotDir, args)
				}
				if mode == "failure" || mode == "edit-failure" && calls == 2 {
					return nil, fmt.Errorf("gh: not authenticated")
				}
				if mode == "bad-output" {
					return []byte("unexpected"), nil
				}
				if calls == 1 {
					if !strings.Contains(joined, "issue create") || !strings.Contains(joined, "--title Cloud task") {
						t.Fatalf("create args: %v", args)
					}
					return []byte("https://github.com/example/tracker/issues/42\n"), nil
				}
				if !strings.Contains(joined, "issue edit 42") || !strings.Contains(joined, "https://github.com/example/tracker/blob/HEAD/issues/GH42-cloud-task.md") {
					t.Fatalf("missing exact ticket link: %v", args)
				}
				return nil, nil
			}
			cmd := newIssuesCmdWithGHRunner(runner)
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			args := []string{"new", "-d", dir, "Cloud task"}
			if mode != "env" && mode != "empty-env" {
				args = append(args, "--remote")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			path := filepath.Join(dir, "issues/GH42-cloud-task.md")
			if mode == "failure" || mode == "bad-output" || mode == "edit-failure" {
				if err == nil {
					t.Fatal("expected remote error")
				}
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("created ticket on error: %v", statErr)
				}
				files, scanErr := issues.Scan(filepath.Join(dir, "issues"))
				if scanErr != nil || len(files) != 1 {
					t.Fatalf("failure allocated a fallback: %v %v", files, scanErr)
				}
				if !strings.Contains(err.Error(), "gh") {
					t.Fatalf("unclear error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(path)
			if err != nil || !strings.HasPrefix(string(body), "# GH42 — Cloud task\n") {
				t.Fatalf("ticket: %s %v", body, err)
			}
			if calls != 2 || !strings.Contains(out.String(), "GH42\tissues/GH42-cloud-task.md") || !strings.Contains(stderr.String(), "remote") {
				t.Fatalf("output: %s / %s; calls %d", &out, &stderr, calls)
			}
		})
	}
}

func TestGHTicketLifecycle(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	for _, id := range []string{"GH900", "GH100", "GH42", "GH9"} {
		writeDuplicateFile(t, dir, "issues/"+id+"-cloud.md", "# "+id+" — Cloud task\n\n**Status**: Open\n")
	}
	for arg, want := range map[string]string{"GH42": "GH42", "gh42": "GH42", "42": "042"} {
		f, err := findTicketFile(filepath.Join(dir, "issues"), arg)
		if err != nil || f.Number != want {
			t.Fatalf("%s: %v %v", arg, f, err)
		}
	}
	if leadingIssueNumberCount([]string{"GH42", "gh9", "42", "reason"}) != 3 {
		t.Fatal("GH batch parsing")
	}
	result, _, err := runIssuesVerb(&bytes.Buffer{}, "close", "gh42", []string{"shipped"}, issuesRunOptions{Dir: dir, NoCommit: true})
	if err != nil || result.Number != "GH42" {
		t.Fatalf("close: %+v %v", result, err)
	}
	table, err := index.IssuesTable(filepath.Join(dir, "issues"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := issues.ParseTrackerTable(table)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"042", "GH9", "GH100", "GH900"} {
		if rows[i].Number != id {
			t.Fatalf("sort: %+v", rows)
		}
	}
	closed, err := os.ReadFile(filepath.Join(dir, "issues", "CLOSED.md"))
	if err != nil || !strings.Contains(string(closed), "GH42-cloud.md") {
		t.Fatalf("closed index missing GH42: %s, err=%v", closed, err)
	}
	files, err := issues.Scan(filepath.Join(dir, "issues"))
	if err != nil {
		t.Fatal(err)
	}
	if next := issues.NextNumberFromFiles(files); next != "043" {
		t.Fatalf("local next: %s", next)
	}
	query, err := findpkg.ParseQuery("is:closed")
	if err != nil {
		t.Fatal(err)
	}
	matches := findpkg.Search(files, query)
	if len(matches) != 1 || matches[0].Number != "GH42" || matches[0].PlainTitle != "Cloud task" {
		t.Fatalf("find: %+v", matches)
	}
	if _, _, err := runIssuesMv(&bytes.Buffer{}, "GH42", "", issuesRunOptions{Dir: dir, NoCommit: true}); err == nil {
		t.Fatal("GH ids must be permanent")
	}
	if err := runIndex(&bytes.Buffer{}, indexOptions{Dir: dir}); err != nil {
		t.Fatalf("index: %v", err)
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err != nil {
		t.Fatalf("lint: %v", err)
	}
	writeDuplicateFile(t, dir, "issues/GH42-duplicate.md", "# GH42 — Duplicate\n\n**Status**: Open\n")
	if err := issues.CheckDuplicateNumbers(filepath.Join(dir, "issues")); err == nil || !strings.Contains(err.Error(), "GH42") {
		t.Fatalf("duplicate: %v", err)
	}
	if err := runIndex(&bytes.Buffer{}, indexOptions{Dir: dir}); err == nil {
		t.Fatal("index accepted duplicate GH id")
	}
	if err := runIssuesLint(&bytes.Buffer{}, dir); err == nil || !strings.Contains(err.Error(), "duplicate ticket number GH42") {
		t.Fatalf("duplicate lint: %v", err)
	}
}

func TestIssuesRemotePreconditions(t *testing.T) {
	for _, mode := range []string{"no-remote", "no-title", "missing-gh"} {
		t.Run(mode, func(t *testing.T) {
			dir, _ := issuesFixtureRepo(t, sampleTicket)
			if mode != "no-remote" {
				repoRunGit(t, dir, "remote", "add", "github", "https://github.com/example/tracker.git")
			}
			title := "Cloud"
			if mode == "no-title" {
				title = ""
			}
			calls := 0
			err := runIssuesNewRemote(&bytes.Buffer{}, dir, title, false, func(string, ...string) ([]byte, error) {
				calls++
				return nil, fmt.Errorf("gh executable not found in PATH")
			})
			if err == nil {
				t.Fatal("expected precondition error")
			}
			if mode == "missing-gh" {
				if calls != 1 || !strings.Contains(err.Error(), "gh auth login") || !strings.Contains(err.Error(), "no local fallback") {
					t.Fatalf("missing gh: %v (calls %d)", err, calls)
				}
			} else if calls != 0 {
				t.Fatal("called gh before validating inputs")
			}
			files, err := issues.Scan(filepath.Join(dir, "issues"))
			if err != nil || len(files) != 1 {
				t.Fatalf("precondition created a ticket: %+v %v", files, err)
			}
		})
	}
}

func TestIssuesRemoteDuplicateGateRunsBeforeGitHubCreate(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	repoRunGit(t, dir, "remote", "add", "github", "https://github.com/example/tracker.git")
	calls := 0
	err := runIssuesNewRemote(&bytes.Buffer{}, dir, "Example ticket", false, func(string, ...string) ([]byte, error) {
		calls++
		return []byte("https://github.com/example/tracker/issues/99"), nil
	})
	if err == nil || !strings.Contains(err.Error(), "harnez issues open 042") {
		t.Fatalf("duplicate remote title: got %v, want an open hint", err)
	}
	if calls != 0 {
		t.Fatalf("duplicate gate called gh %d times before refusing", calls)
	}
	files, scanErr := issues.Scan(filepath.Join(dir, "issues"))
	if scanErr != nil || len(files) != 1 {
		t.Fatalf("duplicate remote title changed local tracker: files=%d err=%v", len(files), scanErr)
	}
}

func TestIssuesRemoteForceBypassesDuplicateGate(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	repoRunGit(t, dir, "remote", "add", "github", "https://github.com/example/tracker.git")
	calls := 0
	err := runIssuesNewRemoteWithForce(&bytes.Buffer{}, dir, "Example ticket", false, true, func(_ string, args ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("https://github.com/example/tracker/issues/99"), nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("--force remote create: %v", err)
	}
	if calls != 2 {
		t.Fatalf("--force remote create made %d gh calls, want create and edit", calls)
	}
}

func TestIssuesNewLocalModeAndRemoteJSON(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	if err := os.Unsetenv("CLAUDE_CODE_REMOTE"); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	cmd := newIssuesCmdWithGHRunner(func(string, ...string) ([]byte, error) { t.Fatal("local allocation called gh"); return nil, nil })
	cmd.SetArgs([]string{"new", "-d", dir, "Local"})
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "043\tissues/043-local.md") || !strings.Contains(stderr.String(), "local mode") {
		t.Fatalf("local output: %s / %s", &out, &stderr)
	}
	repoRunGit(t, dir, "remote", "add", "github", "https://github.com/example/tracker.git")
	out.Reset()
	if err := runIssuesNewRemote(&out, dir, "Cloud", true, func(_ string, args ...string) ([]byte, error) {
		return []byte("https://github.com/example/tracker/issues/42\n"), nil
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"number":"GH42"`) || !strings.Contains(out.String(), `"path":"issues/GH42-cloud.md"`) {
		t.Fatalf("remote JSON: %s", &out)
	}
}

func TestGHLintHeaderMismatch(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	writeDuplicateFile(t, dir, "issues/GH42-cloud.md", "# GH43 — Cloud\n\n**Status**: Open\n")
	if err := runIssuesLint(&bytes.Buffer{}, dir); err == nil || !strings.Contains(err.Error(), "filename/header mismatch") {
		t.Fatalf("GH header mismatch: %v", err)
	}
}
