// issues implements `harnez issues <verb> <ticket-number> [reason...]`, a
// single-call replacement for the four-step hand-edit + `harnez index` +
// `git add` + `git commit` dance an agent otherwise has to perform for
// every ticket status change. See
// issues/232-harnez-issues-verb-command-for-single-call-status-changes-with-index-sync-and-commit.md
// for the full design.
//
// This is a command group sibling to `find` (read-only query) and `index`
// (index resync only) -- not a subcommand of either. `find` stays a pure
// query surface; `issues` is its write-side counterpart, and always
// resyncs issues/README.md and commits by default, mirroring
// docs/IssueTracking.md's Lifecycle Invariants 2 (Atomic Index
// Synchronization) and 3 (Immediate Tracker Commit).
//
// `issues new [title]` is the one exception to the resync-and-commit
// default: it atomically reserves the next free ticket number and creates a
// Draft placeholder file (internal/issues.Reserve), the same mutation
// `harnez find issues next --reserve` used to perform before issue 233 moved
// it here to keep `find` a pure query surface. Like the old --reserve path,
// it never commits -- there's nothing yet worth resyncing the README for
// until the placeholder has real content.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/index"
	"ubunatic.com/harnez/internal/issues"
)

// issuesResult is the structured shape `harnez issues <verb>` reports, both
// as --json output and as the basis of its single deterministic text line,
// matching the exact field shape issue 232 §3 specifies.
type issuesResult struct {
	Number        string `json:"number"`
	File          string `json:"file"`
	OldStatus     string `json:"old_status"`
	NewStatus     string `json:"new_status"`
	ReadmeUpdated bool   `json:"readme_updated"`
	Committed     bool   `json:"committed"`
	CommitSHA     string `json:"commit_sha"`
	Noop          bool   `json:"noop"`
}

// issuesRunOptions bundles runIssuesVerb's inputs.
type issuesRunOptions struct {
	Dir       string
	Check     bool
	JSON      bool
	NoCommit  bool
	CommitMsg string // explicit --commit override; empty means "compose the default"
}

