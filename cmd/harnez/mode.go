package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/mode"
)

func newModeCmd() *cobra.Command {
	var filePath string
	var quiet bool
	var dryRun bool

	runWithTier := func(cmd *cobra.Command, tierStr string) error {
		t, err := mode.ParseTier(tierStr)
		if err != nil {
			return err
		}

		res, err := mode.SetMode(t, mode.Options{
			FilePath: filePath,
			Quiet:    quiet,
			DryRun:   dryRun,
		})
		if err != nil {
			return err
		}

		if !quiet && res.Directive != "" {
			fmt.Fprintln(cmd.OutOrStdout(), res.Directive)
		}
		if res.FileUpdate != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "harnez: %s\n", res.FileUpdate)
		}
		return nil
	}

	cmd := &cobra.Command{
		Use:     "mode [tier]",
		Aliases: []string{"concise"},
		Short:   "Switch ConciseMode terseness level and synchronize AGENTS.md",
		Long: `mode dynamically switches the operational terseness level for the active session
and updates the managed 'Concise Mode' section in ./AGENTS.md.

Tiers:
  lite (1)        Level 1: Concise Lite (Professional Terse, no pleasantries)
  std (2)         Level 2: Concise Standard (Telegraphic fragments, zero filler)
  ultra (3)       Level 3: Concise Ultra (Diffs/status only, zero narrative)
  off (0, reset)  Disable ConciseMode / reset AGENTS.md to default

Flags:
  -q, --quiet    Suppress in-flight LLM directive on stdout (file update only)
  -f, --file     Target instructions file (default: ./AGENTS.md)
  --dry-run      Show directive and changes without writing to disk`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runWithTier(cmd, args[0])
		},
	}

	cmd.PersistentFlags().StringVarP(&filePath, "file", "f", "./AGENTS.md", "path to AGENTS.md file")
	cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress stdout directive (file sync only)")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "display directive and planned file changes without modifying disk")

	// Subcommands for direct invocation: `harnez mode lite`, `harnez mode std`, `harnez mode ultra`, `harnez mode off`
	subTiers := []struct {
		name    string
		aliases []string
		short   string
	}{
		{"lite", []string{"1", "level1"}, "Switch to Concise Lite (Level 1)"},
		{"std", []string{"standard", "2", "level2"}, "Switch to Concise Standard (Level 2)"},
		{"ultra", []string{"3", "level3"}, "Switch to Concise Ultra (Level 3)"},
		{"off", []string{"reset", "default", "0"}, "Disable ConciseMode / remove AGENTS.md directive"},
	}

	for _, st := range subTiers {
		tierName := st.name
		subCmd := &cobra.Command{
			Use:     tierName,
			Aliases: st.aliases,
			Short:   st.short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runWithTier(cmd, tierName)
			},
		}
		cmd.AddCommand(subCmd)
	}

	return cmd
}
