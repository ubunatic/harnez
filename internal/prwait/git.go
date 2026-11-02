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

// RemoteURL returns the fetch URL of remote.
func (SystemGit) RemoteURL(dir, remote string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", remote).Output()
	if err != nil {
		return "", fmt.Errorf("git remote %q: no such remote", remote)
	}
	return strings.TrimSpace(string(out)), nil
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
	err := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", sha, ref).Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s failed", sha, ref)
}
