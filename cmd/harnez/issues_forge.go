package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetoken"
	"ubunatic.com/harnez/internal/forgetracker"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/prwait"
)

// findIssuesLocalOnly lists `find issues` words that the forge reader does not serve. On a
// forge repo they are refused with forgeLocalOnlyError; closed is not in the set because it
// maps to status:closed (issue 1198), similar scores forge tickets via find.Similar (issue 1196),
// and next reports a forge-assigned status with zero API calls (issue 1237).
var findIssuesLocalOnly = map[string]bool{
	"last": true,
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

// resolveForgeTracker reports the forge that holds dir's tickets.
// ok=false keeps the local tracker behaviour.
func resolveForgeTracker(stderr io.Writer, dir string) (forgetracker.Target, forgetracker.Source, prwait.Spec, bool, error) {
	issuesDir := filepath.Join(dir, "issues")
	_, statErr := os.Stat(issuesDir)
	hasIssues := !errors.Is(statErr, fs.ErrNotExist)
	if hasIssues && (!trackerIsNone(dir) || hasLocalTickets(issuesDir)) {
		return forgetracker.Target{}, nil, prwait.Spec{}, false, nil
	}
	spec, err := prwait.LoadSpec()
	if err != nil {
		return forgetracker.Target{}, nil, spec, false, err
	}
	t, err := forgetracker.ResolveRemote(dir, spec)
	if errors.Is(err, forgetracker.ErrNotApplicable) {
		return t, nil, spec, false, nil
	}
	if err != nil {
		return t, nil, spec, false, err
	}
	if hasIssues {
		fmt.Fprintf(stderr, "issues.tracker is none: tickets live on %s\n", t)
	} else {
		fmt.Fprintf(stderr, "no local issues/ dir: tickets live on %s\n", t)
	}
	src := forgeTrackerSource(spec)
	forgetracker.SetFallbackStderr(src, stderr)
	return t, src, spec, true, nil
}

// hasLocalTickets conservatively retains local mode if the directory cannot be read.
func hasLocalTickets(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "archive" && entry.IsDir() {
			return true
		}
		if !entry.IsDir() && name == "README.md" {
			return true
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		i := 0
		for i < len(name) && name[i] >= '0' && name[i] <= '9' {
			i++
		}
		if i > 0 && i < len(name) && name[i] == '-' {
			return true
		}
	}
	return false
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

func fjLoginHint(host string) string {
	return fmt.Sprintf(`fj login for %s failed; run "fj auth login" if your fj wrapper sets the host, otherwise "fj -H %s auth login"`, host, host)
}

func requireHostSession(verb string) error {
	if callerAgentRole() != "" || (os.Getenv(agentSessionEnv) != "" && interactiveCodexHost() == "") {
		return fmt.Errorf("issues %s: forge writes run only in a host session (this is harnez worker %s); hand the write to the host", verb, os.Getenv(agentSessionEnv))
	}
	return nil
}

var issueWriteLookup = forgetoken.LookupWrite
var issueWriteClient = func(timeout time.Duration) *http.Client { return &http.Client{Timeout: timeout} }
var issueWriteMetadata = func(host string) (bool, error) {
	_, store, err := forgeTokenStoreFor(true)
	if err != nil {
		return false, err
	}
	_, ok, err := store.Meta(host)
	return ok, err
}

func issueWriteAPI(t forgetracker.Target) (forge.Writer, bool, error) {
	sp, err := prwait.LoadSpec()
	if err != nil {
		return forge.Writer{}, false, err
	}
	client := issueWriteClient(sp.IssueScan.Timeout)
	w, ok := forgetracker.WriteAPI(t, client, issueWriteLookup)
	return w, ok, nil
}

// runIssuesComment checks the issue before invoking fj with an explicit target.
func runIssuesComment(w io.Writer, dir string, t forgetracker.Target, src forgetracker.Source, number, body string, dryRun, allowClosed bool) error {
	if t.Forge.Kind != "forgejo" {
		return fmt.Errorf("issues comment: forge writes support Forgejo only; GitHub is ticket 1209")
	}
	n, err := strconv.Atoi(number)
	if err != nil || n <= 0 {
		return fmt.Errorf("issues comment: invalid issue number %q", number)
	}
	if body == "-" {
		return fmt.Errorf("issues comment: -F must name a body file")
	}
	body, err = filepath.Abs(body)
	if err != nil {
		return fmt.Errorf("issues comment: body file: %w", err)
	}
	if info, err := os.Stat(body); err != nil {
		return fmt.Errorf("issues comment: body file: %w", err)
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("issues comment: body file must be a regular file")
	}
	content, err := os.ReadFile(body)
	if err != nil {
		return fmt.Errorf("issues comment: body file: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("issues comment: body file is empty or whitespace-only")
	}
	if !dryRun {
		if err := requireHostSession("comment"); err != nil {
			return err
		}
	}
	is, err := src.Issue(t.Forge, t.Owner, t.Repo, n)
	if err != nil {
		var apiErr *forge.APIError
		if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403 || apiErr.Status == 404) {
			return fmt.Errorf("issues comment: preflight: %w; if the API requires authentication, %s; %s", err, privateRepoHint(t.Forge), fjLoginHint(t.Forge.Host))
		}
		return fmt.Errorf("issues comment: preflight: %w", err)
	}
	target := fmt.Sprintf("%s/%s#%d", t.Owner, t.Repo, n)
	if is.PullRequest != nil {
		return fmt.Errorf("issues comment: %s is a pull request", target)
	}
	isOpen := strings.EqualFold(is.State, "open")
	closedAllowed := allowClosed && strings.EqualFold(is.State, "closed")
	if !isOpen && !closedAllowed {
		return fmt.Errorf("issues comment: %s has state %q; closed issues require --closed", target, is.State)
	}
	c := forgetracker.FJCommentCommand(t, n, body)
	if dryRun {
		stored, err := issueWriteMetadata(t.Forge.Host)
		if err != nil {
			return err
		}
		if stored {
			_, err = fmt.Fprintf(w, "dry run: would comment on %s/%s over the API (write token)\n", t.Forge.Host, target)
			return err
		}
		_, err = fmt.Fprintf(w, "dry run: %s\n", c)
		return err
	}
	wr, useAPI, err := issueWriteAPI(t)
	if err != nil {
		return err
	}
	if useAPI {
		url, err := wr.Comment(t.Forge, t.Owner, t.Repo, n, string(content))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "commented on %s/%s: %s\n", t.Forge.Host, target, url)
		return err
	}
	if _, err := issueCLILookPath("fj"); err != nil {
		return fmt.Errorf("issues comment: fj not found: %w", err)
	}
	if _, err := fmt.Fprintf(w, "commenting on %s %s: %s\n", t.Forge.Host, target, is.Title); err != nil {
		return err
	}
	out, err := issueCLIRun(dir, c)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "token was already used") || (strings.Contains(lower, "keys.rs") && strings.Contains(lower, "panic")) {
			return fmt.Errorf("issues comment: %s", fjLoginHint(t.Forge.Host))
		}
		if strings.Contains(strings.ToLower(msg), "401") || strings.Contains(strings.ToLower(msg), "token is required") {
			return fmt.Errorf("issues comment: %s", fjLoginHint(t.Forge.Host))
		}
		return fmt.Errorf("issues comment: fj failed: %w: %s\nrun manually: %s", err, msg, c)
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "commented on %s#%d\n", t, n)
	return err
}

