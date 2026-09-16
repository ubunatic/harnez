package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/issues"
)

// issuesFixtureRepo builds a minimal git repo with an issues/ tree
// containing one ticket + issues/README.md, so runIssuesVerb can be
// exercised end to end (rewrite + resync + commit).
func issuesFixtureRepo(t *testing.T, ticketContent string) (dir, ticketPath string) {
	t.Helper()
	dir = repoInit(t)
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ticketPath = filepath.Join(issuesDir, "042-example-ticket.md")
	if err := os.WriteFile(ticketPath, []byte(ticketContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "README.md"),
		[]byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "issues")
	repoRunGit(t, dir, "commit", "-q", "-m", "add ticket 042")
	return dir, ticketPath
}

const sampleTicket = "# 042 — Example Ticket\n\n**Status**: Open\n\n---\n\nSee [[041-other-ticket]] for context.\n"

func TestComposeNewStatus_Table(t *testing.T) {
	cases := []struct {
		verb, reason, want string
		wantErr            bool
	}{
		{"open", "", "Open", false},
		{"open", "deferred", "Open — deferred", false},
		{"start", "", "In Progress", false},
		{"start", "picking this up to unblock 043", "In Progress — picking this up to unblock 043", false},
		{"block", "waiting on upstream", "Blocked — waiting on upstream", false},
		{"block", "", "", true},
		{"close", "", "Closed", false},
		{"close", "resolved", "Closed — resolved", false},
		{"done", "", "Closed", false},
		{"done", "resolved", "Closed — resolved", false},
		{"draft", "", "Draft", false},
		{"draft", "x", "Draft — x", false},
		{"review", "", "", true},
	}
	for _, c := range cases {
		got, err := composeNewStatus(c.verb, c.reason)
		if c.wantErr {
			if err == nil {
				t.Errorf("composeNewStatus(%q,%q): expected error, got %q", c.verb, c.reason, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("composeNewStatus(%q,%q): unexpected error: %v", c.verb, c.reason, err)
			continue
		}
		if got != c.want {
			t.Errorf("composeNewStatus(%q,%q) = %q, want %q", c.verb, c.reason, got, c.want)
		}
	}
}

func TestRunIssuesVerb_CloseRewritesStatusAndPreservesRestOfFile(t *testing.T) {
	dir, ticketPath := issuesFixtureRepo(t, sampleTicket)

	var out bytes.Buffer
	result, drift, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesVerb: %v", err)
	}
	if drift {
		t.Errorf("expected drift=false outside --check")
	}
	if result.OldStatus != "Open" || result.NewStatus != "Closed — resolved" {
		t.Errorf("unexpected status transition: %+v", result)
	}
	if result.Noop {
		t.Errorf("expected non-noop change")
	}
	if !result.ReadmeUpdated {
		t.Errorf("expected README to be updated")
	}
	if !result.Committed || result.CommitSHA == "" {
		t.Errorf("expected a commit to be made, got %+v", result)
	}

	got, err := os.ReadFile(ticketPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "# 042 — Example Ticket\n\n**Status**: Closed — resolved\n\n---\n\nSee [[041-other-ticket]] for context.\n"
	if string(got) != want {
		t.Errorf("ticket file rewritten unexpectedly.\ngot:  %q\nwant: %q", string(got), want)
	}

	readme, err := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "Closed — resolved") {
		t.Errorf("README not resynced with new status, got:\n%s", readme)
	}

	// Verify the commit actually happened and staged exactly the two files.
	logOut := repoRunGitOutput(t, dir, "log", "-1", "--name-only", "--format=%s")
	if !strings.Contains(logOut, "docs(issues): close 042, resolved") {
		t.Errorf("unexpected commit message, got:\n%s", logOut)
	}
	if !strings.Contains(logOut, "issues/042-example-ticket.md") || !strings.Contains(logOut, "issues/README.md") {
		t.Errorf("expected commit to touch ticket file + README, got:\n%s", logOut)
	}
}

func TestRunIssuesVerb_BareCloseDoesNotFabricateReason(t *testing.T) {
	dir, ticketPath := issuesFixtureRepo(t, sampleTicket)

	var out bytes.Buffer
	result, _, err := runIssuesVerb(&out, "close", "42", nil, issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesVerb: %v", err)
	}
	if result.NewStatus != "Closed" {
		t.Errorf("expected bare 'Closed', got %q", result.NewStatus)
	}
	got, _ := os.ReadFile(ticketPath)
	if !strings.Contains(string(got), "**Status**: Closed\n") {
		t.Errorf("ticket file does not have bare Closed status:\n%s", got)
	}
}

func TestRunIssuesVerb_DoneAliasForClose(t *testing.T) {
	dir, ticketPath := issuesFixtureRepo(t, sampleTicket)

	var out bytes.Buffer
	result, drift, err := runIssuesVerb(&out, "done", "42", []string{"completed"}, issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesVerb: %v", err)
	}
	if drift {
		t.Errorf("expected drift=false outside --check")
	}
	if result.OldStatus != "Open" || result.NewStatus != "Closed — completed" {
		t.Errorf("unexpected status transition: %+v", result)
	}
	got, err := os.ReadFile(ticketPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "# 042 — Example Ticket\n\n**Status**: Closed — completed\n\n---\n\nSee [[041-other-ticket]] for context.\n"
	if string(got) != want {
		t.Errorf("ticket file rewritten unexpectedly.\ngot:  %q\nwant: %q", string(got), want)
	}
	logOut := repoRunGitOutput(t, dir, "log", "-1", "--name-only", "--format=%s")
	if !strings.Contains(logOut, "docs(issues): close 042, completed") {
		t.Errorf("unexpected commit message, got:\n%s", logOut)
	}
}

func TestRunIssuesVerb_IdempotentNoopSkipsWriteReadmeAndCommit(t *testing.T) {
	dir, ticketPath := issuesFixtureRepo(t, sampleTicket)

	// First call: Open -> Closed — resolved.
	var out bytes.Buffer
	first, _, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("first runIssuesVerb: %v", err)
	}
	firstSHA := first.CommitSHA

	beforeReadme, _ := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	beforeTicket, _ := os.ReadFile(ticketPath)

	// Second call with the exact same verb+reason must be a no-op.
	second, drift, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("second runIssuesVerb: %v", err)
	}
	if !second.Noop {
		t.Errorf("expected second identical close to be a no-op, got %+v", second)
	}
	if drift {
		t.Errorf("expected drift=false for a true no-op")
	}
	if second.Committed {
		t.Errorf("no-op must not commit")
	}
	if second.ReadmeUpdated {
		t.Errorf("no-op must not touch README")
	}

	afterReadme, _ := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	afterTicket, _ := os.ReadFile(ticketPath)
	if string(beforeReadme) != string(afterReadme) {
		t.Errorf("README changed on a no-op call")
	}
	if string(beforeTicket) != string(afterTicket) {
		t.Errorf("ticket file changed on a no-op call")
	}

	headSHA := repoRunGitOutput(t, dir, "rev-parse", "--short", "HEAD")
	if strings.TrimSpace(headSHA) != firstSHA {
		t.Errorf("expected no new commit after no-op, HEAD moved from %s to %s", firstSHA, strings.TrimSpace(headSHA))
	}
}

