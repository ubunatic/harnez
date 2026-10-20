package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/usage"
)

// newUsageCmd builds `harnez usage` with its history, export and archive subcommands.
func newUsageCmd() *cobra.Command {
	var usageJSON bool
	var usageAgent string
	var usageOffline bool
	var usageWatch bool
	var usageSplash bool
	var usageRaw bool
	var usageProcesses bool
	var usageMic bool
	var usageNormal bool
	var usageCompact bool
	var usageMinimal bool
	var usageLoom bool
	var usageInterval time.Duration
	var usageHost string
	var usageProject string
	var usageShared bool
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

			selectedMode, explicitMode, err := selectUsageViewMode(usageNormal, usageCompact, usageMinimal)
			if err != nil {
				return err
			}
			if err := validateUsageModeFlags(explicitMode, usageRaw, usageJSON, usageLoom); err != nil {
				return err
			}
			if !explicitMode {
				selectedMode, err = usage.DefaultUsageViewMode()
				if err != nil {
					return err
				}
			}
			usageCompactMode := selectedMode == "compact" || selectedMode == "minimal"
			if err := validateUsageFlags(usageWatch, usageRaw, usageJSON, explicitMode && usageCompactMode, usageLoom, usageSplash); err != nil {
				return err
			}
			_ = usageShared // retained as a compatibility flag; all local views use the collector store.

			// The local config file is optional and this is a convenience
			// default, not a required config — a load error (malformed
			// YAML, unreadable file) must not abort the command (issue
			// 109). Loaded unconditionally (not just when usageHost==""):
			// load.watch_host (issue 110) is independent of usage.host and
			// needed regardless of how usageHost was set.
			localCfg, _, _ := usage.LoadLocalConfig("")
			if usageHost == "" {
				usageHost = resolveUsageHost(usageHost, localCfg)
			}
			loadWatchHost := ""
			if localCfg != nil {
				loadWatchHost = strings.TrimSpace(localCfg.Load.WatchHost)
			}

			if usageLoom {
				var cached usage.UsageSummary
				if usageHost == "" {
					var err error
					cached, err = loadLocalUsage(ctx, usageOffline)
					if err != nil {
						return fmt.Errorf("read usage store: %w", err)
					}
				}
				loadOpt := usage.WatchOptions{
					Compact:        true,
					ShowProcesses:  usageProcesses,
					ShowMic:        usageMic,
					RemoteLoadHost: loadWatchHost,
					Host:           usageHost,
				}
				if usageHost == "" {
					loadOpt.StoredUsageLoader = func(context.Context) usage.UsageSummary { return cached }
				}
				return usage.RunLoom(ctx, "", client, cmd.OutOrStdout(), usageInterval, loadOpt)
			}

			if usageWatch {
				// RemoteLoadSnapshot is intentionally left nil here:
				// RunWatchWithOptions owns fetching it itself (streaming
				// when possible, batch-polling fallback otherwise — issue
				// 110 Decision §2/§3), the same way it owns fetching
				// summary/rates/procs internally rather than the caller
				// pre-fetching a single snapshot up front.
				var storedLoader func(context.Context) usage.UsageSummary
				if usageHost == "" {
					cached, err := loadLocalUsage(ctx, usageOffline)
					if err != nil {
						return fmt.Errorf("read usage store: %w", err)
					}
					storedLoader = func(ctx context.Context) usage.UsageSummary {
						if latest, err := loadLocalUsage(ctx, usageOffline); err == nil {
							cached = latest
						}
						return cached
					}
				}
				return usage.RunWatchWithOptions(ctx, "", client, cmd.OutOrStdout(), usageInterval, "", usage.WatchOptions{
					Host:              usageHost,
					StoredUsageLoader: storedLoader,
					Mode:              selectedMode,
					ShowProcesses:     usageProcesses,
					ShowMic:           usageMic,
					Splash:            usageSplash,
					RemoteLoadHost:    loadWatchHost,
				})
			}

			if usageProject != "" {
				res, err := usage.CollectProjectUsage(usageProject)
				if err != nil {
					return fmt.Errorf("project usage: %w", err)
				}
				if usageJSON {
					data, err := json.MarshalIndent(res, "", "  ")
					if err != nil {
						return fmt.Errorf("render json: %w", err)
					}
					fmt.Println(string(data))
					return nil
				}
				fmt.Print(usage.RenderProjectUsageCard(res))
				return nil
			}

			if usageJSON || usageRaw {
				var summary usage.UsageSummary
				if usageHost != "" {
					s, _, err := usage.CollectRemote(ctx, usageHost, usageProcesses)
					if err != nil && !usageJSON {
						return err
					}
					summary = s
				} else {
					summary, err = loadLocalUsage(ctx, usageOffline)
					if err != nil {
						return fmt.Errorf("read usage store: %w", err)
					}
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
					out, err := renderUsageOutput(summary, true, false)
					if err != nil {
						return err
					}
					fmt.Println(out)
					return nil
				}

				// usageRaw: the detailed per-field text report that used to
				// be `harnez usage`'s only output before the compact
				// dashboard (formerly --summary) became the default.
				var remoteLoadSnap *usage.LoadSnapshot
				if loadWatchHost != "" {
					remoteLoadSnap, _ = usage.CollectRemoteLoadSnapshot(ctx, loadWatchHost)
				}
				out, err := renderUsageOutput(summary, false, true, usage.WatchOptions{RemoteLoadHost: loadWatchHost, RemoteLoadSnapshot: remoteLoadSnap})
				if err != nil {
					return err
				}
				fmt.Print(out)
				return nil
			}

			// The selected spec-defined view is printed once. --watch enters
			// this same view in the interactive redraw loop.
			var remoteLoadSnap *usage.LoadSnapshot
			if loadWatchHost != "" {
				// This is a one-shot print (Decision §2 of issue 110): always
				// a single plain batch SSH call, independent of usageHost.
				remoteLoadSnap, _ = usage.CollectRemoteLoadSnapshot(ctx, loadWatchHost)
			}
			loadOpt := usage.WatchOptions{Mode: selectedMode, RemoteLoadHost: loadWatchHost, RemoteLoadSnapshot: remoteLoadSnap}
			if usageHost != "" {
				usage.RenderSummaryRemote(ctx, usageHost, cmd.OutOrStdout(), usageProcesses, loadOpt)
			} else {
				summary, err := loadLocalUsage(ctx, usageOffline)
				if err != nil {
					return fmt.Errorf("read usage store: %w", err)
				}
				usage.RenderSummaryWithUsage(summary, cmd.OutOrStdout(), usageProcesses, loadOpt)
			}
			return nil
		},
	}
	usageCmd.Flags().StringVar(&usageProject, "project", "", "attribute lifetime tokens and cost-of-change to a repository directory")
	usageCmd.Flags().StringVar(&usageProject, "cwd", "", "alias for --project")
	usageCmd.PersistentFlags().BoolVar(&usageShared, "shared", false, "deprecated compatibility flag; local usage reads the collector store")
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output usage in JSON format")
	usageCmd.Flags().StringVar(&usageAgent, "agent", "", "filter to a specific agent (claude, agy, codex)")
	usageCmd.Flags().StringVar(&usageHost, "host", "", "query usage from a remote host via SSH")
	usageCmd.Flags().BoolVar(&usageOffline, "offline", false, "read stored usage without starting the collector")
	usageCmd.Flags().BoolVarP(&usageWatch, "watch", "w", false, "refresh the stored view in place with collector age markers")
	usageCmd.Flags().BoolVar(&usageSplash, "splash", false, "render startup splash while the first stored frame loads (--watch only)")
	usageCmd.Flags().BoolVar(&usageNormal, "normal", false, "show the usage table (the default view)")
	usageCmd.Flags().BoolVar(&usageCompact, "compact", false, "show the compact all-usage and load panels")
	usageCmd.Flags().BoolVar(&usageMinimal, "minimal", false, "show compact panels without title/status bars or hidden-panel and overflow hints")
	usageCmd.Flags().BoolVar(&usageLoom, "loom", false, "start usage monitor as loom app (compact view)")
	usageCmd.Flags().BoolVarP(&usageRaw, "raw", "r", false, "print the detailed per-field usage report instead of the selected view")
	usageCmd.Flags().BoolVarP(&usageProcesses, "proc", "p", false, "show running agent processes panel in the default view / --watch")
	usageCmd.Flags().BoolVar(&usageProcesses, "processes", false, "show running agent processes panel in the default view / --watch")
	usageCmd.Flags().BoolVar(&usageMic, "mic", false, "show the microphone level/recording panel in --watch (hidden if no audio interface is found)")
	usageCmd.Flags().DurationVar(&usageInterval, "interval", usage.DefaultWatchInterval,
		fmt.Sprintf("refresh interval for --watch (minimum %s, to avoid hammering live quota APIs)", usage.MinWatchInterval))

	usageCmd.AddCommand(newUsageHistoryCmd(&usageOffline, &usageShared), newUsageExportCmd(), newUsageArchiveCmd())
	return usageCmd
}

