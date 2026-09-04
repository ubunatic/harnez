// find implements `harnez find <entity> [options] <query...>`, a deliberate
// query command over repository-data entities. The only entity in v1 is
// "issues" (issues/*.md and issues/archive/*.md); see
// issues/158-find-entity-query-command.md for the full query grammar,
// ranking, and output contract.
// It also provides `harnez find issues next` (issue 194) to calculate
// and reserve the next free issue number.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/issues"
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
	var reserveFlag string
	var jsonFlag bool
	var historyProjectFlag string

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
  harnez find issues next [--reserve [title]] [--json]
           Compute the next free ticket number (max+1, formatted with 3+ digits).
           When --reserve is supplied, atomically writes a Draft placeholder
           ticket file (issues/<NNN>-reserved.md or issues/<NNN>-<title-slug>.md)
           so concurrent callers do not receive colliding numbers. Non-JSON
           output is "<NUMBER>\t<PATH>" so callers write the ticket's real
           content directly to the reserved path instead of re-deriving the
           slug from the title (which can diverge, leaving an orphaned
           placeholder behind -- see issue 202).

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

Output is deterministic, tab-separated, one result per line, no header, no
ANSI:

  NUMBER<TAB>RAW_STATUS<TAB>PLAIN_TITLE<TAB>PATH

Zero matches exits 0 and prints nothing. An invalid entity, an invalid
query, or unreadable/malformed tracker data exits non-zero with an
actionable stderr message.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFind(cmd.OutOrStdout(), dir, args, nextFlag, cmd.Flags().Changed("reserve"), reserveFlag, jsonFlag, historyProjectFlag)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/")
	cmd.Flags().BoolVar(&nextFlag, "next", false, "report or reserve the next free issue number")
	cmd.Flags().StringVar(&reserveFlag, "reserve", "", "reserve the next free issue number with an optional title")
	cmd.Flags().Lookup("reserve").NoOptDefVal = " "
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
	cmd.Flags().StringVar(&historyProjectFlag, "project", "", "with 'history': filter to one project_name")

	return cmd
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

func runFindNext(w io.Writer, dir string, reserve bool, title string, jsonOutput bool) error {
	issuesDir := filepath.Join(dir, "issues")
	if reserve {
		num, filename, err := issues.Reserve(issuesDir, issues.ReserveOptions{Title: strings.TrimSpace(title)})
		if err != nil {
			return fmt.Errorf("reserve issue: %w", err)
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

func runFind(w io.Writer, dir string, args []string, nextFlag, hasReserveFlag bool, reserveTitle string, jsonOutput bool, historyProject string) error {
	entity := args[0]
	if entity != "issues" {
		return fmt.Errorf("find: unsupported entity %q (only \"issues\" is supported)", entity)
	}

	// Handle `harnez find issues history ...` subcommand syntax
	if len(args) > 1 && args[1] == "history" {
		return runFindHistory(w, findHistoryOptions{Project: historyProject, JSON: jsonOutput})
	}

	// Handle `harnez find issues next ...` subcommand syntax
	if len(args) > 1 && args[1] == "next" {
		reserve := hasReserveFlag
		title := reserveTitle
		// If additional arguments are provided after 'next', e.g. `harnez find issues next --reserve "My Title"`
		// or `harnez find issues next "My Title"` (if reserve flag is set)
		if len(args) > 2 {
			extra := strings.TrimSpace(strings.Join(args[2:], " "))
			if extra != "" && strings.TrimSpace(title) == "" {
				title = extra
			}
		}
		return runFindNext(w, dir, reserve, title, jsonOutput)
	}

	// Handle flags on `harnez find issues --next` or `harnez find issues --reserve`
	if nextFlag || hasReserveFlag {
		return runFindNext(w, dir, hasReserveFlag, reserveTitle, jsonOutput)
	}

	query := strings.TrimSpace(strings.Join(args[1:], " "))
	if query == "" {
		return fmt.Errorf("find: query must not be empty")
	}

	q, err := find.ParseQuery(query)
	if err != nil {
		return err
	}

	issuesDir := filepath.Join(dir, "issues")
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}
	for i := range files {
		files[i].RelPath = filepath.ToSlash(filepath.Join("issues", files[i].RelPath))
	}

	for _, r := range find.Search(files, q) {
		fmt.Fprintln(w, find.FormatTSV(r))
	}
	return nil
}

