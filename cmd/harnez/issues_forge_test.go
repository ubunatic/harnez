package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetoken"
	"ubunatic.com/harnez/internal/forgetracker"
	"ubunatic.com/harnez/internal/prwait"
)

func TestForgeWritesAPI(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	_, st, err := forgeTokenStoreFor(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Save(forgetoken.Meta{Host: "codeberg.org", User: "u"}, "write-secret"); err != nil {
		t.Fatal(err)
	}
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" "+string(body))
		if r.Header.Get("Authorization") != "token write-secret" {
			t.Errorf("wrong authorization")
		}
		switch r.URL.Path {
		case "/repos/o/r/labels":
			if r.URL.Query().Get("page") == "1" {
				io.WriteString(w, `[{"id":5,"name":"bug"}]`)
			} else {
				io.WriteString(w, `[]`)
			}
		case "/repos/o/r/issues":
			io.WriteString(w, `{"number":42,"html_url":"https://example/42"}`)
		case "/repos/o/r/issues/7/comments":
			io.WriteString(w, `{"html_url":"https://example/comment"}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", APIBase: srv.URL}, Owner: "o", Repo: "r"}
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte("hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = runIssuesNewForge(&out, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "T", BodyFile: body, Labels: []string{"bug"}}, false, false)
	if err != nil || out.String() != "filed as #42 at https://example/42; no commit\n" {
		t.Fatalf("new %q: %v", out.String(), err)
	}
	want := []string{"GET /repos/o/r/labels?limit=50&page=1 ", "GET /repos/o/r/labels?limit=50&page=2 ", `POST /repos/o/r/issues {"title":"T","body":"hello\n","labels":[5]}`}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests %q", requests)
	}
	requests = nil
	out.Reset()
	err = runIssuesNewForge(&out, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "T", BodyFile: body}, false, true)
	if err != nil || out.String() != "{\"forge\":\"codeberg.org/o/r\",\"number\":42,\"tool\":\"api\",\"url\":\"https://example/42\"}\n" {
		t.Fatalf("new json %q: %v", out.String(), err)
	}
	if !reflect.DeepEqual(requests, []string{`POST /repos/o/r/issues {"title":"T","body":"hello\n"}`}) {
		t.Fatalf("json requests %q", requests)
	}
	requests = nil
	out.Reset()
	err = runIssuesComment(&out, t.TempDir(), target, &stubForgeSource{}, "7", body, false, false)
	if err != nil || out.String() != "commented on codeberg.org/o/r#7: https://example/comment\n" {
		t.Fatalf("comment %q: %v", out.String(), err)
	}
	if !reflect.DeepEqual(requests, []string{`POST /repos/o/r/issues/7/comments {"body":"hello\n"}`}) {
		t.Fatalf("requests %q", requests)
	}
	requests = nil
	out.Reset()
	err = runIssuesNewForge(&out, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "T", BodyFile: body, Labels: []string{"missing"}}, false, false)
	if err == nil || err.Error() != `unknown label "missing"; existing labels: bug` || len(requests) != 2 {
		t.Fatalf("unknown: %v %q", err, requests)
	}
}

func TestForgeNewWorkerAndFallback(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "worker-1")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEBERG_TOKEN", "read-env-secret")
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	oldLook, oldRun, oldLookup := issueCLILookPath, issueCLIRun, issueWriteLookup
	var ran []forgetracker.Command
	issueCLILookPath = func(name string) (string, error) { return name, nil }
	issueCLIRun = func(_ string, c forgetracker.Command) ([]byte, error) {
		ran = append(ran, c)
		return []byte("#42 T\n"), nil
	}
	issueWriteLookup = func(string) string { t.Fatal("unexpected token lookup by worker"); return "" }
	t.Cleanup(func() { issueCLILookPath, issueCLIRun, issueWriteLookup = oldLook, oldRun, oldLookup })
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", IssueCLI: []string{"fj"}}, Owner: "o", Repo: "r"}
	req := forgetracker.CreateRequest{Title: "T", BodyFile: body}
	var out bytes.Buffer
	err := runIssuesNewForge(&out, io.Discard, t.TempDir(), target, req, false, false)
	want := "issues new: forge writes run only in a host session (this is harnez worker worker-1); hand the write to the host"
	if err == nil || err.Error() != want || len(ran) != 0 {
		t.Fatalf("worker %v ran=%v", err, ran)
	}
	if err := runIssuesNewForge(&out, io.Discard, t.TempDir(), target, req, true, false); err != nil || out.String() != "dry run, would file on codeberg.org/o/r: fj issue create --repo codeberg.org/o/r --body-file "+body+" T\n" {
		t.Fatalf("dry: %q %v", out.String(), err)
	}
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	issueWriteLookup = func(string) string { return "" }
	out.Reset()
	if err := runIssuesNewForge(&out, io.Discard, t.TempDir(), target, req, false, false); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || !reflect.DeepEqual(ran[0].Args, []string{"issue", "create", "--repo", "codeberg.org/o/r", "--body-file", body, "T"}) {
		t.Fatalf("argv: %+v", ran)
	}
	if err := os.WriteFile(body, []byte(" \n"), 0600); err != nil {
		t.Fatal(err)
	}
	err = runIssuesNewForge(&out, io.Discard, t.TempDir(), target, req, false, false)
	if err == nil || err.Error() != "issues new: body file is empty or whitespace-only" || len(ran) != 1 {
		t.Fatalf("blank: %v", err)
	}
}

func TestForgeNewStdinBody(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("stdin body"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	oldStdin, oldLook, oldRun, oldLookup := os.Stdin, issueCLILookPath, issueCLIRun, issueWriteLookup
	os.Stdin = f
	issueCLILookPath = func(n string) (string, error) { return n, nil }
	issueWriteLookup = func(string) string { return "" }
	issueCLIRun = func(_ string, c forgetracker.Command) ([]byte, error) {
		data, err := os.ReadFile(c.Args[5])
		if err != nil || string(data) != "stdin body" {
			t.Fatalf("body %q %v", data, err)
		}
		return []byte("#42 T"), nil
	}
	t.Cleanup(func() {
		os.Stdin, issueCLILookPath, issueCLIRun, issueWriteLookup = oldStdin, oldLook, oldRun, oldLookup
		f.Close()
	})
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", IssueCLI: []string{"fj"}}, Owner: "o", Repo: "r"}
	if err := runIssuesNewForge(io.Discard, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "T", BodyFile: "-"}, false, false); err != nil {
		t.Fatal(err)
	}
}

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
	// A quoted query is split into words first (argon review): "open bug" is refused like open bug.
	if _, err := runHarnez(t, "find", "-d", dir, "issues", "open bug"); err == nil || !strings.Contains(err.Error(), "find issues open: not supported") {
		t.Errorf(`find issues "open bug" must be refused like open, got %v`, err)
	}
	if _, err := runHarnez(t, "find", "-d", dir, "--next", "issues"); err == nil || err.Error() != "find issues --next: needs a local issues/ dir; forge tickets support status:open, status:closed, status:active and plain terms" {
		t.Errorf("find --next issues on a forge repo: got %v", err)
	}
	if stub.calls != calls {
		t.Errorf("local-only words reached the forge: %d extra calls", stub.calls-calls)
	}
	// history reads the telemetry DB, so the forge path must leave it alone.
	if _, err := runHarnez(t, "find", "-d", dir, "issues", "history"); err != nil && strings.Contains(err.Error(), "needs a local issues/ dir") {
		t.Errorf("find issues history must not be refused on a forge repo: %v", err)
	}
	if stub.calls != calls {
		t.Errorf("find issues history reached the forge: %d extra calls", stub.calls-calls)
	}
	if out, err := runHarnez(t, "find", "-d", dir, "issues", "closed bug"); err != nil || stub.state != "closed" {
		t.Errorf(`find issues "closed bug": state=%q err=%v\n%s`, stub.state, err, out)
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
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	oldLookup := issueWriteLookup
	issueWriteLookup = func(string) string { return "" }
	t.Cleanup(func() { issueWriteLookup = oldLookup })
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
	if _, err := runHarnez(t, "issues", "show", "-d", dir, "-F", body, "7"); err == nil || err.Error() != "issues: -F/--body-file requires new, comment or edit" {
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

func TestIssuesNewForceForge(t *testing.T) {
	ran := stubIssueCLI(t, "#42 T\n")
	dir := forgeRepo(t)
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0755); err != nil {
		t.Fatal(err)
	}
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := runHarnez(t, "issues", "new", "--forge", "-d", dir, "T", "-F", body)
	if err != nil || out != "filed as #42 at https://codeberg.org/o/r/issues/42; no commit\nissues new: filing on codeberg.org/o/r with tea\n" || len(*ran) != 1 {
		t.Fatalf("%q %v ran=%d", out, err, len(*ran))
	}
	entries, err := os.ReadDir(filepath.Join(dir, "issues"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("local files: %v %v", entries, err)
	}
	stubForge(t)
	out, err = runHarnez(t, "issues", "show", "--forge", "--json", "-d", dir, "7")
	if err != nil || !strings.Contains(out, `"updated_at"`) {
		t.Fatalf("show: %q %v", out, err)
	}
	if _, err := runHarnez(t, "issues", "list", "--forge", "-d", dir); err == nil || err.Error() != "issues: --forge does not support list" {
		t.Fatalf("list: %v", err)
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

func TestIssuesNewFJFailureManualCommand(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	oldLookup := issueWriteLookup
	issueWriteLookup = func(string) string { return "" }
	t.Cleanup(func() { issueWriteLookup = oldLookup })
	oldLook, oldRun := issueCLILookPath, issueCLIRun
	t.Cleanup(func() { issueCLILookPath, issueCLIRun = oldLook, oldRun })
	issueCLILookPath = func(name string) (string, error) {
		if name == "fj" {
			return name, nil
		}
		return "", exec.ErrNotFound
	}
	issueCLIRun = func(string, forgetracker.Command) ([]byte, error) {
		return []byte("ordinary error"), errors.New("exit status 1")
	}
	body := filepath.Join(t.TempDir(), "body file")
	if err := os.WriteFile(body, []byte("private body"), 0600); err != nil {
		t.Fatal(err)
	}
	target := forgetracker.Target{Forge: forge.Forge{Host: "codeberg.org", IssueCLI: []string{"fj"}}, Owner: "o", Repo: "r"}
	err := runIssuesNewForge(io.Discard, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "title", BodyFile: body}, false, false)
	want := "issues new: fj failed: exit status 1\nordinary error\nrun manually: fj issue create --repo codeberg.org/o/r --body-file " + strconv.Quote(body) + " title"
	if err == nil || err.Error() != want || strings.Contains(err.Error(), "private body") {
		t.Fatalf("got %v, want %s", err, want)
	}
}

func TestForgeDryRunMetadataAndGitHub(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "worker")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	_, st, err := forgeTokenStoreFor(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Save(forgetoken.Meta{Host: "codeberg.org"}, "secret"); err != nil {
		t.Fatal(err)
	}
	old := issueWriteLookup
	issueWriteLookup = func(string) string { t.Fatal("secret lookup"); return "" }
	t.Cleanup(func() { issueWriteLookup = old })
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte("body"), 0600); err != nil {
		t.Fatal(err)
	}
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org"}, Owner: "o", Repo: "r"}
	var out bytes.Buffer
	if err := runIssuesNewForge(&out, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "T", BodyFile: body, Labels: []string{"x"}}, true, false); err != nil || out.String() != "dry run, would file on codeberg.org/o/r over the API (write token)\n" {
		t.Fatalf("new: %q %v", out.String(), err)
	}
	out.Reset()
	if err := runIssuesComment(&out, t.TempDir(), target, &stubForgeSource{}, "7", body, true, false); err != nil || out.String() != "dry run: would comment on codeberg.org/o/r#7 over the API (write token)\n" {
		t.Fatalf("comment: %q %v", out.String(), err)
	}
	target.Forge.Kind = "github"
	src := &stubForgeSource{}
	err = runIssuesComment(io.Discard, t.TempDir(), target, src, "7", body, true, false)
	if err == nil || err.Error() != "issues comment: forge writes support Forgejo only; GitHub is ticket 1209" || src.calls != 0 {
		t.Fatalf("github: %v calls=%d", err, src.calls)
	}
}

type editSource struct {
	issue forge.RemoteIssue
	calls int
}

func (s *editSource) Issue(_ forge.Forge, _, _ string, _ int) (forge.RemoteIssue, error) {
	s.calls++
	return s.issue, nil
}
func (s *editSource) Issues(_ forge.Forge, _, _, _ string, _ forge.ScanOptions) ([]forge.RemoteIssue, bool, error) {
	return nil, false, nil
}
func (s *editSource) Repo(_ forge.Forge, _, _ string) error { return nil }

func TestForgeEdit(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	old := issueWriteLookup
	issueWriteLookup = func(string) string { return "secret" }
	t.Cleanup(func() { issueWriteLookup = old })
	var requests []string
	response := `{"body":"body\n"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.Path+" "+string(b))
		io.WriteString(w, response)
	}))
	defer srv.Close()
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", APIBase: srv.URL}, Owner: "o", Repo: "r"}
	ts := "2026-01-02T03:04:05Z"
	when, _ := time.Parse(time.RFC3339, ts)
	src := &editSource{issue: forge.RemoteIssue{State: "open", UpdatedAt: when}}
	var out bytes.Buffer
	if err := runIssuesEditForge(&out, target, src, "7", "X", "", "", true, false, false, false); err != nil || out.String() != "edited codeberg.org/o/r#7\n" {
		t.Fatalf("title: %q %v", out.String(), err)
	}
	if !reflect.DeepEqual(requests, []string{`PATCH /repos/o/r/issues/7 {"title":"X"}`}) {
		t.Fatalf("requests: %q", requests)
	}
	requests = nil
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte("body\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := runIssuesEditForge(io.Discard, target, src, "7", "", body, "", false, false, false, false)
	if err == nil || err.Error() != "issues edit: body edits need --if-updated <updated_at from issues show --json> or --force" {
		t.Fatal(err)
	}
	err = runIssuesEditForge(io.Discard, target, src, "7", "", body, "2020-01-01T00:00:00Z", false, false, false, false)
	if err == nil || err.Error() != "issues edit: o/r#7 changed at 2026-01-02T03:04:05Z, expected 2020-01-01T00:00:00Z; run issues show again" || len(requests) != 0 {
		t.Fatalf("stale: %v %q", err, requests)
	}
	if err := runIssuesEditForge(io.Discard, target, src, "7", "X", body, ts, true, false, false, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(requests, []string{`PATCH /repos/o/r/issues/7 {"title":"X","body":"body\n"}`}) {
		t.Fatalf("requests: %q", requests)
	}
	response = `{"body":"different"}`
	if err := runIssuesEditForge(io.Discard, target, src, "7", "", body, "", false, true, false, false); err == nil || err.Error() != "issues edit: forge returned a different body" {
		t.Fatalf("mismatch: %v", err)
	}
	requests = nil
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "worker")
	err = runIssuesEditForge(io.Discard, target, src, "7", "X", "", "", true, false, false, false)
	if err == nil || err.Error() != "issues edit: forge writes run only in a host session (this is harnez worker worker); hand the write to the host" || len(requests) != 0 {
		t.Fatalf("worker: %v %q", err, requests)
	}
	target.Forge.Kind = "github"
	err = runIssuesEditForge(io.Discard, target, src, "7", "X", "", "", true, false, false, false)
	if err == nil || err.Error() != "issues edit: forge writes support Forgejo only; GitHub is ticket 1209" || len(requests) != 0 {
		t.Fatalf("github: %v", err)
	}
}