const noTokenDiggingDirective = "Do NOT inspect, debug, or modify Forgejo/Codeberg/GitHub token setup, credentials, env vars, or CLI auth"

func warnNoTokenDigging(stderr io.Writer, detail string) {
	if stderr == nil {
		return
	}
	if detail != "" {
		fmt.Fprintf(stderr, "WARNING: %s. %s; use issues/inbox/ on a branch instead.\n", detail, noTokenDiggingDirective)
		return
	}
	fmt.Fprintf(stderr, "WARNING: %s. Ask the owner if host credentials need attention, or file new issues in issues/inbox/ on a branch instead.\n", noTokenDiggingDirective)
}

func isForgeAuthOrWriteErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *forge.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403 || apiErr.Status == 404) {
		return true
	}
	lower := strings.ToLower(err.Error())
	for _, needle := range []string{
		"no write token",
		"forge token",
		"auth login",
		"fj failed",
		"fj not found",
		"tea failed",
		"gh failed",
		"token was already used",
		"token is required",
		"private repo needs a token",
		"forge writes run only in a host session",
		"unauthorized",
		"forbidden",
		"permission denied",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func warnIfForgeAuthErr(stderr io.Writer, err error) error {
	if isForgeAuthOrWriteErr(err) {
		warnNoTokenDigging(stderr, "")
	}
	return err
}

func runFindNextForge(stdout io.Writer, t forgetracker.Target, jsonOut bool) error {
	if jsonOut {
		data, err := json.Marshal(find.NextResultJSON{
			Number:        "",
			Reserved:      false,
			ForgeAssigned: true,
			Forge:         t.String(),
		})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	_, err := fmt.Fprintf(stdout, "forge-assigned on %s (assigned by forge on harnez issues new)\n", t)
	return err
}

var forgeReadTokenFor = func(fg forge.Forge) string {
	return forge.SystemTokenSource(os.Getenv).For(fg)
}

func handleForgeFindErr(cmd *cobra.Command, err error, t forgetracker.Target) error {
	if err == nil {
		return nil
	}
	var apiErr *forge.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403 || apiErr.Status == 404) {
		if strings.TrimSpace(forgeReadTokenFor(t.Forge)) != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "search blocked on %s (HTTP %d; read token rejected or repo inaccessible)\n", t, apiErr.Status)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "search blocked on %s (private repo; no read token or fj login)\n", t)
		}
		warnNoTokenDigging(cmd.ErrOrStderr(), "")
		return silenceIfExitCode(cmd, &exitCodeError{Code: 2})
	}
	return warnIfForgeAuthErr(cmd.ErrOrStderr(), forgeTrackerHint(err, t))
}

