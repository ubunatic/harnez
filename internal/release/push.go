package release

import (
	"fmt"
	"os/exec"
	"strings"
)

// pushRelease treats the repository's configured Git remotes as development
// destinations. Validate every destination before writing any refs, then keep
// branch and tag updates atomic within each remote. Separate servers cannot
// participate in one atomic transaction; a later transport failure or concurrent
// remote update can still require retrying with --continue.
func pushRelease(opt Options, tag string, forge *ForgeInfo) error {
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
