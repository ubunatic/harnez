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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/readcard"
	"ubunatic.com/harnez/internal/telemetry"
)

type nextResultJSON struct {
	Number   string `json:"number"`
	Reserved bool   `json:"reserved"`
	File     string `json:"file,omitempty"`
	Path     string `json:"path,omitempty"`
}

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
           Render the open/closed/draft/unknown ticket-count snapshot
           history 'harnez index' records into the telemetry DB on each run
           (issue 228), oldest first. --project narrows to one project_name
           (the same identity 'harnez stats --project' filters on, issue
           227); omitted shows every project's snapshots. Non-JSON output is
           a formatted table: PROJECT, CREATED_AT, OPEN, CLOSED, DRAFT,
           UNKNOWN.

Query grammar:
  whitespace         AND: 'vram gtt' requires both terms.
  a|b                OR within one compact term: 'vram|gtt' means either.
                     Precedence is fixed: filters and AND bind outside OR,
                     so 'status:open vram|gtt' means
                     'status:open AND (vram OR gtt)'.
  status:VALUE       filter (alias 'is:VALUE'), ANDed with other terms.
                     Accepted values: open, in-progress, blocked, closed,
                     draft. 'open'/'is:open' match every unresolved issue
                     whose leading raw status is Open, In Progress, or
                     Blocked; 'in-progress'/'blocked' narrow to just that
                     raw lifecycle stage; 'closed'/'draft' match their
                     canonical category. Matching ignores case and
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
			opts := findRunOptions{
				Dir:            dir,
				Next:           nextFlag,
				JSON:           jsonFlag,
				Raw:            rawFlag,
				Text:           textFlag,
				Image:          imageFlag,
				HistoryProject: historyProjectFlag,
				Limit:          limitFlag,
				All:            allFlag,
			}
			return runFindWithOptions(cmd.OutOrStdout(), cmd.ErrOrStderr(), args, opts)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/")
	cmd.Flags().BoolVar(&nextFlag, "next", false, "report the next free issue number")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
	cmd.Flags().BoolVarP(&rawFlag, "raw", "r", false, "output raw TSV text directly to stdout")
	cmd.Flags().BoolVarP(&textFlag, "text", "t", false, "output raw TSV text directly to stdout (alias for --raw)")
	cmd.Flags().BoolVarP(&imageFlag, "image", "I", false, "render results as a visual PNG overview card")
	cmd.Flags().StringVar(&historyProjectFlag, "project", "", "with 'history': filter to one project_name")
	cmd.Flags().IntVarP(&limitFlag, "limit", "n", 10, "limit issue results (default: newest 10 for listings)")
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "show all matching issue results")

	return cmd
}

// findRunOptions bundles inputs for runFindWithOptions.
type findRunOptions struct {
	Dir            string
	Next           bool
	JSON           bool
	Raw            bool
	Text           bool
	Image          bool
	HistoryProject string
	Limit          int
	All            bool
}

// findHistoryOptions bundles runFindHistory's inputs. DBPath is a telemetry
// DB path override (empty means telemetry.DefaultDBPath()) used only by
// tests, the same test-only-override pattern as indexOptions.DBPath and
// statsOptions.DBPath elsewhere in this package -- production callers
// (runFind) always leave it empty.
type findHistoryOptions struct {
	Project string
	JSON    bool
	DBPath  string
}

// runFindHistory renders `harnez find issues history` (issue 228): the
// open/closed/draft/unknown ticket-count snapshots 'harnez index' has
// recorded into the telemetry DB, oldest first, optionally filtered to one
// project_name.
func runFindHistory(w io.Writer, opts findHistoryOptions) error {
	dbPath := opts.DBPath
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("find issues history: %w", err)
		}
		dbPath = p
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("find issues history: open telemetry db: %w", err)
	}
	defer db.Close()

	snapshots, err := db.QueryIssueSnapshots(opts.Project)
	if err != nil {
		return fmt.Errorf("find issues history: %w", err)
	}

	if opts.JSON {
		if snapshots == nil {
			snapshots = []telemetry.IssueStatusSnapshot{}
		}
		data, err := json.MarshalIndent(snapshots, "", "  ")
		if err != nil {
			return fmt.Errorf("find issues history: %w", err)
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(snapshots) == 0 {
		fmt.Fprintln(w, "no data: no issue status snapshots recorded yet (run 'harnez index' first)")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tCREATED_AT\tOPEN\tCLOSED\tDRAFT\tUNKNOWN")
	for _, s := range snapshots {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\n",
			s.ProjectName, s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			s.OpenCount, s.ClosedCount, s.DraftCount, s.UnknownCount)
	}
	return tw.Flush()
}

func runFindNext(w io.Writer, dir string, jsonOutput bool) error {
	issuesDir := filepath.Join(dir, "issues")
	num, _, err := issues.NextNumber(issuesDir)
	if err != nil {
		return fmt.Errorf("next issue number: %w", err)
	}
	if jsonOutput {
		data, err := json.Marshal(nextResultJSON{
			Number:   num,
			Reserved: false,
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}
	fmt.Fprintln(w, num)
	return nil
}

func runFind(w io.Writer, dir string, args []string, nextFlag, jsonOutput bool, historyProject string) error {
	if len(args) > 1 && strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
		return fmt.Errorf("find: query must not be empty")
	}
	return runFindWithOptions(w, io.Discard, args, findRunOptions{
		Dir:            dir,
		Next:           nextFlag,
		JSON:           jsonOutput,
		Raw:            true,
		HistoryProject: historyProject,
		All:            true,
	})
}

// isTerminalWriter detects if w is an interactive terminal TTY.
func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
			return true
		}
	}
	return false
}