func hasFJCLI(t forgetracker.Target) bool {
	return slices.Contains(t.Forge.IssueCLI, "fj")
}

func warnInboxFJLabels(stderr io.Writer, t forgetracker.Target, labels []string) {
	if stderr == nil || len(labels) == 0 || !hasFJCLI(t) {
		return
	}
	fmt.Fprintf(stderr, "WARNING: labels (%s) were saved in the draft only; fj cannot set labels — apply via write token, tea, or forge UI when filing.\n", strings.Join(labels, ", "))
}

func inboxRelPath(slug string, n int) string {
	name := slug + ".md"
	if n > 1 {
		name = fmt.Sprintf("%s-%d.md", slug, n)
	}
	return filepath.ToSlash(filepath.Join("issues", "inbox", name))
}

func formatInboxIssue(title, body string, labels []string, warnFJLabels bool) []byte {
	var b strings.Builder
	trimmed := strings.TrimSpace(body)
	if !strings.HasPrefix(trimmed, "# ") {
		b.WriteString("# ")
		b.WriteString(strings.TrimSpace(title))
		b.WriteString("\n\n")
	}
	if len(labels) > 0 {
		b.WriteString("**Labels**: ")
		b.WriteString(strings.Join(labels, ", "))
		b.WriteByte('\n')
		if warnFJLabels {
			b.WriteString("> Note: fj cannot set labels; apply via write token, tea, or forge UI when filing.\n")
		}
		b.WriteByte('\n')
	}
	b.WriteString(trimmed)
	b.WriteByte('\n')
	return []byte(b.String())
}

func nextInboxRelPath(dir, slug string) string {
	for n := 1; n <= 1000; n++ {
		rel := inboxRelPath(slug, n)
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); os.IsNotExist(err) {
			return rel
		}
	}
	return inboxRelPath(slug, 1001)
}

