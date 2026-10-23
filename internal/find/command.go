package find

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"ubunatic.com/harnez/internal/finder"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/readcard"
	"ubunatic.com/harnez/internal/telemetry"
)

// NextResultJSON is the JSON result shared by find and issue allocation commands.
type NextResultJSON struct {
	Number   string `json:"number"`
	Reserved bool   `json:"reserved"`
	File     string `json:"file,omitempty"`
	Path     string `json:"path,omitempty"`
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
	Via            string
	K              int
	JSONL          bool
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

// FindRunOptions bundles inputs for RunFindWithOptions.
type FindRunOptions = findRunOptions

// FindHistoryOptions bundles inputs for RunFindHistory.
type FindHistoryOptions = findHistoryOptions

// ExitCodeError requests a specific process exit code at the CLI boundary.
type ExitCodeError struct{ Code int }

// Error returns an empty string because the command has already written its diagnostic.
func (e *ExitCodeError) Error() string { return "" }

// RunFindWithOptions searches an entity and writes its results.
func RunFindWithOptions(w, errW io.Writer, args []string, opts FindRunOptions) error {
	return runFindWithOptions(w, errW, args, opts)
}

// RunFindHistory renders stored issue status snapshots.
func RunFindHistory(w io.Writer, opts FindHistoryOptions) error {
	return runFindHistory(w, opts)
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

	db, err := telemetry.OpenReadOnly(dbPath)
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
	fmt.Fprintln(tw, "PROJECT\tCREATED_AT\tOPEN\tCLOSED\tDRAFT\tBACKLOG\tUNKNOWN")
	for _, s := range snapshots {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
			s.ProjectName, s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			s.OpenCount, s.ClosedCount, s.DraftCount, s.BacklogCount, s.UnknownCount)
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
		data, err := json.Marshal(NextResultJSON{
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

// IsTerminalWriter detects if w is an interactive terminal TTY.
// IsTerminalWriter reports whether w is connected to a terminal.
func IsTerminalWriter(w io.Writer) bool {
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
	if entity == "code" || entity == "docs" {
		query := strings.TrimSpace(strings.Join(args[1:], " "))
		if query == "" {
			return fmt.Errorf("find %s: query must not be empty", entity)
		}
		resp, err := finder.Search(context.Background(), opts.Dir, entity, query, opts.Via, opts.K, errW)
		if err != nil {
			return err
		}
		if len(resp.Results) == 0 {
			for _, status := range resp.Finders {
				if status.Status != "ok" && status.Status != "partial" {
					// Search already wrote the finder diagnostics to stderr.
					return &ExitCodeError{Code: 2}
				}
			}
		}
		if opts.JSON || opts.JSONL {
			enc := json.NewEncoder(w)
			if opts.JSONL {
				for _, row := range resp.Results {
					if err := enc.Encode(row); err != nil {
						return err
					}
				}
				return nil
			}
			return enc.Encode(resp.Results)
		}
		for _, row := range resp.Results {
			fmt.Fprintf(w, "%s:%d\t%s\t%s\t%.4f\t%s\n", row.Path, row.Line, row.Title, row.Snippet, row.Score, row.Kind)
		}
		return nil
	}
	if entity != "issues" {
		return fmt.Errorf("find: unsupported entity %q (supported: issues, code, docs)", entity)
	}

	// Keep these convenience verbs as read-only aliases over the existing
	// query engine. `last` deliberately uses the normal unfiltered listing so
	// its ordering and limit stay identical to the established default.
	if len(args) > 1 {
		switch args[1] {
		case "similar":
			query := strings.TrimSpace(strings.Join(args[2:], " "))
			if query == "" {
				return fmt.Errorf("find issues similar: title must not be empty")
			}
			return runFindSimilar(w, opts, query)
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
	var q *Query
	if query == "" {
		q = &Query{}
	} else {
		var err error
		q, err = ParseQuery(query)
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

	results := Search(files, q)
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
			results = []Result{}
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
	useVisualCard := opts.Image || (!opts.Raw && !opts.Text && IsTerminalWriter(w))
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
		fmt.Fprintln(w, FormatTSV(r))
	}
	return nil
}

func runFindSimilar(w io.Writer, opts findRunOptions, query string) error {
	files, err := issues.Scan(filepath.Join(opts.Dir, "issues"))
	if err != nil {
		return fmt.Errorf("find issues similar: %w", err)
	}
	for i := range files {
		files[i].RelPath = filepath.ToSlash(filepath.Join("issues", files[i].RelPath))
	}
	results, err := Similar(files, query)
	if err != nil {
		return err
	}
	if opts.JSON {
		if results == nil {
			results = []SimilarResult{}
		}
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("find issues similar: render json: %w", err)
		}
		fmt.Fprintln(w, string(data))
		return nil
	}
	for _, result := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%.3f\t%s\n", result.Number, result.RawStatus, result.PlainTitle, result.Score, result.Path)
	}
	return nil
}
