package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
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
			rep := prwait.Run(spec, prwait.SystemDeps(spec, os.Getenv), prwait.Request{
				Arg:      args[0],
				Remote:   remote,
				Timeout:  timeout,
				Contains: contains,
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
	cmd.Flags().StringVar(&contains, "contains", "", "after a merge, also require this commit to be on the default branch (exit 5 if not)")
	return cmd
}
