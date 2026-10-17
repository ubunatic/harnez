package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
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
var sessionTipSessionOptions = func() resolve.SessionOptions {
	return resolve.SessionOptions{}
}

var sessionTipCounts = sessionCallCounts

func sessionTipHook(cmd *cobra.Command, _ []string) error {
	sessionID, err := resolve.Session(sessionTipSessionOptions())
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
	if cmd.Name() != "apply" {
		// apply migrates the telemetry DB in its command body; the session-tip
		// hook must not open it before that migration runs.
		if dbPath, err := telemetry.DefaultDBPath(); err == nil {
			sessionstate.ApplyCounts(&s, sessionTipCounts(dbPath, sessionID))
		}
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
	if !feedbackDisabled && cmd.Name() != "apply" {
		if dbPath, err := telemetry.DefaultDBPath(); err == nil {
			if db, err := telemetry.Open(dbPath); err == nil {
				if n, err := db.UnratedFailureCount(telemetry.Filter{SessionID: sessionID}); err == nil {
					unratedFailures = int(n)
				}
				db.Close()
			}
		}
	}

	// `harnez agent` output is read by host agents as the worker's reply
	// channel, so proactive tips stay out of it; the tip is left pending for
	// the next non-agent command.
	if tip, ok := sessionstate.GapTip(s, feedbackDisabled, now, unratedFailures); ok && !underAgentCommand(cmd) {
		fmt.Fprintln(cmd.ErrOrStderr(), tip)
		s.TotalAtLastTip = s.Total
	}

	_ = sessionstate.Save(stateDir, s) // best-effort; a lost tick isn't worth surfacing an error for
	return nil
}

// underAgentCommand reports whether cmd is `harnez agent` or one of its subcommands.
func underAgentCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "agent" && c.Parent() != nil && !c.Parent().HasParent() {
			return true
		}
	}
	return false
}

func resolveComponentSelection(cfg *claude.Config, flag []string) (claude.Set, error) {
	if err := claude.ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return claude.Resolve(flag, cfg.ComponentNames, nil)
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
// The spec-declared usage view is the default rendering for a bare
// `harnez usage`; --raw opts into the detailed per-field text report, and --json
// opts into machine-readable output. --watch/--raw/--json conflicts and
// --splash's watch requirement are rejected here.
func validateUsageFlags(usageWatch, usageRaw, usageJSON, usageCompact, usageLoom, usageSplash bool) error {
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
	if usageLoom && usageRaw {
		return fmt.Errorf("--loom has no effect with --raw")
	}
	if usageLoom && usageJSON {
		return fmt.Errorf("--loom and --json cannot be combined")
	}
	if usageSplash && !usageWatch {
		return fmt.Errorf("--splash requires --watch")
	}
	return nil
}

func validateUsageModeFlags(explicitMode, usageRaw, usageJSON, usageLoom bool) error {
	if explicitMode && (usageRaw || usageJSON || usageLoom) {
		return fmt.Errorf("usage view mode flags cannot be combined with --raw, --json, or --loom")
	}
	return nil
}

func selectUsageViewMode(normal, compact, minimal bool) (string, bool, error) {
	selected := 0
	mode := ""
	if normal {
		selected++
		mode = "normal"
	}
	if compact {
		selected++
		mode = "compact"
	}
	if minimal {
		selected++
		mode = "minimal"
	}
	if selected > 1 {
		return "", true, fmt.Errorf("--normal, --compact, and --minimal are mutually exclusive")
	}
	return mode, selected == 1, nil
}

func main() {
	if len(os.Args) > 0 && isGearInvocation(os.Args[0]) {
		os.Args = append([]string{"harnez", "exec"}, os.Args[1:]...)
	}

	root := newRootCmd()
	// executeAndRecord, not root.Execute, is the entry point: issue 326's
	// cli_invocations row can only be written from here, around Execute —
	// see cmd/harnez/clilog.go for why neither of Cobra's hook points works.
	if err := executeAndRecord(root, os.Args[1:], cliLogOptions{}); err != nil {
		os.Exit(exitCodeFromRunError(err))
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:               "harnez",
		Short:             "Manage Claude Code, Prime Agent, and other agent harnesses from a YAML definition",
		Version:           harnez.Version,
		PersistentPreRunE: sessionTipHook,
	}
	root.AddCommand(newAgyMeterCmd())

	root.AddCommand(newChannelCmd(), newComposeCmd())
	root.AddCommand(newApplyCmd(), newDiffCmd(), newScanDocsCmd(), newStatusCmd(), newRevertCmd(), newCleanCmd(), newUsageCmd(), newLoadStreamCmd(), newInitCmd(), newAssessCmd(), newCollectorCmd(), newDistillCmd(), newModeCmd(), newReleaseCmd(), newRateCmd(), newExecCmd(), newStatsCmd(), newIndexCmd(), newRepoStatusCmd(), newFindCmd(), newIssuesCmd(), newCompactCheckCmd(), newFeedbackCmd(), newDocHistoryCmd(), newRepoHistoryCmd(), newCodexHookCmd(), newCodexTelemetryCmd(), newHookCmd(), newStatuslineCmd(), newAGYStatuslineCmd(), newLintCmd(), newLogCmd(), newDocsCmd(), newReadCmd(), newSubagentCmd(), newAgentCmd(), newMCPCmd(), newBenchCmd(), newSkillCmd(), newSkillsCmd(), newDecideCmd(), newGuardCmd(), newCompactCmd(), newTUICmd(), newManCmd(root))
	return root
}

// ensureTelemetrySchema opens and closes the shared telemetry store so apply
// performs pending additive migrations before installing hooks that write to it.
func ensureTelemetrySchema() error {
	path, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	db, err := telemetry.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.ValidateSchema(); err != nil {
		return err
	}
	version, err := db.SchemaVersion()
	if err != nil {
		return err
	}
	migrations := db.Migrations()
	if len(migrations) > 0 {
		fmt.Printf("  telemetry schema: v%d (migrations: %s)\n", version, strings.Join(migrations, ", "))
	}
	return nil
}
