package release

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// errAwaitingMerge stops a protected-main release after it opened or updated
// its PR; the caller reports it as a pause, not a failure.
var errAwaitingMerge = errors.New("release PR awaiting merge")

// releaseTopic is the AGit topic of a release PR, e.g. release-v0.5.6.
func releaseTopic(tag string) string { return "release-" + tag }

// pushReleaseViaPR is the release push for a protected default branch. The
// forge refuses a direct push, so the bump commit goes out as an AGit PR and
// the tag waits until that commit is on the remote default branch. Each run,
// first or --continue, checks the merge: not merged → push or update the PR
// and pause; merged → push the tag everywhere and mirror the merged branch to
// the other remotes.
func pushReleaseViaPR(opt Options, tag, branch string, forge *ForgeInfo) error {
	if forge == nil {
		return fmt.Errorf("protected %s: no Codeberg/Forgejo remote found for the release PR", branch)
	}
	if forge.Host == "github.com" {
		return fmt.Errorf("protected %s on GitHub: AGit is not available; release from a release/%s branch (docs/GoRelease.md)", branch, tag)
	}
	remote := forge.RemoteName
	tagRef := "refs/tags/" + tag
	if opt.DryRun {
		fmt.Fprintf(opt.Out, "  [dry-run]   Would push the bump commit as AGit PR %s:refs/for/%s/%s, then push tag %s after the merge (harnez release --continue)\n", remote, branch, releaseTopic(tag), tag)
		return nil
	}
	tagCommit, err := getGitRef(opt.Dir, tagRef+"^{commit}")
	if err != nil {
		return fmt.Errorf("resolve tag %s: %w", tag, err)
	}
	// A tag recreated at a pulled merge commit would pass the ancestor check
	// trivially; the release tag must sit on the bump commit itself.
	if parents, _ := gitRun(opt.Dir, "rev-list", "--parents", "-n", "1", tagCommit); len(strings.Fields(parents)) > 2 {
		return fmt.Errorf("tag %s points at merge commit %.8s, not the bump commit; delete the local tag and recreate it on the bump commit", tag, tagCommit)
	}
	// Fetch from the push URL: it is the server that refused the direct push
	// and the one the PR merges into.
	urls, err := pushURLs(opt.Dir, remote)
	if err != nil || len(urls) == 0 {
		return fmt.Errorf("push URL of %s: %v", remote, err)
	}
	remoteBranch := "refs/remotes/" + remote + "/" + branch
	if out, err := gitRun(opt.Dir, "fetch", "--quiet", urls[0], "+refs/heads/"+branch+":"+remoteBranch); err != nil {
		return fmt.Errorf("fetch %s %s: %w: %s", remote, branch, err, out)
	}
	merged := exec.Command("git", "merge-base", "--is-ancestor", tagCommit, remoteBranch)
	merged.Dir = opt.Dir
	if merged.Run() != nil {
		return openReleasePR(opt, tag, branch, remote, tagCommit)
	}

	fmt.Fprintf(opt.Out, "  [git]       Release commit is on %s/%s; pushing tag %s\n", remote, branch, tag)
	if out, err := gitRun(opt.Dir, "push", "--no-force", "--", remote, tagRef+":"+tagRef); err != nil {
		return fmt.Errorf("push tag %s to %s: %w: %s", tag, remote, err, out)
	}
	others, err := mirrorRemotes(opt, remote)
	if err != nil {
		return err
	}
	for _, other := range others {
		fmt.Fprintf(opt.Out, "  [git]       Mirroring %s/%s and tag %s to %s\n", remote, branch, tag, other)
		branchRef := "refs/heads/" + branch
		if out, err := gitRun(opt.Dir, "push", "--atomic", "--no-force", "--", other, remoteBranch+":"+branchRef, tagRef+":"+tagRef); err != nil {
			return fmt.Errorf("mirror to %s failed; the tag is on %s, retry with --continue: %w: %s", other, remote, err, out)
		}
	}
	// Local branch still points at the unmerged bump commit; follow the remote.
	if out, err := gitRun(opt.Dir, "merge", "--ff-only", "--quiet", remoteBranch); err != nil {
		fmt.Fprintf(opt.Out, "  [git]       Note: could not fast-forward %s to %s/%s: %s\n", branch, remote, branch, strings.TrimSpace(out))
	}
	return nil
}

func openReleasePR(opt Options, tag, branch, remote, commit string) error {
	ref := fmt.Sprintf("refs/for/%s/%s", branch, releaseTopic(tag))
	fmt.Fprintf(opt.Out, "  [git]       Protected %s: pushing release commit as AGit PR %s\n", branch, ref)
	out, err := gitRun(opt.Dir, "push", "--", remote, commit+":"+ref)
	if err != nil {
		return fmt.Errorf("push release PR %s to %s: %w: %s", ref, remote, err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "/pulls/") {
			fmt.Fprintf(opt.Out, "  [git]       PR: %s\n", strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "remote:")))
		}
	}
	fmt.Fprintf(opt.Out, "  [git]       Merge the PR with a merge commit or fast-forward (squash/rebase would drop commit %.8s), wait with `harnez pr wait <n> --contains %.8s`, then run `harnez release --continue`\n", commit, commit)
	return errAwaitingMerge
}

// mirrorRemotes lists the release remotes other than the forge remote, minus
// remotes whose push URLs another remote already covers.
func mirrorRemotes(opt Options, forgeRemote string) ([]string, error) {
	out, err := gitRun(opt.Dir, "remote")
	if err != nil {
		return nil, fmt.Errorf("list release remotes: %w: %s", err, out)
	}
	remotes := []string{forgeRemote}
	for _, r := range strings.Fields(out) {
		if r != forgeRemote {
			remotes = append(remotes, r)
		}
	}
	kept, _, err := selectPushRemotes(remotes, func(r string) ([]string, error) { return pushURLs(opt.Dir, r) })
	if err != nil {
		return nil, err
	}
	var others []string
	for _, r := range kept {
		if r != forgeRemote {
			others = append(others, r)
		}
	}
	return others, nil
}

func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