func TestForgeEditEdges(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	oldLookup := issueWriteLookup
	issueWriteLookup = func(string) string { return "" }
	t.Cleanup(func() { issueWriteLookup = oldLookup })
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org"}, Owner: "o", Repo: "r"}
	src := &editSource{}
	err := runIssuesEditForge(io.Discard, target, src, "7", "X", "", "", true, false, false, false)
	if err == nil || err.Error() != "no write token for codeberg.org; run: harnez forge token create --write" || src.calls != 0 {
		t.Fatalf("no token: %v", err)
	}
	issueWriteLookup = func(string) string { return "secret" }
	src.issue.State = "closed"
	err = runIssuesEditForge(io.Discard, target, src, "7", "X", "", "", true, false, false, false)
	if err == nil || err.Error() != "issues edit: o/r#7 is closed; use --closed" {
		t.Fatalf("closed: %v", err)
	}
	src.issue.PullRequest = &struct{}{}
	err = runIssuesEditForge(io.Discard, target, src, "7", "X", "", "", true, false, false, true)
	if err == nil || err.Error() != "issues edit: o/r#7 is a pull request" {
		t.Fatalf("pull: %v", err)
	}
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte(" \n"), 0600); err != nil {
		t.Fatal(err)
	}
	err = runIssuesEditForge(io.Discard, target, src, "7", "", body, "", false, true, false, false)
	if err == nil || err.Error() != "issues edit: body file is empty or whitespace-only" {
		t.Fatalf("blank: %v", err)
	}
	var out bytes.Buffer
	if err := runIssuesEditForge(&out, target, src, "7", "X", "", "", true, false, true, false); err != nil || out.String() != "dry run: would edit codeberg.org/o/r#7 over the API --title \"X\"\n" {
		t.Fatalf("dry: %q %v", out.String(), err)
	}
}