func writeInboxPayload(dir, slug string, payload []byte) (string, error) {
	inboxDir := filepath.Join(dir, "issues", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		return "", fmt.Errorf("issues new: create inbox dir: %w", err)
	}
	for n := 1; ; n++ {
		rel := inboxRelPath(slug, n)
		full := filepath.Join(dir, filepath.FromSlash(rel))
		f, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("issues new: write %s: %w", rel, err)
		}
		_, writeErr := f.Write(payload)
		closeErr := f.Close()
		if writeErr != nil {
			return "", fmt.Errorf("issues new: write %s: %w", rel, writeErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("issues new: close %s: %w", rel, closeErr)
		}
		return rel, nil
	}
}

func writeInboxIssue(dir, title, body string, labels []string, warnFJLabels bool) (string, error) {
	slug := issues.Slugify(title)
	if slug == "" {
		slug = "issue"
	}
	return writeInboxPayload(dir, slug, formatInboxIssue(title, body, labels, warnFJLabels))
}

type forgeNewOptions struct {
	Dir           string
	Target        forgetracker.Target
	Source        forgetracker.Source
	Scan          forge.ScanOptions
	Req           forgetracker.CreateRequest
	ExplicitForge bool
	Force         bool
	DryRun        bool
	JSON          bool
}

func printForgeTruncatedNote(stderr io.Writer, opts forge.ScanOptions, fetched int) {
	if stderr == nil {
		return
	}
	count := fetched
	if count <= 0 {
		count = opts.PageSize * opts.MaxPages
	}
	fmt.Fprintf(stderr, "checked the newest %d forge issues; older issues were not fetched\n", count)
}

func runFindSimilarForge(stdout, stderr io.Writer, src forgetracker.Source, t forgetracker.Target, opts forge.ScanOptions, query string, jsonOut bool) error {
	files, truncated, err := forgetracker.FetchIssueFiles(src, t, opts)
	if err != nil {
		return fmt.Errorf("find issues similar: %s: %w", t, err)
	}
	if truncated {
		printForgeTruncatedNote(stderr, opts, len(files))
	}
	results, err := find.Similar(files, query)
	if err != nil {
		return err
	}
	return find.RenderSimilar(stdout, results, jsonOut)
}

// checkForgeNewDuplicates scans forge tickets for duplicates before dry-run or
// creation. CLI entrypoints always set opt.Source; a nil Source is only valid
// for direct unit tests that intentionally omit forge reads.
func checkForgeNewDuplicates(stderr io.Writer, opt forgeNewOptions) error {
	if opt.Source == nil || (!opt.DryRun && opt.Force) {
		return nil
	}
	files, truncated, err := forgetracker.FetchIssueFiles(opt.Source, opt.Target, opt.Scan)
	if err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "issues new: duplicate check skipped on %s (%v)\n", opt.Target, forgeTrackerHint(err, opt.Target))
		}
		return nil
	}
	if truncated {
		printForgeTruncatedNote(stderr, opt.Scan, len(files))
	}
	matches, threshold, err := findIssueDuplicatesFromFiles(files, opt.Req.Title)
	if err != nil {
		return err
	}
	dupOpts := issues.DuplicateCheckOptions{Forge: true, ExplicitForge: opt.ExplicitForge}
	if !opt.Force {
		if dupErr := issues.DuplicateErrorWithOptions(opt.Req.Title, matches, threshold, dupOpts); dupErr != nil {
			return dupErr
		}
	}
	if opt.DryRun && len(matches) > 0 && stderr != nil {
		fmt.Fprintf(stderr, "issues new: similar forge ticket(s) found (strong-match threshold %d%%):\n", threshold)
		for _, m := range matches {
			strength := ""
			if m.Strong {
				strength = ", strong"
			}
			fmt.Fprintf(stderr, "  #%s %s — %s (score %.3f%s)\n", m.Number, m.RawStatus, m.PlainTitle, m.Score, strength)
		}
	}
	return nil
}

