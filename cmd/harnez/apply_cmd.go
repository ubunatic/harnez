package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
)

// newApplyCmd builds `harnez apply` (global config only; see docs/CLIDesign.md).
func newApplyCmd() *cobra.Command {
	var configPath string
	var target string
	var applyDocs []string
	var forceDocs bool
	var applyCleanDocs bool
	var applyRemoveDocs bool
	var applyNoDocs bool
	var applySystemd bool
	var applyShell bool
	var applyVariant string
	var componentNames []string
	var debloat bool
	var debloatPreset string
	var debloatNotebookEdit bool
	var debloatCron bool
	var debloatDisableBundledSkills bool
	var debloatDisableWorkflows bool
	var debloatDisableRemoteControl bool
	var debloatDisableClaudeAiConnectors bool
	var debloatDisableArtifact bool
	var codexTarget string
	var agyTarget string
	apply := &cobra.Command{
		Use:   "apply",
		Short: "Apply config.yaml to global Claude Code and agent harness directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			set, err := resolveComponentSelection(cfg, componentNames)
			if err != nil {
				return err
			}
			if set.Has(claude.Telemetry) {
				if err := ensureTelemetrySchema(); err != nil {
					return fmt.Errorf("initialize telemetry: %w", err)
				}
			}
			if codexTarget != "" {
				cfg.CodexHooksTarget = codexTarget
			}
			if agyTarget != "" {
				cfg.AgyTarget = agyTarget
			}
			if (debloat || debloatPreset != "") && len(cfg.Debloat.CodexFeatures) > 0 && cfg.CodexHooksTarget == "" {
				return fmt.Errorf("codex_hooks_target is required for Codex debloat")
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			fmt.Printf("Applying %s → %s\n", name, t)
			if applyNoDocs || applyRemoveDocs {
				applyDocs = []string{}
				applyCleanDocs = true
			}
			if applyCleanDocs {
				if _, err := claude.CleanUnmanagedDocs(t, cfg, applyDocs); err != nil {
					return err
				}
			}
			applyOpts := claude.ApplyOptions{
				Selection: set, Docs: applyDocs, ForceDocs: forceDocs,
				InstallSystemd: applySystemd, InstallShell: applyShell, DocVariant: applyVariant,
			}
			if err := claude.ApplyAllVariant(t, cfg, applyOpts); err != nil {
				return err
			}
			if cfg.AgyTarget != "" && set.Has(claude.Usage) {
				agyStatusLinePath := agy.StatusLineSettingsPath(fsutil.ExpandHome(cfg.AgyTarget))
				if cfg.StatusLine {
					changed, err := agy.ApplyStatusLine(agyStatusLinePath)
					if err != nil {
						return fmt.Errorf("Antigravity status line: %w", err)
					}
					if changed {
						fmt.Printf("  updated %s [statusLine]\n", agyStatusLinePath)
					}
				} else {
					changed, err := agy.RemoveStatusLine(agyStatusLinePath)
					if err != nil {
						return fmt.Errorf("Antigravity status line: %w", err)
					}
					if changed {
						fmt.Printf("  removed %s [statusLine]\n", agyStatusLinePath)
					}
				}
			}
			if cfg.AgyTarget != "" {
				// issue 1002: agy gets the equivalent of Claude's permission rules.
				agyDir := fsutil.ExpandHome(cfg.AgyTarget)
				if _, err := os.Stat(agyDir); err == nil {
					allow, deny, unmapped := agy.PermissionRules(cfg.Permissions.Allow, cfg.Permissions.Deny)
					agyPermPath := agy.StatusLineSettingsPath(agyDir)
					changed, err := agy.ApplyPermissions(agyPermPath, allow, deny)
					if err != nil {
						return fmt.Errorf("Antigravity permissions: %w", err)
					}
					if changed {
						fmt.Printf("  updated %s [permissions: %d allow, %d deny]\n", agyPermPath, len(allow), len(deny))
					}
					if len(unmapped) > 0 {
						fmt.Printf("  agy permissions: %d Claude rule(s) have no agy equivalent: %v\n", len(unmapped), unmapped)
					}
				}
			}
			if cfg.CodexHooksTarget != "" && set.Has(claude.Usage) {
				codexConfigPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
				if cfg.StatusLine {
					changed, err := codex.ApplyStatusLine(codexConfigPath)
					if err != nil {
						return fmt.Errorf("Codex status line: %w", err)
					}
					if changed {
						fmt.Printf("  updated %s [tui.status_line]\n", codexConfigPath)
					}
				} else if changed, err := codex.RemoveStatusLine(codexConfigPath); err != nil {
					return fmt.Errorf("Codex status line: %w", err)
				} else if changed {
					fmt.Printf("  removed Harnez status items from %s [tui.status_line]\n", codexConfigPath)
				}
			}
			opts := claude.DebloatOptions{
				NotebookEdit:              debloatNotebookEdit,
				Cron:                      debloatCron,
				DisableBundledSkills:      debloatDisableBundledSkills,
				DisableWorkflows:          debloatDisableWorkflows,
				DisableRemoteControl:      debloatDisableRemoteControl,
				DisableClaudeAiConnectors: debloatDisableClaudeAiConnectors,
				DisableArtifact:           debloatDisableArtifact,
			}
			if debloat {
				opts.Preset = debloatPreset
				if opts.Preset == "" {
					opts.Preset = claude.DebloatPresetMinimal
				}
			} else if debloatPreset != "" {
				opts.Preset = debloatPreset
			}
			if opts.Requested() {
				if agy.IsAgyTarget(t) {
					fmt.Printf("Applying Antigravity debloat (preset=%q) → %s\n", opts.Preset, filepath.Join(t, "settings.json"))
					if _, err := agy.ApplyDebloat(t, cfg.Debloat.Agy, opts.Preset); err != nil {
						return err
					}
					return nil
				}
				fmt.Printf("Applying debloat (preset=%q) → %s\n", opts.Preset, filepath.Join(t, "settings.json"))
				if err := claude.ApplyDebloat(t, cfg.Debloat, opts); err != nil {
					return err
				}
				if opts.Preset != "" && cfg.CodexHooksTarget != "" {
					codexPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
					fmt.Printf("Applying Codex debloat → %s\n", codexPath)
					if _, err := codex.ApplyDebloat(codexPath, cfg.Debloat.CodexFeatures); err != nil {
						return err
					}
				}
				if opts.Preset != "" && cfg.AgyTarget != "" {
					agyPath := fsutil.ExpandHome(cfg.AgyTarget)
					fmt.Printf("Applying Antigravity debloat → %s\n", filepath.Join(agyPath, "settings.json"))
					if _, err := agy.ApplyDebloat(agyPath, cfg.Debloat.Agy, opts.Preset); err != nil {
						return err
					}
				}
				return nil
			}
			return nil
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	apply.Flags().StringSliceVar(&componentNames, "components", nil, "component(s) to install, comma-separated or repeated (e.g. docs-only,telemetry)")
	apply.Flags().StringVar(&codexTarget, "codex-target", "", "Codex config.toml path (default: codex_hooks_target from config.yaml)")
	apply.Flags().StringVar(&agyTarget, "agy-target", "", "Antigravity config directory (default: agy_target from config.yaml)")
	apply.Flags().StringSliceVarP(&applyDocs, "docs", "d", nil, "doc(s) to install globally, comma-separated or repeated (e.g. golang,canary,all)")
	apply.Flags().BoolVar(&forceDocs, "force-docs", false, "overwrite existing docs with bundled versions")
	apply.Flags().BoolVar(&applyCleanDocs, "clean-docs", false, "remove unmanaged global docs from ~/.claude/docs/ and ~/.prime/agent/docs/")
	apply.Flags().BoolVar(&applyRemoveDocs, "remove-docs", false, "remove all global docs from ~/.claude/docs/ and ~/.prime/agent/docs/")
	apply.Flags().BoolVar(&applyNoDocs, "no-docs", false, "do not install global docs, removing any previously installed global docs")
	apply.Flags().StringVar(&applyVariant, "variant", "full", "doc variant to install: lite or full (docs without a lite variant fall back to full)")
	apply.Flags().BoolVar(&applySystemd, "systemd", false,
		"write the harnez-agent-collector systemd --user unit; use `harnez server install` to enable and start it")
	apply.Flags().BoolVarP(&applyShell, "shell", "s", false,
		"inject harnez environment source into ~/.bashrc and ~/.zshrc")
	apply.Flags().BoolVar(&debloat, "debloat", false,
		"apply the minimal Claude, Codex, and Antigravity context-saving preset from config.yaml")
	apply.Flags().StringVar(&debloatPreset, "debloat-preset", "",
		`debloat preset: "minimal" (default) or "aggressive"; both disable bundled skills`)
	apply.Flags().BoolVar(&debloatNotebookEdit, "debloat-notebook-edit", false, "also deny NotebookEdit")
	apply.Flags().BoolVar(&debloatCron, "debloat-cron", false, "also deny CronCreate/CronDelete/CronList")
	apply.Flags().BoolVar(&debloatDisableBundledSkills, "debloat-disable-bundled-skills", false, "disable bundled skills without requiring a debloat preset")
	apply.Flags().BoolVar(&debloatDisableWorkflows, "debloat-disable-workflows", false, "set disableWorkflows: true")
	apply.Flags().BoolVar(&debloatDisableRemoteControl, "debloat-disable-remote-control", false, "set disableRemoteControl: true")
	apply.Flags().BoolVar(&debloatDisableClaudeAiConnectors, "debloat-disable-claude-ai-connectors", false, "set disableClaudeAiConnectors: true")
	apply.Flags().BoolVar(&debloatDisableArtifact, "debloat-disable-artifact", false, "set disableArtifact: true")
	return apply
}

// newDiffCmd builds `harnez diff`.
func newDiffCmd() *cobra.Command {
	var configPath string
	var target string
	var componentNames []string
	var diffExitCode bool
	var captureDocs bool
	var captureOutput string
	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what apply would change in managed blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			set, err := resolveComponentSelection(cfg, componentNames)
			if err != nil {
				return err
			}
			if captureOutput != "" && !captureDocs {
				return fmt.Errorf("--out requires --capture-docs")
			}
			if captureDocs {
				repoDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("resolve current repository: %w", err)
				}
				path, changed, err := claude.CaptureDocsDrift(repoDir, captureOutput, cfg)
				if err != nil {
					return err
				}
				fmt.Printf("Captured managed docs drift in %s\n", path)
				if diffExitCode && changed {
					return silenceIfExitCode(cmd, &exitCodeError{Code: 1})
				}
				return nil
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			changed, err := claude.DiffAll(t, cfg, set)
			if err != nil {
				return err
			}
			if diffExitCode && changed {
				return silenceIfExitCode(cmd, &exitCodeError{Code: 1})
			}
			return nil
		},
	}
	diff.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	diff.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	diff.Flags().StringSliceVar(&componentNames, "components", nil, "component(s) to inspect, comma-separated or repeated (e.g. docs-only,telemetry)")
	diff.Flags().BoolVarP(&diffExitCode, "exit-code", "e", false, "exit with status 1 if drift/changes are found")
	diff.Flags().BoolVar(&captureDocs, "capture-docs", false, "write configured project-doc drift to an inbox Markdown report")
	diff.Flags().StringVar(&captureOutput, "out", "", "output path for --capture-docs (default: issues/inbox/managed-docs-drift-<timestamp>.md)")
	return diff
}

