// dochistory implements `harnez dochistory [files...]` (alias `doc-history`, `repo-history`),
// analyzing Git document history, tracking token evolution, metrics, and category
// distributions across commits. See issues/192-integrate-git-doc-history-cli-command.md
// and issues/376-multi-track-git-history-evolution-sparks-across-code-tests-docs-skills-and-issues.md.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/assess"
)

type docHistoryOptions struct {
	Dir     string
	Color   bool
	NoColor bool
	JSON    bool
	Tracks  bool
	Diff    bool
	Targets []string
}

func newDocHistoryCmd() *cobra.Command {
	var dir string
	var color bool = true
	var noColor bool
	var jsonOut bool
	var tracks bool
	var diff bool

	cmd := &cobra.Command{
		Use:     "dochistory [files...]",
		Aliases: []string{"doc-history", "repo-history"},
		Short:   "Analyze Git document and multi-track repository evolution history",
		Long: `dochistory analyzes Git history, tracking token evolution,
metrics, multi-track categories (code, tests, docs, skills, issues), and distributions across commits.

Invocations:
  harnez dochistory                    # Analyze default managed documentation (docs/ and AGENTS.md)
  harnez dochistory --tracks           # Multi-track repository evolution sparks (code, tests, docs, skills, issues)
  harnez dochistory --tracks --diff    # Expanded additions and removals breakdown table
  harnez dochistory docs/lang/Go.md    # Single-file timeline and token sparkline
  harnez dochistory docs/ issues       # Multi-directory / glob stacked category breakdown
  harnez dochistory --json             # Machine-readable JSON output`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			isTracks := tracks
			if diff && len(args) == 0 {
				isTracks = true
			}
			return runDocHistory(cmd.OutOrStdout(), docHistoryOptions{
				Dir:     dir,
				Color:   color,
				NoColor: noColor,
				JSON:    jsonOut,
				Tracks:  isTracks,
				Diff:    diff,
				Targets: args,
			})
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "repository root directory (default: current directory or git root)")
	cmd.Flags().BoolVar(&color, "color", true, "enable ANSI color output")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI color output")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output history in JSON format")
	cmd.Flags().BoolVar(&tracks, "tracks", false, "analyze multi-track repository evolution (code, tests, docs, skills, issues)")
	cmd.Flags().BoolVar(&diff, "diff", false, "show additions and removals breakdown in multi-track history")
	cmd.Flags().BoolVar(&diff, "diffs", false, "alias for --diff")
	_ = cmd.Flags().MarkHidden("diffs")

	return cmd
}

func newRepoHistoryCmd() *cobra.Command {
	var dir string
	var color bool = true
	var noColor bool
	var jsonOut bool
	var diff bool

	cmd := &cobra.Command{
		Use:   "repo-history",
		Short: "Multi-track Git history evolution sparks across code, tests, docs, skills, and issues",
		Long: `repo-history extracts commit-by-commit multi-track evolution sparks across
the 5 functional tracks: code, tests, docs, agent skills, and issue tickets.

Invocations:
  harnez repo-history        # Terminal multi-track evolution card
  harnez repo-history --diff # Expanded additions and removals breakdown table
  harnez repo-history --json # Structured JSON evolution time-series`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocHistory(cmd.OutOrStdout(), docHistoryOptions{
				Dir:     dir,
				Color:   color,
				NoColor: noColor,
				JSON:    jsonOut,
				Tracks:  true,
				Diff:    diff,
			})
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "repository root directory (default: current directory or git root)")
	cmd.Flags().BoolVar(&color, "color", true, "enable ANSI color output")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI color output")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output history in JSON format")
	cmd.Flags().BoolVar(&diff, "diff", false, "show additions and removals breakdown in multi-track history")
	cmd.Flags().BoolVar(&diff, "diffs", false, "alias for --diff")
	_ = cmd.Flags().MarkHidden("diffs")

	return cmd
}

func isSingleRegularFile(repoDir, target string) bool {
	if strings.ContainsAny(target, "*?[]") {
		return false
	}
	fullPath := target
	if repoDir != "" && !filepath.IsAbs(target) {
		fullPath = filepath.Join(repoDir, target)
	}
	fi, err := os.Stat(fullPath)
	if err == nil {
		return !fi.IsDir()
	}
	if strings.HasSuffix(target, "/") || strings.HasSuffix(target, string(filepath.Separator)) {
		return false
	}
	return true
}

func runDocHistory(w io.Writer, opts docHistoryOptions) error {
	useColor := opts.Color && !opts.NoColor

	// Multi-track mode
	if opts.Tracks {
		res, err := assess.ExtractMultiTrackHistory(opts.Dir)
		if err != nil {
			return fmt.Errorf("dochistory --tracks: %w", err)
		}
		if opts.JSON {
			data, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return fmt.Errorf("render json: %w", err)
			}
			fmt.Fprintln(w, string(data))
			return nil
		}
		if opts.Diff {
			fmt.Fprint(w, assess.RenderMultiTrackHistoryTableWithDiffs(res, assess.RenderTracksOptions{Color: useColor}))
			return nil
		}
		fmt.Fprint(w, assess.RenderMultiTrackCard(res, assess.RenderMultiTrackCardOptions{Color: useColor}))
		return nil
	}

	// Single regular file invocation
	if len(opts.Targets) == 1 && isSingleRegularFile(opts.Dir, opts.Targets[0]) {
		filePath := opts.Targets[0]
		res, err := assess.ExtractDocHistory(opts.Dir, filePath)
		if err != nil {
			return fmt.Errorf("dochistory %s: %w", filePath, err)
		}
		if opts.JSON {
			data, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return fmt.Errorf("render json: %w", err)
			}
			fmt.Fprintln(w, string(data))
			return nil
		}
		fmt.Fprint(w, assess.RenderDocHistoryTable(res))
		return nil
	}

	// Multi-file / directory / globs / default invocation
	res, err := assess.ExtractMultiDocHistory(opts.Dir, opts.Targets)
	if err != nil {
		return fmt.Errorf("dochistory: %w", err)
	}
	if opts.JSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		fmt.Fprintln(w, string(data))
		return nil
	}
	fmt.Fprint(w, assess.RenderMultiDocHistory(res, assess.RenderMultiDocOptions{Color: useColor}))
	return nil
}