func TestIssuesEditLocal(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := runHarnez(t, "issues", "edit", "-d", dir, "7", "--title", "X")
	if err == nil || err.Error() != "issues edit: local tickets are files; edit issues/<n>-*.md and commit" {
		t.Fatal(err)
	}
}

func TestForgeStateWrites(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	old := issueWriteLookup
	issueWriteLookup = func(string) string { return "secret" }
	t.Cleanup(func() { issueWriteLookup = old })
	var calls []string
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.Path+" "+string(body))
		w.WriteHeader(status)
		if status == 200 {
			io.WriteString(w, `{}`)
		} else {
			io.WriteString(w, `{"message":"safe detail"}`)
		}
	}))
	defer srv.Close()
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", APIBase: srv.URL}, Owner: "o", Repo: "r"}
	src := &editSource{issue: forge.RemoteIssue{State: "open"}}
	var out bytes.Buffer
	if err := runIssuesStateForge(&out, target, src, "close", "7", "", false); err != nil || out.String() != "closed codeberg.org/o/r#7\n" {
		t.Fatalf("close %q %v", out.String(), err)
	}
	if !reflect.DeepEqual(calls, []string{`PATCH /repos/o/r/issues/7 {"state":"closed"}`}) {
		t.Fatalf("calls %q", calls)
	}
	calls = nil
	out.Reset()
	if err := runIssuesStateForge(&out, target, src, "close", "7", "done now", false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{`POST /repos/o/r/issues/7/comments {"body":"done now"}`, `PATCH /repos/o/r/issues/7 {"state":"closed"}`}) {
		t.Fatalf("calls %q", calls)
	}
	calls = nil
	status = 403
	err := runIssuesStateForge(io.Discard, target, src, "close", "7", "done now", false)
	if err == nil || err.Error() != "forge API request failed: HTTP 403 safe detail" || !reflect.DeepEqual(calls, []string{`POST /repos/o/r/issues/7/comments {"body":"done now"}`}) {
		t.Fatalf("failed comment %v calls=%q", err, calls)
	}
	calls = nil
	status = 200
	src.issue.State = "closed"
	out.Reset()
	if err := runIssuesStateForge(&out, target, src, "reopen", "7", "", false); err != nil || out.String() != "reopened codeberg.org/o/r#7\n" || !reflect.DeepEqual(calls, []string{`PATCH /repos/o/r/issues/7 {"state":"open"}`}) {
		t.Fatalf("reopen %q %v %q", out.String(), err, calls)
	}
	calls = nil
	out.Reset()
	if err := runIssuesStateForge(&out, target, src, "close", "7", "", false); err != nil || out.String() != "codeberg.org/o/r#7 is already closed\n" || len(calls) != 0 {
		t.Fatalf("already %q %v", out.String(), err)
	}
	src.issue.State = "open"
	out.Reset()
	if err := runIssuesStateForge(&out, target, src, "reopen", "7", "", false); err != nil || out.String() != "codeberg.org/o/r#7 is already open\n" || len(calls) != 0 {
		t.Fatalf("already %q %v", out.String(), err)
	}
	status = 412
	err = runIssuesStateForge(io.Discard, target, src, "close", "7", "", false)
	if err == nil || err.Error() != "forge API request failed: HTTP 412 safe detail" {
		t.Fatal(err)
	}
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "worker")
	calls = nil
	err = runIssuesStateForge(io.Discard, target, src, "close", "7", "", false)
	if err == nil || err.Error() != "issues close: forge writes run only in a host session (this is harnez worker worker); hand the write to the host" || len(calls) != 0 {
		t.Fatalf("worker %v", err)
	}
	target.Forge.Kind = "github"
	err = runIssuesStateForge(io.Discard, target, src, "reopen", "7", "", false)
	if err == nil || err.Error() != "issues reopen: forge writes support Forgejo only; GitHub is ticket 1209" || len(calls) != 0 {
		t.Fatalf("github %v", err)
	}
}

