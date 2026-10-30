package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIssuesNewSkipsNumbersOnFetchedRemoteBranch checks that new and --number
// treat a ticket on a fetched remote branch as taken.
func TestIssuesNewSkipsNumbersOnFetchedRemoteBranch(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)

	upstream := repoInit(t)
	elsewhere := filepath.Join(upstream, "issues", "1009-reserved-elsewhere.md")
	if err := os.MkdirAll(filepath.Dir(elsewhere), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(elsewhere, []byte("# 1009 — Reserved elsewhere\n\n**Status**: Draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, upstream, "add", "issues")
	repoRunGit(t, upstream, "commit", "-q", "-m", "reserve 1009 elsewhere")
	repoRunGit(t, dir, "remote", "add", "origin", upstream)

	repoRunGit(t, dir, "fetch", "-q", "origin")

	var out bytes.Buffer
	cmd := newIssuesCmd()
	cmd.SetArgs([]string{"new", "--number", "1009", "-d", dir, "Taken elsewhere"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "already exists: origin/main:issues/1009-reserved-elsewhere.md") {
		t.Fatalf("new --number 1009 after fetch: got %v, want refusal naming origin/main", err)
	}

	out.Reset()
	cmd = newIssuesCmd()
	cmd.SetArgs([]string{"new", "-d", dir, "Next free"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("new: %v", err)
	}
	if !strings.HasPrefix(out.String(), "1010\t") {
		t.Fatalf("new output %q, want number 1010", out.String())
	}
}
