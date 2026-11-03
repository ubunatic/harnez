package forgetracker

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/prwait"
)

type fakeSource struct {
	issues    []forge.RemoteIssue
	truncated bool
}

func (f *fakeSource) Issue(_ forge.Forge, _, _ string, n int) (forge.RemoteIssue, error) {
	for _, is := range f.issues {
		if is.Number == n {
			return is, nil
		}
	}
	return forge.RemoteIssue{}, &forge.APIError{Status: 404, Detail: "not found"}
}

func (f *fakeSource) Issues(_ forge.Forge, _, _ string, _ forge.ScanOptions) ([]forge.RemoteIssue, bool, error) {
	return f.issues, f.truncated, nil
}

var target = Target{Forge: forge.Forge{Host: "codeberg.org"}, Owner: "o", Repo: "r"}

func sample() *fakeSource {
	return &fakeSource{issues: []forge.RemoteIssue{
		{Number: 3, Title: "Crash on start", Body: "segfault in loader", State: "open", HTMLURL: "u3"},
		{Number: 2, Title: "Loader is slow", Body: "takes ages", State: "closed", HTMLURL: "u2"},
		{Number: 1, Title: "Docs typo", Body: "", State: "open", HTMLURL: "u1"},
	}}
}

func TestShowPrintsTicket(t *testing.T) {
	var out bytes.Buffer
	if err := Show(&out, sample(), target, "#3", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# 3 — Crash on start", "**Status**: open", "**Source**: u3", "segfault in loader"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out.String())
		}
	}
}

func TestShowRejectsPullRequestAndBadNumber(t *testing.T) {
	src := sample()
	src.issues[0].PullRequest = &struct{}{}
	if err := Show(&bytes.Buffer{}, src, target, "3", false); err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Fatalf("want pull request error, got %v", err)
	}
	if err := Show(&bytes.Buffer{}, src, target, "abc", false); err == nil {
		t.Fatal("want invalid number error")
	}
}

func TestShowWrapsAPIError(t *testing.T) {
	err := Show(&bytes.Buffer{}, sample(), target, "99", false)
	var apiErr *forge.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 404 {
		t.Fatalf("want wrapped 404 APIError, got %v", err)
	}
}

func TestFindMatchesAllTermsCaseInsensitive(t *testing.T) {
	src := sample()
	var out bytes.Buffer
	if err := Find(&out, src, target, forge.ScanOptions{}, []string{"LOADER"}, 10, false, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "# 2 matches") || !strings.Contains(got, "3\topen\tCrash on start\tu3") || !strings.Contains(got, "2\tclosed") || strings.Contains(got, "Docs typo") {
		t.Fatalf("unexpected rows:\n%s", got)
	}
	out.Reset()
	if err := Find(&out, src, target, forge.ScanOptions{}, []string{"loader slow"}, 10, false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "# 1 matches") || !strings.Contains(out.String(), "Loader is slow") {
		t.Fatalf("want one AND match, got %q", out.String())
	}
}

func TestFindRejectsFiltersAndNotesTruncation(t *testing.T) {
	if err := Find(&bytes.Buffer{}, sample(), target, forge.ScanOptions{}, []string{"status:open"}, 10, false, false); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("want unsupported filter error, got %v", err)
	}
	src := sample()
	src.truncated = true
	var out bytes.Buffer
	if err := Find(&out, src, target, forge.ScanOptions{PageSize: 50, MaxPages: 4}, nil, 10, false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "searched only the newest 200") {
		t.Fatalf("want truncation note, got %s", out.String())
	}
}

func TestFindLimitAllAndJSON(t *testing.T) {
	var out bytes.Buffer
	if err := Find(&out, sample(), target, forge.ScanOptions{}, nil, 1, false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "# 3 matches on codeberg.org/o/r, showing 1") {
		t.Fatalf("limit not applied: %s", out.String())
	}
	out.Reset()
	if err := Find(&out, sample(), target, forge.ScanOptions{}, nil, 1, true, true); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 3 {
		t.Fatalf("want 3 JSON rows with --all, got %d (%v): %s", len(rows), err, out.String())
	}
}

func gitRepo(t *testing.T, origin string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", origin}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func TestResolve(t *testing.T) {
	spec := prwait.Spec{DefaultRemote: "origin", Forges: []prwait.Forge{{Host: "codeberg.org", Kind: "forgejo"}}}

	tg, err := Resolve(gitRepo(t, "git@codeberg.org:harnez/taia.git"), spec)
	if err != nil || tg.String() != "codeberg.org/harnez/taia" {
		t.Fatalf("want codeberg target, got %v %v", tg, err)
	}

	withIssues := gitRepo(t, "git@codeberg.org:harnez/taia.git")
	if err := os.Mkdir(filepath.Join(withIssues, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(withIssues, spec); !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("local issues/ must win, got %v", err)
	}
	if _, err := Resolve(gitRepo(t, "git@example.com:o/r.git"), spec); !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("unknown forge must not apply, got %v", err)
	}
	if _, err := Resolve(t.TempDir(), spec); !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("no git repo must not apply, got %v", err)
	}
}