func TestIssuesForgeOpenAndLocalReopen(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@codeberg.org:o/r.git"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	_, err := runHarnez(t, "issues", "open", "-d", dir, "7")
	if err == nil || err.Error() != "issues open: on a forge use reopen" {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err = runHarnez(t, "issues", "reopen", "-d", dir, "7")
	if err == nil || err.Error() != "issues reopen: local tracker; use harnez issues open <n>" {
		t.Fatal(err)
	}
}

func TestForgeEditStdin(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	oldLookup, oldStdin := issueWriteLookup, os.Stdin
	issueWriteLookup = func(string) string { return "secret" }
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("one\n\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	os.Stdin = f
	t.Cleanup(func() { issueWriteLookup, os.Stdin = oldLookup, oldStdin; f.Close() })
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		io.WriteString(w, `{"body":"one\n\ntwo\n"}`)
	}))
	defer srv.Close()
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", APIBase: srv.URL}, Owner: "o", Repo: "r"}
	err = runIssuesEditForge(io.Discard, target, &editSource{issue: forge.RemoteIssue{State: "open"}}, "7", "", "-", "", false, true, false, false)
	if err != nil || got != `{"body":"one\n\ntwo\n"}` {
		t.Fatalf("body %q: %v", got, err)
	}
}

func TestForgeDryRunDoesNotTouchLocalTicket(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	stubForge(t)
	dir := forgeRepo(t)
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "issues", "007-local.md")
	original := []byte("# 007 — local\n\n**Status**: Open\n**Priority**: P2\n\n---\nbody\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "config", "user.email", "test@example.test")
	repoRunGit(t, dir, "config", "user.name", "Test")
	repoRunGit(t, dir, "add", "issues/007-local.md")
	repoRunGit(t, dir, "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(dir, "code"), []byte("code"), 0644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "code")
	repoRunGit(t, dir, "commit", "-m", "implement #7")
	before := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"close", "7"}, "dry run: would set codeberg.org/o/r#7 to closed over the API\n"},
		{[]string{"close", "7", "done", "now"}, "dry run: would comment \"done now\" on codeberg.org/o/r#7 and set state to closed over the API\n"},
		{[]string{"reopen", "7"}, "dry run: would set codeberg.org/o/r#7 to open over the API\n"},
		{[]string{"edit", "7", "--title", "X"}, "dry run: would edit codeberg.org/o/r#7 over the API --title \"X\"\n"},
	}
	for _, tc := range cases {
		args := append([]string{"issues", "--forge", "--dry-run", "-d", dir}, tc.args...)
		out, err := runHarnez(t, args...)
		if err != nil || out != tc.want {
			t.Errorf("%v: out=%q err=%v", tc.args, out, err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatalf("ticket changed: %q %v", got, err)
		}
	}
	if after := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); after != before {
		t.Fatalf("unexpected commit: %s -> %s", before, after)
	}
	if out, err := runHarnez(t, "issues", "close", "-d", dir, "7", "reason"); err != nil {
		t.Fatalf("local close: %q %v", out, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(got), "**Status**: Closed — reason") {
		t.Fatalf("local close: %q %v", got, err)
	}

}

