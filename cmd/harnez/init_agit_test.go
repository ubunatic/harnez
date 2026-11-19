package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initAgitGit runs git in dir and returns its combined output and error.
func initAgitGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func initAgitMustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := initAgitGit(dir, args...); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// initAgitLocalConfig returns the repo-local value of key, or "" when unset.
func initAgitLocalConfig(t *testing.T, dir, key string) string {
	t.Helper()
	out, err := initAgitGit(dir, "config", "--local", "--get", key)
	if err != nil {
		return ""
	}
	return out
}

func initAgitRepo(t *testing.T, origin string) string {
	t.Helper()
	dir := t.TempDir()
	initAgitMustGit(t, dir, "init", "-q", "-b", "main")
	if origin != "" {
		initAgitMustGit(t, dir, "remote", "add", "origin", origin)
	}
	return dir
}

func TestInitAgitTeamSetsPullKeysAndIsIdempotent(t *testing.T) {
	dir := initAgitRepo(t, "/srv/git/harnez.git")
	if err := runInitCmd(t, dir, "--repo-mode", "team"); err != nil {
		t.Fatal(err)
	}
	if got := initAgitLocalConfig(t, dir, "branch.autoSetupMerge"); got != "always" {
		t.Errorf("branch.autoSetupMerge = %q, want always", got)
	}
	if got := initAgitLocalConfig(t, dir, "checkout.defaultRemote"); got != "origin" {
		t.Errorf("checkout.defaultRemote = %q, want origin", got)
	}
	if err := runInitCmd(t, dir, "--repo-mode", "team"); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if out, _ := initAgitGit(dir, "config", "--local", "--get-all", "branch.autoSetupMerge"); strings.Count(out, "\n") != 0 {
		t.Errorf("second init duplicated branch.autoSetupMerge: %q", out)
	}
}

func TestInitAgitSoloOnCodebergSetsPullKeys(t *testing.T) {
	dir := initAgitRepo(t, "https://codeberg.org/ubunatic/harnez.git")
	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}
	if got := initAgitLocalConfig(t, dir, "branch.autoSetupMerge"); got != "always" {
		t.Errorf("solo on Codeberg: branch.autoSetupMerge = %q, want always", got)
	}
	if got := initAgitLocalConfig(t, dir, "checkout.defaultRemote"); got != "origin" {
		t.Errorf("solo on Codeberg: checkout.defaultRemote = %q, want origin", got)
	}
}

func TestInitAgitRefsForPushSignalSetsPullKeys(t *testing.T) {
	dir := initAgitRepo(t, "/srv/git/harnez.git")
	initAgitMustGit(t, dir, "config", "--add", "remote.origin.push", "HEAD:refs/for/main/topic")
	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}
	if got := initAgitLocalConfig(t, dir, "branch.autoSetupMerge"); got != "always" {
		t.Errorf("refs/for push: branch.autoSetupMerge = %q, want always", got)
	}
}

func TestInitAgitNonAgitRepoUntouched(t *testing.T) {
	dir := initAgitRepo(t, "https://github.com/ubunatic/harnez.git")
	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"branch.autoSetupMerge", "checkout.defaultRemote"} {
		if got := initAgitLocalConfig(t, dir, key); got != "" {
			t.Errorf("solo on GitHub: %s = %q, want unset", key, got)
		}
	}
}

func TestInitAgitNonGitDirIsSafe(t *testing.T) {
	dir := t.TempDir()
	if err := runInitCmd(t, dir); err != nil {
		t.Fatalf("init in a non-git directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("init created a .git entry in a non-git directory: %v", err)
	}
}

// TestInitAgitPullWorksOnNewBranch is the real-remote check: a branch cut from a
// local main, in a repo whose origin is a bare repository with no upstream set,
// must accept a plain `git pull` once init has configured the AGit keys.
func TestInitAgitPullWorksOnNewBranch(t *testing.T) {
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

	initAgitMustGit(t, dir, "switch", "-q", "-c", "before-init")
	if out, err := initAgitGit(dir, "pull"); err == nil {
		t.Fatalf("pull without tracking unexpectedly succeeded:\n%s", out)
	}
	initAgitMustGit(t, dir, "switch", "-q", "main")
	initAgitMustGit(t, dir, "branch", "-q", "-D", "before-init")

	if err := runInitCmd(t, dir); err != nil {
		t.Fatal(err)
	}
	initAgitMustGit(t, dir, "switch", "-q", "-c", "topic")
	if out, err := initAgitGit(dir, "pull"); err != nil {
		t.Fatalf("plain git pull on new branch failed after init: %v\n%s", err, out)
	}
	upstream, err := initAgitGit(dir, "rev-parse", "--abbrev-ref", "topic@{upstream}")
	if err != nil || upstream != "main" {
		t.Fatalf("topic upstream = %q (err %v), want main", upstream, err)
	}
}
