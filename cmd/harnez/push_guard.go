package main

import (
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/gitflow"
)

// newPushGuardCmd is the hidden check the pre-push hook runs once per ref
// (issue 1206): `harnez push-guard refs/heads/main` exits non-zero when the
// repository's git.protected_main policy refuses a direct push to that ref.
func newPushGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "push-guard <remote-ref>",
		Short:        "Check one push destination against the repo's push-to-main policy",
		Hidden:       true,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return gitflow.GuardPush(".", args[0])
		},
	}
}
