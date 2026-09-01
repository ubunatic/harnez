package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/assess"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/usage"
)

// resolveUsageHost decides the effective --host value for `harnez usage`:
// the explicit flag always wins (issue 109 acceptance criterion 3); when the
// flag is omitted and cfg (loaded from ~/.config/harnez/local.yaml) carries
// a non-empty usage.default_host, that value is used as a convenience
// default. cfg may be nil (file absent or failed to load), in which case the
// flag value (possibly empty) is returned unchanged.
func resolveUsageHost(flagHost string, cfg *usage.LocalConfig) string {
	if flagHost != "" {
		return flagHost
	}
	if cfg == nil {
		return flagHost
	}
	return cfg.Usage.DefaultHost
}

// validateUsageFlags rejects `harnez usage` flag combinations that don't
// make sense together, ahead of any collection or rendering work.
//
// --compact requires --watch or --summary because both share the same
// compact, btop-style renderer (buildWatchFrame / compactWatchSections) that
// --compact toggles; the flat `harnez usage` report (neither flag set) has
// no compact mode to toggle at all (issue 102).
func validateUsageFlags(usageWatch, usageSummary, usageCompact bool) error {
	if usageWatch && usageSummary {
		return fmt.Errorf("--watch and --summary cannot be combined")
	}
	if usageCompact && !usageWatch && !usageSummary {
		return fmt.Errorf("--compact requires --watch or --summary")
	}
	return nil
}

