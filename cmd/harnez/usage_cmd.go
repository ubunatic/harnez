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
	sharedusage "ubunatic.com/harnez/usage"
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
			if usageShared && usageHost != "" {
				return fmt.Errorf("--shared cannot be used with --host; shared usage is local-only")
			}
			if usageShared && usageProject != "" {
				return fmt.Errorf("--shared cannot be used with --project; project attribution uses its own local data source")
			}
			var sharedClient sharedusage.Client
			if usageShared {
				var err error
				sharedClient, err = sharedusage.Open(sharedusage.Options{
					StartIfAbsent: !usageOffline,
					Collector:     realUsageCollector{client: client},
				})
				if err != nil {
					return fmt.Errorf("open shared usage controller: %w", err)
				}
				defer sharedClient.Close()
				releaseShared, err := pinSharedClient(ctx, sharedClient)
				if err != nil {
					return err
				}
				defer releaseShared()
			}
			sharedCollect := func(ctx context.Context) usage.UsageSummary {
				return collectSharedUsage(ctx, sharedClient, !usageOffline)
			}

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
				loadOpt := usage.WatchOptions{
					Compact:        true,
					ShowProcesses:  usageProcesses,
					ShowMic:        usageMic,
					RemoteLoadHost: loadWatchHost,
					Host:           usageHost,
				}
				if sharedClient != nil {
					loadOpt.SharedUsageCollector = sharedCollect
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
				var sharedWatchCollector func(context.Context) usage.UsageSummary
				if sharedClient != nil {
					sharedWatchCollector = sharedCollect
				}
				return usage.RunWatchWithOptions(ctx, "", client, cmd.OutOrStdout(), usageInterval, "", usage.WatchOptions{
					Host:                 usageHost,
					SharedUsageCollector: sharedWatchCollector,
					Mode:                 selectedMode,
					ShowProcesses:        usageProcesses,
					ShowMic:              usageMic,
					Splash:               usageSplash,
					RemoteLoadHost:       loadWatchHost,
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
					if sharedClient != nil {
						summary = sharedCollect(ctx)
					} else {
						summary = usage.CollectAll(ctx, "", client)
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
					// Only attach a load snapshot when this invocation is
					// itself the local collection (usageHost == ""), which
					// is also the case when this process is the one
					// CollectRemote runs over SSH on the remote host. A
					// local `--json --host` combo already carries whatever
					// load snapshot the remote side attached, via summary
					// from CollectRemote above.
					if usageHost == "" && summary.Load == nil {
						snap := usage.CollectLoadSnapshot()
						summary.Load = &snap
					}
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
				if sharedClient != nil {
					loadOpt.SharedUsageCollector = sharedCollect
				}
				if sharedClient == nil {
					summary := usage.CollectAll(ctx, "", client)
					stored, err := usage.StoreCompactSummary(ctx, "", summary)
					if err != nil {
						debugLog("usage store persistence failed: %v", err)
					}
					usage.RenderSummaryWithUsage(stored, cmd.OutOrStdout(), usageProcesses, loadOpt)
				} else {
					usage.RenderSummary(ctx, "", client, cmd.OutOrStdout(), usageProcesses, loadOpt)
				}
			}
			return nil
		},
	}
	usageCmd.Flags().StringVar(&usageProject, "project", "", "attribute lifetime tokens and cost-of-change to a repository directory")
	usageCmd.Flags().StringVar(&usageProject, "cwd", "", "alias for --project")
	usageCmd.PersistentFlags().BoolVar(&usageShared, "shared", false, "use the shared local usage controller")
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output usage in JSON format")
	usageCmd.Flags().StringVar(&usageAgent, "agent", "", "filter to a specific agent (claude, agy, codex)")
	usageCmd.Flags().StringVar(&usageHost, "host", "", "query usage from a remote host via SSH")
	usageCmd.Flags().BoolVar(&usageOffline, "offline", false, "disable live network queries and use local caches only")
	usageCmd.Flags().BoolVarP(&usageWatch, "watch", "w", false, "live-refresh the selected view in place with a tokens/min trend")
	usageCmd.Flags().BoolVar(&usageSplash, "splash", false, "render startup splash screen while initial collection is in flight (--watch only)")
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
// offline and shared point at the parent usage command's flag values:
// `history record` reads the inherited --shared persistent flag.
func newUsageHistoryCmd(offline, shared *bool) *cobra.Command {
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
			var client *http.Client
			if !*offline {
				client = &http.Client{Timeout: 5 * time.Second}
			}
			var summary usage.UsageSummary
			if *shared {
				sharedClient, err := sharedusage.Open(sharedusage.Options{StartIfAbsent: !*offline, Collector: realUsageCollector{client: client}})
				if err != nil {
					return fmt.Errorf("open shared usage controller: %w", err)
				}
				defer sharedClient.Close()
				releaseShared, err := pinSharedClient(ctx, sharedClient)
				if err != nil {
					return err
				}
				defer releaseShared()
				summary = collectSharedUsage(ctx, sharedClient, !*offline)
			} else {
				summary = usage.CollectAll(ctx, "", client)
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
		Short: "Run the background usage-collector daemon (see systemd/harnez-agent-collector.service)",
		Long: "Runs the Claude/Codex/AGY usage collectors on a timer and atomically writes one JSON\n" +
			"snapshot per agent to the shared harnez state directory (see `harnez usage --json` for the\n" +
			"schema of each snapshot's \"usage\" field). `harnez usage` and the --watch TUI read this\n" +
			"cache first, falling back to a live collect if it's missing or stale. Intended to run as\n" +
			"a systemd --user service; install the unit with `harnez apply` (see docs/CLIDesign.md).",
		RunE: func(cmd *cobra.Command, args []string) error {
			var client *http.Client
			if !collectorOffline {
				client = &http.Client{Timeout: 5 * time.Second}
			}
			if collectorOnce {
				summary := usage.CollectAllLive(cmd.Context(), "", client)
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
			return usage.RunCollector(ctx, "", client, collectorInterval, cmd.OutOrStdout(), collectorOffline)
		},
	}
	collectorCmd.Flags().DurationVar(&collectorInterval, "interval", usage.DefaultCollectorInterval(),
		"snapshot collection interval")
	collectorCmd.Flags().BoolVar(&collectorOnce, "once", false, "collect and write snapshots once, then exit (no timer loop)")
	collectorCmd.Flags().BoolVar(&collectorOffline, "offline", false, "disable live network queries and use local caches only")
	return collectorCmd
}
