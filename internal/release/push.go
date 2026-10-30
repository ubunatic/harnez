package release

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/gitflow"
)

// refuseProtectedBranchPush stops a release from pushing the default branch
// when git.protected_main is set. It runs before any remote is contacted, so
// no remote can be updated while another is refused.
func refuseProtectedBranchPush(opt Options, tag string) error {
	cfg, err := gitflow.Load(opt.Dir)
	if err != nil {
		return err
	}
	if !cfg.ProtectedMain {
		return nil
	}
	branch, err := getGitCurrentBranch(opt.Dir)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	if !slices.Contains(gitflow.ProtectedBranches(opt.Dir, cfg), branch) {
		return nil
	}
	return fmt.Errorf("git.protected_main is set in .harnez/repo.yaml: refusing to push '%s' with release %s; run: git switch -c release/%s, then release from that branch and open a PR", branch, tag, tag)
}

// pushRelease treats the repository's configured Git remotes as development
// destinations. Validate every destination before writing any refs, then keep
// branch and tag updates atomic within each remote. Separate servers cannot
// participate in one atomic transaction; a later transport failure or concurrent
// remote update can still require retrying with --continue.
func pushRelease(opt Options, tag string, forge *ForgeInfo) error {
	if err := refuseProtectedBranchPush(opt, tag); err != nil {
		return err
	}
	cmd := exec.Command("git", "remote")
	cmd.Dir = opt.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("list release remotes: %w: %s", err, out)
	}
	remotes := strings.Fields(string(out))
	if len(remotes) == 0 {
		return fmt.Errorf("no Git remotes configured for release push")
	}
	// Preserve the publishing forge's priority (Codeberg first for releases).
	if forge != nil {
		for i, remote := range remotes {
			if remote == forge.RemoteName {
				remotes = append([]string{remote}, append(remotes[:i], remotes[i+1:]...)...)
				break
			}
		}
	}
	if opt.DryRun {
		for _, remote := range remotes {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would push branch and tag %s to %s\n", tag, remote)
		}
		return nil
	}
	branch, err := getGitCurrentBranch(opt.Dir)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	if branch == "HEAD" {
		return fmt.Errorf("release push requires a checked-out branch")
	}
	branchRef := "refs/heads/" + branch
	tagRef := "refs/tags/" + tag
	push := func(remote string, dryRun bool) error {
		args := []string{"push", "--atomic", "--no-force"}
		if dryRun {
			args = append(args, "--dry-run")
		}
		args = append(args, "--", remote, branchRef+":"+branchRef, tagRef+":"+tagRef)
		cmd := exec.Command("git", args...)
		cmd.Dir = opt.Dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("remote %s: %w: %s", remote, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	for _, remote := range remotes {
		if err := push(remote, true); err != nil {
			return fmt.Errorf("release push preflight failed (divergence, conflicting tag, or unreachable remote); no remotes pushed: %w", err)
		}
	}
	for _, remote := range remotes {
		fmt.Fprintf(opt.Out, "  [git]       Pushing branch %s and tag %s to %s...\n", branch, tag, remote)
		if err := push(remote, false); err != nil {
			return fmt.Errorf("release push failed; earlier remotes may have been updated, retry with --continue: %w", err)
		}
	}
	return nil
}
