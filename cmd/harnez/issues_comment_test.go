package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	wantDry := "dry run: fj --host codeberg.org issue comment o/r#7 --body-file " + strconv.Quote(body) + "\n"
	if err != nil || !strings.HasPrefix(out, wantDry) {
		t.Fatalf("dry run: %s %v", out, err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("dry run executed fj")
	}
	pull = true
	if _, err := run("--closed"); err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Fatalf("PR: %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("PR refusal executed fj")
	}
	pull, state = false, "closed"
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "--closed") {
		t.Fatalf("closed: %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("closed refusal executed fj")
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
	for _, message := range []string{"token is required", "401 Unauthorized"} {
		if err := os.WriteFile(filepath.Join(bin, "fj"), []byte("#!/bin/sh\necho '"+message+"' >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := run(); err == nil || !strings.Contains(err.Error(), "fj -H codeberg.org auth login") {
			t.Fatalf("fj auth %q: %v", message, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := run("--dry-run"); err == nil || !strings.Contains(err.Error(), "--forge") {
		t.Fatalf("local tracker refusal: %v", err)
	}
	out, err = run("--forge", "--dry-run")
	if err != nil || out != wantDry {
		t.Fatalf("local tracker --forge: %q %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(bin, "fj"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FJ_TEST_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	out, err = run("--forge")
	if err != nil || !strings.Contains(out, "commenting on codeberg.org o/r#7: A ticket") {
		t.Fatalf("local tracker comment: %q %v", out, err)
	}
	got, err = os.ReadFile(log)
	if err != nil || string(got) != strings.Join([]string{"--host", "codeberg.org", "issue", "comment", "o/r#7", "--body-file", body, ""}, "\n") {
		t.Fatalf("local tracker argv: %q %v", got, err)
	}
	if _, err := runHarnez(t, "issues", "show", "-d", dir, "7", "--forge"); err == nil || !strings.Contains(err.Error(), "--forge requires comment") {
		t.Fatalf("forge flag on show: %v", err)
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
