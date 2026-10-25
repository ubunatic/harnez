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
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/index"
	"ubunatic.com/harnez/internal/issues"
	issuesmaintenance "ubunatic.com/harnez/internal/issues/maintenance"
	"ubunatic.com/harnez/internal/readcard"
)

// issuesResult is the structured shape `harnez issues <verb>` reports, both
// as --json output and as the basis of its single deterministic text line,
// matching the exact field shape issue 232 §3 specifies.
type issuesResult = issues.StatusResult

// issuesRunOptions bundles runIssuesVerb's inputs.
type issuesRunOptions struct {
	Dir                            string
	ClosedByModel, ClosedBySession string
	Check                          bool
	JSON                           bool
	NoCommit                       bool
	AmendReason                    bool
	NoCode                         bool
	CommitMsg                      string // explicit --commit override; empty means "compose the default"
	Stderr                         io.Writer
}

func newIssuesCmd() *cobra.Command {
	return newIssuesCmdWithGHRunner(runGH)
}

func newIssuesCmdWithGHRunner(gh ghRunner) *cobra.Command {
	var remoteFlag bool
	var forceFlag bool
	var dir string
	var checkFlag bool
	var dryRunFlag bool
	var jsonFlag bool
	var rawFlag bool
	var textFlag bool
	var imageFlag bool
	var noCommitFlag bool
	var amendReasonFlag bool
	var noCodeFlag bool
	var commitMsgFlag string
	var closedByModelFlag string
	var closedBySessionFlag string
	var cachedFlag bool
	var limitFlag int
	var allFlag bool
	var listFlag bool

	cmd := &cobra.Command{
		Use:   "issues <verb> <ticket-number...> [reason...]",
		Short: "Change a ticket's Status line, resync issue indexes, and commit -- in one call",
		Long: `issues collapses today's four-step ticket-status-change dance (hand-edit the
'**Status**:' line, run 'harnez index', 'git add' the changed files, 'git
commit') into one call: it rewrites the ticket's Status line, regenerates
issues/README.md, BACKLOG.md, and CLOSED.md in-process (the same logic
'harnez index' uses), and commits the changed files by default.

Verbs (closed set, mirroring docs/IssueTracking.md's Allowed Values):

  open [reason]    Status: Open, or "Open — <reason>"
  start [reason]   Status: In Progress, or "In Progress — <reason>"
  block <reason>   Status: "Blocked — <reason>" (reason required)
  close [reason]   Status: Closed (bare), or "Closed — <reason>"
  done [reason]    Status: Closed (bare), or "Closed — <reason>" (alias for close)
  draft [reason]   Status: Draft, or "Draft — <reason>"
  backlog [reason] Status: Backlog, or "Backlog — <reason>"
  promote          Move a Backlog ticket back to Open

Status verbs accept one or more ticket numbers. Leading numeric or GH-prefixed arguments
are targets; the first non-number argument starts the reason. A numeric-only
reason is therefore ambiguous: include text in the reason (for example,
'"123 tickets"'). Batch targets are all validated before any file is changed,
then the README is regenerated and the batch is committed once.
  new --remote [--force] "title" Create a GitHub issue on the github remote and reserve
                    its permanent GH<number> ID. Requires authenticated gh;
                    also selected when CLAUDE_CODE_REMOTE is set. Strong
                    duplicate matches block before gh issue create unless
                    --force is supplied. No fallback.
  new [--force] [title] Atomically reserve the next free issue number and create
                    a Draft placeholder file (issues/<NNN>-<title-slug>.md,
                    or issues/<NNN>-reserved.md with no title), using
                    O_CREATE|O_EXCL so concurrent callers never collide. Non-
                    JSON output is "<NUMBER>\t<PATH>" -- write the ticket's
                    real content directly to that printed path instead of
                    re-deriving the slug from the title by hand (a
                    hand-derived slug can diverge and leave an orphaned
                    placeholder behind, see issue 202). Strong duplicate
                    matches block unless --force is supplied. Unlike every
                    other verb, 'new' takes no ticket number (there isn't one
                    yet) and never commits.
  show <number>    Inspect a ticket. In interactive terminal / TTY or with
                    -I/--image, renders a bounded visual card (using
                    internal/readcard). Supports --raw/--text and --json.
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
                    filename/H1 numbers, and drift across all three indexes.
  status            Show lifecycle counts and warn when active tickets exceed
                    the configured spec cap.
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
commit is made) -- supply it explicitly, or correct it with --amend-reason.

For close and done, --model and --session record optional attribution directly
under Status as **Closed-By**. They default to HARNEZ_AGENT_MODEL and
HARNEZ_SESSION_ID; explicit flags take precedence. Missing values are omitted,
and generated indexes do not include attribution.

Close gate: close and done refuse unless a commit with changes outside issues/
references the ticket number, such as a subject ending in "(issue 42)". Commits
that touch only issues/ do not count, and a near-miss number does not match (86
does not match 867). For a ticket that needed no code, pass --no-code: the close
goes through and its commit body says so. Batch targets are checked before any
file is written.

Idempotency: re-running a verb that would produce the exact same Status
line text already on disk is a no-op -- exit 0, no file/README/commit
changes. Closing an already-Closed ticket is always a no-op, regardless of
the requested reason. Use close (or done) --amend-reason to explicitly
correct its reason in one commit; this requires an already-Closed ticket.
Other verbs still allow reason changes on an already-matching status.

--check (alias --dry-run) reports what would change without writing or
committing, and exits 1 if there is drift from the requested state (mirrors
'harnez index --check').

A nonexistent or ambiguous ticket number is a caller-bug error (non-zero
exit, actionable stderr) -- unlike 'harnez find', where zero matches is a
valid, exit-0 answer.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if noCodeFlag && (listFlag || len(args) == 0 || (args[0] != "close" && args[0] != "done")) {
				return fmt.Errorf("issues: --no-code requires close or done")
			}
			if amendReasonFlag && (listFlag || len(args) == 0 || (args[0] != "close" && args[0] != "done")) {
				return fmt.Errorf("issues: --amend-reason requires close or done")
			}
			if (cmd.Flags().Changed("model") || cmd.Flags().Changed("session")) && (listFlag || len(args) == 0 || (args[0] != "close" && args[0] != "done")) {
				return fmt.Errorf("issues: --model and --session require close or done")
			}
			if remoteFlag && (len(args) == 0 || args[0] != "new") {
				return fmt.Errorf("issues: --remote requires new")
			}
			if forceFlag && (len(args) == 0 || args[0] != "new") {
				return fmt.Errorf("issues: --force requires new")
			}
			if listFlag {
				if len(args) != 0 {
					return fmt.Errorf("issues -l: does not accept positional arguments")
				}
				return nil
			}
			if len(args) == 0 {
				return fmt.Errorf("issues: requires a verb (open, start, block, close, done, draft, backlog, promote, new, show, mv, rebase, lint, status, list)")
			}
			if args[0] == "new" {
				return nil // [title] is optional, no ticket-number argument exists yet
			}
			if args[0] == "list" {
				return nil // [filter] is optional, defaults to "is:open"
			}
			if args[0] == "show" {
				if len(args) != 2 {
					return fmt.Errorf("issues show: requires exactly 1 ticket number argument")
				}
				return nil
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
			if args[0] == "status" {
				if len(args) != 1 {
					return fmt.Errorf("issues status: accepts no arguments")
				}
				return nil
			}
			if args[0] == "merge-driver" {
				if len(args) != 4 && len(args) != 5 {
					return fmt.Errorf("issues merge-driver: expected %%O %%A %%B [%%P] paths")
				}
				return nil
			}
			return cobra.MinimumNArgs(2)(cmd, args)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listFlag {
				return runIssuesList(cmd.OutOrStdout(), cmd.ErrOrStderr(), dir, nil, jsonFlag, rawFlag, textFlag, imageFlag, limitFlag, allFlag)
			}
			if args[0] == "merge-driver" {
				target := "issues/README.md"
				if len(args) == 4 {
					return issuesmaintenance.RunIssuesMergeDriverVersions(args[1], args[2], args[3], target)
				}
				if len(args) == 5 {
					target = args[4]
				}
				return issuesmaintenance.RunIssuesMergeDriverVersions(args[1], args[2], args[3], target)
			}
			if args[0] == "rebase" {
				upstream := "@{upstream}"
				if len(args) == 2 {
					upstream = args[1]
				}
				return issuesmaintenance.RunIssuesRebase(cmd.OutOrStdout(), dir, upstream, checkFlag || dryRunFlag)
			}
			if args[0] == "lint" {
				if cachedFlag {
					return issuesmaintenance.RunIssuesLintCached(cmd.OutOrStdout(), dir)
				}
				return issuesmaintenance.RunIssuesLint(cmd.OutOrStdout(), dir)
			}
			if args[0] == "status" {
				return issuesmaintenance.RunIssuesStatus(cmd.OutOrStdout(), dir)
			}
			if args[0] == "new" {
				title := strings.TrimSpace(strings.Join(args[1:], " "))
				_, remoteEnv := os.LookupEnv("CLAUDE_CODE_REMOTE")
				if remoteFlag || remoteEnv {
					fmt.Fprintln(cmd.ErrOrStderr(), "issues new: remote mode")
					return runIssuesNewRemoteWithForce(cmd.OutOrStdout(), dir, title, jsonFlag, forceFlag, gh)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "issues new: local mode")
				return runIssuesNewWithForce(cmd.OutOrStdout(), dir, title, jsonFlag, forceFlag)
			}
			if args[0] == "show" {
				if cmd.Flags().Changed("no-commit") || cmd.Flags().Changed("commit") || checkFlag || dryRunFlag {
					return fmt.Errorf("issues show: read-only verb, does not accept --check/--dry-run/--commit/--no-commit")
				}
				return runIssuesShow(cmd.OutOrStdout(), dir, args[1], jsonFlag, rawFlag, textFlag, imageFlag)
			}
			if args[0] == "list" {
				if cmd.Flags().Changed("no-commit") || cmd.Flags().Changed("commit") || checkFlag || dryRunFlag {
					return fmt.Errorf("issues list: read-only verb, does not accept --check/--dry-run/--commit/--no-commit")
				}
				return runIssuesList(cmd.OutOrStdout(), cmd.ErrOrStderr(), dir, args[1:], jsonFlag, rawFlag, textFlag, imageFlag, limitFlag, allFlag)
			}
			opts := issuesRunOptions{
				Dir:             dir,
				ClosedByModel:   closedByModelFlag,
				ClosedBySession: closedBySessionFlag,
				Check:           checkFlag || dryRunFlag,
				JSON:            jsonFlag,
				NoCommit:        noCommitFlag,
				AmendReason:     amendReasonFlag,
				NoCode:          noCodeFlag,
				CommitMsg:       commitMsgFlag,
				Stderr:          cmd.ErrOrStderr(),
			}
			if !cmd.Flags().Changed("model") {
				opts.ClosedByModel = os.Getenv("HARNEZ_AGENT_MODEL")
			}
			if !cmd.Flags().Changed("session") {
				opts.ClosedBySession = os.Getenv("HARNEZ_SESSION_ID")
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
			numberCount := leadingIssueNumberCount(args[1:])
			if numberCount > 1 {
				results, drift, err := runIssuesVerbBatch(args[0], args[1:1+numberCount], args[1+numberCount:], opts)
				if err != nil {
					return err
				}
				if opts.JSON {
					data, err := json.Marshal(results)
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				} else {
					for _, result := range results {
						printIssuesResult(cmd.OutOrStdout(), result, opts)
					}
				}
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
		ValidArgsFunction: issuesCompletion,
	}
	cmd.Flags().BoolVar(&remoteFlag, "remote", false, "with new: allocate a permanent GH id using authenticated gh on the github remote")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "with new: create despite strong duplicate matches")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/")
	cmd.Flags().BoolVar(&cachedFlag, "cached", false, "validate the staged Git snapshot (issues lint only)")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "report what would change without writing or committing; exit 1 on drift")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "alias for --check")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output a single JSON object instead of a text line")
	cmd.Flags().BoolVarP(&rawFlag, "raw", "r", false, "output raw text directly to stdout")
	cmd.Flags().BoolVarP(&textFlag, "text", "t", false, "output raw text directly to stdout (alias for --raw)")
	cmd.Flags().BoolVarP(&imageFlag, "image", "I", false, "render visual PNG card context")
	cmd.Flags().BoolVar(&noCommitFlag, "no-commit", false, "rewrite the ticket file and README but do not git add/commit")
	cmd.Flags().BoolVar(&amendReasonFlag, "amend-reason", false, "with close/done: amend the reason of an already-Closed ticket")
	cmd.Flags().BoolVar(&noCodeFlag, "no-code", false, "with close/done: the ticket needed no code change, so skip the commit check and record that in the close commit")
	cmd.Flags().StringVar(&closedByModelFlag, "model", "", "with close/done: record the closing model (defaults to HARNEZ_AGENT_MODEL)")
	cmd.Flags().StringVar(&closedBySessionFlag, "session", "", "with close/done: record the closing session (defaults to HARNEZ_SESSION_ID)")
	cmd.Flags().StringVar(&commitMsgFlag, "commit", "", `override the default commit message (default: "docs(issues): <verb> <ticket-number>[, <reason>]")`)
	cmd.Flags().IntVarP(&limitFlag, "limit", "n", 10, "with 'list': limit results (default: newest 10)")
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "with 'list': show all matching results")
	cmd.Flags().BoolVarP(&listFlag, "list", "l", false, "list open issues (alias for the list verb)")

	return cmd
}

func leadingIssueNumberCount(args []string) int {
	count := 0
	for _, arg := range args {
		if arg == "" {
			break
		}
		if _, err := issues.NormalizeID(arg); err != nil {
			break
		}
		count++
	}
	return count
}

// issueShowJSON represents structured JSON output for `harnez issues show`.
type issueShowJSON struct {
	Number     string `json:"number"`
	Title      string `json:"title"`
	RawStatus  string `json:"raw_status"`
	Canonical  string `json:"canonical_status"`
	Path       string `json:"path"`
	Body       string `json:"body"`
	FullSource string `json:"full_source"`
}

// runIssuesShow implements `harnez issues show <ticket-number>`.
func runIssuesShow(w io.Writer, dir, ticketArg string, jsonOutput, rawOutput, textOutput, imageOutput bool) error {
	issuesDir := filepath.Join(dir, "issues")
	f, err := issues.FindTicketFile(issuesDir, ticketArg)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(issuesDir, f.RelPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("issues show: read %s: %w", fullPath, err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(issueShowJSON{
			Number:     f.Number,
			Title:      f.Title,
			RawStatus:  f.RawStatus,
			Canonical:  string(f.Canonical),
			Path:       filepath.ToSlash(filepath.Join("issues", f.RelPath)),
			Body:       f.Body,
			FullSource: string(content),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	useVisualCard := imageOutput || (!rawOutput && !textOutput && find.IsTerminalWriter(w))
	if useVisualCard {
		lines := strings.Split(string(content), "\n")
		renderRes, err := readcard.RenderFileToCards(lines, filepath.Base(f.RelPath), readcard.RenderOptions{
			Title:           f.Title,
			FontName:        "pixel",
			ShowLineNumbers: true,
		})
		if err == nil {
			for _, p := range renderRes.Files {
				fmt.Fprintf(w, "See PNG card: %s (%dx%d px, %d col, %d lines)\n", p, renderRes.Width, renderRes.Height, renderRes.Columns, renderRes.TotalLines)
			}
			fmt.Fprintf(w, "Token Breakdown: ~%d ViT tokens (Claude) vs ~%d text tokens\n", renderRes.TokenStats.ClaudeTokens, renderRes.TokenStats.TextTokens)
			return nil
		}
	}

	fmt.Fprint(w, string(content))
	return nil
}

// runIssuesList implements the read-only `harnez issues list [filter]`
// verb (issue 318): a thin wrapper over find's existing issues-query engine
// rather than a second, diverging filter/limit implementation, so the two
// commands never drift on ranking/filtering semantics. Defaults to the
// "is:open" filter when no filter text is given; an explicit filter always
// replaces the default rather than being ANDed with it.
func runIssuesList(w, errW io.Writer, dir string, filterArgs []string, jsonOutput, rawOutput, textOutput, imageOutput bool, limit int, all bool) error {
	filter := strings.TrimSpace(strings.Join(filterArgs, " "))
	if filter == "" {
		filter = "is:open"
	}
	if !jsonOutput && !rawOutput && !textOutput && !imageOutput && find.IsTerminalWriter(w) {
		textOutput = true
	}
	return find.RunFindWithOptions(w, errW, []string{"issues", filter}, find.FindRunOptions{
		Dir:   dir,
		JSON:  jsonOutput,
		Raw:   rawOutput,
		Text:  textOutput,
		Image: imageOutput,
		Limit: limit,
		All:   all,
	})
}

// runIssuesVerbBatch validates every target before writing, then updates all
// changed tickets, regenerates the README once, and makes at most one commit.
func issuesStatusDependencies(opts issuesRunOptions) issues.StatusBatchOptions {
	return issues.StatusBatchOptions{
		Dir: opts.Dir, Check: opts.Check, NoCommit: opts.NoCommit, AmendReason: opts.AmendReason, NoCode: opts.NoCode,
		CommitMsg: opts.CommitMsg, ClosedByModel: opts.ClosedByModel, ClosedBySession: opts.ClosedBySession, Stderr: opts.Stderr,
		UpdateIndexes: func(issuesDir string) (bool, error) {
			return index.UpdateIssuesReadme(filepath.Join(issuesDir, "README.md"), issuesDir)
		},
		SnapshotIndexes: func(issuesDir string) (any, error) {
			return issuesmaintenance.SnapshotIssueIndexes(issuesDir)
		},
		RestoreIndexes: func(issuesDir string, snapshot any) error {
			indexSnapshot, ok := snapshot.(issuesmaintenance.IssueIndexSnapshot)
			if !ok {
				return fmt.Errorf("issues: invalid index snapshot %T", snapshot)
			}
			return issuesmaintenance.RestoreIssueIndexes(issuesDir, indexSnapshot)
		},
		IndexPaths:              issuesmaintenance.IssueIndexRelPaths,
		Commit:                  gitAddAndCommit,
		PathsChanged:            gitPathsChanged,
		DefaultMessage:          defaultCommitMessage,
		WarnHeaderMismatch:      warnIssueHeaderMismatch,
		HasImplementationCommit: hasImplementationCommit,
	}
}

// runIssuesVerbBatch keeps CLI concerns in cmd/harnez and injects index,
// snapshot, commit and warning effects into the issues core.
func runIssuesVerbBatch(verb string, ticketArgs, reasonArgs []string, opts issuesRunOptions) ([]issuesResult, bool, error) {
	return issues.ApplyStatusBatch(verb, ticketArgs, reasonArgs, issuesStatusDependencies(opts))
}

func issuesCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return []string{
			"open\tmark a ticket open",
			"start\tmark a ticket in progress",
			"block\tblock a ticket",
			"close\tclose a ticket",
			"done\tclose a ticket (alias for close)",
			"draft\tmark a ticket draft",
			"backlog\tdefer a ticket to Backlog",
			"promote\tmove a Backlog ticket to Open",
			"new\tcreate a ticket placeholder",
			"show\tinspect a ticket",
			"mv\trenumber a ticket",
			"rebase\treplay local issue commits",
			"lint\tvalidate the issue tracker",
			"status\tshow issue lifecycle counts",
			"list\tlist matching tickets",
		}, cobra.ShellCompDirectiveNoFileComp
	}
	if args[0] != "list" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	files, err := issues.Scan(filepath.Join(dir, "issues"))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	completions := make([]string, 0, len(issues.StatusFilterValues())+len(files))
	for _, filter := range issues.StatusFilterValues() {
		completions = append(completions, "is:"+filter+"\tstatus filter "+filter)
	}
	for _, f := range files {
		completions = append(completions, f.Number+"\t"+issues.PlainTitle(f.Title))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// runIssuesNew implements `harnez issues new [title]`: atomically reserve
// the next free ticket number and create a Draft placeholder file, using
// the exact internal/issues.Reserve call and output contract
// `harnez find issues next --reserve` used before issue 233 moved the
// mutation out of `find`. It refreshes issues/README.md after the reserved
// file has been created; the index initially shows the placeholder title
// until the caller writes ticket content. It intentionally does not require
// (or accept) a ticket number -- there isn't one until this call allocates it
// -- and never commits.
func runIssuesNew(w io.Writer, dir, title string, jsonOutput bool) error {
	return runIssuesNewWithForce(w, dir, title, jsonOutput, false)
}

func runIssuesNewWithForce(w io.Writer, dir, title string, jsonOutput, force bool) error {
	issuesDir := filepath.Join(dir, "issues")
	adoptTracker := !dirExists(issuesDir)
	if !force {
		if err := checkIssueDuplicates(issuesDir, title); err != nil {
			return err
		}
	}
	num, filename, err := issues.Reserve(issuesDir, issues.ReserveOptions{Title: title})
	if err != nil {
		return fmt.Errorf("issues new: %w", err)
	}
	if adoptTracker {
		if err := claude.AdoptRepoIssuesTracker(dir); err != nil {
			return fmt.Errorf("issues new: record tracker adoption: %w", err)
		}
		fmt.Fprintln(os.Stderr, "issues new: adopted file-based tracker (tracker: harnez)")
	}
	return finishIssuesNew(w, dir, num, filename, jsonOutput)
}

func finishIssuesNew(w io.Writer, dir, num, filename string, jsonOutput bool) error {
	issuesDir := filepath.Join(dir, "issues")
	readmePath := filepath.Join(issuesDir, "README.md")
	if _, err := index.UpdateIssuesReadme(readmePath, issuesDir); err != nil {
		return fmt.Errorf("issues new: update README: %w", err)
	}
	relPath := filepath.ToSlash(filepath.Join("issues", filename))
	if jsonOutput {
		data, err := json.Marshal(find.NextResultJSON{
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
	fmt.Fprintln(os.Stderr, "issues new: README refreshed")
	return nil
}

// findTicketFile locates the single issues/*.md (or issues/archive/*.md)
// file whose Number matches ticketArg (accepting "232", "32", or "032"
// alike, normalized to the same %03d width Scan/ParseTrackerTable use).
// Non-numeric arguments (such as "266-slug.md" or "archive/266-slug.md")
// resolve by path, filename, or slug to support disambiguating colliding
// numbers. A nonexistent or ambiguous number is a caller-bug error, per
// issue 232 §3's "fail loudly, unlike find's zero-matches-is-valid convention".

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
	f, err := issues.FindTicketFile(issuesDir, oldArg)
	if err != nil {
		return issuesResult{}, false, err
	}

	if strings.HasPrefix(f.Number, "GH") {
		return issuesResult{}, false, fmt.Errorf("issues mv: %s is a permanent GitHub id and cannot be renumbered", f.Number)
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
		return issuesResult{}, false, fmt.Errorf("issues mv: %s: %w; add a '# %s — Title' header and retry", oldGitRelPath, err, f.Number)
	}
	newContent = issues.RewriteReferences(newContent, oldGitRelPath, newGitRelPath)
	referenceEdits, err := issues.PlanReferences(opts.Dir, oldGitRelPath, newGitRelPath)
	if err != nil {
		return issuesResult{}, false, fmt.Errorf("issues mv: scan references: %w", err)
	}

	readmePath := filepath.Join(issuesDir, "README.md")

	if opts.Check {
		if _, err := os.Stat(newPath); err == nil {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: destination file %s already exists", newGitRelPath)
		}
		indexSnapshot, snapshotErr := issuesmaintenance.SnapshotIssueIndexes(issuesDir)
		if snapshotErr != nil {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: snapshot indexes: %w", snapshotErr)
		}

		if err := os.WriteFile(newPath, []byte(newContent), 0o644); err != nil {
			return issuesResult{}, false, fmt.Errorf("issues mv --check: write %s: %w", newPath, err)
		}
		_ = os.Remove(oldPath)

		readmeChanged, updErr := index.UpdateIssuesReadme(readmePath, issuesDir)

		// Always restore
		_ = os.WriteFile(oldPath, content, 0o644)
		_ = os.Remove(newPath)
		_ = issuesmaintenance.RestoreIssueIndexes(issuesDir, indexSnapshot)

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

	if err := issues.WriteReferences(referenceEdits, false); err != nil {
		_ = issues.WriteReferences(referenceEdits, true)
		_ = os.WriteFile(oldPath, content, 0o644)
		_ = os.Remove(newPath)
		return issuesResult{}, false, fmt.Errorf("issues mv: rewrite references: %w", err)
	}
	readmeChanged, err := index.UpdateIssuesReadme(readmePath, issuesDir)
	if err != nil {
		_ = issues.WriteReferences(referenceEdits, true)
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
		paths := append([]string{oldGitRelPath, newGitRelPath}, issuesmaintenance.IssueIndexRelPaths()...)
		for _, edit := range referenceEdits {
			rel, err := filepath.Rel(opts.Dir, edit.Path)
			if err != nil {
				return issuesResult{}, false, err
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		sha, err := gitAddAndCommit(opts.Dir, paths, msg)
		if err != nil {
			return issuesResult{}, false, fmt.Errorf("issues mv: commit: %w", err)
		}
		result.Committed = true
		result.CommitSHA = sha
	}

	return result, false, nil
}

// hasImplementationCommit reports whether a commit that changes a path outside
// issues/ references this exact ticket number. Ticket status and index commits
// alone therefore do not count. A git error is returned, not treated as absent.
func hasImplementationCommit(dir, ticketNumber string) (bool, error) {
	cmd := exec.Command("git", "log", "--all", "--format=%H%x09%B%x00")
	cmd.Dir = dir
	logData, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git log: %w", err)
	}
	for _, commit := range strings.Split(string(logData), "\x00") {
		fields := strings.SplitN(strings.TrimSpace(commit), "\t", 2)
		if len(fields) < 2 || !issues.MessageReferencesTicket(fields[1], ticketNumber) {
			continue
		}
		pathsCmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", fields[0])
		pathsCmd.Dir = dir
		paths, err := pathsCmd.Output()
		if err != nil {
			continue
		}
		for _, path := range strings.Split(string(paths), "\n") {
			path = strings.TrimSpace(path)
			if path != "" && !strings.HasPrefix(filepath.ToSlash(path), "issues/") {
				return true, nil
			}
		}
	}
	return false, nil
}

// runIssuesVerb performs the full status-change: locate the ticket,
// compose the new Status line, rewrite the file (or simulate the rewrite
// under --check), resync issues/README.md, and commit (unless --no-commit
// or --check). It returns the structured result plus whether the
// requested state would represent drift from what's on disk today (only
// meaningful for --check; the caller decides the process exit code on it,
// since this function is also exercised directly by tests).
func runIssuesVerb(w io.Writer, verb, ticketArg string, reasonArgs []string, opts issuesRunOptions) (issuesResult, bool, error) {
	deps := issuesStatusDependencies(opts)
	deps.Stderr = opts.Stderr
	return issues.ApplyStatus(verb, ticketArg, strings.Join(reasonArgs, " "), deps)
}

func warnIssueHeaderMismatch(w io.Writer, relPath, fileNumber, headerNumber string) {
	if headerNumber == "" || strings.TrimLeft(headerNumber, "0") == strings.TrimLeft(fileNumber, "0") {
		return
	}
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "warning: issue file %s declares heading #%s (file number #%s)\n", relPath, headerNumber, fileNumber)
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
	trackedCmd := exec.Command("git", append([]string{"ls-files", "--cached", "--"}, paths...)...)
	trackedCmd.Dir = dir
	trackedOut, err := trackedCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-files: %w", err)
	}
	tracked := make(map[string]bool)
	for _, path := range strings.Split(strings.TrimSpace(string(trackedOut)), "\n") {
		if path != "" {
			tracked[path] = true
		}
	}
	commitPaths := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path))); err == nil || tracked[path] {
			commitPaths = append(commitPaths, path)
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat %s: %w", path, err)
		}
	}
	if len(commitPaths) == 0 {
		return "", fmt.Errorf("git commit: no requested paths exist")
	}
	addCmd := exec.Command("git", append([]string{"add", "--"}, commitPaths...)...)
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add %s: %w: %s", strings.Join(commitPaths, " "), err, strings.TrimSpace(string(out)))
	}

	commitCmd := exec.Command("git", "commit", "-m", message, "--")
	commitCmd.Args = append(commitCmd.Args, commitPaths...)
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

func gitPathsChanged(dir string, paths []string) (bool, error) {
	args := []string{"status", "--porcelain", "--untracked-files=all", "--"}
	args = append(args, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
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
		line := fmt.Sprintf("%s: already %s (no change)", r.Number, r.NewStatus)
		if issues.CanonicalizeStatus(r.NewStatus) == issues.StatusClosed {
			line += "; use --amend-reason to change the reason"
		}
		return line
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
