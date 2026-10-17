package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/lint"
)

type lintOptions struct {
	Lang    string
	Check   string // "" = report only; otherwise the lowest failing severity
	JSON    bool
	Verbose bool
}

func newLintCmd() *cobra.Command {
	var opts lintOptions

	cmd := &cobra.Command{
		Use:   "lint [flags] [file|dir]...",
		Short: "Check files against harnez repository rules and conventions",
		Long: `lint verifies source files, scripts, and documentation against harnez
standards and invariants (e.g. Bash 3-line conditionals and source-over-dot,
Markdown and Makefile marker integrity).

With no arguments, lint checks every git-tracked file it recognises; a
directory argument checks the tracked files below it. Explicit files are
linted even when untracked.

Language is automatically inferred from file extensions or shebangs, or can
be overridden explicitly with --lang. In directory mode, --lang selects only
files of that language.

Without --check, lint reports and exits 0. --check fails on errors;
--check=warning also fails on warnings.

Errors are always listed. When linting directories (or the whole repo),
warnings are summarised by rule and by the files with the most warnings;
--verbose lists every warning. --json always includes every finding.`,
		Example: `  harnez lint                      # all tracked files
  harnez lint --check scripts docs # gate on errors below two dirs
  harnez lint --check=warning --lang go`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd.OutOrStdout(), cmd.ErrOrStderr(), args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Lang, "lang", "auto", "language override (auto, bash, go, make, markdown)")
	cmd.Flags().StringVar(&opts.Check, "check", "", "exit non-zero on findings at or above this level (error, warning)")
	cmd.Flags().Lookup("check").NoOptDefVal = string(lint.SeverityError)
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output findings as JSON")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "list every warning instead of a summary in directory mode")

	return cmd
}

// lintTarget is one file to lint and the language to lint it as.
type lintTarget struct {
	path string
	lang lint.Language
}

// lintTargets expands args into files: explicit files stay as given and take
// lang as an override, while directories (or no args) expand to the
// git-tracked files they contain, filtered to lang and detected per file.
func lintTargets(args []string, lang lint.Language) ([]lintTarget, error) {
	var targets []lintTarget
	var dirs []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			dirs = append(dirs, arg)
		} else {
			targets = append(targets, lintTarget{arg, lang})
		}
	}
	if len(args) > 0 && len(dirs) == 0 {
		return targets, nil
	}
	tracked, err := lint.TrackedFiles(".", dirs)
	if err != nil {
		return nil, err
	}
	for _, f := range tracked {
		if matchesLang(f, lang) {
			targets = append(targets, lintTarget{f, lint.LangAuto})
		}
	}
	return targets, nil
}

// matchesLang keeps tracked files lint understands, of lang unless auto.
func matchesLang(path string, lang lint.Language) bool {
	detected := lint.DetectLanguage(path, nil)
	if detected == lint.LangUnknown && filepath.Ext(path) == "" {
		head := make([]byte, 128)
		if f, err := os.Open(path); err == nil {
			n, _ := f.Read(head)
			f.Close()
			detected = lint.DetectLanguage(path, head[:n])
		}
	}
	if detected == lint.LangUnknown {
		return false
	}
	return lang == lint.LangAuto || detected == lang
}

func runLint(out, errOut io.Writer, args []string, opts lintOptions) error {
	targetLang, err := lint.ParseLanguage(opts.Lang)
	if err != nil {
		return err
	}
	var failLevel lint.Severity
	if opts.Check != "" {
		if failLevel, err = lint.ParseSeverity(opts.Check); err != nil {
			return err
		}
	}
	targets, err := lintTargets(args, targetLang)
	if err != nil {
		return err
	}

	spec, err := lint.LoadSpec()
	if err != nil {
		return err
	}
	linter, err := lint.DefaultLinter()
	if err != nil {
		return err
	}
	allFindings := []lint.Finding{}
	for _, t := range targets {
		findings, err := linter.LintFile(t.path, t.lang)
		if err != nil {
			return err
		}
		allFindings = append(allFindings, findings...)
	}

	if opts.JSON {
		data, err := json.MarshalIndent(allFindings, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Fprintln(out, string(data))
	} else {
		listWarnings := opts.Verbose || !lintsDirs(args)
		writeLintReport(out, allFindings, len(targets), listWarnings, spec.Report.TopFiles)
	}

	if failLevel == "" {
		return nil
	}
	failing := 0
	for _, f := range allFindings {
		if lint.SeverityRank(f.Severity) >= lint.SeverityRank(failLevel) {
			failing++
		}
	}
	if failing > 0 {
		return fmt.Errorf("%d lint finding(s) at or above %s", failing, failLevel)
	}
	return nil
}

// lintsDirs reports whether args select directory mode (no args or any dir).
func lintsDirs(args []string) bool {
	for _, arg := range args {
		if info, err := os.Stat(arg); err == nil && info.IsDir() {
			return true
		}
	}
	return len(args) == 0
}

// writeLintReport lists findings and summarises them by rule. Errors are
// always listed; warnings are listed only when listWarnings is set, otherwise
// the files with the most warnings are shown.
func writeLintReport(out io.Writer, findings []lint.Finding, checked int, listWarnings bool, topFiles int) {
	var warnings []lint.Finding
	for _, f := range findings {
		if f.Severity == lint.SeverityWarning && !listWarnings {
			warnings = append(warnings, f)
			continue
		}
		fmt.Fprintln(out, lint.FormatFindingUnix(f))
	}
	if len(findings) == 0 {
		fmt.Fprintf(out, "%d file(s) checked, no issues found.\n", checked)
		return
	}
	fileCount := make(map[string]bool)
	for _, f := range findings {
		fileCount[f.File] = true
	}
	fmt.Fprintf(out, "\nFound %d lint issue(s) across %d of %d file(s).\n", len(findings), len(fileCount), checked)
	for _, rc := range lint.SummarizeByRule(findings) {
		fmt.Fprintf(out, "  %5d  %-22s %s\n", rc.Count, rc.RuleID, rc.Severity)
	}
	if len(warnings) == 0 {
		return
	}
	byFile := lint.TopFiles(warnings, len(warnings))
	shown := byFile[:min(topFiles, len(byFile))]
	fmt.Fprintf(out, "\nMost warnings (top %d of %d file(s); --verbose lists all):\n", len(shown), len(byFile))
	for _, fc := range shown {
		fmt.Fprintf(out, "  %5d  %s\n", fc.Count, fc.File)
	}
}
