package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
)

// newStatusCmd builds `harnez status`.
func newStatusCmd() *cobra.Command {
	var configPath string
	var target string
	var componentNames []string
	var codexTarget string
	var agyTarget string
	var statusDebloat bool
	status := &cobra.Command{
		Use:          "status",
		Short:        "Show config summary and applied state",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			set, err := resolveComponentSelection(cfg, componentNames)
			if err != nil {
				return err
			}
			if codexTarget != "" {
				cfg.CodexHooksTarget = codexTarget
			}
			if agyTarget != "" {
				cfg.AgyTarget = agyTarget
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			if statusDebloat {
				if agy.IsAgyTarget(t) {
					return agy.StatusDebloat(t, cfg.Debloat.Agy)
				}
				if err := claude.StatusDebloat(t, cfg.Debloat); err != nil {
					return err
				}
				if cfg.CodexHooksTarget != "" {
					if err := codex.StatusDebloat(fsutil.ExpandHome(cfg.CodexHooksTarget), cfg.Debloat.CodexFeatures); err != nil {
						return err
					}
				}
				if cfg.AgyTarget != "" {
					if err := agy.StatusDebloat(fsutil.ExpandHome(cfg.AgyTarget), cfg.Debloat.Agy); err != nil {
						return err
					}
				}
				if cfg.CodexHooksTarget != "" {
					codexConfigPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
					if changed, err := codex.RemoveStatusLine(codexConfigPath); err != nil {
						return fmt.Errorf("Codex status line: %w", err)
					} else if changed {
						fmt.Printf("  removed Harnez status items from %s [tui.status_line]\n", codexConfigPath)
					}
				}
				return nil
			}
			return claude.RunStatus(name, cfg, t, set)
		},
	}
	status.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	status.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	status.Flags().StringSliceVar(&componentNames, "components", nil, "component(s) to inspect, comma-separated or repeated (e.g. docs-only,telemetry)")
	status.Flags().StringVar(&codexTarget, "codex-target", "", "Codex config.toml path (default: codex_hooks_target from config.yaml)")
	status.Flags().StringVar(&agyTarget, "agy-target", "", "Antigravity config directory (default: agy_target from config.yaml)")
	status.Flags().BoolVar(&statusDebloat, "debloat", false, "show debloat-managed deny entries and toggles instead of full status (issue 316)")
	return status
}

// newScanDocsCmd builds `harnez scan-docs`.
func newScanDocsCmd() *cobra.Command {
	var configPath string
	scanDocs := &cobra.Command{
		Use:          "scan-docs <dir>",
		Short:        "Scan immediate child agent projects for managed-docs drift",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			report, err := claude.ScanDocs(args[0], cfg)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), report)
			return err
		},
	}
	scanDocs.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	return scanDocs
}
