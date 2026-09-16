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
	OldNumber     string `json:"old_number,omitempty"`
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
	var cachedFlag bool
	var limitFlag int
	var allFlag bool

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
  done [reason]    Status: Closed (bare), or "Closed — <reason>" (alias for close)
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
  mv <old> [new]   Renumber a ticket to [new] (default: next free number
                    from scanning issuesDir). Renames the file keeping the
                    same slug, rewrites the '# <new> — <title>' header line,
                    resyncs issues/README.md, and commits both files by default.
                    Guarded by O_CREATE|O_EXCL so concurrent claims or existing
                    tickets are never overwritten.
  rebase [upstream] Replay local commits onto upstream, deriving local ticket
                    ownership from history and deterministically repairing any
                    number collisions in an explicit final commit. --dry-run
                    prints the plan without changing Git state.
  lint              Read-only validation for duplicate numbers, conflicting
                    filename/H1 numbers, and generated-index drift.
  list [filter]     Read-only: list tickets matching [filter] (default:
                    "is:open"), a thin wrapper over 'harnez find issues' --
                    not a status-mutation verb, and does not accept
                    --check/--dry-run/--commit/--no-commit. Supports the
                    same -n/--limit/--all flags as 'find', plus the bare
                    git-log-style '-N' shorthand (e.g. 'issues list -3').

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
				return fmt.Errorf("issues: requires a verb (open, start, block, close, done, draft, new, mv, rebase, lint, list)")
			}
			if args[0] == "new" {
				return nil // [title] is optional, no ticket-number argument exists yet
			}
			if args[0] == "list" {
				return nil // [filter] is optional, defaults to "is:open"
			}
			if args[0] == "mv" {
				if len(args) < 2 || len(args) > 3 {
					return fmt.Errorf("issues mv: accepts 1 or 2 arguments: <ticket-number> [new-number]")
				}
				return nil
			}
			if args[0] == "rebase" {
				if len(args) > 2 {
					return fmt.Errorf("issues rebase: accepts at most one argument: [upstream]")
				}
				return nil
			}
			if args[0] == "lint" {
				if len(args) != 1 {
					return fmt.Errorf("issues lint: accepts no arguments")
				}
				return nil
			}
			if args[0] == "merge-driver" {
				if len(args) != 4 {
					return fmt.Errorf("issues merge-driver: expected %%O %%A %%B paths")
				}
				return nil
			}
			return cobra.MinimumNArgs(2)(cmd, args)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "merge-driver" {
				return runIssuesMergeDriver(args[2])
			}
			if args[0] == "rebase" {
				upstream := "@{upstream}"
				if len(args) == 2 {
					upstream = args[1]
				}
				return runIssuesRebase(cmd.OutOrStdout(), dir, upstream, checkFlag || dryRunFlag)
			}
			if args[0] == "lint" {
				if cachedFlag {
					return runIssuesLintCached(cmd.OutOrStdout(), dir)
				}
				return runIssuesLint(cmd.OutOrStdout(), dir)
			}
			if args[0] == "new" {
				title := strings.TrimSpace(strings.Join(args[1:], " "))
				return runIssuesNew(cmd.OutOrStdout(), dir, title, jsonFlag)
			}
			if args[0] == "list" {
				if cmd.Flags().Changed("no-commit") || cmd.Flags().Changed("commit") || checkFlag || dryRunFlag {
					return fmt.Errorf("issues list: read-only verb, does not accept --check/--dry-run/--commit/--no-commit")
				}
				return runIssuesList(cmd.OutOrStdout(), dir, args[1:], jsonFlag, limitFlag, allFlag)
			}
			opts := issuesRunOptions{
				Dir:       dir,
				Check:     checkFlag || dryRunFlag,
				JSON:      jsonFlag,
				NoCommit:  noCommitFlag,
				CommitMsg: commitMsgFlag,
			}
			if args[0] == "mv" {
				oldArg := args[1]
				var newArg string
				if len(args) > 2 {
					newArg = args[2]
				}
				result, drift, err := runIssuesMv(cmd.OutOrStdout(), oldArg, newArg, opts)
				if err != nil {
					return err
				}
				printIssuesResult(cmd.OutOrStdout(), result, opts)
				if opts.Check && drift {
					return silenceIfExitCode(cmd, &exitCodeError{Code: 1})
				}
				return nil
			}
			result, drift, err := runIssuesVerb(cmd.OutOrStdout(), args[0], args[1], args[2:], opts)
			if err != nil {
				return err
			}
			printIssuesResult(cmd.OutOrStdout(), result, opts)
			if opts.Check && drift {
				return silenceIfExitCode(cmd, &exitCodeError{Code: 1})
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/")
	cmd.Flags().BoolVar(&cachedFlag, "cached", false, "validate the staged Git snapshot (issues lint only)")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "report what would change without writing or committing; exit 1 on drift")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "alias for --check")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output a single JSON object instead of a text line")
	cmd.Flags().BoolVar(&noCommitFlag, "no-commit", false, "rewrite the ticket file and README but do not git add/commit")
	cmd.Flags().StringVar(&commitMsgFlag, "commit", "", `override the default commit message (default: "docs(issues): <verb> <ticket-number>[, <reason>]")`)
	cmd.Flags().IntVarP(&limitFlag, "limit", "n", 10, "with 'list': limit results (default: newest 10)")
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "with 'list': show all matching results")

	return cmd
}