func TestForgeMissingNumber(t *testing.T) {
	for _, verb := range []string{"close", "edit", "open"} {
		_, err := runHarnez(t, "issues", verb, "--forge")
		if err == nil || err.Error() != "issues "+verb+": requires an issue number" {
			t.Errorf("%s: %v", verb, err)
		}
	}
}

func TestForgeCloseReportsPostedReasonOnFailure(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	old := issueWriteLookup
	issueWriteLookup = func(string) string { return "secret" }
	t.Cleanup(func() { issueWriteLookup = old })
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		if r.Method == "POST" {
			io.WriteString(w, `{"html_url":"https://example.test/comment"}`)
			return
		}
		w.WriteHeader(412)
		io.WriteString(w, `{"message":"conflict"}`)
	}))
	defer srv.Close()
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", APIBase: srv.URL}, Owner: "o", Repo: "r"}
	err := runIssuesStateForge(io.Discard, target, &editSource{issue: forge.RemoteIssue{State: "open"}}, "close", "7", "done now", false)
	want := "issues close: the reason was posted as a comment (https://example.test/comment) but closing failed: forge API request failed: HTTP 412 conflict; run close again without a reason"
	if err == nil || err.Error() != want || !reflect.DeepEqual(calls, []string{"POST", "PATCH"}) {
		t.Fatalf("%v calls=%v", err, calls)
	}
}