// TestRunIssuesVerb_ReasonChangeOnAlreadyClosedIsNotIdempotent covers issue
// 232's distinction: correcting the reason text on an already-Closed
// ticket (e.g. filling in a real commit sha per issue 126 option A) is a
// real, expected action and must proceed normally, not be treated as a
// no-op just because the canonical category (Closed) is unchanged.
func TestRunIssuesVerb_ReasonChangeOnAlreadyClosedIsNotIdempotent(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	var out bytes.Buffer
	if _, _, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir}); err != nil {
		t.Fatalf("first close: %v", err)
	}

	result, _, err := runIssuesVerb(&out, "close", "42", []string{"resolved in abc1234"}, issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("reason-correcting close: %v", err)
	}
	if result.Noop {
		t.Errorf("a reason change on an already-closed ticket must not be treated as a no-op")
	}
	if !result.Committed {
		t.Errorf("expected reason correction to commit")
	}
	if result.NewStatus != "Closed — resolved in abc1234" {
		t.Errorf("unexpected new status: %q", result.NewStatus)
	}
}

func TestRunIssuesVerb_CheckReportsDriftAndDoesNotWriteOrCommit(t *testing.T) {
	dir, ticketPath := issuesFixtureRepo(t, sampleTicket)
	beforeTicket, _ := os.ReadFile(ticketPath)
	beforeReadme, _ := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	beforeHead := repoRunGitOutput(t, dir, "rev-parse", "--short", "HEAD")

	var out bytes.Buffer
	result, drift, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir, Check: true})
	if err != nil {
		t.Fatalf("runIssuesVerb --check: %v", err)
	}
	if !drift {
		t.Errorf("expected drift=true for a real status change under --check")
	}
	if !result.ReadmeUpdated {
		t.Errorf("expected --check to report the README would be updated")
	}
	if result.Committed {
		t.Errorf("--check must never commit")
	}

	afterTicket, _ := os.ReadFile(ticketPath)
	afterReadme, _ := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	afterHead := repoRunGitOutput(t, dir, "rev-parse", "--short", "HEAD")
	if string(beforeTicket) != string(afterTicket) {
		t.Errorf("--check must not modify the ticket file on disk")
	}
	if string(beforeReadme) != string(afterReadme) {
		t.Errorf("--check must not modify README.md on disk")
	}
	if beforeHead != afterHead {
		t.Errorf("--check must not create a commit")
	}
}

