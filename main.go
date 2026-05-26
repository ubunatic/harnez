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
		if len(cfgTarget) >= 2 && cfgTarget[:2] == "~/" {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, cfgTarget[2:])
		}
		return cfgTarget
	}
	return defaultTarget()
}

func openConfig(configPath string) (*Config, string, error) {
	if configPath == "" {
		cfg, err := loadConfigEmbedded()
		return cfg, "(embedded)", err
	}
	cfg, err := loadConfig(configPath)
	return cfg, configPath, err
}

func main() {
	var configPath string
	var target string

	root := &cobra.Command{
		Use:   "claudeconfig",
		Short: "Manage Claude Code configuration declaratively from a YAML definition",
	}

	var langs []string
	var project string
	apply := &cobra.Command{
		Use:   "apply",
		Short: "Apply config.yaml to the Claude Code config directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := openConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			fmt.Printf("Applying %s → %s\n", name, t)
			return applyAll(t, project, cfg, langs)
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	apply.Flags().StringVarP(&project, "project", "p", ".", "project directory for local agents_md targets")
	apply.Flags().StringArrayVarP(&langs, "lang", "l", nil, "language doc(s) to install (e.g. golang, bash)")

	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what apply would change in managed blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := openConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			return diffAll(t, cfg)
		},
	}
	diff.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	diff.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	clean := &cobra.Command{
		Use:   "clean",
		Short: "Remove managed blocks written by apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := openConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			fmt.Printf("Cleaning %s\n", name)
			return cleanAll(t, cfg)
		},
	}
	clean.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	clean.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	status := &cobra.Command{
		Use:          "status",
		Short:        "Show config summary and applied state",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := openConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := expandTarget(target, cfg.TargetDir)
			return runStatus(name, cfg, t)
		},
	}
	status.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	status.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")

	root.AddCommand(apply, diff, clean, status)
	root.Execute()
}
