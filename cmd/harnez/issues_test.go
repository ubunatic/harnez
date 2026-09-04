package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		{"start", "x", "", true},
		{"block", "waiting on upstream", "Blocked — waiting on upstream", false},
		{"block", "", "", true},
		{"close", "", "Closed", false},
		{"close", "resolved", "Closed — resolved", false},
		{"draft", "", "Draft", false},
		{"draft", "x", "", true},
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
