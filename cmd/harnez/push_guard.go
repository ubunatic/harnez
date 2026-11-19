package main

import (
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/gitflow"
)

// newPushGuardCmd is the hidden check the pre-push hook runs once per ref
// (issue 1206): `harnez push-guard --remote origin --url <url> refs/heads/main`
// exits non-zero when the repository's git.protected_main policy refuses a
// direct push to that ref on that remote. Without --remote and --url the policy
// applies to every remote.
func newPushGuardCmd() *cobra.Command {
	var remote, url string
	cmd := &cobra.Command{
		Use:          "push-guard <remote-ref>",
		Short:        "Check one push destination against the repo's push-to-main policy",
		Hidden:       true,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return gitflow.GuardPushTarget(".", remote, url, args[0])
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "", "remote name the pre-push hook passes as $1 (empty: policy applies to every remote)")
	cmd.Flags().StringVar(&url, "url", "", "push URL the pre-push hook passes as $2 (names a forge host)")
	return cmd
}
