package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/prfixup"
	"ubunatic.com/harnez/internal/prwait"
)

func newPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Work with pull requests on Codeberg (Forgejo) and GitHub",
	}
	cmd.AddCommand(newPRWaitCmd())
	return cmd
}

func newPRWaitCmd() *cobra.Command {
	var remote, contains string
	var timeout time.Duration
	var fixup bool

	cmd := &cobra.Command{
		Use:   "wait <number|url>",
		Short: "Wait until a pull request is merged or closed",
		Long: `Poll a pull request on Codeberg (Forgejo) or GitHub until it is merged,
closed without a merge, or the timeout passes. Run it in the background to be
woken by the merge without anyone saying so.

The forge and repository come from the git remote's URL (default origin), or
from a pull request URL argument. The first poll happens at once, so a PR that
is already merged exits 0 without waiting.

Exit codes:
  0  merged
  1  usage, configuration, or local git error
  2  closed without a merge
  3  still open when the timeout passed
  4  API error (not found, not authorized, or repeated transient failures)
  5  merged, but the --contains commit is not on the default branch
  6  merged, but whether the --contains commit is on the default branch is
     unverified: neither the forge API nor local git could check it
  7  open, but the head conflicts with the base: the summary names the files
     and the push that updates this PR

A conflict is checked on every poll, so one that appears after another PR
merges ends the wait too. When the forge calls the PR unmergeable, pr wait
fetches the base and refs/pull/<n>/head and merges them in memory with git
merge-tree, ignoring custom merge drivers the forge does not run. A clean
local merge (the forge still rechecking, or a work-in-progress PR) keeps it
polling; if git cannot check, two unmergeable polls in a row end the wait.

--fixup handles the common conflict itself: one only in the generated
indexes (issues/README.md, BACKLOG.md, CLOSED.md, docs/README.md). In a
temporary worktree it merges the base into the PR head, regenerates them as
harnez index does, fails on a ticket number the base already uses, commits
the merge and pushes it to the PR's own head (its AGit topic, or its branch).
It prints what it pushed and keeps waiting. Any other conflict, a failed
check or a rejected push exits 7 with the reason, without pushing. It fixes
once per wait; a PR that conflicts again exits 7. An AGit PR updates only on
a push by the user who opened it; another user's push opens a new PR, which
--fixup reports.

--contains asks the forge's compare API first and falls back to git fetch
plus git merge-base when the API cannot answer (for example an unpushed sha).

Tokens are optional. Forgejo reads the env vars in spec/pr.yaml (for example
CODEBERG_TOKEN); GitHub reads GITHUB_TOKEN, else gh auth token. No token is
ever printed.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			deps := prwait.SystemDeps(spec, os.Getenv)
			if fixup {
				deps.Fix = prfixup.Fixer{}
			}
			rep := prwait.Run(spec, deps, prwait.Request{
				Arg:      args[0],
				Remote:   remote,
				Timeout:  timeout,
				Contains: contains,
				Progress: cmd.OutOrStdout(),
			})
			if rep.Line != "" {
				fmt.Fprintln(cmd.OutOrStdout(), rep.Line)
			}
			if rep.Err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "harnez pr wait: %v\n", rep.Err)
			}
			if code := rep.Outcome.ExitCode(); code != 0 {
				return silenceIfExitCode(cmd, &exitCodeError{Code: code})
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository (default: default_remote in spec/pr.yaml)")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "give up after this long (default: default_timeout in spec/pr.yaml)")
	cmd.Flags().StringVar(&contains, "contains", "", "after a merge, also require this commit to be on the default branch (exit 5 if not, 6 if unverifiable)")
	cmd.Flags().BoolVar(&fixup, "fixup", false, "on a conflict only in generated indexes, merge the base in, regenerate them, push to the PR's head and keep waiting (once per wait)")
	return cmd
}
