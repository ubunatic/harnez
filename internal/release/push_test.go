package release

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// All push URLs are local bare repositories; forge URLs are discovery metadata only.
func TestReleasePushAllRemotes(t *testing.T) {
	for _, scenario := range []string{"two-remotes", "single-remote", "diverged-branch", "conflicting-tag", "race-success", "race-fail"} {
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
				if i == 1 && (scenario == "race-success" || scenario == "race-fail") {
					// We match $5 to the remote NAME which is 'name'
					wrapper := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = push ] && [ \"$2\" = \"--atomic\" ] && [ \"$3\" = \"--no-force\" ] && [ \"$4\" = \"--\" ] && [ \"$5\" = \"%s\" ]; then\n", name)
					if scenario == "race-success" {
						wrapper += fmt.Sprintf("\t/usr/bin/git -C %q push %q main:refs/heads/main v1.0.1:refs/tags/v1.0.1\n", dir, bare)
					} else {
						wrapper += fmt.Sprintf("\tother=$(mktemp -d)\n\t/usr/bin/git clone --branch main %q \"$other\"\n\t/usr/bin/git -C \"$other\" -c user.name=test -c user.email=test@example.com commit --allow-empty -m \"race\"\n\t/usr/bin/git -C \"$other\" push %q main:refs/heads/main\n", bare, bare)
					}
					wrapper += "\techo 'remote rejected' >&2\n\texit 1\nfi\nexec /usr/bin/git \"$@\"\n"
					if err := os.WriteFile(bin+"/git", []byte(wrapper), 0755); err != nil {
						t.Fatal(err)
					}
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
			if scenario == "race-fail" {
				if err == nil || !strings.Contains(err.Error(), "earlier remotes may have been updated") {
					t.Fatalf("expected earlier remotes updated error, got: %v", err)
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

// A remote with several push URLs (issue 1074: `both` pushes to Codeberg and
// GitHub) is skipped when the other remotes already push to all of them, so its
// pre-push hook never runs during a release. The hook refuses `both`, so any
// push to it fails the release.
func TestReleasePushSkipsRemoteCoveredByOthers(t *testing.T) {
	for _, scenario := range []string{"fan-out-refused", "identical-urls"} {
		t.Run(scenario, func(t *testing.T) {
			dir := setupGitRepoWithVersion(t, "1.0.0", false)
			runGitCmd(t, dir, "branch", "-M", "main")
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "fj"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			codeberg := t.TempDir()
			runGitCmd(t, codeberg, "init", "--bare")
			github := t.TempDir()
			runGitCmd(t, github, "init", "--bare")
			runGitCmd(t, dir, "remote", "add", "origin", "https://codeberg.org/test/repo.git")
			runGitCmd(t, dir, "remote", "set-url", "--push", "origin", codeberg)
			runGitCmd(t, dir, "push", "origin", "main")
			wantPushes := 1
			if scenario == "fan-out-refused" {
				wantPushes = 2
				runGitCmd(t, dir, "remote", "add", "github", "https://github.com/test/repo.git")
				runGitCmd(t, dir, "remote", "set-url", "--push", "github", github)
				runGitCmd(t, dir, "push", "github", "main")
				runGitCmd(t, dir, "remote", "add", "both", "https://codeberg.org/test/repo.git")
				runGitCmd(t, dir, "remote", "set-url", "--push", "both", codeberg)
				runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "both", github)
				hook := "#!/bin/sh\nif test \"$1\" = both\nthen\n\techo 'pre-push: refusing both' >&2\n\texit 1\nfi\nexit 0\n"
				if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-push"), []byte(hook), 0755); err != nil {
					t.Fatal(err)
				}
				// Precondition: the hook really refuses a push to both.
				probe := exec.Command("git", "-C", dir, "push", "both", "main:refs/heads/probe")
				if out, err := probe.CombinedOutput(); err == nil {
					t.Fatalf("hook did not refuse push to both: %s", out)
				}
			} else {
				runGitCmd(t, dir, "remote", "add", "mirror", "https://codeberg.org/test/repo.git")
				runGitCmd(t, dir, "remote", "set-url", "--push", "mirror", codeberg)
			}
			var out bytes.Buffer
			err := Run(Options{Dir: dir, Bump: "patch", Force: true, SkipBuild: true, SkipSign: true, SkipPublish: true, Out: &out})
			if err != nil {
				t.Fatalf("release: %v\n%s", err, out.String())
			}
			skipped := "both"
			if scenario == "identical-urls" {
				skipped = "mirror"
			}
			if !strings.Contains(out.String(), "[skip]") || !strings.Contains(out.String(), skipped) {
				t.Errorf("want a skip line naming %s, got:\n%s", skipped, out.String())
			}
			if got := strings.Count(out.String(), "Pushing branch"); got != wantPushes {
				t.Errorf("pushed to %d remotes, want %d:\n%s", got, wantPushes, out.String())
			}
			targets := map[string]string{"codeberg": codeberg}
			if wantPushes == 2 {
				targets["github"] = github
			}
			for _, ref := range []string{"refs/heads/main", "refs/tags/v1.0.1"} {
				want := pushTestGitOutput(t, dir, "rev-parse", ref)
				for name, bare := range targets {
					if got := pushTestGitOutput(t, bare, "rev-parse", ref); got != want {
						t.Errorf("%s %s = %s, want %s", name, ref, got, want)
					}
				}
			}
		})
	}
}

func TestReleasePushDryRunListsOnlyKeptRemotes(t *testing.T) {
	dir := setupGitRepoWithVersion(t, "1.0.0", false)
	codeberg, github := t.TempDir(), t.TempDir()
	runGitCmd(t, dir, "remote", "add", "origin", "https://codeberg.org/test/repo.git")
	runGitCmd(t, dir, "remote", "set-url", "--push", "origin", codeberg)
	runGitCmd(t, dir, "remote", "add", "github", "https://github.com/test/repo.git")
	runGitCmd(t, dir, "remote", "set-url", "--push", "github", github)
	runGitCmd(t, dir, "remote", "add", "both", "https://codeberg.org/test/repo.git")
	runGitCmd(t, dir, "remote", "set-url", "--push", "both", codeberg)
	runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "both", github)
	var out bytes.Buffer
	if err := pushRelease(Options{Dir: dir, DryRun: true, Out: &out}, "v1.0.1", nil); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if strings.Contains(out.String(), "Would push branch and tag v1.0.1 to both") {
		t.Errorf("dry run lists the fan-out remote:\n%s", out.String())
	}
	for _, want := range []string{"Would push branch and tag v1.0.1 to origin", "Would push branch and tag v1.0.1 to github"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry run lacks %q:\n%s", want, out.String())
		}
	}
}

func TestSelectPushRemotesKeepsOneOfIdenticalRemotes(t *testing.T) {
	urls := map[string][]string{
		"origin": {"/a"},
		"mirror": {"/a"},
		"both":   {"/a", "/b"},
		"github": {"/b"},
	}
	kept, skipped, err := selectPushRemotes([]string{"origin", "mirror", "both", "github"}, func(r string) ([]string, error) { return urls[r], nil })
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(kept, ","); got != "origin,github" {
		t.Errorf("kept = %s, want origin,github", got)
	}
	if got := strings.Join(skipped, ","); got != "both,mirror" {
		t.Errorf("skipped = %s, want both,mirror", got)
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
