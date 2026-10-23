// find implements `harnez find <entity> [options] <query...>`, a deliberate
// query command over repository-data entities. The only entity in v1 is
// "issues" (issues/*.md and issues/archive/*.md); see
// issues/158-find-entity-query-command.md for the full query grammar,
// ranking, and output contract.
// It also provides `harnez find issues next` (issue 194) to calculate the
// next free issue number, a pure read-only query. Reserving/creating that
// number is `harnez issues new` (cmd/harnez/issues.go) -- find stays a pure
// query surface, mirroring issues.go's own "find is read-only, issues is the
// write-side counterpart" doc comment.
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/find"
)

func newFindCmd() *cobra.Command {
	var dir string
	var nextFlag bool
	var jsonFlag bool
	var rawFlag bool
	var textFlag bool
	var imageFlag bool
	var historyProjectFlag string
	var limitFlag int
	var allFlag bool
	var viaFlag string
	var kFlag int
	var jsonlFlag bool

	cmd := &cobra.Command{
		Use:   "find <entity> [options] <query...>",
		Short: "Query repository-data entities with a short fuzzy-text/filter grammar",
		Long: `find searches a repository-data entity with a short query language that
mixes Google/Gmail-style fuzzy text discovery with compact filters, instead
of a formal Boolean/regex grammar.

Entities:
  issues   searches issues/*.md and issues/archive/*.md (excludes
           issues/README.md and every other file/directory). Both active and
           archived tickets are in scope by default; use a status filter to
           narrow lifecycle state.
  code     searches source files through configured finders (neus first,
           with rg as the code fallback when neus is unavailable).
           Outside a Git worktree, an explicit -d/--dir is required.
  docs     searches documentation through configured finders, with built-in
           fuzzy text matching as a fallback.

Subcommands / Allocation:
  harnez find issues last [--json]
           List the newest issues (the same deterministic ticket-number order
           as an unfiltered listing). The default limit is 10.

  harnez find issues open|blocked|closed|draft [query...]
           Shorthand for the matching status filter, optionally combined with
           ordinary fuzzy text terms.

  harnez find issues next [--json]
           Compute and report the next free ticket number (max+1, formatted
           with 3+ digits). Read-only: it does not create or reserve
           anything. To atomically reserve that number and create a Draft
           placeholder ticket file, use 'harnez issues new [title]' instead
           (cmd/harnez/issues.go) -- find never mutates.

  harnez find issues history [--project <name>] [--json]
           Render the open/closed/draft/backlog/unknown ticket-count snapshot
           history 'harnez index' records into the telemetry DB on each run
           (issue 228), oldest first. --project narrows to one project_name
           (the same identity 'harnez stats --project' filters on, issue
           227); omitted shows every project's snapshots. Non-JSON output is
           a formatted table: PROJECT, CREATED_AT, OPEN, CLOSED, DRAFT,
           BACKLOG, UNKNOWN.

  harnez find issues similar "<title>" [--json]
           OR-rank likely duplicate tickets across every lifecycle status.
           Results show score and raw status; threshold and top count come
           from spec/issues.yaml.

Query grammar:
  whitespace         AND: 'vram gtt' requires both terms.
  a|b                OR within one compact term: 'vram|gtt' means either.
                     Precedence is fixed: filters and AND bind outside OR,
                     so 'status:open vram|gtt' means
                     'status:open AND (vram OR gtt)'.
  status:VALUE       filter (alias 'is:VALUE'), ANDed with other terms.
                     Accepted values come from spec/issues.yaml. 'open'
                     excludes Backlog; 'backlog' selects it, and
                     'unresolved' includes Open, In Progress, Blocked,
                     Draft, and Backlog. Matching ignores case and
                     explanatory status suffixes (e.g. "Blocked — waiting
                     for upstream").

An unquoted '|' is a shell pipeline operator, so an OR query needs shell
quoting:

  harnez find issues status:open vram gtt
  harnez find issues "status:open vram|gtt"

Not supported (usage error, not a guessed interpretation): parentheses,
literal quote characters, negation, an empty filter, unknown fields, an
unsupported status value, or a standalone/leading/trailing/doubled '|'.
Regex-looking punctuation in ordinary text is normalized away, never
executed.

Matching: the H1 title (its leading ticket number stripped) and the ticket
body (everything after the first metadata-closing horizontal rule) are
searched; the metadata header itself is not. Both the searchable text and
every query alternative are normalized by Unicode-lowercasing, replacing
Markdown syntax/non-letter/non-number characters with spaces, and
collapsing whitespace -- so punctuation-separated identifiers like
"time-gauge" become adjacent searchable tokens. A bare alternative matches a
field when its full normalized text is a substring of that field, or when
every one of its normalized tokens matches a field token by prefix or
bounded typo distance (Damerau-Levenshtein, transpositions counted as one
edit): terms of 4 characters or fewer get no typo tolerance, 5-8 characters
allow 1 edit, 9+ characters allow 2 edits -- this protects short
identifiers like "gtt"/"vram" from noisy fuzzy expansion.

Ranking: matches are classed, best to worst -- (1) exact title substring,
(2) title token-prefix, (3) title fuzzy, (4) exact body substring, (5) body
token-prefix, (6) body fuzzy -- and ranked by worst group class, then the
sum of all group classes, then ticket number, then path. This never
silently relaxes an AND query to OR on zero results.

Output formatting:
  code/docs: exit 2 if a finder fails and no results are available;
             partial results keep exit 0 with diagnostics on stderr.
  Default: in interactive terminal / TTY sessions or with -I/--image,
           renders a bounded visual overview card (using internal/readcard)
           or compact bounded summaries.
  --raw / --text: outputs deterministic TSV lines (one per result).
  --json: outputs structured JSON.
  In non-TTY pipes/scripts without -I, automatically defaults to TSV text.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limitFlag <= 0 {
				return fmt.Errorf("find: --limit must be greater than zero")
			}
			if args[0] == "code" && !cmd.Flags().Changed("dir") {
				out, err := exec.CommandContext(cmd.Context(), "git", "-C", dir, "rev-parse", "--is-inside-work-tree").Output()
				if err != nil || strings.TrimSpace(string(out)) != "true" {
					return fmt.Errorf("find code: outside a Git worktree; use -d/--dir <directory> to select the search scope")
				}
			}
			opts := find.FindRunOptions{
				Dir:            dir,
				Next:           nextFlag,
				JSON:           jsonFlag,
				Raw:            rawFlag,
				Text:           textFlag,
				Image:          imageFlag,
				HistoryProject: historyProjectFlag,
				Limit:          limitFlag,
				All:            allFlag,
				Via:            viaFlag,
				K:              kFlag,
				JSONL:          jsonlFlag,
			}
			return silenceIfExitCode(cmd, find.RunFindWithOptions(cmd.OutOrStdout(), cmd.ErrOrStderr(), args, opts))
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "search directory (required for code outside a Git worktree)")
	cmd.Flags().BoolVar(&nextFlag, "next", false, "report the next free issue number")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
	cmd.Flags().BoolVarP(&rawFlag, "raw", "r", false, "output raw TSV text directly to stdout")
	cmd.Flags().BoolVarP(&textFlag, "text", "t", false, "output raw TSV text directly to stdout (alias for --raw)")
	cmd.Flags().BoolVarP(&imageFlag, "image", "I", false, "render results as a visual PNG overview card")
	cmd.Flags().StringVar(&historyProjectFlag, "project", "", "with 'history': filter to one project_name")
	cmd.Flags().IntVarP(&limitFlag, "limit", "n", 10, "limit issue results (default: newest 10 for listings)")
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "show all matching issue results")
	cmd.Flags().StringVar(&viaFlag, "via", "", "use only the named code/docs finder")
	cmd.Flags().IntVarP(&kFlag, "count", "k", 10, "maximum code/docs results")
	cmd.Flags().BoolVar(&jsonlFlag, "jsonl", false, "output JSON Lines (code/docs only)")

	return cmd
}
