package main

import (
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/dependency"
)

func newInstallCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Install an optional dependency and run its bootstrap",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dependency.Install(cmd.Context(), args[0], dryRun, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print commands without installing")
	return cmd
}