// newUsageHistoryCmd builds `harnez usage history` and its subcommands.
// offline points at the parent usage command's flag value.
func newUsageHistoryCmd(offline, shared *bool) *cobra.Command {
	_ = shared // retained in the constructor to avoid breaking internal callers.
	var historyJSON bool
	historyCmd := &cobra.Command{
		Use:     "history",
		Aliases: []string{"timeline"},
		Short:   "Inspect and manage recorded usage history and timelines",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := renderUsageTimeline(usage.HistoryDir(""), historyJSON)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		},
	}
	historyCmd.PersistentFlags().BoolVar(&historyJSON, "json", false, "output history in JSON format")

	historyTimelineCmd := &cobra.Command{
		Use:   "timeline",
		Short: "Display snapshot and quota history across all recorded machine logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := renderUsageTimeline(usage.HistoryDir(""), historyJSON)
			if err != nil {
				return err
			}
			fmt.Println(out)
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
			summary, err := loadLocalUsage(ctx, *offline)
			if err != nil {
				return fmt.Errorf("read usage store: %w", err)
			}
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
	return historyCmd
}

// newLoadStreamCmd builds the hidden load-stream command used by remote load panels.
func newLoadStreamCmd() *cobra.Command {
	loadStreamCmd := &cobra.Command{
		Use:    "load-stream",
		Short:  "Stream local CPU/GPU load samples as NDJSON (internal: driven remotely by usage --watch, issue 110)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return usage.RunLoadStream(ctx, cmd.OutOrStdout(), cmd.InOrStdin())
		},
	}
	return loadStreamCmd
}