// runIssuesList implements the read-only `harnez issues list [filter]`
// verb (issue 318): a thin wrapper over find's existing issues-query engine
// rather than a second, diverging filter/limit implementation, so the two
// commands never drift on ranking/filtering semantics. Defaults to the
// "is:open" filter when no filter text is given; an explicit filter always
// replaces the default rather than being ANDed with it.
func runIssuesList(w io.Writer, dir string, filterArgs []string, jsonOutput bool, limit int, all bool) error {
	filter := strings.TrimSpace(strings.Join(filterArgs, " "))
	if filter == "" {
		filter = "is:open"
	}
	return runFindWithOptions(w, dir, []string{"issues", filter}, false, jsonOutput, "", limit, all)
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
	case "close", "done":
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
		return "", fmt.Errorf("issues: unknown verb %q (expected one of: open, start, block, close, done, draft)", verb)
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
// Non-numeric arguments (such as "266-slug.md" or "archive/266-slug.md")
// resolve by path, filename, or slug to support disambiguating colliding
// numbers. A nonexistent or ambiguous number is a caller-bug error, per
// issue 232 §3's "fail loudly, unlike find's zero-matches-is-valid convention".
func findTicketFile(issuesDir, ticketArg string) (issues.IssueFile, error) {
	ticketArg = strings.TrimSpace(ticketArg)
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return issues.IssueFile{}, fmt.Errorf("issues: %w", err)
	}

	n, err := strconv.Atoi(ticketArg)
	if err == nil {
		if n < 0 {
			return issues.IssueFile{}, fmt.Errorf("issues: invalid ticket number %q (expected a non-negative integer)", ticketArg)
		}
		ticketNum := fmt.Sprintf("%03d", n)
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

	cleaned := strings.TrimPrefix(filepath.ToSlash(ticketArg), "issues/")
	var matches []issues.IssueFile
	for _, f := range files {
		base := filepath.Base(f.RelPath)
		slug := strings.TrimSuffix(base, ".md")
		if f.RelPath == cleaned || base == cleaned || slug == cleaned {
			matches = append(matches, f)
		}
	}
	switch len(matches) {
	case 0:
		return issues.IssueFile{}, fmt.Errorf("issues: no ticket found for %q under %s (checked issues/*.md and issues/archive/*.md)", ticketArg, issuesDir)
	case 1:
		return matches[0], nil
	default:
		var paths []string
		for _, m := range matches {
			paths = append(paths, m.RelPath)
		}
		return issues.IssueFile{}, fmt.Errorf("issues: ambiguous ticket %q matches multiple files: %s", ticketArg, strings.Join(paths, ", "))
	}
}

// runIssuesMv implements `harnez issues mv <old> [new]`:
// 1. Resolve old ticket with findTicketFile.
// 2. Resolve target number (newArg if given, else issues.NextNumberFromFiles).
// 3. Reject if target ticket number is already claimed by another file.
// 4. Rename file (keeping slug intact), rewrite header '# <new> — <title>', resync issues/README.md.
// 5. Guard against collisions using O_CREATE|O_EXCL on target filename.
// 6. Handle --check / --dry-run (simulate and restore), --no-commit, --json.
// 7. Commit changes via git staging if not --no-commit.
func runIssuesMv(w io.Writer, oldArg, newArg string, opts issuesRunOptions) (issuesResult, bool, error) {
	issuesDir := filepath.Join(opts.Dir, "issues")
	f, err := findTicketFile(issuesDir, oldArg)
	if err != nil {
		return issuesResult{}, false, err
	}

	files, err := issues.Scan(issuesDir)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues mv: %w", err)
	}

	var newNum string
	if strings.TrimSpace(newArg) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(newArg))
		if err != nil || n < 0 {
			return issuesResult{}, false, fmt.Errorf("issues mv: invalid new ticket number %q (expected a non-negative integer)", newArg)
		}
		newNum = fmt.Sprintf("%03d", n)
	} else {
		newNum = issues.NextNumberFromFiles(files)
	}

	oldPath := filepath.Join(issuesDir, f.RelPath)
	oldGitRelPath := filepath.ToSlash(filepath.Join("issues", f.RelPath))

	content, err := os.ReadFile(oldPath)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues mv: read %s: %w", oldPath, err)
	}

	_, status, _ := issues.ParseIssueFile(string(content))

	result := issuesResult{
		Number:    newNum,
		File:      oldGitRelPath,
		OldStatus: status,
		NewStatus: status,
		OldNumber: f.Number,
	}

	if f.Number == newNum {
		result.Noop = true
		return result, false, nil
	}

	for _, f2 := range files {
		if f2.Number == newNum && f2.RelPath != f.RelPath {
			return issuesResult{}, false, fmt.Errorf("issues mv: target ticket number %s already exists (%s)", newNum, f2.RelPath)
		}
	}

	dirPart, filePart := filepath.Split(f.RelPath)
	var newBaseName string
	if strings.HasPrefix(filePart, f.Number+"-") {
		newBaseName = fmt.Sprintf("%s-%s", newNum, strings.TrimPrefix(filePart, f.Number+"-"))
	} else if strings.HasPrefix(filePart, f.Number) {
		newBaseName = fmt.Sprintf("%s%s", newNum, strings.TrimPrefix(filePart, f.Number))
	} else {
		newBaseName = fmt.Sprintf("%s-%s", newNum, filePart)
	}

	newRelPath := filepath.Join(dirPart, newBaseName)
	newPath := filepath.Join(issuesDir, newRelPath)
	newGitRelPath := filepath.ToSlash(filepath.Join("issues", newRelPath))
	result.File = newGitRelPath

	newContent, err := issues.RewriteHeaderNumber(string(content), newNum)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues mv: %w", err)
	}

	readmePath := filepath.Join(issuesDir, "README.md")

	if opts.Check {
		if _, err := os.Stat(newPath); err == nil {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: destination file %s already exists", newGitRelPath)
		}
		origReadme, readErr := os.ReadFile(readmePath)
		readmeExisted := readErr == nil
		if readErr != nil && !os.IsNotExist(readErr) {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: read %s: %w", readmePath, readErr)
		}

		if err := os.WriteFile(newPath, []byte(newContent), 0o644); err != nil {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: write %s: %w", newPath, err)
		}
		_ = os.Remove(oldPath)

		readmeChanged, updErr := index.UpdateIssuesReadme(readmePath, issuesDir)

		// Always restore
		_ = os.WriteFile(oldPath, content, 0o644)
		_ = os.Remove(newPath)
		if readmeExisted {
			_ = os.WriteFile(readmePath, origReadme, 0o644)
		} else {
			_ = os.Remove(readmePath)
		}

		if updErr != nil {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: %w", updErr)
		}

		result.ReadmeUpdated = readmeChanged
		return result, true, nil
	}

	claimFile, err := os.OpenFile(newPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return issuesResult{}, false, fmt.Errorf("issues mv: destination file %s already exists: %w", newGitRelPath, err)
		}
		return issuesResult{}, false, fmt.Errorf("issues mv: claim destination %s: %w", newGitRelPath, err)
	}
	if _, err := claimFile.WriteString(newContent); err != nil {
		_ = claimFile.Close()
		_ = os.Remove(newPath)
		return issuesResult{}, false, fmt.Errorf("issues mv: write %s: %w", newPath, err)
	}
	if err := claimFile.Close(); err != nil {
		_ = os.Remove(newPath)
		return issuesResult{}, false, fmt.Errorf("issues mv: close %s: %w", newPath, err)
	}

	if err := os.Remove(oldPath); err != nil {
		_ = os.Remove(newPath)
		return issuesResult{}, false, fmt.Errorf("issues mv: remove old file %s: %w", oldPath, err)
	}

	readmeChanged, err := index.UpdateIssuesReadme(readmePath, issuesDir)
	if err != nil {
		_ = os.WriteFile(oldPath, content, 0o644)
		_ = os.Remove(newPath)
		return issuesResult{}, false, fmt.Errorf("issues mv: update README: %w", err)
	}
	result.ReadmeUpdated = readmeChanged

	if !opts.NoCommit {
		msg := opts.CommitMsg
		if msg == "" {
			msg = fmt.Sprintf("docs(issues): renumber %s to %s", f.Number, newNum)
		}
		readmeRelPath := filepath.ToSlash(filepath.Join("issues", "README.md"))
		sha, err := gitAddAndCommit(opts.Dir, []string{oldGitRelPath, newGitRelPath, readmeRelPath}, msg)
		if err != nil {
			return issuesResult{}, false, fmt.Errorf("issues mv: commit: %w", err)
		}
		result.Committed = true
		result.CommitSHA = sha
	}

	return result, false, nil
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
	if verb == "done" {
		verb = "close"
	}
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
		if r.OldNumber != "" && r.OldNumber != r.Number {
			return fmt.Sprintf("%s -> %s: already at %s (no change)", r.OldNumber, r.Number, r.File)
		}
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
	if r.OldNumber != "" && r.OldNumber != r.Number {
		return fmt.Sprintf("%s -> %s: %s (%s)", r.OldNumber, r.Number, r.File, strings.Join(parts, ", "))
	}
	return fmt.Sprintf("%s: %s -> %s (%s)", r.Number, r.OldStatus, r.NewStatus, strings.Join(parts, ", "))
}
