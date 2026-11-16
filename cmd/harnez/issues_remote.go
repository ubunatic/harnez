package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/issues"
	issuesmaintenance "ubunatic.com/harnez/internal/issues/maintenance"
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

type remoteNewOptions struct {
	Dir    string
	Title  string
	JSON   bool
	Force  bool
	DryRun bool
	GH     ghRunner
}

func runIssuesNewRemote(w io.Writer, dir, title string, jsonOutput bool, gh ghRunner) error {
	return runIssuesNewRemoteWithForce(w, dir, title, jsonOutput, false, gh)
}

func runIssuesNewRemoteWithForce(w io.Writer, dir, title string, jsonOutput, force bool, gh ghRunner) error {
	return runIssuesNewRemoteWithOptions(w, nil, remoteNewOptions{Dir: dir, Title: title, JSON: jsonOutput, Force: force, GH: gh})
}

func runIssuesNewRemoteWithOptions(w, stderr io.Writer, opt remoteNewOptions) error {
	if strings.TrimSpace(opt.Title) == "" {
		return fmt.Errorf("issues new --remote: a title is required")
	}
	if !opt.Force {
		if err := checkIssueDuplicates(filepath.Join(opt.Dir, "issues"), opt.Title); err != nil {
			return err
		}
	}
	remote, err := issuesmaintenance.GitOutput(opt.Dir, "remote", "get-url", "github")
	if err != nil {
		return fmt.Errorf("issues new --remote: requires the repository's github remote: %w", err)
	}
	remote = strings.TrimSpace(remote)
	slug := issues.Slugify(opt.Title)
	if slug == "" {
		slug = "reserved"
	}
	body := fmt.Sprintf("In-repository ticket: issues/GH<issue-number>-%s.md (exact path follows allocation).", slug)
	if opt.DryRun {
		relPath := "issues/GH<number>-" + slug + ".md"
		if opt.JSON {
			return json.NewEncoder(w).Encode(map[string]any{
				"remote":  remote,
				"title":   strings.TrimSpace(opt.Title),
				"path":    relPath,
				"dry_run": true,
			})
		}
		_, err = fmt.Fprintf(w, "dry run, would file on %s via gh issue create and reserve %s\n", remote, relPath)
		return err
	}
	if err := requireHostSession("new"); err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "hint: write a draft to issues/inbox/ instead with `harnez issues new -d %s --inbox %q` or ask the host session to file it\n", opt.Dir, opt.Title)
		}
		return err
	}
	out, err := opt.GH(opt.Dir, "issue", "create", "--repo", remote, "--title", opt.Title, "--body", body)
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
	if _, err := opt.GH(opt.Dir, "issue", "edit", strings.TrimPrefix(num, "GH"), "--repo", remote, "--body", body); err != nil {
		return fmt.Errorf("issues new --remote: gh issue edit failed for %s; issue exists but no ticket file was written: %w", issueURL, err)
	}
	allocated, filename, err := issues.Reserve(filepath.Join(opt.Dir, "issues"), issues.ReserveOptions{Title: opt.Title, RemoteID: num})
	if err != nil {
		return fmt.Errorf("issues new --remote: GitHub issue %s exists, but ticket reservation failed: %w", issueURL, err)
	}
	return finishIssuesNew(w, opt.Dir, allocated, filename, opt.JSON)
}