// newCollectorCmd builds the usage snapshot collector command.
func newCollectorCmd() *cobra.Command {
	var collectorInterval time.Duration
	var collectorOnce bool
	var collectorOffline bool
	collectorCmd := &cobra.Command{
		Use:   "agent-collector",
		Short: "Run the local server and usage collector (compatibility alias for server)",
		Long: "Compatibility command for `harnez server`. Runs the local server and usage collector,\n" +
			"which writes normalized observations, history, and JSON snapshots. The --once option keeps\n" +
			"its one-shot snapshot behavior; the systemd user unit runs `harnez server`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var client *http.Client
			if !collectorOffline {
				client = &http.Client{Timeout: 5 * time.Second}
			}
			if collectorOnce {
				summary := usage.CollectAllLive(cmd.Context(), "", client)
				if err := usage.InitializeUsageStore(cmd.Context(), "", ""); err != nil {
					return fmt.Errorf("initialize usage store: %w", err)
				}
				if err := usage.PersistUsageSummary(cmd.Context(), "", "", summary); err != nil {
					return fmt.Errorf("persist usage store: %w", err)
				}
				stateDir := usage.StateDir("")
				for _, agent := range summary.Agents {
					if err := usage.PersistAgentSnapshot(stateDir, agent, collectorOffline); err != nil {
						return fmt.Errorf("write %s snapshot: %w", agent.AgentID, err)
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote agent usage snapshots to %s\n", stateDir)
				return nil
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runHarnezServer(ctx, collectorInterval, collectorOffline)
		},
	}
	collectorCmd.Flags().DurationVar(&collectorInterval, "interval", usage.DefaultCollectorInterval(),
		"snapshot collection interval")
	collectorCmd.Flags().BoolVar(&collectorOnce, "once", false, "collect and write snapshots once, then exit (no timer loop)")
	collectorCmd.Flags().BoolVar(&collectorOffline, "offline", false, "disable live network queries and use local caches only")
	return collectorCmd
}