func TestRunIssuesVerb_CheckNoopReportsNoDrift(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	var out bytes.Buffer
	// Ticket is already "Open"; requesting "open" again should be a no-op,
	// not drift.
	result, drift, err := runIssuesVerb(&out, "open", "42", nil, issuesRunOptions{Dir: dir, Check: true})
	if err != nil {
		t.Fatalf("runIssuesVerb --check: %v", err)
	}
	if drift {
		t.Errorf("expected drift=false for a true no-op under --check")
	}
	if !result.Noop {
		t.Errorf("expected Noop=true")
	}
}

func TestRunIssuesVerb_NoCommitSkipsGit(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	beforeHead := repoRunGitOutput(t, dir, "rev-parse", "--short", "HEAD")

	var out bytes.Buffer
	result, _, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir, NoCommit: true})
	if err != nil {
		t.Fatalf("runIssuesVerb: %v", err)
	}
	if result.Committed {
		t.Errorf("--no-commit must not commit")
	}
	if !result.ReadmeUpdated {
		t.Errorf("--no-commit should still resync README")
	}

	afterHead := repoRunGitOutput(t, dir, "rev-parse", "--short", "HEAD")
	if beforeHead != afterHead {
		t.Errorf("--no-commit must not create a commit")
	}

	status := repoRunGitOutput(t, dir, "status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		t.Errorf("expected uncommitted changes to remain in the working tree")
	}
}

func TestRunIssuesVerb_CustomCommitMessage(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	var out bytes.Buffer
	_, _, err := runIssuesVerb(&out, "close", "42", []string{"resolved"}, issuesRunOptions{Dir: dir, CommitMsg: "custom message here"})
	if err != nil {
		t.Fatalf("runIssuesVerb: %v", err)
	}
	subject := repoRunGitOutput(t, dir, "log", "-1", "--format=%s")
	if strings.TrimSpace(subject) != "custom message here" {
		t.Errorf("expected custom commit message, got %q", subject)
	}
}

func TestRunIssuesVerb_TicketNotFound(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	var out bytes.Buffer
	_, _, err := runIssuesVerb(&out, "close", "999", nil, issuesRunOptions{Dir: dir})
	if err == nil {
		t.Fatalf("expected an error for a nonexistent ticket number")
	}
	if !strings.Contains(err.Error(), "no ticket found") {
		t.Errorf("expected an actionable 'no ticket found' message, got: %v", err)
	}
}

