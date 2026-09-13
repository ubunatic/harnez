// index implements `harnez index`, which regenerates the hand-maintained
// issues/README.md ticket table and docs/README.md docs/studies/ table from
// their source files. See issues/148-harnez-index-command-for-issues-docs-studies.md.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/index"
	"ubunatic.com/harnez/internal/telemetry"
)

// indexOptions bundles runIndex's inputs. DBPath is a telemetry DB path
// override, consulted only by the issue-snapshot write path (issue 228);
// it is not exposed as a CLI flag (production always uses
// telemetry.DefaultDBPath()) -- it exists so tests can point snapshot
// writes at a throwaway DB file instead of the user's real one, the same
// override-for-tests-only pattern statsOptions.DBPath and execOptions.DBPath
// already use elsewhere in this package.
type indexOptions struct {
	Dir    string
	Check  bool
	DBPath string
}

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
mirrors 'harnez diff --exit-code' (issue 037). --check also prints a unified
diff of the drift, so the output is self-contained enough for an agent
running the command directly in-session to see exactly what changed and
fix the offending ticket(s) without a separate diff step.

Each run that isn't --check also appends a dated open/closed/draft/unknown
ticket count snapshot for this repo to the shared telemetry DB (issue 228),
deduped against the project's most recently recorded snapshot so repeated
no-op runs don't grow the history unboundedly. Snapshot-write failures are
logged (DEBUG=1) and otherwise swallowed -- they never fail this command.
View recorded snapshots with 'harnez find issues history'.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return silenceIfExitCode(cmd, runIndex(cmd.OutOrStdout(), indexOptions{Dir: dir, Check: check}))
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/ and docs/")
	cmd.Flags().BoolVar(&check, "check", false, "report drift without writing; exit 1 if regeneration would change a file")
	return cmd
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func runIndex(w io.Writer, opts indexOptions) error {
	dir := opts.Dir
	issuesReadme := filepath.Join(dir, "issues", "README.md")
	issuesDir := filepath.Join(dir, "issues")
	docsReadme := filepath.Join(dir, "docs", "README.md")
	docsDir := filepath.Join(dir, "docs")

	hasIssues := dirExists(issuesDir) || pathExists(issuesReadme)
	hasDocs := dirExists(filepath.Join(docsDir, "studies")) && pathExists(docsReadme)

	if !hasIssues && !hasDocs {
		return fmt.Errorf("index: no issues/ or docs/studies/ found in %s", dir)
	}

	if opts.Check {
		return runIndexCheck(w, issuesReadme, issuesDir, docsReadme, docsDir, hasIssues, hasDocs)
	}

	if hasIssues {
		issuesChanged, err := index.UpdateIssuesReadme(issuesReadme, issuesDir)
		if err != nil {
			return fmt.Errorf("index: %w", err)
		}
		printIndexResult(w, issuesReadme, issuesChanged)
		recordIssueSnapshot(issuesDir, opts.DBPath)
	}
	if hasDocs {
		docsChanged, err := index.UpdateDocsReadme(docsReadme, docsDir)
		if err != nil {
			return fmt.Errorf("index: %w", err)
		}
		printIndexResult(w, docsReadme, docsChanged)
	}
	return nil
}

// recordIssueSnapshot computes the current open/closed/draft/unknown ticket
// counts under issuesDir and appends a deduped snapshot row to the
// telemetry DB (issue 228). It is best-effort by design, per this ticket's
// "snapshot writes must not fail index" requirement: every failure path
// logs via debugLog (DEBUG=1) and returns without propagating an error to
// runIndex's caller.
func recordIssueSnapshot(issuesDir, dbPath string) {
	open, closed, draft, unknown, err := index.StatusCounts(issuesDir)
	if err != nil {
		debugLog("index: compute issue status counts failed, dropping snapshot: %v", err)
		return
	}

	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			debugLog("index: DefaultDBPath failed, dropping issue snapshot: %v", err)
			return
		}
		dbPath = p
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		debugLog("index: open telemetry db failed, dropping issue snapshot: %v", err)
		return
	}
	defer db.Close()

	project := projectNameForDir(issuesDir)
	inserted, err := db.InsertIssueSnapshot(telemetry.IssueStatusSnapshot{
		ProjectName:  project,
		OpenCount:    open,
		ClosedCount:  closed,
		DraftCount:   draft,
		UnknownCount: unknown,
	})
	if err != nil {
		debugLog("index: insert issue snapshot failed: %v", err)
		return
	}
	if inserted {
		debugLog("index: recorded issue snapshot project=%q open=%d closed=%d draft=%d unknown=%d",
			project, open, closed, draft, unknown)
	} else {
		debugLog("index: skipped issue snapshot for project=%q (unchanged since last recorded snapshot)", project)
	}
}

