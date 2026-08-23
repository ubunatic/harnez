package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/usage"
)

func main() {
	var configPath string
	var target string

	root := &cobra.Command{
		Use:   "harnez",
		Short: "Manage Claude Code, Prime Agent, and other agent harnesses from a YAML definition",
	}

	var usageJSON bool
	var usageAgent string
	var usageOffline bool
	var usageWatch bool
	var usageSummary bool
	var usageProcesses bool
	var usageInterval time.Duration
	var usageHost string
	usageCmd := &cobra.Command{
		Use:     "usage",
		Aliases: []string{"quota", "tokens"},
		Short:   "Show unified token, session, and quota status across AI coding agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var client *http.Client
			if !usageOffline {
				client = &http.Client{Timeout: 5 * time.Second}
			}

			if usageWatch && usageSummary {
				return fmt.Errorf("--watch and --summary cannot be combined")
			}

			if usageWatch {
				if usageJSON {
					return fmt.Errorf("--watch and --json cannot be combined")
				}
				return usage.RunWatchWithHost(ctx, "", client, cmd.OutOrStdout(), usageInterval, "", usageHost, usageProcesses)
			}

			if usageSummary {
				if usageJSON {
					return fmt.Errorf("--summary and --json cannot be combined")
				}
				if usageHost != "" {
					usage.RenderSummaryRemote(ctx, usageHost, cmd.OutOrStdout(), usageProcesses)
				} else {
					usage.RenderSummary(ctx, "", client, cmd.OutOrStdout(), usageProcesses)
				}
				return nil
			}

			var summary usage.UsageSummary
			if usageHost != "" {
				s, _, err := usage.CollectRemote(ctx, usageHost, usageProcesses)
				if err != nil && !usageJSON {
					return err
				}
				summary = s
			} else {
				summary = usage.CollectAll(ctx, "", client)
			}
			if usageAgent != "" {
				var filtered []usage.AgentUsage
				for _, a := range summary.Agents {
					if strings.EqualFold(a.AgentID, usageAgent) {
						filtered = append(filtered, a)
					}
				}
				summary.Agents = filtered
			}

			if usageJSON {
				out, err := usage.RenderJSON(summary)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}

			fmt.Print(usage.RenderText(summary))
			return nil
		},
	}
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output usage in JSON format")
	usageCmd.Flags().StringVar(&usageAgent, "agent", "", "filter to a specific agent (claude, agy, codex)")
	usageCmd.Flags().StringVar(&usageHost, "host", "", "query usage from a remote host via SSH")
	usageCmd.Flags().BoolVar(&usageOffline, "offline", false, "disable live network queries and use local caches only")
	usageCmd.Flags().BoolVarP(&usageWatch, "watch", "w", false, "live-refresh the dashboard in place with a tokens/min trend")
	usageCmd.Flags().BoolVarP(&usageSummary, "summary", "s", false, "print the compact --watch-style dashboard once and exit")
	usageCmd.Flags().BoolVarP(&usageProcesses, "proc", "p", false, "show running agent processes panel in --watch / --summary")
	usageCmd.Flags().BoolVar(&usageProcesses, "processes", false, "show running agent processes panel in --watch / --summary")
	usageCmd.Flags().DurationVar(&usageInterval, "interval", usage.DefaultWatchInterval,
		fmt.Sprintf("refresh interval for --watch (minimum %s, to avoid hammering live quota APIs)", usage.MinWatchInterval))

	var historyJSON bool
	historyCmd := &cobra.Command{
		Use:     "history",
		Aliases: []string{"timeline"},
		Short:   "Inspect and manage recorded usage history and timelines",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := usage.ReadHistory(usage.HistoryDir(""))
			if err != nil {
				return err
			}
			if historyJSON {
				out, err := usage.RenderTimelineJSON(entries)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			fmt.Print(usage.RenderTimelineText(entries))
			return nil
		},
	}
	historyCmd.PersistentFlags().BoolVar(&historyJSON, "json", false, "output history in JSON format")

	historyTimelineCmd := &cobra.Command{
		Use:   "timeline",
		Short: "Display the merged usage history timeline across all recorded machine logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := usage.ReadHistory(usage.HistoryDir(""))
			if err != nil {
				return err
			}
			if historyJSON {
				out, err := usage.RenderTimelineJSON(entries)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			fmt.Print(usage.RenderTimelineText(entries))
			return nil
		},
	}

	historyFetchCmd := &cobra.Command{
		Use:   "fetch <host>",
		Short: "Fetch remote usage history files from an SSH host into local history dir",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			historyDir := usage.HistoryDir("")
			if _, err := usage.FetchRemoteHistory(cmd.Context(), host, historyDir, cmd.OutOrStdout()); err != nil {
				return fmt.Errorf("fetch remote history: %w", err)
			}
			return nil
		},
	}

	historyRecordCmd := &cobra.Command{
		Use:     "record",
		Aliases: []string{"append"},
		Short:   "Collect and record a usage snapshot to the local machine history log",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var client *http.Client
			if !usageOffline {
				client = &http.Client{Timeout: 5 * time.Second}
			}
			summary := usage.CollectAll(ctx, "", client)
			historyDir := usage.HistoryDir("")
			if err := usage.AppendHistory(historyDir, summary); err != nil {
				return fmt.Errorf("record usage history: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "recorded usage history snapshot to %s\n", historyDir)
			return nil
		},
	}

	historyStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Display aggregate statistics across recorded usage history",
		RunE: func(cmd *cobra.Command, args []string) error {
			historyDir := usage.HistoryDir("")
			stats, err := usage.HistorySummaryStats(historyDir)
			if err != nil {
				return fmt.Errorf("history stats: %w", err)
			}
			if historyJSON {
				out, err := usage.RenderHistoryStatsJSON(stats)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			fmt.Print(usage.RenderHistoryStatsText(stats, historyDir))
			return nil
		},
	}

	historyCmd.AddCommand(historyTimelineCmd, historyFetchCmd, historyRecordCmd, historyStatsCmd)
	usageCmd.AddCommand(historyCmd)

	var applyDocs []string
	var forceDocs bool
	apply := &cobra.Command{
		Use:   "apply",
		Short: "Apply config.yaml to global Claude Code and agent harness directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, name, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			fmt.Printf("Applying %s → %s\n", name, t)
			return claude.ApplyAll(t, cfg, applyDocs, forceDocs)
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	apply.Flags().StringSliceVarP(&applyDocs, "docs", "d", nil, "doc(s) to install globally, comma-separated or repeated (e.g. golang,canary)")
	apply.Flags().BoolVar(&forceDocs, "force-docs", false, "overwrite existing docs with bundled versions")

	var diffExitCode bool
	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what apply would change in managed blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			changed, err := claude.DiffAll(t, cfg)
			if err != nil {
				return err
			}
			if diffExitCode && changed {
				os.Exit(1)
			}
			return nil
		},
	}
	diff.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	diff.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	diff.Flags().BoolVarP(&diffExitCode, "exit-code", "e", false, "exit with status 1 if drift/changes are found")

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
	var initDocs []string
	var initConfigPath string
	var initRepoMode string
	var initSummary, initUpdate, initReplace, initYes bool
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Set up a project directory with AGENTS.md, language docs, and Makefile targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(initConfigPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return claude.RunInit(initDir, cfg, initDocs, initRepoMode, initYes, initSummary, initUpdate, initReplace)
		},
	}
	initCmd.Flags().StringVarP(&initConfigPath, "config", "c", "", "path to config YAML file (default: embedded)")
	initCmd.Flags().StringVarP(&initDir, "dir", "d", ".", "project directory to initialise (default: current directory)")
	initCmd.Flags().StringSliceVar(&initDocs, "docs", nil, "docs to set up in the project, comma-separated or repeated (e.g. golang,canary)")
	initCmd.Flags().StringVarP(&initRepoMode, "repo-mode", "m", "", "repo git setup to note in AGENTS.md (solo, fork, team)")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "assume yes when reconciling Makefile targets (no prompt)")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "run claude -p to generate a project summary and add it to AGENTS.md")
	initCmd.Flags().BoolVar(&initUpdate, "update", false, "re-fetch and refresh the project summary (implies --summary)")
	initCmd.Flags().BoolVar(&initReplace, "replace", false, "delete existing AGENTS.md and recreate from template before init")

	root.AddCommand(apply, diff, clean, status, usageCmd, initCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
