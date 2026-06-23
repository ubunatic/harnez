package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/claudeconfig/internal/claude"
)

func main() {
	var configPath string
	var target string

	root := &cobra.Command{
		Use:   "claudeconfig",
		Short: "Manage Claude Code configuration declaratively from a YAML definition",
	}

	var applyLangs []string
	var forceDocs bool
	apply := &cobra.Command{
		Use:   "apply",
		Short: "Apply config.yaml to the Claude Code config directory (~/.claude)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			fmt.Printf("Applying %s → %s\n", name, t)
			return claude.ApplyAll(t, cfg, applyLangs, forceDocs)
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	apply.Flags().StringArrayVarP(&applyLangs, "lang", "l", nil, "extra language doc(s) to install globally (e.g. golang, bash)")
	apply.Flags().BoolVar(&forceDocs, "force-docs", false, "overwrite existing language docs with bundled versions")

	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what apply would change in managed blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			return claude.DiffAll(t, cfg)
		},
	}
	diff.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	diff.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	clean := &cobra.Command{
		Use:   "clean",
		Short: "Remove managed blocks written by apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			fmt.Printf("Cleaning %s\n", name)
			return claude.CleanAll(t, cfg)
		},
	}
	clean.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	clean.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	status := &cobra.Command{
		Use:          "status",
		Short:        "Show config summary and applied state",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			return claude.RunStatus(name, cfg, t)
		},
	}
	status.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	status.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	var initDir string
	var initLangs []string
	var initConfigPath string
	var initSummary, initUpdate, initReplace bool
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Set up a project directory with AGENTS.md, language docs, and Makefile targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(initConfigPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return claude.RunInit(initDir, cfg, initLangs, initSummary, initUpdate, initReplace)
		},
	}
	initCmd.Flags().StringVarP(&initConfigPath, "config", "c", "", "path to config YAML file (default: embedded)")
	initCmd.Flags().StringVarP(&initDir, "dir", "d", ".", "project directory to initialise (default: current directory)")
	initCmd.Flags().StringArrayVarP(&initLangs, "lang", "l", nil, "language(s) to set up in the project (e.g. golang, make)")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "run claude -p to generate a project summary and add it to AGENTS.md")
	initCmd.Flags().BoolVar(&initUpdate, "update", false, "re-fetch and refresh the project summary (implies --summary)")
	initCmd.Flags().BoolVar(&initReplace, "replace", false, "delete existing AGENTS.md and recreate from template before init")

	root.AddCommand(apply, diff, clean, status, initCmd)
	root.Execute()
}