// runIssuesNewForge files a ticket on the forge via API or CLI (issue 1195),
// and falls back to issues/inbox/<slug>.md on forge/auth/worker failure. It
// makes no commit. With DryRun it prints the plan and creates nothing.
func runIssuesNewForge(stdout, stderr io.Writer, opt forgeNewOptions) error {
	dir, t, req, dryRun, jsonOut := opt.Dir, opt.Target, opt.Req, opt.DryRun, opt.JSON
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("issues new: a title is required to file on %s", t)
	}
	if req.BodyFile == "" {
		return fmt.Errorf("issues new: -F/--body-file is required to file on %s", t)
	}
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
	var content []byte
	var err error
	if req.BodyFile == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(req.BodyFile)
	}
	if err != nil {
		return fmt.Errorf("issues new: body file: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("issues new: body file is empty or whitespace-only")
	}
	opt.Req = req
	if err := checkForgeNewDuplicates(stderr, opt); err != nil {
		return err
	}
	if dryRun {
		if t.Forge.Kind == "forgejo" {
			stored, err := issueWriteMetadata(t.Forge.Host)
			if err != nil {
				return err
			}
			if stored {
				_, err = fmt.Fprintf(stdout, "dry run, would file on %s over the API (write token)\n", t)
				return err
			}
		}
		c, err := forgetracker.PlanCreate(t, req, issueCLILookPath)
		if err != nil {
			slug := issues.Slugify(req.Title)
			if slug == "" {
				slug = "issue"
			}
			warnNoTokenDigging(stderr, fmt.Sprintf("forge CLI unavailable (%v)", err))
			warnInboxFJLabels(stderr, t, req.Labels)
			_, werr := fmt.Fprintf(stdout, "dry run, would file in inbox at %s (forge fallback; no commit)\n", inboxRelPath(slug, 1))
			return werr
		}
		_, err = fmt.Fprintf(stdout, "dry run, would file on %s: %s\n", t, c)
		return err
	}
	handled, err := tryCreateOnForge(forgeCreateCall{stdout: stdout, stderr: stderr, dir: dir, target: t, req: req, content: content, jsonOut: jsonOut})
	if handled {
		return err
	}
	rel, inboxErr := writeInboxIssue(dir, req.Title, string(content), req.Labels, hasFJCLI(t))
	if inboxErr != nil {
		return fmt.Errorf("%w (after forge failure: %v)", inboxErr, err)
	}
	warnNoTokenDigging(stderr, fmt.Sprintf("forge filing on %s failed (%v); filed in %s instead — commit and push it on a branch/PR", t, err, rel))
	warnInboxFJLabels(stderr, t, req.Labels)
	if jsonOut {
		return json.NewEncoder(stdout).Encode(map[string]any{
			"path":   rel,
			"inbox":  true,
			"forge":  t.String(),
			"tool":   "inbox",
			"reason": err.Error(),
		})
	}
	_, werr := fmt.Fprintf(stdout, "filed in inbox at %s (forge fallback; no commit)\n", rel)
	return werr
}

type forgeCreateCall struct {
	stdout  io.Writer
	stderr  io.Writer
	dir     string
	target  forgetracker.Target
	req     forgetracker.CreateRequest
	content []byte
	jsonOut bool
}

