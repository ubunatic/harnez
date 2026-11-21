package prwait

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GitOps is the git access pr wait needs: the remote URL, the --contains ancestry check and
// the local check that a PR the forge calls unmergeable really conflicts.
type GitOps interface {
	RemoteURL(dir, remote string) (string, error)
	FetchRemote(dir, remote string) error
	IsAncestor(dir, sha, ref string) (bool, error)
	Conflicts(dir, remote, base string, number int, head string) ([]string, error)
}

// emptyTree is git's well-known empty tree object. Reading attributes from it turns off every
// custom merge driver, such as the issue index driver, which the forge does not run either.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Conflicts fetches base and the PR head (refs/pull/<number>/head, served by Forgejo and
// GitHub), then merges head into remote/base in memory with git merge-tree. It returns the
// conflicting paths, or none when the merge is clean. It writes no worktree, index or branch.
func (SystemGit) Conflicts(dir, remote, base string, number int, head string) ([]string, error) {
	pullRef := fmt.Sprintf("refs/pull/%d/head", number)
	var stderr strings.Builder
	fetch := exec.Command("git", "-C", dir, "fetch", "--quiet", remote, base, pullRef)
	fetch.Stderr = &stderr
	if err := fetch.Run(); err != nil {
		return nil, gitError(err, stderr.String(), "git fetch %s %s %s", remote, base, pullRef)
	}
	return mergeTreeConflicts(dir, remote+"/"+base, head)
}

// mergeTreeConflicts merges b into a in memory. merge-tree exits 1 for conflicts and prints
// the result tree, then one conflicting path per line.
func mergeTreeConflicts(dir, a, b string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "--attr-source="+emptyTree,
		"merge-tree", "--write-tree", "--name-only", "--no-messages", a, b)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return nil, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return nil, gitError(err, stderr.String(), "git merge-tree %s %s", a, b)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines[1:] {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("git merge-tree %s %s: exit 1 without conflicting paths", a, b)
	}
	return files, nil
}

// SystemGit runs the real git binary.
type SystemGit struct{}

// RemoteURL returns the fetch URL of remote. Only git's exit 2 means the remote is missing;
// any other failure (unreadable config, not a repository) keeps git's exit code and stderr (issue 1249).
func (SystemGit) RemoteURL(dir, remote string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", remote)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return "", fmt.Errorf("git remote %q: no such remote", remote)
		}
		return "", gitError(err, stderr.String(), "git remote get-url %s", remote)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitError names the git command, its exit code and its stderr.
func gitError(err error, stderr, format string, args ...any) error {
	what := fmt.Sprintf(format, args...)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		what += fmt.Sprintf(": exit %d", exitErr.ExitCode())
	} else {
		what += ": " + err.Error()
	}
	if msg := strings.TrimSpace(stderr); msg != "" {
		what += ": " + msg
	}
	return errors.New(what)
}

// FetchRemote fetches remote so the --contains check sees the latest default branch.
func (SystemGit) FetchRemote(dir, remote string) error {
	if out, err := exec.Command("git", "-C", dir, "fetch", "--quiet", remote).CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch %s failed: %s", remote, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsAncestor reports whether sha is reachable from ref. Exit status 1 means "not an ancestor".
func (SystemGit) IsAncestor(dir, sha, ref string) (bool, error) {
	cmd := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", sha, ref)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, gitError(err, stderr.String(), "git merge-base --is-ancestor %s %s", sha, ref)
}
