package main

import (
	"github.com/spf13/cobra"
	"os"
	"ubunatic.com/harnez/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use: "mcp", Short: "Serve Harnez agent tools over MCP stdio",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			return (mcp.Server{In: cmd.InOrStdin(), Out: cmd.OutOrStdout(), Command: exe}).Run(cmd.Context())
		},
	}
}