// TestRunIssuesNew_NoTitleCreatesReservedPlaceholder covers the plain
// `harnez issues new` case (no title): it must allocate the next free
// number, create issues/<NNN>-reserved.md with Status: Draft, and print
// "<NUMBER>\t<PATH>" -- without requiring any existing ticket file, unlike
// every other issues verb (findTicketFile is never consulted).
func TestRunIssuesNew_NoTitleCreatesReservedPlaceholder(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)

	var out bytes.Buffer
	if err := runIssuesNew(&out, dir, "", false); err != nil {
		t.Fatalf("runIssuesNew: %v", err)
	}
	want := "043\tissues/043-reserved.md\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
	reservedPath := filepath.Join(dir, "issues", "043-reserved.md")
	content, err := os.ReadFile(reservedPath)
	if err != nil {
		t.Fatalf("expected reserved file to exist: %v", err)
	}
	if !strings.Contains(string(content), "**Status**: Draft") {
		t.Errorf("expected Draft placeholder status, got:\n%s", content)
	}
}

// TestRunIssuesNew_WithTitleSlugifiesFilename covers the titled case and
// the --json output shape, mirroring the exact fields
// `find issues next --reserve --json` used to emit before issue 233.
func TestRunIssuesNew_WithTitleSlugifiesFilename(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)

	var out bytes.Buffer
	if err := runIssuesNew(&out, dir, "New Feature", false); err != nil {
		t.Fatalf("runIssuesNew: %v", err)
	}
	want := "043\tissues/043-new-feature.md\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}

	out.Reset()
	if err := runIssuesNew(&out, dir, "JSON Feature", true); err != nil {
		t.Fatalf("runIssuesNew --json: %v", err)
	}
	wantJSON := `{"number":"044","reserved":true,"file":"044-json-feature.md","path":"issues/044-json-feature.md"}` + "\n"
	if out.String() != wantJSON {
		t.Errorf("runIssuesNew --json = %q, want %q", out.String(), wantJSON)
	}
}

