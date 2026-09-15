package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/assess"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/sessionstate"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

// isGearInvocation reports whether the program was invoked under the ⚙ alias / multicall name.
func isGearInvocation(arg0 string) bool {
	base := filepath.Base(arg0)
	return base == "⚙" || base == "⚙️" || base == "\xe2\x9a\x99" || base == "\xe2\x9a\x99\xef\xb8\x8f"
}

// sessionTipHook is `harnez`'s CLI-dispatch hook point for issue 183's
// session-state tracking: it fires (as root's PersistentPreRunE) on every
// subcommand invocation, records it against this session's usage history,
// and — best-effort, never fatal — prints at most one short proactive tip
// to stderr when sessionstate.GapTip finds something worth surfacing. Any
// failure here (can't resolve a session id, can't read/write the state
// file) is swallowed silently: this is a nice-to-have nudge, not something
// that should ever block or fail a real command.
func sessionTipHook(cmd *cobra.Command, _ []string) error {
	sessionID, err := resolve.Session(resolve.SessionOptions{})
	if err != nil || sessionID == "" {
		return nil
	}
	stateDir := resolve.DefaultStateDir()

	s, err := sessionstate.Load(stateDir, sessionID)
	if err != nil {
		return nil
	}
	now := time.Now()
	// Issue 328: cli_invocations (issue 326) is the authoritative count of
	// what this session has run, so the counting half of sessionstate's
	// state is re-sourced from it here rather than accumulated twice.
	// ApplyCounts lands the durable baseline first; Record then adds the
	// in-flight invocation, whose own row is only written after the command
	// body returns. A nil map (no DB, unreadable DB, empty table) means the
	// JSON file's own counts stand and the tips keep working unchanged.
	if dbPath, err := telemetry.DefaultDBPath(); err == nil {
		sessionstate.ApplyCounts(&s, sessionCallCounts(dbPath, sessionID))
	}
	sessionstate.Record(&s, cmd.Name(), now)

	// feedbackDisabled mirrors issue 142's opt-out: a session with the Tool
	// Feedback Protocol disabled (config.yaml's feedback.disable_rate_protocol
	// or $HARNEZ_DISABLE_RATE_FEEDBACK) shouldn't get nagged about either
	// half of it — the failure-rating reminder or issue 179's --ok heartbeat
	// nudge. Best-effort: a failed config load just leaves tips enabled,
	// matching this hook's overall "never block a real command" stance.
	feedbackDisabled := false
	if cfg, cfgErr := claude.LoadConfigEmbedded(); cfgErr == nil {
		feedbackDisabled = claude.RateFeedbackDisabled(cfg, nil)
	}

	// Issue 188: query telemetry for genuinely-failed tool calls this
	// session that have gone unrated since the last `harnez rate` call —
	// see telemetry.UnratedFailureCount's doc comment for the session-window
	// linkage heuristic. Best-effort like everything else in this hook: a
	// missing/unopenable DB just means the sharper nudge can't fire this
	// call, falling back to GapTip's plain count/time-based tips.
	unratedFailures := 0
	if !feedbackDisabled {
		if dbPath, err := telemetry.DefaultDBPath(); err == nil {
			if db, err := telemetry.Open(dbPath); err == nil {
				if n, err := db.UnratedFailureCount(telemetry.Filter{SessionID: sessionID}); err == nil {
					unratedFailures = int(n)
				}
				db.Close()
			}
		}
	}

	if tip, ok := sessionstate.GapTip(s, feedbackDisabled, now, unratedFailures); ok {
		fmt.Fprintln(cmd.ErrOrStderr(), tip)
		s.TotalAtLastTip = s.Total
	}

	_ = sessionstate.Save(stateDir, s) // best-effort; a lost tick isn't worth surfacing an error for
	return nil
}

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
// The compact one-shot dashboard (formerly gated behind a now-removed
// --summary flag) is the default rendering for a bare `harnez usage`; --raw
// opts into the detailed per-field text report instead, and --json opts
// into machine-readable output. All three are mutually exclusive render
// targets, so --watch/--raw/--json pairwise conflicts are rejected here, and
// --compact (a panel-selection toggle, not a render target) is rejected
// alongside --raw since --raw has no panel concept to toggle.
func validateUsageFlags(usageWatch, usageRaw, usageJSON, usageCompact bool) error {
	if usageWatch && usageJSON {
		return fmt.Errorf("--watch and --json cannot be combined")
	}
	if usageWatch && usageRaw {
		return fmt.Errorf("--watch and --raw cannot be combined")
	}
	if usageRaw && usageJSON {
		return fmt.Errorf("--raw and --json cannot be combined")
	}
	if usageCompact && usageRaw {
		return fmt.Errorf("--compact has no effect with --raw")
	}
	return nil
}

