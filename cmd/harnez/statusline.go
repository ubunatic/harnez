package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/statusline"
	"ubunatic.com/harnez/internal/usage"
)

func newStatuslineCmd() *cobra.Command {
	var agent string
	cmd := &cobra.Command{
		Use:   "statusline",
		Short: "Render context and usage data for Claude Code or Antigravity CLI",
		Long: `statusline reads Claude Code or Antigravity CLI statusLine JSON payloads from stdin.
For Claude Code it prints the current directory, context usage, cache-read
percentage, and rate-limit usage. With --agent agy it prints context usage,
cache-read percentage, and quota buckets.

The command is installed for Claude Code and Antigravity CLI by 'harnez apply'
when the 'usage' component and 'status_line' config.yaml key are enabled.
Codex uses its native TUI status items, configured separately by 'harnez apply'.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return err
			}
			if agent == "agy" {
				line, err := statusline.RenderContextUsage(bytes.NewReader(data))
				if err != nil {
					return err
				}
				if line != "" {
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
				usage.ObserveStatusline(cmd.Context(), "agy", data)
				return nil
			}
			if agent != "claude" {
				return fmt.Errorf("unsupported statusline agent %q (choose claude or agy)", agent)
			}
			home, err := os.UserHomeDir()
			if err != nil {
				home = ""
			}
			line, err := statusline.Render(bytes.NewReader(data), home)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
			usage.ObserveStatusline(cmd.Context(), "claude", data)
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "claude", "status payload provider: claude or agy")
	return cmd
}
