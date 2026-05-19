package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func expandTarget(flag, cfgTarget string) string {
	if flag != "" {
		return flag
	}
	if cfgTarget != "" {
		if cfgTarget[:2] == "~/" {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, cfgTarget[2:])
		}
		return cfgTarget
	}
	return defaultTarget()
}

func main() {
	var configPath string
	var target string

	root := &cobra.Command{
		Use:   "claudeconfig",
		Short: "Manage Claude Code configuration declaratively from a YAML definition",
	}

	apply := &cobra.Command{
		Use:   "apply",
		Short: "Apply config.yaml to the Claude Code config directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			fmt.Printf("Applying %s → %s\n", configPath, t)
			return applyAll(t, cfg)
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config YAML file")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what apply would change in managed blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			return diffAll(t, cfg)
		},
	}
	diff.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config YAML file")
	diff.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	clean := &cobra.Command{
		Use:   "clean",
		Short: "Remove managed blocks written by apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			fmt.Printf("Cleaning %s\n", t)
			return cleanAll(t, cfg)
		},
	}
	clean.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config YAML file")
	clean.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	root.AddCommand(apply, diff, clean)
	root.Execute()
}
