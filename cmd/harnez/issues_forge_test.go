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

type stubForgeSource struct {
	calls   int
	state   string // state argument of the last Issues request
	repoErr error  // Repo probe result; nil means the repo is reachable
	missing bool   // Issue answers 404 for every number
}

func (s *stubForgeSource) Issue(_ forge.Forge, _, _ string, n int) (forge.RemoteIssue, error) {
	s.calls++
	if s.missing {
		return forge.RemoteIssue{}, &forge.APIError{Status: 404, Detail: "issue not found"}
	}
	return forge.RemoteIssue{Number: n, Title: "forge ticket", State: "open", HTMLURL: "https://codeberg.org/o/r/issues/7"}, nil
}

func (s *stubForgeSource) Issues(_ forge.Forge, _, _ string, state string, _ forge.ScanOptions) ([]forge.RemoteIssue, bool, error) {
	s.calls++
	s.state = state
	return []forge.RemoteIssue{{Number: 7, Title: "forge ticket", State: "open"}}, false, nil
}

func (s *stubForgeSource) Repo(forge.Forge, string, string) error { return s.repoErr }

// TestFindForgeClosedWordAndActiveAlias checks that the bare word closed and -a status:active
// reach the forge as its issue state (issue 1198), and that local-only words are refused
// before any forge call.
func TestFindForgeClosedWordAndActiveAlias(t *testing.T) {
	stub := stubForge(t)
	dir := forgeRepo(t)
	if out, err := runHarnez(t, "find", "-d", dir, "issues", "closed"); err != nil || stub.state != "closed" {
		t.Fatalf("find issues closed: state=%q err=%v\n%s", stub.state, err, out)
	}
	if out, err := runHarnez(t, "find", "-d", dir, "issues", "-a", "status:active"); err != nil || stub.state != "open" {
		t.Fatalf("find -a issues status:active: state=%q err=%v\n%s", stub.state, err, out)
	}
	calls := stub.calls
	refused := []struct{ word, want string }{
		{"open", "find issues open: not supported for forge tickets; use find issues status:open"},
		{"blocked", "find issues blocked: forge tickets have only open and closed states; use status:open or status:closed"},
		{"draft", "find issues draft: forge tickets have only open and closed states; use status:open or status:closed"},
		{"next", "find issues next: needs a local issues/ dir; forge tickets support status:open, status:closed, status:active and plain terms"},
		{"last", "find issues last: needs a local issues/ dir; forge tickets support status:open, status:closed, status:active and plain terms"},
		{"history", "find issues history: needs a local issues/ dir; forge tickets support status:open, status:closed, status:active and plain terms"},
		{"similar", "find issues similar: needs a local issues/ dir; forge tickets support status:open, status:closed, status:active and plain terms"},
	}
	for _, c := range refused {
		out, err := runHarnez(t, "find", "-d", dir, "issues", c.word)
		if err == nil {
			t.Errorf("find issues %s on a forge repo must be refused, got:\n%s", c.word, out)
			continue
		}
		if err.Error() != c.want {
			t.Errorf("find issues %s: error\n got: %q\nwant: %q", c.word, err.Error(), c.want)
		}
	}
	if stub.calls != calls {
		t.Errorf("local-only words reached the forge: %d extra calls", stub.calls-calls)
	}
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

// TestIssuesShowMissingIssueOnForge (issue 1199): a missing issue on a reachable repo is
// reported as not found without the token hint; a missing repo keeps the hint.
func TestIssuesShowMissingIssueOnForge(t *testing.T) {
	stub := stubForge(t)
	stub.missing = true
	dir := forgeRepo(t)
	_, err := runHarnez(t, "issues", "show", "-d", dir, "99")
	if err == nil || !strings.Contains(err.Error(), "issue #99 not found on codeberg.org/o/r") || strings.Contains(err.Error(), "private repo") {
		t.Fatalf("reachable repo: want not-found without the hint, got %v", err)
	}
	stub.repoErr = &forge.APIError{Status: 404, Detail: "repo"}
	_, err = runHarnez(t, "issues", "show", "-d", dir, "99")
	if err == nil || !strings.Contains(err.Error(), "a private repo needs a token") {
		t.Fatalf("missing repo: want the token hint, got %v", err)
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

func stubIssueCLI(t *testing.T, out string) *[]forgetracker.Command {
	t.Helper()
	var ran []forgetracker.Command
	oldLook, oldRun := issueCLILookPath, issueCLIRun
	issueCLILookPath = func(name string) (string, error) {
		if name == "tea" {
			return "/usr/bin/tea", nil
		}
		return "", exec.ErrNotFound
	}
	issueCLIRun = func(_ string, c forgetracker.Command) ([]byte, error) {
		ran = append(ran, c)
		return []byte(out), nil
	}
	t.Cleanup(func() { issueCLILookPath, issueCLIRun = oldLook, oldRun })
	return &ran
}

func TestIssuesNewFilesOnForgeWithoutIssuesDir(t *testing.T) {
	ran := stubIssueCLI(t, "#42 Forge bug\n")
	dir := forgeRepo(t)
	body := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(body, []byte("details"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runHarnez(t, "issues", "new", "-d", dir, "Forge bug", "-F", body, "--label", "bug", "--dry-run")
	if err != nil || !strings.Contains(out, "dry run, would file on codeberg.org/o/r: tea issues create --repo o/r --title \"Forge bug\"") || len(*ran) != 0 {
		t.Fatalf("dry run: %v ran=%d\n%s", err, len(*ran), out)
	}
	out, err = runHarnez(t, "issues", "new", "-d", dir, "Forge bug", "-F", body, "--label", "bug")
	if err != nil || !strings.Contains(out, "filed as #42 at https://codeberg.org/o/r/issues/42; no commit") || len(*ran) != 1 {
		t.Fatalf("create: %v ran=%d\n%s", err, len(*ran), out)
	}
	if got := strings.Join((*ran)[0].Args, " "); !strings.Contains(got, "--labels bug") {
		t.Fatalf("labels not passed: %s", got)
	}
	out, err = runHarnez(t, "issues", "new", "-d", dir, "Forge bug", "-F", body, "--json")
	if err != nil || !strings.Contains(out, `"number":42`) || !strings.Contains(out, `"url":"https://codeberg.org/o/r/issues/42"`) {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "issues")); !os.IsNotExist(err) {
		t.Fatalf("forge create must not make issues/: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "log", "--oneline").CombinedOutput(); err == nil {
		t.Fatalf("forge create must not commit, got log %s", out)
	}
}

func TestIssuesNewForgeErrors(t *testing.T) {
	stubIssueCLI(t, "")
	dir := forgeRepo(t)
	if _, err := runHarnez(t, "issues", "new", "-d", dir, "T", "-F", filepath.Join(dir, "absent.md")); err == nil || !strings.Contains(err.Error(), "body file") {
		t.Fatalf("absent body: %v", err)
	}
	body := filepath.Join(dir, "b.md")
	if err := os.WriteFile(body, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runHarnez(t, "issues", "new", "-d", dir, "T", "-F", body); err == nil || !strings.Contains(err.Error(), "printed no ticket number") {
		t.Fatalf("silent CLI: %v", err)
	}
	if _, err := runHarnez(t, "issues", "show", "-d", dir, "-F", body, "7"); err == nil || !strings.Contains(err.Error(), "require new") {
		t.Fatalf("-F outside new: %v", err)
	}
}

func TestIssuesNewLocalRefusesForgeFlags(t *testing.T) {
	dir := forgeRepo(t)
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := runHarnez(t, "issues", "new", "-d", dir, "T", "--label", "bug"); err == nil || !strings.Contains(err.Error(), "apply only when tickets live on the forge") {
		t.Fatalf("local --label: %v", err)
	}
}

func TestIssuesNewWithoutForgeSignalStaysLocal(t *testing.T) {
	ran := stubIssueCLI(t, "#1 x\n")
	dir := forgeRepo(t)
	if _, err := runHarnez(t, "issues", "new", "-d", dir, "First ticket", "--offline"); err != nil {
		t.Fatalf("fresh repo local new: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "issues")); err != nil || len(*ran) != 0 {
		t.Fatalf("fresh repo must adopt issues/ (issue 853), ran=%d err=%v", len(*ran), err)
	}
}

func TestIssuesNewTrackerNoneUsesForge(t *testing.T) {
	stubIssueCLI(t, "")
	dir := forgeRepo(t)
	writeIndexRepoConfig(t, dir, "none", false)
	if _, err := runHarnez(t, "issues", "new", "-d", dir, "No body"); err == nil || !strings.Contains(err.Error(), "--body-file is required") {
		t.Fatalf("tracker none without -F: %v", err)
	}
}
