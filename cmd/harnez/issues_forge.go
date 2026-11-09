package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetracker"
	"ubunatic.com/harnez/internal/prwait"
)

// findIssuesLocalOnly lists `find issues` words that the forge reader does not serve. On a
// forge repo they are refused with forgeLocalOnlyError; closed is not in the set because it
// maps to status:closed (issue 1198).
var findIssuesLocalOnly = map[string]bool{
	"next": true, "last": true, "similar": true,
	"open": true, "blocked": true, "draft": true,
}

// forgeLocalOnlyError refuses a local-only find word on a forge repo and names the forge
// form that does work (issue 1198).
func forgeLocalOnlyError(word string) error {
	switch word {
	case "open":
		return fmt.Errorf("find issues open: not supported for forge tickets; use find issues status:open")
	case "blocked", "draft":
		return fmt.Errorf("find issues %s: forge tickets have only open and closed states; use status:open or status:closed", word)
	default:
		return fmt.Errorf("find issues %s: needs a local issues/ dir; forge tickets support status:open, status:closed, status:active and plain terms", word)
	}
}

// forgeFindArgs splits args into words, as forgetracker.Find does, so a quoted query like
// "closed bug" is read the same as closed bug. args excludes "issues".
func forgeFindArgs(args []string) []string {
	return strings.Fields(strings.Join(args, " "))
}

// forgeFindTerms rewrites the leading bare word closed to status:closed, so it is a state
// filter on the forge rather than a search term (issue 1198). words come from forgeFindArgs.
func forgeFindTerms(words []string) []string {
	if len(words) > 0 && words[0] == "closed" {
		return append([]string{"status:closed"}, words[1:]...)
	}
	return words
}

// forgeTrackerSource is the live forge reader; tests replace it.
var forgeTrackerSource = func(spec prwait.Spec) forgetracker.Source { return forgetracker.NewSource(spec) }

// resolveForgeTracker reports the forge that holds dir's tickets when dir has no
// issues/ dir (issue 1116). ok=false keeps the local tracker behaviour.
func resolveForgeTracker(stderr io.Writer, dir string) (forgetracker.Target, forgetracker.Source, prwait.Spec, bool, error) {
	// A local issues/ dir wins before the spec is even read, so a broken spec
	// never changes the local tracker's behaviour.
	if _, err := os.Stat(filepath.Join(dir, "issues")); !errors.Is(err, fs.ErrNotExist) {
		return forgetracker.Target{}, nil, prwait.Spec{}, false, nil
	}
	spec, err := prwait.LoadSpec()
	if err != nil {
		return forgetracker.Target{}, nil, spec, false, err
	}
	t, err := forgetracker.Resolve(dir, spec)
	if errors.Is(err, forgetracker.ErrNotApplicable) {
		return t, nil, spec, false, nil
	}
	if err != nil {
		return t, nil, spec, false, err
	}
	fmt.Fprintf(stderr, "no local issues/ dir: tickets live on %s\n", t)
	return t, forgeTrackerSource(spec), spec, true, nil
}

// forgeTrackerHint adds the token hint to a 404 or 401, which a private repo returns
// when no token is set.
func forgeTrackerHint(err error, t forgetracker.Target) error {
	if err == nil {
		return nil
	}
	var apiErr *forge.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 404 || apiErr.Status == 401) {
		return fmt.Errorf("%w (%s)", err, privateRepoHint(t.Forge))
	}
	return err
}

// privateRepoHint says how to give harnez a token for a private repo. Forgejo tokens are
// stored by harnez, so the stored-token commands come first; the env vars are the fallback.
func privateRepoHint(fg forge.Forge) string {
	env := strings.Join(fg.TokenEnv, ", ")
	if fg.Kind == "forgejo" {
		return "a private repo needs a token: run harnez forge token import or harnez forge token create, or set one of " + env
	}
	return "a private repo needs a token: set one of " + env
}

// trackerIsNone reports whether .harnez/repo.yaml sets issues.tracker: none.
// A missing repo.yaml is not "none": a fresh repo still adopts a local tracker.
func trackerIsNone(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "repo.yaml")); err != nil {
		return false
	}
	cfg, err := loadIndexRepoConfig(dir)
	return err == nil && cfg != nil && cfg.Issues.Tracker == "none"
}

// issueCLILookPath and issueCLIRun find and run the forge CLI; tests replace them.
var (
	issueCLILookPath = exec.LookPath
	issueCLIRun      = func(dir string, c forgetracker.Command) ([]byte, error) {
		cmd := exec.Command(c.Tool, c.Args...)
		cmd.Dir = dir
		cmd.Stdin = os.Stdin // -F - reads the body from stdin
		return cmd.CombinedOutput()
	}
)

// runIssuesNewForge files a ticket on the forge with its CLI (issue 1195). It makes no
// commit. With dryRun it prints the command and creates nothing.
func runIssuesNewForge(stdout, stderr io.Writer, dir string, t forgetracker.Target, req forgetracker.CreateRequest, dryRun, jsonOut bool) error {
	if req.BodyFile != "" && req.BodyFile != "-" {
		// The CLI runs in dir, so a relative path must be resolved here first.
		abs, err := filepath.Abs(req.BodyFile)
		if err != nil {
			return fmt.Errorf("issues new: body file: %w", err)
		}
		req.BodyFile = abs
		if _, err := os.Stat(req.BodyFile); err != nil {
			return fmt.Errorf("issues new: body file: %w", err)
		}
	}
	c, err := forgetracker.PlanCreate(t, req, issueCLILookPath)
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Fprintf(stdout, "dry run, would file on %s: %s\n", t, c)
		return nil
	}
	fmt.Fprintf(stderr, "issues new: filing on %s with %s\n", t, c.Tool)
	out, err := issueCLIRun(dir, c)
	if err != nil {
		return fmt.Errorf("issues new: %s failed: %w\n%s", c.Tool, err, strings.TrimSpace(string(out)))
	}
	n, url, ok := forgetracker.ParseCreated(t, string(out), req.Title)
	if !ok {
		return fmt.Errorf("issues new: %s ran but printed no ticket number; check %s before retrying\n%s", c.Tool, t, strings.TrimSpace(string(out)))
	}
	if jsonOut {
		return json.NewEncoder(stdout).Encode(map[string]any{"number": n, "url": url, "forge": t.String(), "tool": c.Tool})
	}
	fmt.Fprintf(stdout, "filed as #%d at %s; no commit\n", n, url)
	return nil
}