func newIssuesCmd() *cobra.Command {
	var dir string
	var checkFlag bool
	var dryRunFlag bool
	var jsonFlag bool
	var noCommitFlag bool
	var commitMsgFlag string

	cmd := &cobra.Command{
		Use:   "issues <verb> <ticket-number> [reason...]",
		Short: "Change a ticket's Status line, resync issues/README.md, and commit -- in one call",
		Long: `issues collapses today's four-step ticket-status-change dance (hand-edit the
'**Status**:' line, run 'harnez index', 'git add' the changed files, 'git
commit') into one call: it rewrites the ticket's Status line, regenerates
issues/README.md in-process (the same logic 'harnez index' uses), and
commits both files by default.

Verbs (closed set, mirroring docs/IssueTracking.md's Allowed Values):

  open [reason]    Status: Open, or "Open — <reason>"
  start [reason]   Status: In Progress, or "In Progress — <reason>"
  block <reason>   Status: "Blocked — <reason>" (reason required)
  close [reason]   Status: Closed (bare), or "Closed — <reason>"
  draft [reason]   Status: Draft, or "Draft — <reason>"
  new [title]      Atomically reserve the next free issue number and create
                    a Draft placeholder file (issues/<NNN>-<title-slug>.md,
                    or issues/<NNN>-reserved.md with no title), using
                    O_CREATE|O_EXCL so concurrent callers never collide. Non-
                    JSON output is "<NUMBER>\t<PATH>" -- write the ticket's
                    real content directly to that printed path instead of
                    re-deriving the slug from the title by hand (a
                    hand-derived slug can diverge and leave an orphaned
                    placeholder behind, see issue 202). Unlike every other
                    verb, 'new' takes no ticket number (there isn't one yet)
                    and never commits.

'close' with no reason writes bare "Closed", never an auto-fabricated
"Closed — resolved" -- both are common in the corpus and this command does
not guess which the caller means. It also never guesses a commit sha for a
"resolved in <sha>" reason (that sha does not exist until this command's own
commit is made) -- supply it explicitly, or correct it in a small follow-up
call.

Idempotency: re-running a verb that would produce the exact same Status
line text already on disk is a no-op -- exit 0, no file/README/commit
changes. Changing the reason text on an already-matching status (e.g.
correcting a placeholder commit sha) is a real, non-idempotent action and
proceeds normally.

--check (alias --dry-run) reports what would change without writing or
committing, and exits 1 if there is drift from the requested state (mirrors
'harnez index --check').

A nonexistent or ambiguous ticket number is a caller-bug error (non-zero
exit, actionable stderr) -- unlike 'harnez find', where zero matches is a
valid, exit-0 answer.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("issues: requires a verb (open, start, block, close, draft, new)")
			}
			if args[0] == "new" {
				return nil // [title] is optional, no ticket-number argument exists yet
			}
			return cobra.MinimumNArgs(2)(cmd, args)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "new" {
				title := strings.TrimSpace(strings.Join(args[1:], " "))
				return runIssuesNew(cmd.OutOrStdout(), dir, title, jsonFlag)
			}
			opts := issuesRunOptions{
				Dir:       dir,
				Check:     checkFlag || dryRunFlag,
				JSON:      jsonFlag,
				NoCommit:  noCommitFlag,
				CommitMsg: commitMsgFlag,
			}
			result, drift, err := runIssuesVerb(cmd.OutOrStdout(), args[0], args[1], args[2:], opts)
			if err != nil {
				return err
			}
			printIssuesResult(cmd.OutOrStdout(), result, opts)
			if opts.Check && drift {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "report what would change without writing or committing; exit 1 on drift")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "alias for --check")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output a single JSON object instead of a text line")
	cmd.Flags().BoolVar(&noCommitFlag, "no-commit", false, "rewrite the ticket file and README but do not git add/commit")
	cmd.Flags().StringVar(&commitMsgFlag, "commit", "", `override the default commit message (default: "docs(issues): <verb> <ticket-number>[, <reason>]")`)

	return cmd
}

// composeNewStatus renders the "**Status**:" value a verb+reason pair
// should produce, per docs/IssueTracking.md's Allowed Values and issue
// 232 §2's verb table. It intentionally never fabricates a reason (e.g.
// "resolved" for a bare 'close', or a commit sha) -- reason text always
// comes verbatim from the caller.
func composeNewStatus(verb, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	switch verb {
	case "open":
		if reason == "" {
			return "Open", nil
		}
		return "Open — " + reason, nil
	case "start":
		if reason == "" {
			return "In Progress", nil
		}
		return "In Progress — " + reason, nil
	case "block":
		if reason == "" {
			return "", fmt.Errorf(`issues block: reason is required, e.g. 'harnez issues block <number> "waiting on upstream fix"'`)
		}
		return "Blocked — " + reason, nil
	case "close":
		if reason == "" {
			return "Closed", nil
		}
		return "Closed — " + reason, nil
	case "draft":
		if reason == "" {
			return "Draft", nil
		}
		return "Draft — " + reason, nil
	default:
		return "", fmt.Errorf("issues: unknown verb %q (expected one of: open, start, block, close, draft)", verb)
	}
}

// runIssuesNew implements `harnez issues new [title]`: atomically reserve
// the next free ticket number and create a Draft placeholder file, using
// the exact internal/issues.Reserve call and output contract
// `harnez find issues next --reserve` used before issue 233 moved the
// mutation out of `find`. It intentionally does not require (or accept) a
// ticket number -- there isn't one until this call allocates it -- and never
// touches issues/README.md or git, unlike every other `issues` verb.
func runIssuesNew(w io.Writer, dir, title string, jsonOutput bool) error {
	issuesDir := filepath.Join(dir, "issues")
	num, filename, err := issues.Reserve(issuesDir, issues.ReserveOptions{Title: title})
	if err != nil {
		return fmt.Errorf("issues new: %w", err)
	}
	relPath := filepath.ToSlash(filepath.Join("issues", filename))
	if jsonOutput {
		data, err := json.Marshal(nextResultJSON{
			Number:   num,
			Reserved: true,
			File:     filename,
			Path:     relPath,
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}
	fmt.Fprintf(w, "%s\t%s\n", num, relPath)
	return nil
}

// findTicketFile locates the single issues/*.md (or issues/archive/*.md)
// file whose Number matches ticketArg (accepting "232", "32", or "032"
// alike, normalized to the same %03d width Scan/ParseTrackerTable use).
// A nonexistent or ambiguous number is a caller-bug error, per issue 232
// §3's "fail loudly, unlike find's zero-matches-is-valid convention".
func findTicketFile(issuesDir, ticketArg string) (issues.IssueFile, error) {
	n, err := strconv.Atoi(strings.TrimSpace(ticketArg))
	if err != nil || n < 0 {
		return issues.IssueFile{}, fmt.Errorf("issues: invalid ticket number %q (expected a non-negative integer)", ticketArg)
	}
	ticketNum := fmt.Sprintf("%03d", n)

	files, err := issues.Scan(issuesDir)
	if err != nil {
		return issues.IssueFile{}, fmt.Errorf("issues: %w", err)
	}

	var matches []issues.IssueFile
	for _, f := range files {
		if f.Number == ticketNum {
			matches = append(matches, f)
		}
	}
	switch len(matches) {
	case 0:
		return issues.IssueFile{}, fmt.Errorf("issues: no ticket found for number %q under %s (checked issues/*.md and issues/archive/*.md)", ticketNum, issuesDir)
	case 1:
		return matches[0], nil
	default:
		var paths []string
		for _, m := range matches {
			paths = append(paths, m.RelPath)
		}
		return issues.IssueFile{}, fmt.Errorf("issues: ambiguous ticket number %q matches multiple files: %s", ticketNum, strings.Join(paths, ", "))
	}
}

// runIssuesVerb performs the full status-change: locate the ticket,
// compose the new Status line, rewrite the file (or simulate the rewrite
// under --check), resync issues/README.md, and commit (unless --no-commit
// or --check). It returns the structured result plus whether the
// requested state would represent drift from what's on disk today (only
// meaningful for --check; the caller decides the process exit code on it,
// since this function is also exercised directly by tests).
func runIssuesVerb(w io.Writer, verb, ticketArg string, reasonArgs []string, opts issuesRunOptions) (issuesResult, bool, error) {
	reason := strings.TrimSpace(strings.Join(reasonArgs, " "))
	newStatus, err := composeNewStatus(verb, reason)
	if err != nil {
		return issuesResult{}, false, err
	}

	issuesDir := filepath.Join(opts.Dir, "issues")
	f, err := findTicketFile(issuesDir, ticketArg)
	if err != nil {
		return issuesResult{}, false, err
	}
	filePath := filepath.Join(issuesDir, f.RelPath)
	relPath := filepath.ToSlash(filepath.Join("issues", f.RelPath))

	content, err := os.ReadFile(filePath)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues: read %s: %w", filePath, err)
	}
	_, oldStatus, hasStatus := issues.ParseIssueFile(string(content))
	if !hasStatus {
		return issuesResult{}, false, fmt.Errorf("issues: %s has no '**Status**:' line to update", relPath)
	}

	result := issuesResult{
		Number:    f.Number,
		File:      relPath,
		OldStatus: oldStatus,
		NewStatus: newStatus,
	}

	if oldStatus == newStatus {
		// Idempotent no-op (issue 232 §3): exact status+reason text already
		// matches -- skip the rewrite, README resync, and commit entirely.
		result.Noop = true
		return result, false, nil
	}

	readmePath := filepath.Join(issuesDir, "README.md")
	newContent, _, err := issues.RewriteStatus(string(content), newStatus)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues: %w", err)
	}

	if opts.Check {
		origReadme, readErr := os.ReadFile(readmePath)
		readmeExisted := readErr == nil
		if readErr != nil && !os.IsNotExist(readErr) {
			return issuesResult{}, false, fmt.Errorf("issues --check: read %s: %w", readmePath, readErr)
		}

		if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
			return issuesResult{}, false, fmt.Errorf("issues --check: write %s: %w", filePath, err)
		}
		readmeChanged, updErr := index.UpdateIssuesReadme(readmePath, issuesDir)

		// Always restore, regardless of updErr, so --check never leaves a
		// side effect on disk (mirrors runIndexCheck's restore pattern).
		if werr := os.WriteFile(filePath, content, 0o644); werr != nil {
			return issuesResult{}, false, fmt.Errorf("issues --check: restore %s: %w", filePath, werr)
		}
		if readmeExisted {
			os.WriteFile(readmePath, origReadme, 0o644)
		} else {
			os.Remove(readmePath)
		}
		if updErr != nil {
			return issuesResult{}, false, fmt.Errorf("issues --check: %w", updErr)
		}

		result.ReadmeUpdated = readmeChanged
		return result, true, nil
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
		return issuesResult{}, false, fmt.Errorf("issues: write %s: %w", filePath, err)
	}
	readmeChanged, err := index.UpdateIssuesReadme(readmePath, issuesDir)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues: %w", err)
	}
	result.ReadmeUpdated = readmeChanged

	if !opts.NoCommit {
		msg := opts.CommitMsg
		if msg == "" {
			msg = defaultCommitMessage(verb, f.Number, reason)
		}
		readmeRelPath := filepath.ToSlash(filepath.Join("issues", "README.md"))
		sha, err := gitAddAndCommit(opts.Dir, []string{relPath, readmeRelPath}, msg)
		if err != nil {
			return issuesResult{}, false, fmt.Errorf("issues: %w", err)
		}
		result.Committed = true
		result.CommitSHA = sha
	}

	return result, false, nil
}

// defaultCommitMessage composes "docs(issues): <verb> <ticket-number>[,
// <reason>]", this repo's actual convention (see e.g. the git log entry
// "docs(issues): close 228, record resolved-in commit sha and regenerate
// index").
func defaultCommitMessage(verb, ticketNumber, reason string) string {
	msg := fmt.Sprintf("docs(issues): %s %s", verb, ticketNumber)
	if reason != "" {
		msg += ", " + reason
	}
	return msg
}

// gitAddAndCommit stages exactly paths and commits them with message,
// returning the resulting commit's short sha. Attribution (Co-Authored-By
// trailers etc.) is deliberately not added here -- this tool is meant to be
// used by any agent or human caller, and attribution is a caller-session
// concern, not something baked into the command (issue 232).
func gitAddAndCommit(dir string, paths []string, message string) (string, error) {
	addCmd := exec.Command("git", append([]string{"add"}, paths...)...)
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add %s: %w: %s", strings.Join(paths, " "), err, strings.TrimSpace(string(out)))
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit: %w: %s", err, strings.TrimSpace(string(out)))
	}

	shaCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	shaCmd.Dir = dir
	out, err := shaCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --short HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// printIssuesResult writes result either as one JSON object (--json) or as
// one deterministic text line, per issue 232 §3's "agent scanning
// transcript output for confirmation should not have to parse a paragraph"
// requirement.
func printIssuesResult(w io.Writer, result issuesResult, opts issuesRunOptions) {
	if opts.JSON {
		data, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(w, `{"error":%q}`+"\n", err.Error())
			return
		}
		fmt.Fprintln(w, string(data))
		return
	}
	fmt.Fprintln(w, formatIssuesLine(result, opts.Check))
}

func formatIssuesLine(r issuesResult, check bool) string {
	if r.Noop {
		return fmt.Sprintf("%s: already %s (no change)", r.Number, r.NewStatus)
	}
	var parts []string
	if check {
		if r.ReadmeUpdated {
			parts = append(parts, "would update README")
		}
		parts = append(parts, "would commit")
	} else {
		if r.ReadmeUpdated {
			parts = append(parts, "README updated")
		}
		if r.Committed {
			parts = append(parts, fmt.Sprintf("committed %s", r.CommitSHA))
		} else {
			parts = append(parts, "not committed")
		}
	}
	return fmt.Sprintf("%s: %s -> %s (%s)", r.Number, r.OldStatus, r.NewStatus, strings.Join(parts, ", "))
}
