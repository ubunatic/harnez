// repostatus implements `harnez repo-status`, a brief, quiet-by-default
// summary of git working-tree state for use inside agent sessions instead
// of running raw `git status` (see issues/154-brief-git-repo-status-command.md).
//
// Name/placement: a new top-level command, not a flag on an existing one.
// `harnez status` (see main.go) already has an established, unrelated
// meaning -- config/managed-doc apply state, not git state -- so overloading
// it here would be confusing; `harnez distill -- git status` (distill.go)
// strips generic output noise but has no git-specific semantics (it can't
// tell "clean, 3 ahead" from a genuinely noteworthy diff). Neither existing
// command is a natural home, so `repo-status` is its own subcommand.
package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/gitstatus"
)

func newRepoStatusCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "repo-status",
		Short: "Brief, quiet-by-default summary of git working-tree state",
		Long: `repo-status summarizes git working-tree state using
'git status --porcelain=v2 --branch' (machine-parseable, not scraped
human-readable text).

Quiet path (default): the tree is clean -- nothing staged, unstaged, or
untracked, no conflicts, HEAD not detached, and not behind the upstream --
prints one short line, e.g. "clean, up to date" or "clean, 3 ahead of
origin/main". Being ahead of the upstream alone does NOT trigger the
verbose path: unpushed local commits are the expected normal state in a
solo/no-PR-workflow repo, not something worth narrating on every check.

Verbose path: any staged/unstaged/untracked file, merge conflict, detached
HEAD, or being behind the upstream by any amount (including a full
ahead-and-behind divergence) prints a short structured summary -- counts
by category, branch divergence, and the affected file/conflict paths --
instead of the full raw 'git status' dump.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoStatus(cmd.OutOrStdout(), dir)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "git repo directory to inspect")
	return cmd
}

func runRepoStatus(w io.Writer, dir string) error {
	s, err := gitstatus.Collect(dir)
	if err != nil {
		return fmt.Errorf("repo-status: %w", err)
	}

	if s.Quiet() {
		fmt.Fprintln(w, quietLine(s))
		return nil
	}

	printVerboseStatus(w, s)
	return nil
}

// quietLine renders the one-line quiet-path summary.
func quietLine(s gitstatus.Status) string {
	branch := s.Branch
	if s.Detached {
		branch = "detached"
	}
	switch {
	case !s.HasUpstream:
		return fmt.Sprintf("clean, %s, no upstream", branch)
	case s.Ahead > 0:
		return fmt.Sprintf("clean, %s, %d ahead of %s", branch, s.Ahead, s.Upstream)
	default:
		return fmt.Sprintf("clean, %s, up to date with %s", branch, s.Upstream)
	}
}

// printVerboseStatus renders the short structured verbose-path summary:
// counts by category, branch divergence, and the specific affected paths --
// still far shorter than a raw `git status` dump, but enough to act on
// without a follow-up `git status` call.
func printVerboseStatus(w io.Writer, s gitstatus.Status) {
	branch := s.Branch
	if s.Detached {
		branch = "HEAD (detached)"
	}
	fmt.Fprintf(w, "branch: %s\n", branch)

	switch {
	case s.Diverged():
		fmt.Fprintf(w, "diverged from %s: %d ahead, %d behind\n", s.Upstream, s.Ahead, s.Behind)
	case s.Behind > 0:
		fmt.Fprintf(w, "behind %s by %d\n", s.Upstream, s.Behind)
	case s.HasUpstream && s.Ahead > 0:
		fmt.Fprintf(w, "%d ahead of %s\n", s.Ahead, s.Upstream)
	}

	fmt.Fprintf(w, "staged=%d unstaged=%d untracked=%d conflicts=%d\n",
		len(s.StagedFiles), len(s.UnstagedFiles), len(s.UntrackedFiles), len(s.ConflictFiles))

	printFileList(w, "conflicts", s.ConflictFiles)
	printFileList(w, "staged", s.StagedFiles)
	printFileList(w, "unstaged", s.UnstagedFiles)
	printFileList(w, "untracked", s.UntrackedFiles)
}

// maxListedFiles caps how many paths are printed per category so a very
// large change set still stays "brief" rather than reproducing a full file
// listing; the leading count line already conveys the total.
const maxListedFiles = 10

func printFileList(w io.Writer, label string, files []string) {
	if len(files) == 0 {
		return
	}
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)
	shown := sorted
	suffix := ""
	if len(shown) > maxListedFiles {
		shown = shown[:maxListedFiles]
		suffix = fmt.Sprintf(" (+%d more)", len(sorted)-maxListedFiles)
	}
	fmt.Fprintf(w, "%s: %s%s\n", label, strings.Join(shown, ", "), suffix)
}
