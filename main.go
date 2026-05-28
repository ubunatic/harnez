package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// mergeLangs returns the union of config-declared langs and CLI --lang flags,
// preserving order (config first, then any extras from the flag).
func mergeLangs(fromConfig, fromFlag []string) []string {
	seen := make(map[string]struct{}, len(fromConfig)+len(fromFlag))
	result := make([]string, 0, len(fromConfig)+len(fromFlag))
	for _, l := range fromConfig {
		seen[l] = struct{}{}
		result = append(result, l)
	}
	for _, l := range fromFlag {
		if _, ok := seen[l]; !ok {
			result = append(result, l)
		}
	}
	return result
}

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
			return applyAll(t, project, cfg, mergeLangs(cfg.Langs, langs))
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

	var initDir string
	var initSummary bool
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create AGENTS.md and CLAUDE.md symlink in a project directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(initDir, initSummary)
		},
	}
	initCmd.Flags().StringVarP(&initDir, "dir", "d", ".", "project directory to initialise (default: current directory)")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "run claude -p to generate a project summary and add it to AGENTS.md")

	root.AddCommand(apply, diff, clean, status, initCmd)
	root.Execute()
}
