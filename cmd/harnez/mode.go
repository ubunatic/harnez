package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/mode"
)

func newModeCmd() *cobra.Command {
	var filePath string
	var quiet bool
	var dryRun bool
	var ephemeral bool
	var noFile bool
	var persist bool
	var mainFlag bool

	runWithTier := func(cmd *cobra.Command, tierStr string) error {
		t, err := mode.ParseTier(tierStr)
		if err != nil {
			return err
		}

		res, err := mode.SetMode(t, mode.Options{
			FilePath:  filePath,
			Persist:   persist || mainFlag,
			Ephemeral: ephemeral || noFile,
			Quiet:     quiet,
			DryRun:    dryRun,
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
		Short:   "Switch ConciseMode terseness level and synchronize AGENTS.local.md overlay",
		Long: `mode dynamically switches the operational terseness level for the active session
and updates the managed 'Concise Mode' section in ./AGENTS.local.md (or ./AGENTS.md with --persist).

Tiers:
  lite (1)        Level 1: Concise Lite (Professional Terse, no pleasantries)
  std (2)         Level 2: Concise Standard (Telegraphic fragments, zero filler)
  ultra (3)       Level 3: Concise Ultra (Diffs/status only, zero narrative)
  vision (4)      Vision Mode: Multimodal visual cheatsheet context (@docs/vision/*.png)
  off (0, reset)  Disable ConciseMode / reset local overlay

Flags:
  -e, --ephemeral, --no-file  Emit stdout directive only; do not write any file to disk
      --persist, --main       Write to ./AGENTS.md instead of ./AGENTS.local.md
  -f, --file                  Target instructions file (default: ./AGENTS.local.md)
  -q, --quiet                 Suppress in-flight LLM directive on stdout (file update only)
      --dry-run               Show directive and changes without writing to disk`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runWithTier(cmd, args[0])
		},
	}

	cmd.PersistentFlags().StringVarP(&filePath, "file", "f", "", "target instructions file (default: ./AGENTS.local.md, or ./AGENTS.md with --persist)")
	cmd.PersistentFlags().BoolVarP(&ephemeral, "ephemeral", "e", false, "emit stdout directive only; do not write any file to disk")
	cmd.PersistentFlags().BoolVar(&noFile, "no-file", false, "alias for --ephemeral")
	cmd.PersistentFlags().BoolVar(&persist, "persist", false, "write to ./AGENTS.md instead of ./AGENTS.local.md")
	cmd.PersistentFlags().BoolVar(&mainFlag, "main", false, "alias for --persist")
	cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress stdout directive (file sync only)")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "display directive and planned file changes without modifying disk")

	// Subcommands for direct invocation: `harnez mode lite`, `harnez mode std`, `harnez mode ultra`, `harnez mode vision`, `harnez mode off`
	subTiers := []struct {
		name    string
		aliases []string
		short   string
	}{
		{"lite", []string{"1", "level1"}, "Switch to Concise Lite (Level 1)"},
		{"std", []string{"standard", "2", "level2"}, "Switch to Concise Standard (Level 2)"},
		{"ultra", []string{"3", "level3"}, "Switch to Concise Ultra (Level 3)"},
		{"vision", []string{"4", "level4", "v", "vis"}, "Switch to Vision Mode (Multimodal visual cards)"},
		{"off", []string{"reset", "default", "0"}, "Disable ConciseMode / remove overlay directive"},
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

	for _, setting := range []struct {
		name    string
		enforce bool
		short   string
	}{
		{"enforce-read", true, "Enable native large-read enforcement"},
		{"autonomous-read", false, "Allow native reads and observe their opportunity cost"},
	} {
		setting := setting
		cmd.AddCommand(&cobra.Command{
			Use:   setting.name,
			Short: setting.short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolve home directory: %w", err)
				}
				path := filepath.Join(home, ".harnez", "config.yaml")
				data, err := os.ReadFile(path)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("read config: %w", err)
				}
				var cfg map[string]any
				if len(data) > 0 {
					if err := yaml.Unmarshal(data, &cfg); err != nil {
						return fmt.Errorf("parse config: %w", err)
					}
				}
				if cfg == nil {
					cfg = map[string]any{}
				}
				discipline, _ := cfg["reading_discipline"].(map[string]any)
				if discipline == nil {
					discipline = map[string]any{}
				}
				discipline["enforce"] = setting.enforce
				cfg["reading_discipline"] = discipline
				encoded, err := yaml.Marshal(cfg)
				if err != nil {
					return fmt.Errorf("encode config: %w", err)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					return fmt.Errorf("create config directory: %w", err)
				}
				if err := os.WriteFile(path, encoded, 0o600); err != nil {
					return fmt.Errorf("write config: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "reading_discipline.enforce: %t\n", setting.enforce)
				return nil
			},
		})
	}

	return cmd
}
