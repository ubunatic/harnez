package release

import (
	"fmt"
	"os/exec"
	"slices"
	"sort"
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
	remotes, skipped, err := selectPushRemotes(remotes, func(remote string) ([]string, error) {
		return pushURLs(opt.Dir, remote)
	})
	if err != nil {
		return err
	}
	for _, remote := range skipped {
		fmt.Fprintf(opt.Out, "  [skip]      %s: every push URL is already pushed by another remote\n", remote)
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
	localBranchSha, err := getGitRef(opt.Dir, branchRef)
	if err != nil {
		return fmt.Errorf("get local branch sha: %w", err)
	}
	localTagSha, err := getGitRef(opt.Dir, tagRef)
	if err != nil {
		return fmt.Errorf("get local tag sha: %w", err)
	}

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

			urls, urlErr := pushURLs(opt.Dir, remote)
			if urlErr == nil && len(urls) > 0 {
				allMatched := true
				for _, url := range urls {
					cmd := exec.Command("git", "ls-remote", url, branchRef, tagRef)
					cmd.Dir = opt.Dir
					if out, lsErr := cmd.CombinedOutput(); lsErr == nil {
						var remoteBranchSha, remoteTagSha string
						for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
							parts := strings.Fields(line)
							if len(parts) == 2 {
								switch parts[1] {
								case branchRef:
									remoteBranchSha = parts[0]
								case tagRef:
									remoteTagSha = parts[0]
								}
							}
						}
						if remoteBranchSha == "" || remoteBranchSha != localBranchSha || remoteTagSha != localTagSha {
							allMatched = false
							break
						}
					} else {
						allMatched = false
						break
					}
				}
				if allMatched {
					fmt.Fprintf(opt.Out, "  [git]       Remote %s already has the correct branch %s and tag %s (concurrently updated), continuing...\n", remote, branch, tag)
					continue
				}
			}
			return fmt.Errorf("release push failed; earlier remotes may have been updated, retry with --continue: %w", err)
		}
	}
	return nil
}

func getGitRef(dir, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// pushURLs lists the push URLs of a remote. A remote without an explicit push
// URL pushes to its fetch URL, which git reports here as well.
func pushURLs(dir, remote string) ([]string, error) {
	cmd := exec.Command("git", "remote", "get-url", "--push", "--all", remote)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list push URLs of remote %s: %w", remote, err)
	}
	var urls []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

// selectPushRemotes returns the remotes to push, in their given order, and the
// ones it skips. A remote is skipped when each of its push URLs is also a push
// URL of another kept remote, so a fan-out remote such as `both` (Codeberg and
// GitHub) does not run its hooks beside the single-URL remotes it duplicates.
// Remotes with more push URLs are judged first. Among equal ones, later remotes
// are judged first, so the earlier remote (the forge) is the one kept. A remote
// with no listed push URL is always kept, so its error surfaces during the push.
func selectPushRemotes(remotes []string, urlsOf func(remote string) ([]string, error)) (kept, skipped []string, err error) {
	urls := make(map[string][]string, len(remotes))
	for _, remote := range remotes {
		list, err := urlsOf(remote)
		if err != nil {
			return nil, nil, err
		}
		urls[remote] = list
	}
	order := make([]int, len(remotes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		na, nb := len(urls[remotes[order[a]]]), len(urls[remotes[order[b]]])
		if na != nb {
			return na > nb
		}
		return order[a] > order[b]
	})
	keep := make(map[string]bool, len(remotes))
	for _, remote := range remotes {
		keep[remote] = true
	}
	for _, i := range order {
		remote := remotes[i]
		if !coveredByKept(remote, urls, keep) {
			continue
		}
		keep[remote] = false
		skipped = append(skipped, remote)
	}
	for _, remote := range remotes {
		if keep[remote] {
			kept = append(kept, remote)
		}
	}
	return kept, skipped, nil
}

// coveredByKept reports whether every push URL of remote is a push URL of
// another kept remote. Dropping a remote this way never drops a URL.
func coveredByKept(remote string, urls map[string][]string, keep map[string]bool) bool {
	mine := urls[remote]
	if len(mine) == 0 {
		return false
	}
	for _, u := range mine {
		found := false
		for other, kept := range keep {
			if kept && other != remote && slices.Contains(urls[other], u) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