// projectNameForDir derives the stable project identity (issue 227's
// project_name convention: filepath.Base of the repo directory) from
// issuesDir, which is <repo>/issues -- so its parent is the repo root
// regardless of what -d was passed as (relative, absolute, or ".").
func projectNameForDir(issuesDir string) string {
	repoRoot := filepath.Dir(issuesDir)
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return filepath.Base(repoRoot)
	}
	return filepath.Base(abs)
}

func printIndexResult(w io.Writer, path string, changed bool) {
	if changed {
		fmt.Fprintf(w, "updated %s\n", path)
	} else {
		fmt.Fprintf(w, "%s up to date\n", path)
	}
}

// runIndexCheck regenerates each table into the real file, using its
// returned changed bool to report drift, then restores the original bytes
// so --check never has a side effect on disk (mirrors the compare-before-
// write pattern; a temp-file compare would avoid the touch/restore
// round-trip, but --check is not on any latency-sensitive path, and this
// keeps the check path reusing the exact same write-and-compare logic
// runIndex uses instead of a second, divergent implementation). On drift
// it prints a unified diff of exactly what changed -- so an agent running
// this command directly in a session sees the specifics inline and can act
// on them -- then exits 1, mirroring `harnez diff --exit-code` (037).
func runIndexCheck(w io.Writer, issuesReadme, issuesDir, docsReadme, docsDir string, hasIssues, hasDocs bool) error {
	drift := false

	type target struct {
		path   string
		update func() (bool, error)
	}
	var targets []target

	if hasIssues {
		targets = append(targets, target{
			path:   issuesReadme,
			update: func() (bool, error) { return index.UpdateIssuesReadme(issuesReadme, issuesDir) },
		})
	}
	if hasDocs {
		targets = append(targets, target{
			path:   docsReadme,
			update: func() (bool, error) { return index.UpdateDocsReadme(docsReadme, docsDir) },
		})
	}

	for _, t := range targets {
		orig, err := os.ReadFile(t.path)
		if err != nil {
			if os.IsNotExist(err) {
				orig = nil
			} else {
				return fmt.Errorf("index --check: read %s: %w", t.path, err)
			}
		}
		changed, err := t.update()
		if err != nil {
			return fmt.Errorf("index --check: %w", err)
		}
		if changed {
			drift = true
			newContent, err := os.ReadFile(t.path)
			if err != nil {
				return fmt.Errorf("index --check: read regenerated %s: %w", t.path, err)
			}
			fmt.Fprintf(w, "would update %s\n", t.path)
			if err := printUnifiedDiff(w, t.path, orig, newContent); err != nil {
				return fmt.Errorf("index --check: %w", err)
			}
			if orig == nil {
				os.Remove(t.path)
			} else if err := os.WriteFile(t.path, orig, 0o644); err != nil {
				return fmt.Errorf("index --check: restore %s: %w", t.path, err)
			}
		} else {
			fmt.Fprintf(w, "%s up to date\n", t.path)
		}
	}

	if drift {
		return &exitCodeError{Code: 1}
	}
	return nil
}

// printUnifiedDiff shells out to `diff -u`, matching the pattern
// internal/markdown.diffSection already uses for `harnez diff`, so drift
// output looks the same across both commands.
func printUnifiedDiff(w io.Writer, label string, oldContent, newContent []byte) error {
	writeTemp := func(b []byte) (string, error) {
		f, err := os.CreateTemp("", "harnez-index-diff-*")
		if err != nil {
			return "", err
		}
		_, werr := f.Write(b)
		cerr := f.Close()
		if werr != nil {
			return f.Name(), werr
		}
		return f.Name(), cerr
	}

	oldFile, err := writeTemp(oldContent)
	if err != nil {
		return err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newContent)
	if err != nil {
		return err
	}
	defer os.Remove(newFile)

	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("diff %s: %w", label, err)
	}
	return nil
}