func TestForgeRefinementEdges(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	oldLookup, oldMeta := issueWriteLookup, issueWriteMetadata
	t.Cleanup(func() { issueWriteLookup, issueWriteMetadata = oldLookup, oldMeta })
	target := forgetracker.Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org", IssueCLI: []string{"fj"}}, Owner: "o", Repo: "r"}
	src := &editSource{issue: forge.RemoteIssue{State: "open"}}
	issueWriteLookup = func(string) string { return "" }
	for _, verb := range []string{"close", "reopen"} {
		err := runIssuesStateForge(io.Discard, target, src, verb, "7", "", false)
		if err == nil || err.Error() != "no write token for codeberg.org; run: harnez forge token create --write" {
			t.Fatalf("%s: %v", verb, err)
		}
	}
	issueWriteLookup = func(string) string { return "secret" }
	src.issue.PullRequest = &struct{}{}
	for _, verb := range []string{"close", "reopen"} {
		err := runIssuesStateForge(io.Discard, target, src, verb, "7", "", false)
		if err == nil || err.Error() != "issues "+verb+": o/r#7 is a pull request" {
			t.Fatalf("%s: %v", verb, err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Method != "PATCH" || string(b) != `{"title":"X"}` {
			t.Errorf("request: %s %s", r.Method, b)
		}
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	target.Forge.APIBase = srv.URL
	src.issue = forge.RemoteIssue{State: "closed"}
	var out bytes.Buffer
	if err := runIssuesEditForge(&out, target, src, "7", "X", "", "", true, false, false, true); err != nil || out.String() != "edited codeberg.org/o/r#7\n" {
		t.Fatalf("closed edit: %q %v", out.String(), err)
	}
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "worker")
	issueWriteLookup = func(string) string { t.Fatal("worker looked up secret"); return "" }
	err := runIssuesEditForge(io.Discard, target, src, "7", "X", "", "", true, false, false, true)
	if err == nil || err.Error() != "issues edit: forge writes run only in a host session (this is harnez worker worker); hand the write to the host" {
		t.Fatalf("edit: %v", err)
	}
	for _, verb := range []string{"close", "reopen"} {
		err := runIssuesStateForge(io.Discard, target, src, verb, "7", "", false)
		if err == nil || err.Error() != "issues "+verb+": forge writes run only in a host session (this is harnez worker worker); hand the write to the host" {
			t.Fatalf("%s: %v", verb, err)
		}
	}
	issueWriteMetadata = func(string) (bool, error) { return false, nil }
	body := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(body, []byte("body"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runIssuesNewForge(&out, io.Discard, t.TempDir(), target, forgetracker.CreateRequest{Title: "T", BodyFile: body}, true, false); err != nil || out.String() != "dry run, would file on codeberg.org/o/r: fj issue create --repo codeberg.org/o/r --body-file "+body+" T\n" {
		t.Fatalf("new dry: %q %v", out.String(), err)
	}
	out.Reset()
	if err := runIssuesComment(&out, t.TempDir(), target, &stubForgeSource{}, "7", body, true, false); err != nil || out.String() != "dry run: fj issue comment codeberg.org/o/r#7 --body-file "+body+"\n" {
		t.Fatalf("comment dry: %q %v", out.String(), err)
	}
}

func TestTrackerNoneDraftsRoutesForge(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "m9-worker")
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	if err := os.Unsetenv("CLAUDE_CODE_REMOTE"); err != nil {
		t.Fatal(err)
	}
	stub := stubForge(t)
	dir := forgeRepo(t)
	for _, d := range []string{".harnez", "issues/inbox"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".harnez/repo.yaml"), []byte("version: 1\nissues:\n  tracker: none\n  merge_driver: false\ndocs:\n  studies: false\n  copies: []\nrules:\n  quota: false\nmake:\n  container_test: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if cfg, err := loadIndexRepoConfig(dir); err != nil || cfg.Issues.Tracker != "none" {
		t.Fatalf("config: %+v %v", cfg, err)
	}
	body := filepath.Join(dir, "issues/inbox/a.md")
	if err := os.WriteFile(body, []byte("draft"), 0644); err != nil {
		t.Fatal(err)
	}
	oldLookup := issueWriteLookup
	issueWriteLookup = func(string) string { return "test-token" }
	t.Cleanup(func() { issueWriteLookup = oldLookup })
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"issues", "new", "T", "-F", body, "--dry-run"}, "dry run, would file on codeberg.org/o/r: fj issue create --repo codeberg.org/o/r --body-file " + body + " T\n"},
		{[]string{"issues", "comment", "7", "-F", body, "--dry-run"}, "dry run: fj issue comment codeberg.org/o/r#7 --body-file " + body + "\n"},
		{[]string{"issues", "show", "7"}, "# 7 — forge ticket"},
		{[]string{"issues", "edit", "7", "--title", "X", "--dry-run"}, "dry run: would edit codeberg.org/o/r#7 over the API --title \"X\"\n"},
		{[]string{"issues", "close", "7", "--dry-run"}, "dry run: would set codeberg.org/o/r#7 to closed over the API\n"},
		{[]string{"issues", "reopen", "7", "--dry-run"}, "dry run: would set codeberg.org/o/r#7 to open over the API\n"},
		{[]string{"find", "issues", "forge"}, "7\topen\tforge ticket"},
		{[]string{"issues", "list"}, "7\topen\tforge ticket"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args[:2], "_"), func(t *testing.T) {
			args := append([]string{}, tc.args...)
			args = append(args, "-d", dir)
			out, err := runHarnez(t, args...)
			if err != nil || !strings.Contains(out, tc.want) || !strings.Contains(out, "issues.tracker is none: tickets live on codeberg.org/o/r\n") {
				t.Fatalf("got %q, %v; want %q", out, err, tc.want)
			}
		})
	}
	if stub.calls == 0 {
		t.Error("forge source was not called")
	}
}