// newRevertCmd builds `harnez revert`.
func newRevertCmd() *cobra.Command {
	var configPath string
	var target string
	var codexTarget string
	var agyTarget string
	var revertDebloat bool
	var revertManaged bool
	revert := &cobra.Command{
		Use:          "revert",
		Short:        "Revert managed configuration or one-off changes",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if codexTarget != "" {
				cfg.CodexHooksTarget = codexTarget
			}
			if agyTarget != "" {
				cfg.AgyTarget = agyTarget
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			if revertManaged && revertDebloat {
				return fmt.Errorf("revert accepts only one of --managed or --debloat")
			}
			if revertManaged {
				fmt.Printf("Cleaning managed configuration in %s\n", t)
				if err := claude.CleanAll(t, cfg); err != nil {
					return err
				}
				if cfg.AgyTarget != "" {
					agyStatusLinePath := agy.StatusLineSettingsPath(fsutil.ExpandHome(cfg.AgyTarget))
					if changed, err := agy.RemoveStatusLine(agyStatusLinePath); err != nil {
						return fmt.Errorf("Antigravity status line: %w", err)
					} else if changed {
						fmt.Printf("  removed %s [statusLine]\n", agyStatusLinePath)
					}
					allow, deny, _ := agy.PermissionRules(cfg.Permissions.Allow, cfg.Permissions.Deny)
					if changed, err := agy.RemovePermissions(agyStatusLinePath, allow, deny); err != nil {
						return fmt.Errorf("Antigravity permissions: %w", err)
					} else if changed {
						fmt.Printf("  removed harnez rules from %s [permissions]\n", agyStatusLinePath)
					}
				}
				return nil
			}
			if !revertDebloat {
				return fmt.Errorf("revert requires --managed or --debloat")
			}
			if agy.IsAgyTarget(t) {
				reverted, err := agy.RevertDebloat(t)
				if err != nil {
					return err
				}
				if !reverted {
					return fmt.Errorf("no debloat record found at %s (nothing to revert)", t)
				}
				fmt.Printf("Reverted Antigravity debloat changes in %s\n", filepath.Join(t, "settings.json"))
				return nil
			}
			claudeRecord := filepath.Join(t, ".harnez-debloat.json")
			claudeReverted := false
			if _, err := os.Stat(claudeRecord); err == nil {
				if err := claude.RevertDebloat(t); err != nil {
					return err
				}
				claudeReverted = true
			} else if !os.IsNotExist(err) {
				return err
			}
			codexReverted := false
			if cfg.CodexHooksTarget != "" {
				codexPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
				var err error
				codexReverted, err = codex.RevertDebloat(codexPath)
				if err != nil {
					return err
				}
				if codexReverted {
					fmt.Printf("Reverted Codex debloat changes in %s\n", codexPath)
				}
			}
			agyReverted := false
			if cfg.AgyTarget != "" {
				agyPath := fsutil.ExpandHome(cfg.AgyTarget)
				var err error
				agyReverted, err = agy.RevertDebloat(agyPath)
				if err != nil {
					return err
				}
				if agyReverted {
					fmt.Printf("Reverted Antigravity debloat changes in %s\n", filepath.Join(agyPath, "settings.json"))
				}
			}
			if !claudeReverted && !codexReverted && !agyReverted {
				return fmt.Errorf("no debloat record found")
			}
			if claudeReverted {
				fmt.Printf("Reverted Claude debloat changes in %s\n", filepath.Join(t, "settings.json"))
			}
			return nil
		},
	}
	revert.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	revert.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	revert.Flags().StringVar(&codexTarget, "codex-target", "", "Codex config.toml path (default: codex_hooks_target from config.yaml)")
	revert.Flags().StringVar(&agyTarget, "agy-target", "", "Antigravity config directory (default: agy_target from config.yaml)")
	revert.Flags().BoolVar(&revertManaged, "managed", false, "remove managed configuration blocks and keys")
	revert.Flags().BoolVar(&revertDebloat, "debloat", false, "restore Claude, Codex, and Antigravity settings to their pre-debloat state")
	return revert
}