// TestRunIssuesNew_ODirectExclPreventsCollisions reproduces the O_EXCL
// collision-safety behavior issue 233 requires 'new' to preserve exactly:
// if the target file already exists (e.g. another concurrent caller won the
// race for that number), Reserve retries with the next number rather than
// clobbering the existing file or erroring out.
func TestRunIssuesNew_ODirectExclPreventsCollisions(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)

	// Pre-create the file the next reservation would naturally land on,
	// simulating a concurrent winner of ticket 043.
	collidingPath := filepath.Join(dir, "issues", "043-reserved.md")
	if err := os.WriteFile(collidingPath, []byte("pre-existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runIssuesNew(&out, dir, "", false); err != nil {
		t.Fatalf("runIssuesNew: %v", err)
	}
	want := "044\tissues/044-reserved.md\n"
	if out.String() != want {
		t.Errorf("got %q, want %q (expected collision to be skipped past)", out.String(), want)
	}

	// The pre-existing colliding file must be untouched.
	got, err := os.ReadFile(collidingPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "pre-existing content\n" {
		t.Errorf("collision victim file was overwritten, got:\n%s", got)
	}
}

// TestRunIssuesNew_DoesNotRequireExistingTicketFile confirms 'new' does not
// go through findTicketFile the way every other verb does: it must succeed
// in an issues/ directory that starts out completely empty.
func TestRunIssuesNew_DoesNotRequireExistingTicketFile(t *testing.T) {
	dir := repoInit(t)
	// Deliberately no issues/ directory at all yet.
	var out bytes.Buffer
	if err := runIssuesNew(&out, dir, "First Ticket", false); err != nil {
		t.Fatalf("runIssuesNew on empty repo: %v", err)
	}
	want := "001\tissues/001-first-ticket.md\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

func TestRunIssuesVerb_BlockRequiresReason(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	var out bytes.Buffer
	_, _, err := runIssuesVerb(&out, "block", "42", nil, issuesRunOptions{Dir: dir})
	if err == nil {
		t.Fatalf("expected an error when 'block' is given no reason")
	}
}

func TestPrintIssuesResult_JSONShape(t *testing.T) {
	result := issuesResult{
		Number:        "232",
		File:          "issues/232-example.md",
		OldStatus:     "Open",
		NewStatus:     "Closed — resolved",
		ReadmeUpdated: true,
		Committed:     true,
		CommitSHA:     "abc1234",
		Noop:          false,
	}
	var out bytes.Buffer
	printIssuesResult(&out, result, issuesRunOptions{JSON: true})

	var decoded map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	for _, key := range []string{"number", "file", "old_status", "new_status", "readme_updated", "committed", "commit_sha", "noop"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("JSON output missing key %q, got: %s", key, out.String())
		}
	}
	if decoded["number"] != "232" || decoded["new_status"] != "Closed — resolved" {
		t.Errorf("unexpected JSON field values: %s", out.String())
	}
}

func TestFormatIssuesLine_NoopVsChange(t *testing.T) {
	noop := issuesResult{Number: "042", NewStatus: "Closed", Noop: true}
	if line := formatIssuesLine(noop, false); !strings.Contains(line, "already Closed") || !strings.Contains(line, "no change") {
		t.Errorf("unexpected noop line: %q", line)
	}

	changed := issuesResult{Number: "042", OldStatus: "Open", NewStatus: "Closed — resolved", ReadmeUpdated: true, Committed: true, CommitSHA: "abc1234"}
	line := formatIssuesLine(changed, false)
	if !strings.Contains(line, "Open -> Closed — resolved") || !strings.Contains(line, "committed abc1234") {
		t.Errorf("unexpected change line: %q", line)
	}

	checkLine := formatIssuesLine(changed, true)
	if !strings.Contains(checkLine, "would commit") {
		t.Errorf("expected --check line to say 'would commit', got: %q", checkLine)
	}
}

// repoRunGitOutput runs a git command in dir and returns its trimmed stdout,
// failing the test on a nonzero exit.
func repoRunGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func TestRunIssuesMv_HappyPath(t *testing.T) {
	dir, oldTicketPath := issuesFixtureRepo(t, sampleTicket)

	var out bytes.Buffer
	result, drift, err := runIssuesMv(&out, "42", "268", issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesMv: %v", err)
	}
	if drift {
		t.Errorf("expected drift=false outside --check")
	}
	if result.Number != "268" || result.OldNumber != "042" {
		t.Errorf("unexpected numbers in result: %+v", result)
	}
	if result.File != "issues/268-example-ticket.md" {
		t.Errorf("unexpected file in result: %q", result.File)
	}
	if !result.ReadmeUpdated {
		t.Errorf("expected README to be updated")
	}
	if !result.Committed || result.CommitSHA == "" {
		t.Errorf("expected commit to be created: %+v", result)
	}

	// Old file must be gone
	if _, err := os.Stat(oldTicketPath); !os.IsNotExist(err) {
		t.Errorf("old ticket file still exists at %s", oldTicketPath)
	}

	// New file must exist with rewritten header
	newPath := filepath.Join(dir, "issues", "268-example-ticket.md")
	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read new ticket file: %v", err)
	}
	wantContent := "# 268 — Example Ticket\n\n**Status**: Open\n\n---\n\nSee [[041-other-ticket]] for context.\n"
	if string(got) != wantContent {
		t.Errorf("new ticket content mismatch:\ngot:  %q\nwant: %q", string(got), wantContent)
	}

	// README must refer to 268 and not 042
	readme, err := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if strings.Contains(string(readme), "042") {
		t.Errorf("README still contains 042:\n%s", readme)
	}
	if !strings.Contains(string(readme), "268") || !strings.Contains(string(readme), "268-example-ticket.md") {
		t.Errorf("README does not contain 268 row:\n%s", readme)
	}

	// Commit must stage old file deletion/rename, new file addition, and README modification
	logOut := repoRunGitOutput(t, dir, "log", "-1", "--name-status", "--format=%s")
	if !strings.Contains(logOut, "docs(issues): renumber 042 to 268") {
		t.Errorf("unexpected commit message: %s", logOut)
	}
	if !strings.Contains(logOut, "issues/042-example-ticket.md") || !strings.Contains(logOut, "issues/268-example-ticket.md") || !strings.Contains(logOut, "issues/README.md") {
		t.Errorf("commit does not touch all expected files:\n%s", logOut)
	}
}