func TestTrackerNoneLocalCases(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "developer")
	t.Setenv("HARNEZ_SESSION_ID", "m9-worker")
	for _, tc := range []struct{ name, tracker, entry, origin string }{
		{"ticket", "none", "007-x.md", ""},
		{"archive", "none", "archive/", ""},
		{"harnez", "harnez", "inbox/a.md", ""},
		{"missing config", "", "inbox/a.md", ""},
		{"unknown origin", "none", "inbox/a.md", "https://example.invalid/o/r.git"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := stubForge(t)
			dir := forgeRepo(t)
			if tc.origin != "" {
				if out, err := exec.Command("git", "-C", dir, "remote", "set-url", "origin", tc.origin).CombinedOutput(); err != nil {
					t.Fatalf("remote: %s %v", out, err)
				}
			}
			if tc.tracker != "" {
				if err := os.MkdirAll(filepath.Join(dir, ".harnez"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".harnez/repo.yaml"), []byte("version: 1\nissues:\n  tracker: "+tc.tracker+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			entry := filepath.Join(dir, "issues", tc.entry)
			if strings.HasSuffix(tc.entry, "/") {
				if err := os.MkdirAll(entry, 0755); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(entry), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(entry, []byte("# 007 — local\n\n**Status**: Open\n**Priority**: P2\n\n---\nbody\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := runHarnez(t, "issues", "new", "-d", dir, "T", "--label", "bug")
			want := "issues new: -F/--body-file and --label apply only when tickets live on the forge; with a local issues/ dir use --forge"
			if err == nil || err.Error() != want {
				t.Fatalf("got %v", err)
			}
			if tc.name == "ticket" {
				out, err := runHarnez(t, "issues", "show", "-d", dir, "--text", "7")
				if err != nil || !strings.Contains(out, "local") {
					t.Fatalf("show %q: %v", out, err)
				}
				out, err = runHarnez(t, "issues", "close", "-d", dir, "--no-commit", "--no-code", "7", "r")
				if err != nil {
					t.Fatalf("close %q: %v", out, err)
				}
				data, err := os.ReadFile(entry)
				if err != nil || !strings.Contains(string(data), "**Status**: Closed") {
					t.Fatalf("closed ticket %q: %v", data, err)
				}
			}
			if stub.calls != 0 {
				t.Fatalf("forge calls: %d", stub.calls)
			}
		})
	}
}
