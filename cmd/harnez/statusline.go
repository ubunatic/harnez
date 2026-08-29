package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/statusline"
)

func newStatuslineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "statusline",
		Short: "Claude Code statusLine command: print the current working directory",
		Long: `statusline reads a Claude Code statusLine JSON payload from stdin and prints the
current working directory (tilde-collapsed relative to $HOME).

MVP scope is cwd only — no git branch, model, or cost info. Configured
automatically by 'harnez apply' via the 'status_line' config.yaml key.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				home = ""
			}
			line, err := statusline.Render(cmd.InOrStdin(), home)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
			return nil
		},
	}
}
