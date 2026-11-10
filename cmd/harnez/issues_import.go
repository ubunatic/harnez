package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/prwait"
)

// issueFetcher reads one remote issue; tests replace it.
type issueFetcher func(fg forge.Forge, owner, repo string, n int) (forge.RemoteIssue, error)

// runIssuesImport copies a forge issue (Codeberg/Forgejo or GitHub) into a new local
// Open ticket that links back to it (issue 1064). It only reads the forge.
func runIssuesImport(w io.Writer, dir, rawURL string, jsonOutput, offline bool, fetch issueFetcher) error {
	host, owner, repo, n, err := issues.ParseIssueURL(rawURL)
	if err != nil {
		return fmt.Errorf("issues import: %w", err)
	}
	spec, err := prwait.LoadSpec()
	if err != nil {
		return fmt.Errorf("issues import: %w", err)
	}
	fg, ok := spec.ForgeFor(host)
	if !ok {
		return fmt.Errorf("issues import: %s is not a configured forge (spec/pr.yaml forges)", host)
	}
	if fetch == nil {
		fetch = forge.NewPullLister(spec.IssueScan, forge.SystemTokenSource(os.Getenv).For).Issue
	}
	is, err := fetch(fg, owner, repo, n)
	if err != nil {
		return fmt.Errorf("issues import: %s/%s#%d: %w", owner, repo, n, err)
	}
	if is.PullRequest != nil {
		return fmt.Errorf("issues import: %s is a pull request, not an issue", rawURL)
	}
	if is.HTMLURL == "" {
		is.HTMLURL = rawURL
	}
	issuesDir := filepath.Join(dir, "issues")
	if prev := ticketLinking(issuesDir, is.HTMLURL); prev != "" {
		return fmt.Errorf("issues import: %s is already imported as issues/%s", is.HTMLURL, prev)
	}
	elsewhere, err := issues.Claims(dir, issues.ClaimOptions{Offline: offline, Warn: os.Stderr})
	if err != nil {
		return fmt.Errorf("issues import: %w", err)
	}
	num, filename, err := issues.Reserve(issuesDir, issues.ReserveOptions{Title: is.Title, Elsewhere: elsewhere})
	if err != nil {
		return fmt.Errorf("issues import: %w", err)
	}
	body := issues.ImportedTicket(num, is, "P2 (Medium)")
	if err := os.WriteFile(filepath.Join(issuesDir, filename), []byte(body), 0o644); err != nil {
		return fmt.Errorf("issues import: %w", err)
	}
	return finishIssuesNew(w, dir, num, filename, jsonOutput)
}

// ticketLinking returns the ticket file whose header already links url, or "".
func ticketLinking(issuesDir, url string) string {
	entries, _ := os.ReadDir(issuesDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(issuesDir, e.Name()))
		if err == nil && strings.Contains(string(data), "**Related**: "+url+" ") {
			return e.Name()
		}
	}
	return ""
}