// tryCreateOnForge attempts to create the issue via the write API or forge CLI.
// handled=false means no ticket was created and the caller should fall back to
// issues/inbox/. handled=true preserves non-auth errors (unknown labels or a CLI
// that exited 0 without printing a ticket number).
func tryCreateOnForge(c forgeCreateCall) (bool, error) {
	if err := requireHostSession("new"); err != nil {
		return false, err
	}
	wr, useAPI, err := issueWriteAPI(c.target)
	if err != nil {
		return false, err
	}
	if useAPI {
		var ids []int64
		if len(c.req.Labels) > 0 {
			ids, err = wr.LabelIDs(c.target.Forge, c.target.Owner, c.target.Repo, c.req.Labels)
			if err != nil {
				return strings.HasPrefix(err.Error(), "unknown label "), err
			}
		}
		created, err := wr.CreateIssue(c.target.Forge, c.target.Owner, c.target.Repo, c.req.Title, string(c.content), ids)
		if err != nil {
			return false, err
		}
		if c.jsonOut {
			return true, json.NewEncoder(c.stdout).Encode(map[string]any{"number": created.Number, "url": created.HTMLURL, "forge": c.target.String(), "tool": "api"})
		}
		_, err = fmt.Fprintf(c.stdout, "filed as #%d at %s; no commit\n", created.Number, created.HTMLURL)
		return true, err
	}
	req := c.req
	if req.BodyFile == "-" {
		f, err := os.CreateTemp("", "harnez-issue-body-*")
		if err != nil {
			return false, fmt.Errorf("issues new: temporary body: %w", err)
		}
		defer func() { _ = os.Remove(f.Name()) }()
		if _, err := f.Write(c.content); err != nil {
			f.Close()
			return false, fmt.Errorf("issues new: temporary body: %w", err)
		}
		if err := f.Close(); err != nil {
			return false, fmt.Errorf("issues new: temporary body: %w", err)
		}
		req.BodyFile = f.Name()
	}
	cmd, err := forgetracker.PlanCreate(c.target, req, issueCLILookPath)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(c.stderr, "issues new: filing on %s with %s\n", c.target, cmd.Tool)
	out, err := issueCLIRun(c.dir, cmd)
	if err != nil {
		return false, fmt.Errorf("%s failed: %w: %s", cmd.Tool, err, strings.TrimSpace(string(out)))
	}
	n, url, ok := forgetracker.ParseCreated(c.target, string(out), req.Title)
	if !ok {
		return true, fmt.Errorf("issues new: %s ran but printed no ticket number; check %s before retrying\n%s", cmd.Tool, c.target, strings.TrimSpace(string(out)))
	}
	if c.jsonOut {
		return true, json.NewEncoder(c.stdout).Encode(map[string]any{"number": n, "url": url, "forge": c.target.String(), "tool": cmd.Tool})
	}
	_, err = fmt.Fprintf(c.stdout, "filed as #%d at %s; no commit\n", n, url)
	return true, err
}

func runIssuesEditForge(w io.Writer, t forgetracker.Target, src forgetracker.Source, number, title, bodyFile, ifUpdated string, hasTitle, force, dryRun, allowClosed bool) error {
	if t.Forge.Kind != "forgejo" {
		return fmt.Errorf("issues edit: forge writes support Forgejo only; GitHub is ticket 1209")
	}
	n, err := strconv.Atoi(number)
	if err != nil || n <= 0 {
		return fmt.Errorf("issues edit: invalid issue number %q", number)
	}
	if !hasTitle && bodyFile == "" {
		return fmt.Errorf("issues edit: --title or -F/--body-file is required")
	}
	if hasTitle && strings.TrimSpace(title) == "" {
		return fmt.Errorf("issues edit: title is empty")
	}
	var body string
	if bodyFile != "" {
		if ifUpdated == "" && !force {
			return fmt.Errorf("issues edit: body edits need --if-updated <updated_at from issues show --json> or --force")
		}
		var data []byte
		if bodyFile == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(bodyFile)
		}
		if err != nil {
			return fmt.Errorf("issues edit: body file: %w", err)
		}
		body = string(data)
		if strings.TrimSpace(body) == "" {
			return fmt.Errorf("issues edit: body file is empty or whitespace-only")
		}
	}
	if dryRun {
		plan := fmt.Sprintf("dry run: would edit %s#%d over the API", t, n)
		if hasTitle {
			plan += " --title " + strconv.Quote(title)
		}
		if bodyFile != "" {
			plan += " -F " + strconv.Quote(bodyFile)
		}
		if ifUpdated != "" {
			plan += " --if-updated " + strconv.Quote(ifUpdated)
		}
		if force {
			plan += " --force"
		}
		if allowClosed {
			plan += " --closed"
		}
		_, err = fmt.Fprintln(w, plan)
		return err
	}
	if err := requireHostSession("edit"); err != nil {
		return err
	}
	wr, ok, err := issueWriteAPI(t)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no write token for %s; run: harnez forge token create --write", t.Forge.Host)
	}
	is, err := src.Issue(t.Forge, t.Owner, t.Repo, n)
	if err != nil {
		return fmt.Errorf("issues edit: preflight: %w", err)
	}
	if is.PullRequest != nil {
		return fmt.Errorf("issues edit: %s/%s#%d is a pull request", t.Owner, t.Repo, n)
	}
	if strings.EqualFold(is.State, "closed") && !allowClosed {
		return fmt.Errorf("issues edit: %s/%s#%d is closed; use --closed", t.Owner, t.Repo, n)
	}
	if ifUpdated != "" && !force {
		expected, err := time.Parse(time.RFC3339Nano, ifUpdated)
		if err != nil {
			return fmt.Errorf("issues edit: invalid --if-updated timestamp %q", ifUpdated)
		}
		if !is.UpdatedAt.Equal(expected) {
			return fmt.Errorf("issues edit: %s/%s#%d changed at %s, expected %s; run issues show again", t.Owner, t.Repo, n, is.UpdatedAt.Format(time.RFC3339Nano), ifUpdated)
		}
	}
	fields := forge.EditFields{}
	if hasTitle {
		fields.Title = &title
	}
	if bodyFile != "" {
		fields.Body = &body
	}
	updated, err := wr.EditIssue(t.Forge, t.Owner, t.Repo, n, fields)
	if err != nil {
		return err
	}
	if fields.Body != nil && updated.Body != body {
		return fmt.Errorf("issues edit: forge returned a different body")
	}
	_, err = fmt.Fprintf(w, "edited %s#%d\n", t, n)
	return err
}