func main() {
	var configPath string
	var target string

	root := &cobra.Command{
		Use:     "harnez",
		Short:   "Manage Claude Code, Prime Agent, and other agent harnesses from a YAML definition",
		Version: harnez.Version,
	}

	var usageJSON bool
	var usageAgent string
	var usageOffline bool
	var usageWatch bool
	var usageSummary bool
	var usageProcesses bool
	var usageCompact bool
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

			if err := validateUsageFlags(usageWatch, usageSummary, usageCompact); err != nil {
				return err
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

			if usageWatch {
				if usageJSON {
					return fmt.Errorf("--watch and --json cannot be combined")
				}
				// RemoteLoadSnapshot is intentionally left nil here:
				// RunWatchWithOptions owns fetching it itself (streaming
				// when possible, batch-polling fallback otherwise — issue
				// 110 Decision §2/§3), the same way it owns fetching
				// summary/rates/procs internally rather than the caller
				// pre-fetching a single snapshot up front.
				return usage.RunWatchWithOptions(ctx, "", client, cmd.OutOrStdout(), usageInterval, "", usage.WatchOptions{
					Host:           usageHost,
					Compact:        usageCompact,
					ShowProcesses:  usageProcesses,
					RemoteLoadHost: loadWatchHost,
				})
			}

			if usageSummary {
				if usageJSON {
					return fmt.Errorf("--summary and --json cannot be combined")
				}
				var remoteLoadSnap *usage.LoadSnapshot
				if loadWatchHost != "" {
					// --summary is a one-shot print (Decision §2): always a
					// single plain batch SSH call, independent of usageHost.
					remoteLoadSnap, _ = usage.CollectRemoteLoadSnapshot(ctx, loadWatchHost)
				}
				loadOpt := usage.WatchOptions{Compact: usageCompact, RemoteLoadHost: loadWatchHost, RemoteLoadSnapshot: remoteLoadSnap}
				if usageHost != "" {
					usage.RenderSummaryRemote(ctx, usageHost, cmd.OutOrStdout(), usageProcesses, loadOpt)
				} else {
					usage.RenderSummary(ctx, "", client, cmd.OutOrStdout(), usageProcesses, loadOpt)
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
				// Only attach a load snapshot when this invocation is itself
				// the local collection (usageHost == ""), which is also the
				// case when this process is the one CollectRemote runs over
				// SSH on the remote host. A local `--json --host` combo
				// already carries whatever load snapshot the remote side
				// attached, via summary from CollectRemote above.
				if usageHost == "" && summary.Load == nil {
					snap := usage.CollectLoadSnapshot()
					summary.Load = &snap
				}
				out, err := usage.RenderJSON(summary)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}

			var remoteLoadSnap *usage.LoadSnapshot
			if loadWatchHost != "" {
				remoteLoadSnap, _ = usage.CollectRemoteLoadSnapshot(ctx, loadWatchHost)
			}
			fmt.Print(usage.RenderText(summary, usage.WatchOptions{RemoteLoadHost: loadWatchHost, RemoteLoadSnapshot: remoteLoadSnap}))
			return nil
		},
	}
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output usage in JSON format")
	usageCmd.Flags().StringVar(&usageAgent, "agent", "", "filter to a specific agent (claude, agy, codex)")
	usageCmd.Flags().StringVar(&usageHost, "host", "", "query usage from a remote host via SSH")
	usageCmd.Flags().BoolVar(&usageOffline, "offline", false, "disable live network queries and use local caches only")
	usageCmd.Flags().BoolVarP(&usageWatch, "watch", "w", false, "live-refresh the dashboard in place with a tokens/min trend")
	usageCmd.Flags().BoolVar(&usageCompact, "compact", false, "start --watch with only all-usage and load panels visible")
	usageCmd.Flags().BoolVarP(&usageSummary, "summary", "s", false, "print the compact --watch-style dashboard once and exit")
	usageCmd.Flags().BoolVarP(&usageProcesses, "proc", "p", false, "show running agent processes panel in --watch / --summary")
	usageCmd.Flags().BoolVar(&usageProcesses, "processes", false, "show running agent processes panel in --watch / --summary")
	usageCmd.Flags().DurationVar(&usageInterval, "interval", usage.DefaultWatchInterval,
		fmt.Sprintf("refresh interval for --watch (minimum %s, to avoid hammering live quota APIs)", usage.MinWatchInterval))

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
	collectorCmd.Flags().DurationVar(&collectorInterval, "interval", usage.DefaultCollectorInterval,
		"snapshot collection interval")
	collectorCmd.Flags().BoolVar(&collectorOnce, "once", false, "collect and write snapshots once, then exit (no timer loop)")
	collectorCmd.Flags().BoolVar(&collectorOffline, "offline", false, "disable live network queries and use local caches only")

	var applyDocs []string
	var forceDocs bool
	var applySystemd bool
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
			return claude.ApplyAll(t, cfg, applyDocs, forceDocs, applySystemd)
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	apply.Flags().StringSliceVarP(&applyDocs, "docs", "d", nil, "doc(s) to install globally, comma-separated or repeated (e.g. golang,canary)")
	apply.Flags().BoolVar(&forceDocs, "force-docs", false, "overwrite existing docs with bundled versions")
	apply.Flags().BoolVar(&applySystemd, "systemd", false,
		"install the harnez-agent-collector systemd --user unit to ~/.config/systemd/user (issue 082)")

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
					os.Exit(1)
				}
				return nil
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
	diff.Flags().BoolVar(&captureDocs, "capture-docs", false, "write configured project-doc drift to an inbox Markdown report")
	diff.Flags().StringVar(&captureOutput, "out", "", "output path for --capture-docs (default: issues/inbox/managed-docs-drift-<timestamp>.md)")

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
	var initSummary, initUpdate, initReplace, initYes, initAll bool
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Set up a project directory with AGENTS.md, language docs, and Makefile targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(initConfigPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if initAll {
				return claude.RunInitAll(initDir, cfg, initDocs, initRepoMode, initSummary, initUpdate, initReplace)
			}
			return claude.RunInit(initDir, cfg, initDocs, initRepoMode, initYes, initSummary, initUpdate, initReplace)
		},
	}
	initCmd.Flags().StringVarP(&initConfigPath, "config", "c", "", "path to config YAML file (default: embedded)")
	initCmd.Flags().StringVarP(&initDir, "dir", "d", ".", "project directory to initialise (default: current directory)")
	initCmd.Flags().BoolVar(&initAll, "all", false,
		"treat --dir as a workspace directory and non-interactively init every eligible child (has AGENTS.md/CLAUDE.md); refuses $HOME (see issue 068)")
	initCmd.Flags().StringSliceVar(&initDocs, "docs", nil, "docs to set up in the project, comma-separated or repeated (e.g. golang,canary)")
	initCmd.Flags().StringVarP(&initRepoMode, "repo-mode", "m", "", "repo git setup to note in AGENTS.md (solo, fork, team)")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "assume yes when reconciling Makefile targets (no prompt)")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "run claude -p to generate a project summary and add it to AGENTS.md")
	initCmd.Flags().BoolVar(&initUpdate, "update", false, "re-fetch and refresh the project summary (implies --summary)")
	initCmd.Flags().BoolVar(&initReplace, "replace", false, "delete existing AGENTS.md and recreate from template before init")

	var assessJSON bool
	assessCmd := &cobra.Command{
		Use:   "assess [path]",
		Short: "Fast code/doc metrics, token estimation, and repository feasibility report",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}
			report, err := assess.AssessPath(targetPath)
			if err != nil {
				return fmt.Errorf("assess %s: %w", targetPath, err)
			}
			if assessJSON {
				out, err := assess.RenderJSON(report)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			fmt.Print(assess.RenderText(report))
			return nil
		},
	}
	assessCmd.Flags().BoolVar(&assessJSON, "json", false, "output report in JSON format")

	root.AddCommand(apply, diff, scanDocs, clean, status, usageCmd, loadStreamCmd, initCmd, assessCmd, collectorCmd, newDistillCmd(), newModeCmd(), newReleaseCmd(), newStatuslineCmd(), newRateCmd(), newExecCmd(), newStatsCmd(), newIndexCmd(), newRepoStatusCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
