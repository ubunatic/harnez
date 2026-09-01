package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func repoInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoRunGit(t, dir, "init", "-q", "-b", "main")
	repoRunGit(t, dir, "config", "user.email", "test@example.com")
	repoRunGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "README.md")
	repoRunGit(t, dir, "commit", "-q", "-m", "initial commit")
	return dir
}

func TestRunRepoStatus_QuietOnCleanRepo(t *testing.T) {
	dir := repoInit(t)

	var out bytes.Buffer
	if err := runRepoStatus(&out, dir); err != nil {
		t.Fatalf("runRepoStatus: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if strings.Count(got, "\n") != 0 {
		t.Fatalf("expected one line for clean repo, got:\n%s", got)
	}
	if !strings.HasPrefix(got, "clean, main") {
		t.Errorf("expected quiet line to start with 'clean, main', got: %q", got)
	}
}

func TestRunRepoStatus_VerboseOnDirtyRepo(t *testing.T) {
	dir := repoInit(t)

	// unstaged change
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// untracked file
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// staged file
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "staged.txt")

	var out bytes.Buffer
	if err := runRepoStatus(&out, dir); err != nil {
		t.Fatalf("runRepoStatus: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"staged=1 unstaged=1 untracked=1 conflicts=0",
		"staged: staged.txt",
		"unstaged: README.md",
		"untracked: new.txt",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose output missing %q, got:\n%s", want, got)
		}
	}
	// The verbose path must not just dump the raw `git status` text.
	if strings.Contains(got, "Changes not staged for commit") {
		t.Errorf("expected structured summary, not raw git status text, got:\n%s", got)
	}
}

func TestRunRepoStatus_AheadOfOriginOnlyIsQuiet(t *testing.T) {
	remote := t.TempDir()
	repoRunGit(t, remote, "init", "-q", "--bare", "-b", "main")

	dir := repoInit(t)
	repoRunGit(t, dir, "remote", "add", "origin", remote)
	repoRunGit(t, dir, "push", "-q", "-u", "origin", "main")

	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("more\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "file2.txt")
	repoRunGit(t, dir, "commit", "-q", "-m", "second commit")

	var out bytes.Buffer
	if err := runRepoStatus(&out, dir); err != nil {
		t.Fatalf("runRepoStatus: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if strings.Count(got, "\n") != 0 {
		t.Fatalf("expected one line when only ahead of origin, got:\n%s", got)
	}
	if !strings.Contains(got, "1 ahead of origin/main") {
		t.Errorf("expected 'ahead of origin/main' in quiet line, got: %q", got)
	}
}
