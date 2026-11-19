package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInitAgitPullFollowsOriginMain runs the bare-remote pull path end to end:
// after init, a topic branch cut from origin/main tracks origin/main, and a plain
// `git pull` on it merges a commit that another clone pushed to the bare origin's main.
func TestInitAgitPullFollowsOriginMain(t *testing.T) {
	skipShort(t, "runs real git pulls against a bare remote")

	remote := filepath.Join(t.TempDir(), "harnez.git")
	initAgitMustGit(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", remote)
	dir := initAgitRepo(t, remote)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initAgitMustGit(t, dir, "add", "README.md")
	initAgitMustGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "base")
	initAgitMustGit(t, dir, "push", "-q", "origin", "main")
	initAgitMustGit(t, dir, "config", "--add", "remote.origin.push", "HEAD:refs/for/main/topic")
	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}

	initAgitMustGit(t, dir, "fetch", "-q", "origin")
	initAgitMustGit(t, dir, "switch", "-q", "-c", "topic", "origin/main")
	if upstream, err := initAgitGit(dir, "rev-parse", "--abbrev-ref", "topic@{upstream}"); err != nil || upstream != "origin/main" {
		t.Fatalf("topic upstream = %q (err %v), want origin/main", upstream, err)
	}

	clone := filepath.Join(t.TempDir(), "clone")
	initAgitMustGit(t, filepath.Dir(clone), "clone", "-q", remote, clone)
	if err := os.WriteFile(filepath.Join(clone, "NEW.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initAgitMustGit(t, clone, "add", "NEW.md")
	initAgitMustGit(t, clone, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "upstream")
	initAgitMustGit(t, clone, "push", "-q", "origin", "main")
	want, err := initAgitGit(clone, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse clone HEAD: %v\n%s", err, want)
	}

	if out, err := initAgitGit(dir, "pull"); err != nil {
		t.Fatalf("plain git pull on topic failed after init: %v\n%s", err, out)
	}
	if got, _ := initAgitGit(dir, "rev-parse", "HEAD"); got != want {
		t.Fatalf("after pull HEAD = %s, want the pushed origin/main commit %s", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "NEW.md")); err != nil {
		t.Fatalf("pull did not bring in NEW.md: %v", err)
	}
}
