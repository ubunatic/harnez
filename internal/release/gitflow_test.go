package release

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const releaseProtectedYAML = `version: 1
issues:
  tracker: none
  merge_driver: false
docs:
  studies: false
  copies: []
rules:
  quota: false
make:
  container_test: false
git:
  mode: team
  protected_main: true
`

// gitflowReleaseRepo builds a v1.0.0 repo on main with a bare origin that has
// main pushed. repoYAML, when set, is committed as .harnez/repo.yaml so the
// tree stays clean for the release.
func gitflowReleaseRepo(t *testing.T, repoYAML string) (dir, bare string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	dir = setupGitRepoWithVersion(t, "1.0.0", false)
	runGitCmd(t, dir, "branch", "-M", "main")
	if repoYAML != "" {
		if err := os.MkdirAll(filepath.Join(dir, ".harnez"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".harnez", "repo.yaml"), []byte(repoYAML), 0o644); err != nil {
			t.Fatal(err)
		}
		runGitCmd(t, dir, "add", ".harnez/repo.yaml")
		runGitCmd(t, dir, "commit", "-m", "chore: repo config")
	}
	// A forge-looking fetch URL satisfies forge detection; pushes go to the bare repo.
	stubs := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubs, "fj"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubs+string(os.PathListSeparator)+os.Getenv("PATH"))
	bare = t.TempDir()
	runGitCmd(t, bare, "init", "--bare")
	runGitCmd(t, dir, "remote", "add", "origin", "https://codeberg.org/test/repo.git")
	runGitCmd(t, dir, "remote", "set-url", "--push", "origin", bare)
	runGitCmd(t, dir, "push", "origin", "main")
	return dir, bare
}

func gitflowReleaseOptions(dir string, out *bytes.Buffer) Options {
	return Options{Dir: dir, Bump: "patch", Force: true, SkipBuild: true, SkipSign: true, SkipPublish: true, Out: out}
}

func TestReleaseRefusesOnProtectedMainBeforeBump(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "release"
		if dryRun {
			name = "dry-run"
		}
		t.Run(name, func(t *testing.T) {
			dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
			spec, err := os.ReadFile(filepath.Join(dir, "version.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			head := pushTestGitOutput(t, dir, "rev-parse", "HEAD")
			tags := pushTestGitOutput(t, dir, "tag")
			remoteRefs := pushTestRefs(t, bare)
			var out bytes.Buffer
			opt := gitflowReleaseOptions(dir, &out)
			opt.DryRun = dryRun
			err = Run(opt)
			if err == nil || !strings.Contains(err.Error(), "git.protected_main") || !strings.Contains(err.Error(), "release") {
				t.Fatalf("want a protected_main refusal naming the release, got %v\n%s", err, out.String())
			}
			if got, _ := os.ReadFile(filepath.Join(dir, "version.yaml")); !bytes.Equal(got, spec) {
				t.Errorf("version.yaml changed by a refused release:\n%s", got)
			}
			if got := pushTestGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
				t.Errorf("refused release committed: %s -> %s", head, got)
			}
			if got := pushTestGitOutput(t, dir, "tag"); got != tags {
				t.Errorf("refused release tagged: %q -> %q", tags, got)
			}
			if got := pushTestRefs(t, bare); got != remoteRefs {
				t.Errorf("refused release pushed:\nbefore %s\nafter  %s", remoteRefs, got)
			}
			if strings.Contains(out.String(), "Bumping version") {
				t.Errorf("version bump started before the refusal:\n%s", out.String())
			}
		})
	}
}

