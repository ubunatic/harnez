// dochistory implements `harnez dochistory [files...]` (alias `doc-history`),
// analyzing Git document history, tracking token evolution, metrics, and category
// distributions across commits. See issues/192-integrate-git-doc-history-cli-command.md.
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
	Targets []string
}

func newDocHistoryCmd() *cobra.Command {
	var dir string
	var color bool = true
	var noColor bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "dochistory [files...]",
		Aliases: []string{"doc-history"},
		Short:   "Analyze Git document token evolution and category history",
		Long: `dochistory analyzes Git document history, tracking token evolution,
metrics, and category distributions across commits.

Invocations:
  harnez dochistory                    # Analyze default managed documentation (docs/ and AGENTS.md)
  harnez dochistory docs/lang/Go.md    # Single-file timeline and token sparkline
  harnez dochistory docs/ issues       # Multi-directory / glob stacked category breakdown
  harnez dochistory --json             # Machine-readable JSON output`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocHistory(cmd.OutOrStdout(), docHistoryOptions{
				Dir:     dir,
				Color:   color,
				NoColor: noColor,
				JSON:    jsonOut,
				Targets: args,
			})
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "repository root directory (default: current directory or git root)")
	cmd.Flags().BoolVar(&color, "color", true, "enable ANSI color output")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI color output")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output history in JSON format")

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