func main() {
	if len(os.Args) > 0 && isGearInvocation(os.Args[0]) {
		os.Args = append([]string{"harnez", "exec"}, os.Args[1:]...)
	}

	var configPath string
	var target string

	root := &cobra.Command{
		Use:               "harnez",
		Short:             "Manage Claude Code, Prime Agent, and other agent harnesses from a YAML definition",
		Version:           harnez.Version,
		PersistentPreRunE: sessionTipHook,
	}

	var usageJSON bool
	var usageAgent string
	var usageOffline bool
	var usageWatch bool
	var usageRaw bool
	var usageProcesses bool
	var usageMic bool
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

			if err := validateUsageFlags(usageWatch, usageRaw, usageJSON, usageCompact); err != nil {
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
					ShowMic:        usageMic,
					RemoteLoadHost: loadWatchHost,
				})
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
					out, err := usage.RenderJSON(summary)
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
				fmt.Print(usage.RenderText(summary, usage.WatchOptions{RemoteLoadHost: loadWatchHost, RemoteLoadSnapshot: remoteLoadSnap}))
				return nil
			}

			// Default: the compact one-shot dashboard, formerly gated
			// behind --summary. --summary was removed (issue 171) once this
			// became the unconditional default for a bare `harnez usage`.
			var remoteLoadSnap *usage.LoadSnapshot
			if loadWatchHost != "" {
				// This is a one-shot print (Decision §2 of issue 110): always
				// a single plain batch SSH call, independent of usageHost.
				remoteLoadSnap, _ = usage.CollectRemoteLoadSnapshot(ctx, loadWatchHost)
			}
			loadOpt := usage.WatchOptions{Compact: usageCompact, RemoteLoadHost: loadWatchHost, RemoteLoadSnapshot: remoteLoadSnap}
			if usageHost != "" {
				usage.RenderSummaryRemote(ctx, usageHost, cmd.OutOrStdout(), usageProcesses, loadOpt)
			} else {
				usage.RenderSummary(ctx, "", client, cmd.OutOrStdout(), usageProcesses, loadOpt)
			}
			return nil
		},
	}
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output usage in JSON format")
	usageCmd.Flags().StringVar(&usageAgent, "agent", "", "filter to a specific agent (claude, agy, codex)")
	usageCmd.Flags().StringVar(&usageHost, "host", "", "query usage from a remote host via SSH")
	usageCmd.Flags().BoolVar(&usageOffline, "offline", false, "disable live network queries and use local caches only")
	usageCmd.Flags().BoolVarP(&usageWatch, "watch", "w", false, "live-refresh the dashboard in place with a tokens/min trend")
	usageCmd.Flags().BoolVar(&usageCompact, "compact", false, "show only the all-usage and load panels (default view and --watch)")
	usageCmd.Flags().BoolVarP(&usageRaw, "raw", "r", false, "print the detailed per-field usage report instead of the compact dashboard")
	usageCmd.Flags().BoolVarP(&usageProcesses, "proc", "p", false, "show running agent processes panel in the default view / --watch")
	usageCmd.Flags().BoolVar(&usageProcesses, "processes", false, "show running agent processes panel in the default view / --watch")
	usageCmd.Flags().BoolVar(&usageMic, "mic", false, "show the microphone level/recording panel in --watch (hidden if no audio interface is found)")
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
	usageCmd.AddCommand(historyCmd, newUsageExportCmd())

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
	var applyShell bool
	var debloat bool
	var debloatPreset string
	var debloatNotebookEdit bool
	var debloatCron bool
	var debloatDisableBundledSkills bool
	var debloatDisableWorkflows bool
	var debloatDisableRemoteControl bool
	var debloatDisableClaudeAiConnectors bool
	var debloatDisableArtifact bool
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
			if err := claude.ApplyAll(t, cfg, applyDocs, forceDocs, applySystemd, applyShell); err != nil {
				return err
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
				fmt.Printf("Applying debloat (preset=%q) → %s\n", opts.Preset, filepath.Join(t, "settings.json"))
				return claude.ApplyDebloat(t, opts)
			}
			return nil
		},
	}
	apply.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	apply.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	apply.Flags().StringSliceVarP(&applyDocs, "docs", "d", nil, "doc(s) to install globally, comma-separated or repeated (e.g. golang,canary)")
	apply.Flags().BoolVar(&forceDocs, "force-docs", false, "overwrite existing docs with bundled versions")
	apply.Flags().BoolVar(&applySystemd, "systemd", false,
		"install the harnez-agent-collector systemd --user unit to ~/.config/systemd/user (issue 082)")
	apply.Flags().BoolVarP(&applyShell, "shell", "s", false,
		"inject harnez environment source into ~/.bashrc and ~/.zshrc")
	apply.Flags().BoolVar(&debloat, "debloat", false,
		"deny integration-only tools in settings.json to reduce context/token cost (default preset: minimal; issue 316)")
	apply.Flags().StringVar(&debloatPreset, "debloat-preset", "",
		`debloat preset: "minimal" (default) or "aggressive" (also denies interaction/safety tools; explicit opt-in only)`)
	apply.Flags().BoolVar(&debloatNotebookEdit, "debloat-notebook-edit", false, "also deny NotebookEdit")
	apply.Flags().BoolVar(&debloatCron, "debloat-cron", false, "also deny CronCreate/CronDelete/CronList")
	apply.Flags().BoolVar(&debloatDisableBundledSkills, "debloat-disable-bundled-skills", false, "set disableBundledSkills: true")
	apply.Flags().BoolVar(&debloatDisableWorkflows, "debloat-disable-workflows", false, "set disableWorkflows: true")
	apply.Flags().BoolVar(&debloatDisableRemoteControl, "debloat-disable-remote-control", false, "set disableRemoteControl: true")
	apply.Flags().BoolVar(&debloatDisableClaudeAiConnectors, "debloat-disable-claude-ai-connectors", false, "set disableClaudeAiConnectors: true")
	apply.Flags().BoolVar(&debloatDisableArtifact, "debloat-disable-artifact", false, "set disableArtifact: true")

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
					return silenceIfExitCode(cmd, &exitCodeError{Code: 1})
				}
				return nil
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			changed, err := claude.DiffAll(t, cfg)
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
			t := claude.ExpandTarget(target, cfg.TargetDir)
			if statusDebloat {
				return claude.StatusDebloat(t)
			}
			return claude.RunStatus(name, cfg, t)
		},
	}
	status.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	status.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	status.Flags().BoolVar(&statusDebloat, "debloat", false, "show debloat-managed deny entries and toggles instead of full status (issue 316)")

	var revertDebloat bool
	revert := &cobra.Command{
		Use:          "revert",
		Short:        "Revert managed one-off changes (currently: --debloat)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			if !revertDebloat {
				return fmt.Errorf("revert requires --debloat")
			}
			if err := claude.RevertDebloat(t); err != nil {
				return err
			}
			fmt.Printf("Reverted debloat changes in %s\n", filepath.Join(t, "settings.json"))
			return nil
		},
	}
	revert.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	revert.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	revert.Flags().BoolVar(&revertDebloat, "debloat", false, "restore settings.json to its pre-debloat state (issue 316)")

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

	root.AddCommand(apply, diff, scanDocs, clean, status, revert, usageCmd, loadStreamCmd, newInitCmd(), assessCmd, collectorCmd, newDistillCmd(), newModeCmd(), newReleaseCmd(), newStatuslineCmd(), newRateCmd(), newExecCmd(), newStatsCmd(), newIndexCmd(), newRepoStatusCmd(), newFindCmd(), newIssuesCmd(), newCompactCheckCmd(), newFeedbackCmd(), newDocHistoryCmd(), newCodexHookCmd(), newLintCmd(), newLogCmd())
	// executeAndRecord, not root.Execute, is the entry point: issue 326's
	// cli_invocations row can only be written from here, around Execute —
	// see cmd/harnez/clilog.go for why neither of Cobra's hook points works.
	if err := executeAndRecord(root, os.Args[1:], cliLogOptions{}); err != nil {
		os.Exit(exitCodeFromRunError(err))
	}
}