func runFindWithOptions(w, errW io.Writer, args []string, opts findRunOptions) error {
	entity := args[0]
	if entity != "issues" {
		return fmt.Errorf("find: unsupported entity %q (only \"issues\" is supported)", entity)
	}

	// Keep these convenience verbs as read-only aliases over the existing
	// query engine. `last` deliberately uses the normal unfiltered listing so
	// its ordering and limit stay identical to the established default.
	if len(args) > 1 {
		switch args[1] {
		case "last":
			if len(args) > 2 {
				return fmt.Errorf("find issues last: accepts no query terms")
			}
			args = args[:1]
		case "open", "blocked", "closed", "draft":
			args = append([]string{"issues", "status:" + args[1]}, args[2:]...)
		}
	}

	// Handle `harnez find issues history ...` subcommand syntax
	if len(args) > 1 && args[1] == "history" {
		return runFindHistory(w, findHistoryOptions{Project: opts.HistoryProject, JSON: opts.JSON})
	}

	// Handle `harnez find issues next` subcommand syntax
	if len(args) > 1 && args[1] == "next" {
		return runFindNext(w, opts.Dir, opts.JSON)
	}

	// Handle flag form: `harnez find issues --next`
	if opts.Next {
		return runFindNext(w, opts.Dir, opts.JSON)
	}

	query := strings.TrimSpace(strings.Join(args[1:], " "))
	var q *find.Query
	if query == "" {
		q = &find.Query{}
	} else {
		var err error
		q, err = find.ParseQuery(query)
		if err != nil {
			return err
		}
	}

	issuesDir := filepath.Join(opts.Dir, "issues")
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}
	for i := range files {
		files[i].RelPath = filepath.ToSlash(filepath.Join("issues", files[i].RelPath))
	}

	results := find.Search(files, q)
	total := len(results)
	limit := opts.Limit
	if !opts.All && limit > 0 && total > limit {
		if len(q.Groups) == 0 {
			results = results[total-limit:]
			if errW != nil {
				fmt.Fprintf(errW, "# %d matches, showing last %d (use --all to show all)\n", total, limit)
			}
		} else {
			results = results[:limit]
			if errW != nil {
				fmt.Fprintf(errW, "# %d matches, showing top %d (use --all to show all)\n", total, limit)
			}
		}
	}

	if opts.JSON {
		if results == nil {
			results = []find.Result{}
		}
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	// Check if visual card output is desired:
	// Explicit --image/-I, or default when in interactive TTY and not opted out with --raw/--text.
	useVisualCard := opts.Image || (!opts.Raw && !opts.Text && isTerminalWriter(w))
	if useVisualCard && len(results) > 0 {
		var items []readcard.IssueCardItem
		for _, r := range results {
			items = append(items, readcard.IssueCardItem{
				Number:     r.Number,
				RawStatus:  r.RawStatus,
				PlainTitle: r.PlainTitle,
				Path:       r.Path,
			})
		}
		title := "Issue Tracker Discovery"
		if query != "" {
			title = fmt.Sprintf("Issues matching: %s", query)
		}
		renderRes, err := readcard.RenderIssuesMatrixCard(items, readcard.IssueMatrixOptions{
			Title:    title,
			FontName: "pixel",
		})
		if err == nil {
			fmt.Fprintf(w, "🖼️ Rendered: %s (%dx%d px, %d issues)\n", renderRes.PrimaryPath, renderRes.Width, renderRes.Height, len(items))
			fmt.Fprintf(w, "Token Breakdown: ~%d ViT tokens (Claude) vs ~%d text tokens\n", renderRes.TokenStats.ClaudeTokens, renderRes.TokenStats.TextTokens)
			return nil
		}
	}

	for _, r := range results {
		fmt.Fprintln(w, find.FormatTSV(r))
	}
	return nil
}
