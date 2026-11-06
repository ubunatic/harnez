package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetracker"
	"ubunatic.com/harnez/internal/prwait"
)

type stubForgeSource struct{ calls int }

func (s *stubForgeSource) Issue(_ forge.Forge, _, _ string, n int) (forge.RemoteIssue, error) {
	s.calls++
	return forge.RemoteIssue{Number: n, Title: "forge ticket", State: "open", HTMLURL: "https://codeberg.org/o/r/issues/7"}, nil
}

func (s *stubForgeSource) Issues(forge.Forge, string, string, string, forge.ScanOptions) ([]forge.RemoteIssue, bool, error) {
	s.calls++
	return []forge.RemoteIssue{{Number: 7, Title: "forge ticket", State: "open"}}, false, nil
}

func forgeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@codeberg.org:o/r.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func stubForge(t *testing.T) *stubForgeSource {
	t.Helper()
	stub := &stubForgeSource{}
	old := forgeTrackerSource
	forgeTrackerSource = func(prwait.Spec) forgetracker.Source { return stub }
	t.Cleanup(func() { forgeTrackerSource = old })
	return stub
}

func runHarnez(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err := root.Execute()
	return out.String() + errOut.String(), err
}

func TestIssuesShowAndFindReadForgeWithoutIssuesDir(t *testing.T) {
	stub := stubForge(t)
	dir := forgeRepo(t)
	out, err := runHarnez(t, "issues", "show", "-d", dir, "7")
	if err != nil || !strings.Contains(out, "# 7 — forge ticket") || !strings.Contains(out, "codeberg.org/o/r") {
		t.Fatalf("show via forge: %v\n%s", err, out)
	}
	out, err = runHarnez(t, "find", "-d", dir, "issues", "forge")
	if err != nil || !strings.Contains(out, "7\topen\tforge ticket") {
		t.Fatalf("find via forge: %v\n%s", err, out)
	}
	if stub.calls != 2 {
		t.Fatalf("want 2 forge calls, got %d", stub.calls)
	}
}

func TestIssuesLocalDirSkipsForge(t *testing.T) {
	stub := stubForge(t)
	dir := forgeRepo(t)
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	ticket := "# 007 — local ticket\n\n**Status**: Open\n**Priority**: P2\n\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "issues", "007-local-ticket.md"), []byte(ticket), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runHarnez(t, "issues", "show", "-d", dir, "--text", "7")
	if err != nil || !strings.Contains(out, "local ticket") {
		t.Fatalf("local show: %v\n%s", err, out)
	}
	if out, err := runHarnez(t, "find", "-d", dir, "issues", "--text", "local"); err != nil || !strings.Contains(out, "007") {
		t.Fatalf("local find: %v\n%s", err, out)
	}
	if stub.calls != 0 {
		t.Fatalf("local issues/ must not call the forge, got %d calls", stub.calls)
	}
}

func TestForgeTrackerHint(t *testing.T) {
	tg := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", TokenEnv: []string{"CODEBERG_TOKEN", "FORGEJO_TOKEN"}}}
	err := forgeTrackerHint(&forge.APIError{Status: 404, Detail: "x"}, tg)
	if err == nil || !strings.Contains(err.Error(), "CODEBERG_TOKEN, FORGEJO_TOKEN") {
		t.Fatalf("want token hint, got %v", err)
	}
	// The stored-token route comes first; the env vars follow it.
	msg := err.Error()
	if i, j := strings.Index(msg, "harnez forge token import"), strings.Index(msg, "CODEBERG_TOKEN"); i < 0 || i > j {
		t.Fatalf("hint must name harnez forge token import before the env vars, got %q", msg)
	}
	if !strings.Contains(msg, "harnez forge token create") {
		t.Fatalf("hint must name token create too, got %q", msg)
	}
	gh := forgetracker.Target{Forge: forge.Forge{Kind: "github", TokenEnv: []string{"GITHUB_TOKEN"}}}
	if err := forgeTrackerHint(&forge.APIError{Status: 404, Detail: "x"}, gh); err == nil || strings.Contains(err.Error(), "forge token") {
		t.Fatalf("GitHub has no stored-token route, got %v", err)
	}
	plain := errors.New("boom")
	if got := forgeTrackerHint(plain, tg); got != plain {
		t.Fatalf("non-API error must pass through, got %v", got)
	}
	if forgeTrackerHint(nil, tg) != nil {
		t.Fatal("nil must stay nil")
	}
}
