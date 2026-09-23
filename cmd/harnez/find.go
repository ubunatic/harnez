// find implements `harnez find [options] <query...>`, supporting both
// issue tracker discovery (`harnez find issues ...`) and token-efficient
// architectural code discovery (`harnez find --broad "query"`).
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
	var broadFlag bool
	var maxTokensFlag int
	var noIndexFlag bool
	var nextFlag bool
	var jsonFlag bool
	var rawFlag bool
	var textFlag bool
	var imageFlag bool
	var historyProjectFlag string
	var limitFlag int
	var allFlag bool

	cmd := &cobra.Command{
		Use:   "find [flags] <query...>",
		Short: "Query repository issues or architectural code concepts (--broad)",
		Long: `find searches repository-data entities or performs architectural code discovery.

Architectural Code Discovery (--broad):
  harnez find -b "auth and login logic"
  harnez find --broad --json "session management"

  Searches Go declarations, signatures, package docs, Markdown docs, and infers
  architectural patterns (JWT, cookies, SQL, etc.) using token-efficient indexing.

Issue Tracker Discovery:
  harnez find issues [query...]
  harnez find issues status:open "login"
  harnez find issues next
  harnez find issues history`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limitFlag <= 0 {
				return fmt.Errorf("find: --limit must be greater than zero")
			}
			opts := findRunOptions{
				Dir:            dir,
				Broad:          broadFlag,
				MaxTokens:      maxTokensFlag,
				NoIndex:        noIndexFlag,
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
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/ or code")
	cmd.Flags().BoolVarP(&broadFlag, "broad", "b", false, "enable architectural code discovery mode")
	cmd.Flags().IntVarP(&maxTokensFlag, "max-tokens", "m", 800, "token budget limit for output")
	cmd.Flags().BoolVar(&noIndexFlag, "no-index", false, "force on-the-fly scan bypassing .harnez/index.json")
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
	Broad          bool
	MaxTokens      int
	NoIndex        bool
	Next           bool
	JSON           bool
	Raw            bool
	Text           bool
	Image          bool
	HistoryProject string
	Limit          int
	All            bool
}

type findHistoryOptions struct {
	Project string
	JSON    bool
	DBPath  string
}

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
	if opts.Broad || (len(args) > 0 && args[0] != "issues") {
		return runBroadFind(w, errW, args, opts)
	}

	entity := args[0]
	if entity != "issues" {
		return fmt.Errorf("find: unsupported entity %q (only \"issues\" is supported, or use --broad)", entity)
	}

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

	if len(args) > 1 && args[1] == "history" {
		return runFindHistory(w, findHistoryOptions{Project: opts.HistoryProject, JSON: opts.JSON})
	}

	if len(args) > 1 && args[1] == "next" {
		return runFindNext(w, opts.Dir, opts.JSON)
	}

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

func runBroadFind(w, errW io.Writer, args []string, opts findRunOptions) error {
	queryArgs := args
	if len(args) > 0 && args[0] == "issues" {
		queryArgs = args[1:]
	}
	query := strings.TrimSpace(strings.Join(queryArgs, " "))

	var chunks []find.Chunk
	var err error

	if !opts.NoIndex {
		payload, loadErr := find.LoadIndex(opts.Dir)
		if loadErr == nil {
			chunks = payload.Chunks
		} else {
			if errW != nil {
				fmt.Fprintf(errW, "Warning: .harnez/index.json missing or invalid, falling back to on-the-fly scan (%v)\n", loadErr)
			}
		}
	}

	if len(chunks) == 0 {
		chunks, err = find.ScanRepo(opts.Dir)
		if err != nil {
			return fmt.Errorf("scan repo: %w", err)
		}
	}

	res := find.SearchChunks(chunks, query)

	maxChars := opts.MaxTokens * 4
	if maxChars <= 0 {
		maxChars = 3200
	}

	if opts.JSON {
		return renderBroadJSON(w, res, maxChars)
	}

	return renderBroadMarkdown(w, res, maxChars)
}

func renderBroadMarkdown(w io.Writer, res find.DiscoveryResult, maxChars int) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Architectural Discovery: %s\n\n", res.Query))

	sb.WriteString("## Packages\n")
	if len(res.Packages) == 0 {
		sb.WriteString("- (none)\n")
	} else {
		for _, pkg := range res.Packages {
			sb.WriteString(fmt.Sprintf("- %s\n", pkg))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Detected Patterns\n")
	if len(res.Patterns) == 0 {
		sb.WriteString("- (none)\n")
	} else {
		for _, pat := range res.Patterns {
			sb.WriteString(fmt.Sprintf("- %s\n", pat))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Key Declarations\n")
	omittedDecls := 0
	if len(res.Declarations) == 0 {
		sb.WriteString("- (none)\n")
	} else {
		for i, decl := range res.Declarations {
			entry := fmt.Sprintf("- `%s` (%s:%d-%d)\n  %s\n",
				decl.Identifier, decl.FilePath, decl.LineStart, decl.LineEnd,
				strings.ReplaceAll(decl.Summary, "\n", "\n  "))
			if sb.Len()+len(entry)+60 > maxChars {
				omittedDecls = len(res.Declarations) - i
				break
			}
			sb.WriteString(entry)
		}
		if omittedDecls > 0 {
			sb.WriteString(fmt.Sprintf("[... %d additional entries omitted]\n", omittedDecls))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Relevant Documentation\n")
	omittedDocs := 0
	if len(res.Docs) == 0 {
		sb.WriteString("- (none)\n")
	} else {
		for i, doc := range res.Docs {
			entry := fmt.Sprintf("- `%s` (%s:%d-%d)\n  %s\n",
				doc.Identifier, doc.FilePath, doc.LineStart, doc.LineEnd,
				strings.ReplaceAll(doc.Summary, "\n", "\n  "))
			if sb.Len()+len(entry)+60 > maxChars {
				omittedDocs = len(res.Docs) - i
				break
			}
			sb.WriteString(entry)
		}
		if omittedDocs > 0 {
			sb.WriteString(fmt.Sprintf("[... %d additional entries omitted]\n", omittedDocs))
		}
	}

	output := sb.String()
	if len(output) > maxChars {
		output = output[:maxChars] + "\n[... additional entries omitted]\n"
	}

	fmt.Fprintln(w, output)
	return nil
}

func renderBroadJSON(w io.Writer, res find.DiscoveryResult, maxChars int) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("render json: %w", err)
	}

	if len(data) > maxChars {
		// Truncate declarations and docs iteratively
		for len(res.Declarations) > 0 || len(res.Docs) > 0 {
			if len(res.Declarations) > len(res.Docs) && len(res.Declarations) > 0 {
				res.Declarations = res.Declarations[:len(res.Declarations)-1]
			} else if len(res.Docs) > 0 {
				res.Docs = res.Docs[:len(res.Docs)-1]
			}
			data, _ = json.MarshalIndent(res, "", "  ")
			if len(data) <= maxChars {
				break
			}
		}
	}

	fmt.Fprintln(w, string(data))
	return nil
}
