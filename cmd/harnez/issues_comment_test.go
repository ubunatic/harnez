package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetracker"
	"ubunatic.com/harnez/internal/prwait"
)

func TestIssuesCommentOffline(t *testing.T) {
	state, pull, status := "open", false, http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/issues/7" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected request: %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.WriteHeader(status)
		if status == http.StatusOK {
			pr := "null"
			if pull {
				pr = "{}"
			}
			fmt.Fprintf(w, `{"title":"A ticket","state":%q,"pull_request":%s}`, state, pr)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "--object-format=sha256"}, {"remote", "add", "origin", "git@codeberg.org:o/r.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	body := filepath.Join(t.TempDir(), "body file")
	if err := os.WriteFile(body, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "args")
	if err := os.WriteFile(filepath.Join(bin, "fj"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FJ_TEST_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FJ_TEST_LOG", log)
	old := forgeTrackerSource
	forgeTrackerSource = func(sp prwait.Spec) forgetracker.Source {
		src := forgetracker.NewSource(sp)
		return commentSource{Source: src, api: srv.URL}
	}
	t.Cleanup(func() { forgeTrackerSource = old })
	run := func(flags ...string) (string, error) {
		args := []string{"issues", "comment", "-d", dir, "7", "-F", body}
		return runHarnez(t, append(args, flags...)...)
	}
	out, err := run("--dry-run")
	if err != nil || !strings.Contains(out, "fj --host codeberg.org issue comment o/r#7 --body-file") {
		t.Fatalf("dry run: %s %v", out, err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("dry run executed fj")
	}
	pull = true
	if _, err := run("--closed"); err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Fatalf("PR: %v", err)
	}
	pull, state = false, "closed"
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "--closed") {
		t.Fatalf("closed: %v", err)
	}
	out, err = run("--closed")
	if err != nil || !strings.Contains(out, "commenting on codeberg.org o/r#7: A ticket") {
		t.Fatalf("comment: %s %v", out, err)
	}
	got, err := os.ReadFile(log)
	if err != nil || string(got) != strings.Join([]string{"--host", "codeberg.org", "issue", "comment", "o/r#7", "--body-file", body, ""}, "\n") {
		t.Fatalf("argv: %q %v", got, err)
	}
	status = http.StatusUnauthorized
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "API requires authentication") || !strings.Contains(err.Error(), "fj -H codeberg.org auth login") {
		t.Fatalf("API auth: %v", err)
	}
	status, state = http.StatusOK, "open"
	if err := os.WriteFile(filepath.Join(bin, "fj"), []byte("#!/bin/sh\necho 'token is required' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "fj -H codeberg.org auth login") {
		t.Fatalf("fj auth: %v", err)
	}
}

type commentSource struct {
	forgetracker.Source
	api string
}

func (s commentSource) Issue(fg forge.Forge, owner, repo string, n int) (forge.RemoteIssue, error) {
	fg.APIBase = s.api
	l := forge.PullLister{Client: http.DefaultClient}
	return l.Issue(fg, owner, repo, n)
}
