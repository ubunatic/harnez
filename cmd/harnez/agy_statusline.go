package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/statusline"
)

func newAGYStatuslineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agy-statusline",
		Short: "Render or manage the Antigravity CLI context and quota status line",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			line, err := statusline.RenderContextUsage(cmd.InOrStdin())
			if err != nil {
				return err
			}
			if line != "" {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "apply",
			Short: "Install the Harnez status line in Antigravity CLI settings",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				path := agy.StatusLinePath(home)
				changed, err := agy.ApplyStatusLine(path)
				if err != nil {
					return err
				}
				if changed {
					fmt.Fprintf(cmd.OutOrStdout(), "Installed AGY status line in %s\n", path)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "AGY status line already current in %s\n", path)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Check Harnez's Antigravity CLI status-line configuration",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				path := agy.StatusLinePath(home)
				installed, drifted := agy.StatusLineStatus(path)
				switch {
				case !installed:
					fmt.Fprintln(cmd.OutOrStdout(), "AGY status line is not installed")
				case drifted:
					fmt.Fprintf(cmd.OutOrStdout(), "AGY status line has drifted in %s\n", path)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "AGY status line is current in %s\n", path)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:     "remove",
			Aliases: []string{"clean"},
			Short:   "Remove Harnez's Antigravity CLI status line",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				path := agy.StatusLinePath(home)
				changed, err := agy.RemoveStatusLine(path)
				if err != nil {
					return err
				}
				if changed {
					fmt.Fprintf(cmd.OutOrStdout(), "Removed AGY status line from %s\n", path)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No Harnez AGY status line to remove")
				}
				return nil
			},
		},
	)
	return cmd
}
