package claude

import (
	"fmt"
	"os/exec"
	"strings"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/gitflow"
	"ubunatic.com/harnez/internal/prwait"
)

// agitPullKeys are the repo-local keys that make plain `git pull` work on a
// branch cut from main in an AGit repository (issue 1206). Without
// autoSetupMerge=always, a branch cut from a local main has no upstream and
// `git pull` stops with "There is no tracking information". checkout.defaultRemote
// pins the remote for checkouts of remote-only branches. The remote name comes
// from spec/pr.yaml, like the rest of the forge settings.
const agitAutoSetupMerge = "always"

// installAgitPullConfig writes the AGit pull keys into the repository's own
// .git/config when the repository uses AGit. It does nothing outside a git work
// tree, and it leaves both keys untouched for repositories that do not use AGit.
// Global git config is never written.
func installAgitPullConfig(ir *initReport, dir string, repoConfig *RepoConfig) error {
	if err := exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run(); err != nil {
		return nil
	}
	prSpec, err := prwait.LoadSpec()
	if err != nil {
		return err
	}
	remote := prSpec.DefaultRemote
	agit, err := usesAgit(dir, repoConfig, remote, prSpec)
	if err != nil || !agit {
		return err
	}
	settings := []struct{ key, value string }{
		{"branch.autoSetupMerge", agitAutoSetupMerge},
		{"checkout.defaultRemote", remote},
	}
	for _, s := range settings {
		changed, err := setLocalGitConfig(dir, s.key, s.value)
		if err != nil {
			return err
		}
		if changed {
			fmt.Printf("  configured %s=%s in .git/config (AGit pulls)\n", s.key, s.value)
			ir.changes++
		}
	}
	return nil
}

// usesAgit reports whether the repository uses AGit by any of three signals:
// .harnez/repo.yaml sets git.mode team, the remote's URL resolves to a
// kind: forgejo host in spec/pr.yaml, or the remote's push refspecs target
// refs/for/ (AGit pushes).
func usesAgit(dir string, repoConfig *RepoConfig, remote string, prSpec prwait.Spec) (bool, error) {
	if repoConfig != nil && repoConfig.Git.Mode == gitflow.ModeTeam {
		return true, nil
	}
	if out, err := exec.Command("git", "-C", dir, "remote", "get-url", remote).Output(); err == nil {
		if host, _, _, ok := forge.ParseRemoteURL(string(out)); ok {
			if fg, ok := prSpec.ForgeFor(host); ok && fg.Kind == "forgejo" {
				return true, nil
			}
		}
	}
	// --get-all exits 1 when the key is unset; that is "not AGit", not an error.
	out, err := exec.Command("git", "-C", dir, "config", "--local", "--get-all", "remote."+remote+".push").Output()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(out), "refs/for/"), nil
}

// setLocalGitConfig makes key hold value in the repository's own config. It
// reports whether it wrote anything, so a re-run of init reports no change.
func setLocalGitConfig(dir, key, value string) (bool, error) {
	out, err := exec.Command("git", "-C", dir, "config", "--local", "--get", key).Output()
	if err == nil && strings.TrimSpace(string(out)) == value {
		return false, nil
	}
	if out, err := exec.Command("git", "-C", dir, "config", "--local", "--replace-all", key, value).CombinedOutput(); err != nil {
		return false, fmt.Errorf("set %s: %w: %s", key, err, strings.TrimSpace(string(out)))
	}
	return true, nil
}
