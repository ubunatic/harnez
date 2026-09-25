package main

import (
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/subagent"
)

func newAgyMeterCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "agy-meter-run -- command [args...]",
		Short:  "Run an AGY process with its private usage meter",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := subagent.AgyLaunchEnv(os.Environ(), os.Getenv("HOME"))
			return agymeter.RunWithEnv(cmd.Context(), os.Getenv("HOME"), args[0], args[1:], env, os.Stdout, os.Stderr)
		},
	}
}
