// index implements `harnez index`, which regenerates the hand-maintained
// issues/README.md ticket table and docs/README.md docs/studies/ table from
// their source files. See issues/148-harnez-index-command-for-issues-docs-studies.md.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/index"
)

func newIndexCmd() *cobra.Command {
	var dir string
	var check bool

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Regenerate issues/README.md and docs/README.md's studies table from source files",
		Long: `index regenerates two hand-maintained index tables from the files that are
their actual source of truth, so they stop drifting:

  issues/README.md   <- issues/*.md + issues/archive/*.md metadata
  docs/README.md      <- docs/studies/*.md (docs/studies/ table only)

It is idempotent: run against unchanged sources, it reports no changes.
Pass --check to fail (exit 1) instead of writing, for CI/pre-commit use --
mirrors 'harnez diff --exit-code' (issue 037).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndex(dir, check)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/ and docs/")
	cmd.Flags().BoolVar(&check, "check", false, "report drift without writing; exit 1 if regeneration would change a file")
	return cmd
}

func runIndex(dir string, check bool) error {
	issuesReadme := filepath.Join(dir, "issues", "README.md")
	issuesDir := filepath.Join(dir, "issues")
	docsReadme := filepath.Join(dir, "docs", "README.md")
	docsDir := filepath.Join(dir, "docs")

	if check {
		return runIndexCheck(issuesReadme, issuesDir, docsReadme, docsDir)
	}

	issuesChanged, err := index.UpdateIssuesReadme(issuesReadme, issuesDir)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	docsChanged, err := index.UpdateDocsReadme(docsReadme, docsDir)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}

	printIndexResult(issuesReadme, issuesChanged)
	printIndexResult(docsReadme, docsChanged)
	return nil
}

func printIndexResult(path string, changed bool) {
	if changed {
		fmt.Printf("updated %s\n", path)
	} else {
		fmt.Printf("%s up to date\n", path)
	}
}

// runIndexCheck regenerates each table into the real file, using its
// returned changed bool to report drift, then restores the original bytes
// so --check never has a side effect on disk (mirrors the compare-before-
// write pattern; a temp-file compare would avoid the touch/restore
// round-trip, but --check is not on any latency-sensitive path, and this
// keeps the check path reusing the exact same write-and-compare logic
// runIndex uses instead of a second, divergent implementation). On drift
// it exits 1 after reporting, mirroring `harnez diff --exit-code` (037).
func runIndexCheck(issuesReadme, issuesDir, docsReadme, docsDir string) error {
	drift := false

	for _, t := range []struct {
		path   string
		update func() (bool, error)
	}{
		{issuesReadme, func() (bool, error) { return index.UpdateIssuesReadme(issuesReadme, issuesDir) }},
		{docsReadme, func() (bool, error) { return index.UpdateDocsReadme(docsReadme, docsDir) }},
	} {
		orig, err := os.ReadFile(t.path)
		if err != nil {
			return fmt.Errorf("index --check: read %s: %w", t.path, err)
		}
		changed, err := t.update()
		if err != nil {
			return fmt.Errorf("index --check: %w", err)
		}
		if changed {
			drift = true
			fmt.Printf("would update %s\n", t.path)
			if err := os.WriteFile(t.path, orig, 0o644); err != nil {
				return fmt.Errorf("index --check: restore %s: %w", t.path, err)
			}
		} else {
			fmt.Printf("%s up to date\n", t.path)
		}
	}

	if drift {
		os.Exit(1)
	}
	return nil
}
