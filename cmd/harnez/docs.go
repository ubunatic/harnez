// docs implements `harnez docs <subcommand>`, a parent command for
// project-scoped doc operations that don't fit under init/apply/diff. See
// issue 360.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
)

func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Project-scoped operations on already-installed copyable docs",
	}
	cmd.AddCommand(newDocsVariantCmd())
	return cmd
}

func newDocsVariantCmd() *cobra.Command {
	var dir string
	var configPath string

	cmd := &cobra.Command{
		Use:   "variant <name> <lite|full>",
		Short: "Swap an already-installed doc's content between its lite and full source",
		Long: `variant re-installs a single already project-installed doc, swapping which
source variant (source vs. lite_source, see issue 357) supplies its managed
content. It preserves any harnez:stop-delimited local customization and
touches nothing else — no other doc, no Makefile, no AGENTS.md section.

This is the fast path for "I already have this doc installed, just swap its
content." To select a variant for docs installed for the first time, use
'harnez init --variant lite|full' or 'harnez apply --variant lite|full'.`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, variant := args[0], args[1]
			if variant != "lite" && variant != "full" {
				return fmt.Errorf("invalid variant %q: must be lite or full", variant)
			}
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			changed, err := claude.SwitchDocVariant(dir, cfg, name, variant)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("switched %s to %s variant\n", name, variant)
			} else {
				fmt.Printf("%s already installed as %s variant\n", name, variant)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "project directory containing the installed doc (default: current directory)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	return cmd
}