func runIssuesStateForge(w io.Writer, t forgetracker.Target, src forgetracker.Source, verb, number, reason string, dryRun bool) error {
	if t.Forge.Kind != "forgejo" {
		return fmt.Errorf("issues %s: forge writes support Forgejo only; GitHub is ticket 1209", verb)
	}
	n, err := strconv.Atoi(number)
	if err != nil || n <= 0 {
		return fmt.Errorf("issues %s: invalid issue number %q", verb, number)
	}
	state := "closed"
	if verb == "reopen" {
		state = "open"
	}
	if dryRun {
		if reason != "" {
			_, err = fmt.Fprintf(w, "dry run: would comment %q on %s#%d and set state to %s over the API\n", reason, t, n, state)
		} else {
			_, err = fmt.Fprintf(w, "dry run: would set %s#%d to %s over the API\n", t, n, state)
		}
		return err
	}
	if err := requireHostSession(verb); err != nil {
		return err
	}
	wr, ok, err := issueWriteAPI(t)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no write token for %s; run: harnez forge token create --write", t.Forge.Host)
	}
	is, err := src.Issue(t.Forge, t.Owner, t.Repo, n)
	if err != nil {
		return fmt.Errorf("issues %s: preflight: %w", verb, err)
	}
	if is.PullRequest != nil {
		return fmt.Errorf("issues %s: %s/%s#%d is a pull request", verb, t.Owner, t.Repo, n)
	}
	if strings.EqualFold(is.State, state) {
		_, err = fmt.Fprintf(w, "%s#%d is already %s\n", t, n, state)
		return err
	}
	commentURL := ""
	if reason != "" {
		commentURL, err = wr.Comment(t.Forge, t.Owner, t.Repo, n, reason)
		if err != nil {
			return err
		}
	}
	if _, err := wr.EditIssue(t.Forge, t.Owner, t.Repo, n, forge.EditFields{State: &state}); err != nil {
		if reason != "" {
			return fmt.Errorf("issues close: the reason was posted as a comment (%s) but closing failed: %w; run close again without a reason", commentURL, err)
		}
		return err
	}
	word := "closed"
	if verb == "reopen" {
		word = "reopened"
	}
	_, err = fmt.Fprintf(w, "%s %s#%d\n", word, t, n)
	return err
}
