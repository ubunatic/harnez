package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/lint"
)

type lintOptions struct {
	Lang  string
	Check bool
	JSON  bool
}

func newLintCmd() *cobra.Command {
	var opts lintOptions

	cmd := &cobra.Command{
		Use:   "lint [flags] <file>...",
		Short: "Check files against harnez repository rules and conventions",
		Long: `lint verifies source files, scripts, and documentation against harnez
standards and invariants (e.g. Bash 3-line conditionals and source-over-dot,
Markdown and Makefile marker integrity).

Language is automatically inferred from file extensions or shebangs, or can
be overridden explicitly with --lang.`,
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd.OutOrStdout(), cmd.ErrOrStderr(), args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Lang, "lang", "auto", "language override (auto, bash, go, make, markdown)")
	cmd.Flags().BoolVar(&opts.Check, "check", false, "exit with non-zero code on lint errors")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output findings as JSON")

	return cmd
}

func runLint(out, errOut io.Writer, files []string, opts lintOptions) error {
	targetLang, err := lint.ParseLanguage(opts.Lang)
	if err != nil {
		return err
	}

	linter := lint.DefaultLinter()
	var allFindings []lint.Finding
	checkedFiles := make(map[string]bool)

	for _, file := range files {
		findings, err := linter.LintFile(file, targetLang)
		if err != nil {
			return err
		}
		checkedFiles[file] = true
		allFindings = append(allFindings, findings...)
	}

	if opts.JSON {
		if allFindings == nil {
			allFindings = []lint.Finding{}
		}
		data, err := json.MarshalIndent(allFindings, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Fprintln(out, string(data))
	} else {
		for _, f := range allFindings {
			fmt.Fprintln(out, lint.FormatFindingUnix(f))
		}
		if len(allFindings) > 0 {
			fileCount := make(map[string]bool)
			for _, f := range allFindings {
				fileCount[f.File] = true
			}
			fmt.Fprintf(out, "\nFound %d lint issue(s) across %d file(s).\n", len(allFindings), len(fileCount))
		} else {
			fmt.Fprintf(out, "%d file(s) checked, no issues found.\n", len(checkedFiles))
		}
	}

	if len(allFindings) > 0 {
		return errors.New("lint issues found")
	}

	return nil
}
