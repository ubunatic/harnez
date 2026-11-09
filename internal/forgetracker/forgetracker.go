// Package forgetracker reads tickets from the repository's forge (Codeberg/Forgejo or
// GitHub) for repos that keep no local issues/ dir (issue 1116). `harnez issues show`
// and `harnez find issues` read through it; `harnez issues new` files a ticket through
// the forge's own CLI (issue 1195); closing stays with the forge's tools.
package forgetracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/prwait"
)

// Target is the forge repository that holds the tickets.
type Target struct {
	Forge forge.Forge
	Owner string
	Repo  string
}

// String names the repository as host/owner/repo.
func (t Target) String() string { return t.Forge.Host + "/" + t.Owner + "/" + t.Repo }

// Source reads issues; forge.PullLister satisfies it and tests replace it.
type Source interface {
	Issue(fg forge.Forge, owner, repo string, n int) (forge.RemoteIssue, error)
	Issues(fg forge.Forge, owner, repo, state string, opts forge.ScanOptions) ([]forge.RemoteIssue, bool, error)
	// Repo probes the repo itself; nil means it is reachable with the token in use (issue 1199).
	Repo(fg forge.Forge, owner, repo string) error
}

// ErrNotApplicable means the repo has a local issues/ dir, or no forge origin, so the
// caller keeps its local behaviour.
var ErrNotApplicable = errors.New("forgetracker: not applicable")

// Resolve returns the forge target when dir has no issues/ dir and its origin remote
// points at a forge configured in spec/pr.yaml. Otherwise it returns ErrNotApplicable.
func Resolve(dir string, spec prwait.Spec) (Target, error) {
	if _, err := os.Stat(filepath.Join(dir, "issues")); !errors.Is(err, fs.ErrNotExist) {
		return Target{}, ErrNotApplicable
	}
	remote := spec.DefaultRemote
	if remote == "" {
		remote = "origin"
	}
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", remote).Output()
	if err != nil {
		return Target{}, ErrNotApplicable
	}
	host, owner, repo, ok := forge.ParseRemoteURL(string(out))
	if !ok {
		return Target{}, ErrNotApplicable
	}
	fg, ok := spec.ForgeFor(host)
	if !ok {
		return Target{}, ErrNotApplicable
	}
	return Target{Forge: fg, Owner: owner, Repo: repo}, nil
}

// NewSource builds the live forge reader from the spec and the system token sources.
func NewSource(spec prwait.Spec) Source {
	return forge.NewPullLister(spec.OpenPRScan, forge.SystemTokenSource(os.Getenv).For)
}

type issueJSON struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Body   string `json:"body,omitempty"`
}

// Show prints one forge issue. ticket accepts "12", "#12" or "GH12".
func Show(w io.Writer, src Source, t Target, ticket string, jsonOut bool) error {
	n, err := strconv.Atoi(strings.TrimLeft(strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(ticket)), "GH"), "#"))
	if err != nil || n <= 0 {
		return fmt.Errorf("issues show: invalid ticket number %q", ticket)
	}
	is, err := src.Issue(t.Forge, t.Owner, t.Repo, n)
	if err != nil {
		// A 404 on the issue of a reachable repo means the issue is missing (issue 1199). The
		// result is a plain error, not an APIError, so the private-repo token hint is skipped.
		// Any other probe result keeps the original error and its hint.
		var apiErr *forge.APIError
		if errors.As(err, &apiErr) && apiErr.Status == 404 && src.Repo(t.Forge, t.Owner, t.Repo) == nil {
			return fmt.Errorf("issue #%d not found on %s", n, t)
		}
		return fmt.Errorf("issues show: %s#%d: %w", t, n, err)
	}
	if is.PullRequest != nil {
		return fmt.Errorf("issues show: %s#%d is a pull request, not an issue", t, n)
	}
	if jsonOut {
		return writeJSON(w, issueJSON{Number: is.Number, Title: is.Title, State: is.State, URL: is.HTMLURL, Body: is.Body})
	}
	_, err = fmt.Fprintf(w, "# %d — %s\n\n**Status**: %s\n**Source**: %s\n\n---\n\n%s\n", is.Number, is.Title, is.State, is.HTMLURL, strings.TrimSpace(is.Body))
	return err
}

// Find prints the forge issues whose title or body contains every term, case-insensitively,
// newest first, as "<number>\t<state>\t<title>\t<url>". limit caps the rows unless all is set.
//
// The only filter is status:open or status:closed, which the forge applies as its issue
// state; status:active is an alias for open (issue 1198: forges have no In Progress, Blocked
// or Draft). Any other status: value is an error before the forge is asked.
func Find(w io.Writer, src Source, t Target, opts forge.ScanOptions, terms []string, limit int, all, jsonOut bool) error {
	state := "all"
	statusSet := false
	var lowered []string
	for _, term := range terms {
		for _, f := range strings.Fields(term) {
			if key, val, ok := strings.Cut(f, ":"); ok && key == "status" {
				if statusSet {
					return fmt.Errorf("find issues: only one status: filter is supported for forge tickets")
				}
				if val != "open" && val != "closed" && val != "active" {
					return fmt.Errorf("find issues: %s is not supported for forge tickets; supported values: status:open, status:closed, status:active", f)
				}
				if val == "active" {
					val = "open"
				}
				state, statusSet = val, true
				continue
			}
			if strings.Contains(f, ":") {
				return fmt.Errorf("find issues: filter %q is not supported for forge tickets; use plain search terms", f)
			}
			lowered = append(lowered, strings.ToLower(f))
		}
	}
	list, truncated, err := src.Issues(t.Forge, t.Owner, t.Repo, state, opts)
	if err != nil {
		return fmt.Errorf("find issues: %s: %w", t, err)
	}
	var rows []issueJSON
	for _, is := range list {
		hay := strings.ToLower(is.Title + "\n" + is.Body)
		match := true
		for _, term := range lowered {
			if !strings.Contains(hay, term) {
				match = false
				break
			}
		}
		if match {
			rows = append(rows, issueJSON{Number: is.Number, Title: is.Title, State: is.State, URL: is.HTMLURL})
		}
	}
	total := len(rows)
	if !all && limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	if jsonOut {
		return writeJSON(w, struct {
			Total     int         `json:"total"`
			Truncated bool        `json:"truncated"`
			Rows      []issueJSON `json:"rows"`
		}{total, truncated, rows})
	}
	if _, err := fmt.Fprintf(w, "# %d matches on %s, showing %d (use --all to show all)\n", total, t, len(rows)); err != nil {
		return err
	}
	if truncated {
		if _, err := fmt.Fprintf(w, "# searched the newest %d issues and pull requests; older ones may exist (spec/pr.yaml open_pr_scan page budget)\n", opts.PageSize*opts.MaxPages); err != nil {
			return err
		}
	}
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", r.Number, r.State, r.Title, r.URL); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
