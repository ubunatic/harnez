package prwait

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GitOps is the git access pr wait needs: the remote URL and the --contains ancestry check.
type GitOps interface {
	RemoteURL(dir, remote string) (string, error)
	FetchRemote(dir, remote string) error
	IsAncestor(dir, sha, ref string) (bool, error)
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
