// Package prfixup fixes a pull request whose only conflicts are generated indexes, for
// `harnez pr wait --fixup` (issue 1261). It merges the base into the PR head in a temporary
// worktree, regenerates the indexes, checks ticket numbers against the base and pushes the
// merge to the PR's own head. The caller's checkout is never touched.
package prfixup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/index"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/prwait"
)

// Generated are the files `harnez index` writes from ticket and study files. A conflict in
// any other file needs a human or agent decision, so the fixup refuses it.
var Generated = []string{"issues/README.md", "issues/BACKLOG.md", "issues/CLOSED.md", "docs/README.md"}

// Fixer implements prwait.Fixer with the git binary.
type Fixer struct{}

var _ prwait.Fixer = Fixer{}

// Fixup merges the base into the PR head, regenerates the conflicting indexes and pushes the
// merge commit, a fast-forward of the PR head, to the PR's own head. Nothing is pushed when
// anything fails.
func (Fixer) Fixup(req prwait.FixupRequest) (prwait.FixupResult, error) {
	dest, err := PushDest(req.Target, req.Info, req.Base)
	if err != nil {
		return prwait.FixupResult{}, err
	}
	if other := outside(req.Files); len(other) > 0 {
		return prwait.FixupResult{}, fmt.Errorf("conflicts outside the generated indexes: %s", strings.Join(other, ", "))
	}
	if req.Info.Head.SHA == "" {
		return prwait.FixupResult{}, fmt.Errorf("the forge reported no head commit for PR %d", req.Target.Number)
	}
	tmp, err := os.MkdirTemp("", "harnez-pr-fixup-")
	if err != nil {
		return prwait.FixupResult{}, err
	}
	wt := filepath.Join(tmp, "wt")
	defer func() {
		_, _ = run(req.Dir, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(tmp)
		_, _ = run(req.Dir, "worktree", "prune")
	}()
	if _, err := run(req.Dir, "worktree", "add", "--detach", "--quiet", wt, req.Info.Head.SHA); err != nil {
		return prwait.FixupResult{}, err
	}
	baseRef := req.Remote + "/" + req.Base
	regenerated, err := mergeAndRegenerate(wt, baseRef, req.Files)
	if err != nil {
		return prwait.FixupResult{}, err
	}
	msg := fmt.Sprintf("Merge %s into PR #%d\n\nharnez pr wait --fixup regenerated %s.", baseRef, req.Target.Number, strings.Join(regenerated, ", "))
	if _, err := run(wt, "commit", "--quiet", "-m", msg); err != nil {
		return prwait.FixupResult{}, err
	}
	head, err := run(wt, "rev-parse", "HEAD")
	if err != nil {
		return prwait.FixupResult{}, err
	}
	out, err := run(wt, "push", req.Remote, "HEAD:"+dest)
	if err != nil {
		return prwait.FixupResult{}, fmt.Errorf("push to %s rejected: %w", dest, err)
	}
	if n, ok := pushedPR(out); ok && n != req.Target.Number {
		pusher, _, _ := req.Info.AGitTopic()
		return prwait.FixupResult{}, fmt.Errorf("the push to %s went to PR %d, not PR %d: only %s's push updates PR %d; close PR %d",
			dest, n, req.Target.Number, pusher, req.Target.Number, n)
	}
	return prwait.FixupResult{Head: head, Dest: dest, Regenerated: regenerated}, nil
}

// PushDest is the ref that updates the PR's head: refs/for/<base>/<topic> for an AGit PR, or
// the head branch of a PR from a branch in the same repository. Fork heads are not ours.
func PushDest(t prwait.Target, info prwait.PullInfo, base string) (string, error) {
	if _, topic, ok := info.AGitTopic(); ok {
		return "refs/for/" + base + "/" + topic, nil
	}
	ref := info.Head.Ref
	if ref == "" || strings.HasPrefix(ref, "refs/") {
		return "", fmt.Errorf("cannot tell where PR %d's head lives (head ref %q, label %q)", t.Number, ref, info.Head.Label)
	}
	if repo := info.Head.Repo.FullName; repo != t.Owner+"/"+t.Repo {
		return "", fmt.Errorf("PR %d's head is branch %s of %s, not of %s/%s", t.Number, ref, repo, t.Owner, t.Repo)
	}
	return "refs/heads/" + ref, nil
}

func outside(files []string) []string {
	var other []string
	for _, f := range files {
		if !slices.Contains(Generated, f) {
			other = append(other, f)
		}
	}
	return other
}

// mergeAndRegenerate merges baseRef into the worktree's head without committing, takes the
// base side of each conflicting index and regenerates it from the merged sources.
func mergeAndRegenerate(wt, baseRef string, files []string) ([]string, error) {
	mergeOut, mergeErr := run(wt, "merge", "--no-ff", "--no-commit", baseRef)
	unmerged, err := run(wt, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	left := strings.Fields(unmerged)
	if other := outside(left); len(other) > 0 {
		return nil, fmt.Errorf("conflicts outside the generated indexes: %s", strings.Join(other, ", "))
	}
	if mergeErr != nil && len(left) == 0 {
		return nil, fmt.Errorf("merge %s: %w: %s", baseRef, mergeErr, mergeOut)
	}
	for _, f := range left {
		if _, err := run(wt, "checkout", "--theirs", "--", f); err != nil {
			return nil, err
		}
	}
	if err := checkTickets(wt, baseRef); err != nil {
		return nil, err
	}
	regenerated := slices.Sorted(slices.Values(append(slices.Clone(files), left...)))
	regenerated = slices.Compact(regenerated)
	if err := regenerate(wt, regenerated); err != nil {
		return nil, err
	}
	var present []string
	for _, f := range Generated {
		if _, err := os.Stat(filepath.Join(wt, f)); err == nil {
			present = append(present, f)
		}
	}
	if _, err := run(wt, append([]string{"add", "--"}, present...)...); err != nil {
		return nil, err
	}
	if still, err := run(wt, "diff", "--name-only", "--diff-filter=U"); err != nil || still != "" {
		return nil, errors.Join(err, fmt.Errorf("still unmerged after regenerating: %s", still))
	}
	return regenerated, nil
}

// regenerate rewrites the issue indexes and the studies table that files touch, as
// `harnez index` would, then makes sure no conflict marker is left in them.
func regenerate(wt string, files []string) error {
	issuesDir := filepath.Join(wt, "issues")
	docsDir := filepath.Join(wt, "docs")
	var doIssues, doDocs bool
	for _, f := range files {
		doIssues = doIssues || strings.HasPrefix(f, "issues/")
		doDocs = doDocs || f == "docs/README.md"
	}
	if doIssues {
		if err := issues.CheckDuplicateNumbers(issuesDir); err != nil {
			return err
		}
		if _, err := index.UpdateIssuesReadme(filepath.Join(issuesDir, "README.md"), issuesDir); err != nil {
			return err
		}
	}
	if doDocs {
		if _, err := index.UpdateDocsReadme(filepath.Join(docsDir, "README.md"), docsDir); err != nil {
			return err
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(wt, f))
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("\n<<<<<<< ")) || bytes.Contains(data, []byte("\n>>>>>>> ")) {
			return fmt.Errorf("%s still has conflict markers after regenerating", f)
		}
	}
	return nil
}

// checkTickets is `harnez issues lint --against <base>`: a ticket the PR adds must not reuse a
// number the base gives to another file.
func checkTickets(wt, baseRef string) error {
	if _, err := os.Stat(filepath.Join(wt, "issues")); err != nil {
		return nil
	}
	collisions, ok, err := issues.CheckAgainst(wt, baseRef)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s not found for the ticket number check", baseRef)
	}
	if len(collisions) == 0 {
		return nil
	}
	var lines []string
	for _, c := range collisions {
		lines = append(lines, fmt.Sprintf("ticket %s: %s reuses the number of %s", c.Number, c.Local, c.Remote))
	}
	return fmt.Errorf("ticket number collision with %s; renumber with `harnez issues mv`: %s", baseRef, strings.Join(lines, "; "))
}

var pullRefPattern = regexp.MustCompile(`refs/pull/(\d+)/head`)

// pushedPR reads the PR an AGit push updated from git's report, e.g. "HEAD -> refs/pull/205/head".
func pushedPR(out string) (int, bool) {
	m := pullRefPattern.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	var n int
	_, err := fmt.Sscan(m[1], &n)
	return n, err == nil
}

// run executes git in dir with Git LFS downloads off (the fixup edits only Markdown) and
// returns trimmed combined output. Hooks stay on: they are the repository's own guards.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, text)
	}
	return text, nil
}