func TestReleaseOnBranchPushesBranchAndTag(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	mainBefore := pushTestGitOutput(t, bare, "rev-parse", "refs/heads/main")
	runGitCmd(t, dir, "switch", "-c", "release/v1.0.1")
	var out bytes.Buffer
	if err := Run(gitflowReleaseOptions(dir, &out)); err != nil {
		t.Fatalf("release on a topic branch: %v\n%s", err, out.String())
	}
	for _, ref := range []string{"refs/heads/release/v1.0.1", "refs/tags/v1.0.1"} {
		want := pushTestGitOutput(t, dir, "rev-parse", ref)
		if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
			t.Errorf("remote %s = %s, want %s", ref, got, want)
		}
	}
	if got := pushTestGitOutput(t, bare, "rev-parse", "refs/heads/main"); got != mainBefore {
		t.Errorf("remote main moved: %s -> %s", mainBefore, got)
	}
}

func TestReleaseContinueRefusesPushOfProtectedMain(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	// A bumped, tagged release sitting on main: --continue must not push main.
	if err := os.WriteFile(filepath.Join(dir, "version.yaml"), []byte("version: 1.0.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "commit", "-am", "chore(release): bump version to v1.0.1")
	runGitCmd(t, dir, "tag", "-a", "v1.0.1", "-m", "v1.0.1")
	remoteRefs := pushTestRefs(t, bare)
	var out bytes.Buffer
	opt := gitflowReleaseOptions(dir, &out)
	opt.Continue = true
	err := Run(opt)
	if err == nil || !strings.Contains(err.Error(), "git.protected_main") {
		t.Fatalf("want a protected_main refusal at push time, got %v\n%s", err, out.String())
	}
	if got := pushTestRefs(t, bare); got != remoteRefs {
		t.Errorf("refused push changed the remote:\nbefore %s\nafter  %s", remoteRefs, got)
	}
}

func TestReleaseProtectedPushRefusalPrecedesRemoteLoop(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, releaseProtectedYAML)
	second := t.TempDir()
	runGitCmd(t, second, "init", "--bare")
	runGitCmd(t, dir, "remote", "add", "second", second)
	runGitCmd(t, dir, "push", "second", "main")
	refs := pushTestRefs(t, bare) + "|" + pushTestRefs(t, second)
	err := pushRelease(Options{Dir: dir, Out: &bytes.Buffer{}}, "v9.9.9", nil)
	if err == nil || !strings.Contains(err.Error(), "git.protected_main") {
		t.Fatalf("want a protected_main refusal, got %v", err)
	}
	if got := pushTestRefs(t, bare) + "|" + pushTestRefs(t, second); got != refs {
		t.Errorf("a remote changed despite the refusal:\nbefore %s\nafter  %s", refs, got)
	}
}

// wantSoloReleaseOutput was recorded from a solo release on main before the
// gitflow guard existed; the project name is the temp dir's base name.
const wantSoloReleaseOutput = `==> Preparing release for PROJECT (dry-run=false, continue=false)
  [version]   Bumping version: 1.0.0 -> 1.0.1 (tag: v1.0.1)
  [git]       Committed version bump (chore(release): bump version to v1.0.1)
  [git]       Created annotated tag: v1.0.1
  [forge]     Verified/Enabled 'has_releases' on test/repo
  [git]       Pushing branch main and tag v1.0.1 to origin...
==> Release v1.0.1 completed successfully!
`

func TestReleaseSoloUnchanged(t *testing.T) {
	dir, bare := gitflowReleaseRepo(t, "")
	var out bytes.Buffer
	if err := Run(gitflowReleaseOptions(dir, &out)); err != nil {
		t.Fatalf("solo release on main: %v\n%s", err, out.String())
	}
	got := strings.ReplaceAll(out.String(), filepath.Base(dir), "PROJECT")
	if got != wantSoloReleaseOutput {
		t.Errorf("solo release output changed:\ngot:\n%s\nwant:\n%s", got, wantSoloReleaseOutput)
	}
	for _, ref := range []string{"refs/heads/main", "refs/tags/v1.0.1"} {
		want := pushTestGitOutput(t, dir, "rev-parse", ref)
		if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
			t.Errorf("remote %s = %s, want %s", ref, got, want)
		}
	}
}