func TestRunIssuesMv_DefaultNextTargetNumber(t *testing.T) {
	dir := repoInit(t)
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t1 := filepath.Join(issuesDir, "042-ticket.md")
	if err := os.WriteFile(t1, []byte("# 042 — Ticket One\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t2 := filepath.Join(issuesDir, "099-another.md")
	if err := os.WriteFile(t2, []byte("# 099 — Ticket Two\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "README.md"),
		[]byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "issues")
	repoRunGit(t, dir, "commit", "-q", "-m", "init tickets")

	filesBefore, err := issues.Scan(issuesDir)
	if err != nil {
		t.Fatal(err)
	}
	wantNext := issues.NextNumberFromFiles(filesBefore)

	var out bytes.Buffer
	result, _, err := runIssuesMv(&out, "42", "", issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesMv: %v", err)
	}
	if result.Number != wantNext {
		t.Errorf("result.Number = %q, want %q", result.Number, wantNext)
	}
	if _, err := os.Stat(filepath.Join(issuesDir, wantNext+"-ticket.md")); err != nil {
		t.Errorf("expected target file %s-ticket.md to exist: %v", wantNext, err)
	}
	if _, err := os.Stat(t1); !os.IsNotExist(err) {
		t.Errorf("expected old file 042-ticket.md to be removed")
	}
}

func TestRunIssuesMv_CollisionScenarioGitPull(t *testing.T) {
	// Reproduce the actual git pull collision from issue 269:
	// Two files in the working tree claiming 266 with different slugs,
	// and a 267 file.
	dir := repoInit(t)
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tTelemetry := filepath.Join(issuesDir, "266-telemetry-ticket.md")
	if err := os.WriteFile(tTelemetry, []byte("# 266 — Telemetry Ticket\n\n**Status**: Open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tExec := filepath.Join(issuesDir, "266-exec-timeout.md")
	if err := os.WriteFile(tExec, []byte("# 266 — Exec Timeout\n\n**Status**: In Progress\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tHooks := filepath.Join(issuesDir, "267-hooks-fix.md")
	if err := os.WriteFile(tHooks, []byte("# 267 — Hooks Fix\n\n**Status**: Closed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Conflicted README table with duplicate 266 rows
	readmeContent := `# Issues Tracker

| # | File | Title | Status |
|---|------|-------|--------|
| 266 | [266-telemetry-ticket.md](266-telemetry-ticket.md) | Telemetry Ticket | Open |
| 266 | [266-exec-timeout.md](266-exec-timeout.md) | Exec Timeout | In Progress |
| 267 | [267-hooks-fix.md](267-hooks-fix.md) | Hooks Fix | Closed |
`
	if err := os.WriteFile(filepath.Join(issuesDir, "README.md"), []byte(readmeContent), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "issues")
	repoRunGit(t, dir, "commit", "-q", "-m", "merge conflict state")

	// Verify linter detects the duplicate issue number
	reportBefore, err := issues.Lint(issuesDir)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	hasDup := false
	for _, d := range reportBefore.Diagnostics {
		if d.Kind == issues.DiagDuplicateNumber && d.IssueNum == "266" {
			hasDup = true
			break
		}
	}
	if !hasDup {
		t.Fatalf("expected Lint to report DiagDuplicateNumber before mv, got: %+v", reportBefore.Diagnostics)
	}

	// Attempting bare "266" should fail with ambiguous error because 2 files match number 266
	var out bytes.Buffer
	_, _, err = runIssuesMv(&out, "266", "", issuesRunOptions{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous ticket error for bare '266', got: %v", err)
	}

	// Renumber the colliding 266-exec-timeout.md by filename to next free number (268)
	out.Reset()
	result, _, err := runIssuesMv(&out, "266-exec-timeout.md", "", issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesMv disambiguated: %v", err)
	}
	if result.Number != "268" {
		t.Errorf("result.Number = %q, want 268", result.Number)
	}

	// Assert:
	// 1. Renamed file exists at new path with corrected header
	renamedPath := filepath.Join(issuesDir, "268-exec-timeout.md")
	content, err := os.ReadFile(renamedPath)
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	if !strings.HasPrefix(string(content), "# 268 — Exec Timeout") {
		t.Errorf("unexpected header in renamed file:\n%s", content)
	}

	// 2. Old colliding file is removed
	if _, err := os.Stat(tExec); !os.IsNotExist(err) {
		t.Errorf("old file 266-exec-timeout.md still exists")
	}

	// 3. Slot 266 has exactly one remaining file
	if _, err := os.Stat(tTelemetry); err != nil {
		t.Errorf("266-telemetry-ticket.md missing: %v", err)
	}

	// 4. README has no duplicate-number rows and no conflict marker strings
	readmeBytes, err := os.ReadFile(filepath.Join(issuesDir, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readmeStr := string(readmeBytes)
	for _, marker := range []string{"<<<<<<<", "=======", ">>>>>>>"} {
		if strings.Contains(readmeStr, marker) {
			t.Errorf("README contains leftover conflict marker %q", marker)
		}
	}

	// 5. Lint reports 0 DiagDuplicateNumber diagnostics
	reportAfter, err := issues.Lint(issuesDir)
	if err != nil {
		t.Fatalf("Lint after: %v", err)
	}
	for _, d := range reportAfter.Diagnostics {
		if d.Kind == issues.DiagDuplicateNumber {
			t.Errorf("unexpected duplicate number diagnostic after mv: %+v", d)
		}
	}
}

func TestRunIssuesMv_TargetOccupiedFailsWithoutSideEffects(t *testing.T) {
	dir := repoInit(t)
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t1 := filepath.Join(issuesDir, "042-one.md")
	c1 := "# 042 — One\n\n**Status**: Open\n"
	if err := os.WriteFile(t1, []byte(c1), 0o644); err != nil {
		t.Fatal(err)
	}
	t2 := filepath.Join(issuesDir, "043-two.md")
	c2 := "# 043 — Two\n\n**Status**: Open\n"
	if err := os.WriteFile(t2, []byte(c2), 0o644); err != nil {
		t.Fatal(err)
	}
	readmePath := filepath.Join(issuesDir, "README.md")
	origReadme := "# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n| 042 | [042-one.md](042-one.md) | One | Open |\n| 043 | [043-two.md](043-two.md) | Two | Open |\n"
	if err := os.WriteFile(readmePath, []byte(origReadme), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "issues")
	repoRunGit(t, dir, "commit", "-q", "-m", "init")

	var out bytes.Buffer
	_, _, err := runIssuesMv(&out, "42", "43", issuesRunOptions{Dir: dir})
	if err == nil {
		t.Fatalf("expected error moving to already-occupied number 043")
	}

	// Verify no files changed
	got1, _ := os.ReadFile(t1)
	if string(got1) != c1 {
		t.Errorf("042-one.md modified unexpectedly")
	}
	got2, _ := os.ReadFile(t2)
	if string(got2) != c2 {
		t.Errorf("043-two.md modified unexpectedly")
	}
	gotReadme, _ := os.ReadFile(readmePath)
	if string(gotReadme) != origReadme {
		t.Errorf("README modified unexpectedly")
	}
}

func TestRunIssuesMv_NoCommit(t *testing.T) {
	dir, oldTicketPath := issuesFixtureRepo(t, sampleTicket)
	shaBefore := repoRunGitOutput(t, dir, "rev-parse", "HEAD")

	var out bytes.Buffer
	result, _, err := runIssuesMv(&out, "42", "268", issuesRunOptions{Dir: dir, NoCommit: true})
	if err != nil {
		t.Fatalf("runIssuesMv --no-commit: %v", err)
	}
	if result.Committed || result.CommitSHA != "" {
		t.Errorf("expected Committed=false, CommitSHA='', got %+v", result)
	}

	// File rename and README update happened on disk
	if _, err := os.Stat(oldTicketPath); !os.IsNotExist(err) {
		t.Errorf("old ticket file still exists")
	}
	if _, err := os.Stat(filepath.Join(dir, "issues", "268-example-ticket.md")); err != nil {
		t.Errorf("new ticket file does not exist")
	}

	// But no git commit was made
	shaAfter := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	if shaBefore != shaAfter {
		t.Errorf("commit was created under --no-commit: before=%s, after=%s", shaBefore, shaAfter)
	}
}

func TestRunIssuesMv_CheckDryRun(t *testing.T) {
	dir, oldTicketPath := issuesFixtureRepo(t, sampleTicket)
	shaBefore := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	origReadme, _ := os.ReadFile(filepath.Join(dir, "issues", "README.md"))

	var out bytes.Buffer
	result, drift, err := runIssuesMv(&out, "42", "268", issuesRunOptions{Dir: dir, Check: true})
	if err != nil {
		t.Fatalf("runIssuesMv --check: %v", err)
	}
	if !drift {
		t.Errorf("expected drift=true under --check")
	}
	if !result.ReadmeUpdated {
		t.Errorf("expected ReadmeUpdated=true under --check")
	}

	// Disk should be untouched
	if _, err := os.Stat(oldTicketPath); err != nil {
		t.Errorf("old ticket file missing after --check: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "issues", "268-example-ticket.md")); !os.IsNotExist(err) {
		t.Errorf("new ticket file exists on disk after --check")
	}
	readmeAfter, _ := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	if string(readmeAfter) != string(origReadme) {
		t.Errorf("README was modified during --check")
	}
	shaAfter := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	if shaBefore != shaAfter {
		t.Errorf("git commit was created during --check")
	}
}

func TestRunIssuesMv_NoopSameNumber(t *testing.T) {
	dir, oldTicketPath := issuesFixtureRepo(t, sampleTicket)
	shaBefore := repoRunGitOutput(t, dir, "rev-parse", "HEAD")

	var out bytes.Buffer
	result, drift, err := runIssuesMv(&out, "42", "42", issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatalf("runIssuesMv noop: %v", err)
	}
	if drift {
		t.Errorf("expected drift=false on noop")
	}
	if !result.Noop {
		t.Errorf("expected result.Noop=true on same number")
	}
	if _, err := os.Stat(oldTicketPath); err != nil {
		t.Errorf("old ticket file missing: %v", err)
	}
	shaAfter := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
	if shaBefore != shaAfter {
		t.Errorf("git commit was created on noop")
	}
}

func TestFormatIssuesLine_MvFormatting(t *testing.T) {
	mvResult := issuesResult{
		Number:        "268",
		OldNumber:     "042",
		File:          "issues/268-example.md",
		OldStatus:     "Open",
		NewStatus:     "Open",
		ReadmeUpdated: true,
		Committed:     true,
		CommitSHA:     "abc1234",
	}
	line := formatIssuesLine(mvResult, false)
	want := "042 -> 268: issues/268-example.md (README updated, committed abc1234)"
	if line != want {
		t.Errorf("formatIssuesLine() = %q, want %q", line, want)
	}

	checkLine := formatIssuesLine(mvResult, true)
	wantCheck := "042 -> 268: issues/268-example.md (would update README, would commit)"
	if checkLine != wantCheck {
		t.Errorf("formatIssuesLine(check=true) = %q, want %q", checkLine, wantCheck)
	}
}

func TestIssuesCmd_MvArgsValidation(t *testing.T) {
	cmd := newIssuesCmd()
	cases := []struct {
		args    []string
		wantErr bool
	}{
		{[]string{}, true},
		{[]string{"mv"}, true},
		{[]string{"mv", "42", "268", "extra"}, true},
		{[]string{"mv", "42"}, false},
		{[]string{"mv", "42", "268"}, false},
	}
	for _, c := range cases {
		err := cmd.Args(cmd, c.args)
		if c.wantErr && err == nil {
			t.Errorf("cmd.Args(%v): expected error, got nil", c.args)
		}
		if !c.wantErr && err != nil {
			t.Errorf("cmd.Args(%v): unexpected error: %v", c.args, err)
		}
	}
}


