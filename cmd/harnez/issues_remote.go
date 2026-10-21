package main

import (
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/issues"
)

type ghRunner func(dir string, args ...string) ([]byte, error)

func runGH(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func runIssuesNewRemote(w io.Writer, dir, title string, jsonOutput bool, gh ghRunner) error {
	return runIssuesNewRemoteWithForce(w, dir, title, jsonOutput, false, gh)
}

func runIssuesNewRemoteWithForce(w io.Writer, dir, title string, jsonOutput, force bool, gh ghRunner) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("issues new --remote: a title is required")
	}
	if !force {
		if err := checkIssueDuplicates(filepath.Join(dir, "issues"), title); err != nil {
			return err
		}
	}
	remote, err := gitOutput(dir, "remote", "get-url", "github")
	if err != nil {
		return fmt.Errorf("issues new --remote: requires the repository's github remote: %w", err)
	}
	remote = strings.TrimSpace(remote)
	slug := issues.Slugify(title)
	if slug == "" {
		slug = "reserved"
	}
	body := fmt.Sprintf("In-repository ticket: issues/GH<issue-number>-%s.md (exact path follows allocation).", slug)
	out, err := gh(dir, "issue", "create", "--repo", remote, "--title", title, "--body", body)
	if err != nil {
		return fmt.Errorf("issues new --remote: gh issue create failed; install gh and authenticate with gh auth login (no local fallback): %w", err)
	}
	issueURL := strings.TrimSpace(string(out))
	u, err := url.Parse(issueURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || path.Base(path.Dir(u.Path)) != "issues" {
		return fmt.Errorf("issues new --remote: gh returned an invalid issue URL %q", issueURL)
	}
	num, err := issues.NormalizeID("GH" + path.Base(u.Path))
	if err != nil {
		return fmt.Errorf("issues new --remote: gh returned an invalid issue number in %q", issueURL)
	}
	filename := num + "-" + slug + ".md"
	ticketPath := "issues/" + filename
	repoPath := path.Dir(path.Dir(u.Path))
	u.Path = repoPath + "/blob/HEAD/" + ticketPath
	u.RawPath, u.RawQuery, u.Fragment = "", "", ""
	body = fmt.Sprintf("Tracked in [%s](%s).", ticketPath, u.String())
	if _, err := gh(dir, "issue", "edit", strings.TrimPrefix(num, "GH"), "--repo", remote, "--body", body); err != nil {
		return fmt.Errorf("issues new --remote: gh issue edit failed for %s; issue exists but no ticket file was written: %w", issueURL, err)
	}
	allocated, filename, err := issues.Reserve(filepath.Join(dir, "issues"), issues.ReserveOptions{Title: title, RemoteID: num})
	if err != nil {
		return fmt.Errorf("issues new --remote: GitHub issue %s exists, but ticket reservation failed: %w", issueURL, err)
	}
	return finishIssuesNew(w, dir, allocated, filename, jsonOutput)
}
