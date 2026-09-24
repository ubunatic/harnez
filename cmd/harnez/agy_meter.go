package main

import (
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agymeter"
)

func newAgyMeterCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "agy-meter-run -- command [args...]",
		Short:  "Run an AGY process with its private usage meter",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return agymeter.Run(cmd.Context(), os.Getenv("HOME"), args[0], args[1:], os.Stdout, os.Stderr)
		},
	}
}
