package release

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// All push URLs are local bare repositories; forge URLs are discovery metadata only.
func TestReleasePushAllRemotes(t *testing.T) {
	for _, scenario := range []string{"two-remotes", "single-remote", "diverged-branch", "conflicting-tag"} {
		t.Run(scenario, func(t *testing.T) {
			dir := setupGitRepoWithVersion(t, "1.0.0", false)
			runGitCmd(t, dir, "branch", "-M", "main")
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "fj"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			count := 2
			if scenario == "single-remote" {
				count = 1
			}
			var remotes []string
			for i, name := range []string{"primary", "secondary"}[:count] {
				bare := t.TempDir()
				runGitCmd(t, bare, "init", "--bare")
				runGitCmd(t, dir, "remote", "add", name, "https://codeberg.org/test/repo.git")
				runGitCmd(t, dir, "remote", "set-url", "--push", name, bare)
				runGitCmd(t, dir, "push", name, "main")
				remotes = append(remotes, bare)
				if i == 1 && scenario == "diverged-branch" {
					other := t.TempDir()
					runGitCmd(t, other, "clone", "--branch", "main", bare, ".")
					runGitCmd(t, other, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "remote-only")
					runGitCmd(t, other, "push", "origin", "main")
				}
				if i == 1 && scenario == "conflicting-tag" {
					runGitCmd(t, bare, "tag", "v1.0.1", "main")
				}
			}
			var before []string
			for _, bare := range remotes {
				before = append(before, pushTestRefs(t, bare))
			}
			var out bytes.Buffer
			err := Run(Options{Dir: dir, Bump: "patch", Force: true, SkipBuild: true, SkipSign: true, SkipPublish: true, Out: &out})
			if scenario == "diverged-branch" || scenario == "conflicting-tag" {
				if err == nil || !strings.Contains(err.Error(), "secondary") || !strings.Contains(err.Error(), "no remotes pushed") {
					t.Fatalf("want refusal identifying secondary before any push, got %v", err)
				}
				for i, bare := range remotes {
					if got := pushTestRefs(t, bare); got != before[i] {
						t.Errorf("remote %d changed after refusal: before %s, after %s", i, before[i], got)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("release: %v\n%s", err, out.String())
			}
			for _, ref := range []string{"refs/heads/main", "refs/tags/v1.0.1"} {
				want := pushTestGitOutput(t, dir, "rev-parse", ref)
				for i, bare := range remotes {
					if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
						t.Errorf("remote %d %s = %s, want %s", i, ref, got, want)
					}
				}
			}
		})
	}
}

func pushTestRefs(t *testing.T, dir string) string {
	t.Helper()
	return pushTestGitOutput(t, dir, "show-ref")
}

func pushTestGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
