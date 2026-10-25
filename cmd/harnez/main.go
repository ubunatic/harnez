package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/sessionstate"
	"ubunatic.com/harnez/internal/telemetry"
)

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
			if db, err := telemetry.OpenReadOnly(dbPath); err == nil {
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
		Version:           harnez.BuildVersion(),
		PersistentPreRunE: sessionTipHook,
	}
	root.AddCommand(newAgyMeterCmd())

	root.AddCommand(newChannelCmd(), newComposeCmd(), newInstallCmd(), newSessionCmd())
	root.AddCommand(newApplyCmd(), newDiffCmd(), newScanDocsCmd(), newStatusCmd(), newRevertCmd(), newCleanCmd(), newUsageCmd(), newLoadStreamCmd(), newInitCmd(), newAssessCmd(), newCollectorCmd(), newServerCmd(), newDistillCmd(), newModeCmd(), newReleaseCmd(), newRateCmd(), newExecCmd(), newStatsCmd(), newIndexCmd(), newRepoStatusCmd(), newFindCmd(), newIssuesCmd(), newCompactCheckCmd(), newFeedbackCmd(), newDocHistoryCmd(), newRepoHistoryCmd(), newCodexHookCmd(), newCodexTelemetryCmd(), newHookCmd(), newStatuslineCmd(), newAGYStatuslineCmd(), newLintCmd(), newLogCmd(), newDocsCmd(), newReadCmd(), newSubagentCmd(), newAgentCmd(), newMCPCmd(), newBenchCmd(), newSkillCmd(), newSkillsCmd(), newDecideCmd(), newGuardCmd(), newCompactCmd(), newTUICmd(), newManCmd(root))
	return root
}

// ensureTelemetrySchema asks the server to apply pending additive migrations.
func ensureTelemetrySchema() error {
	config, err := server.LoadConfig()
	if err != nil {
		return err
	}
	socket, err := config.Socket()
	if err != nil {
		return err
	}
	request, err := json.Marshal(server.TelemetryRecord{Kind: "ensure_schema", Payload: json.RawMessage(`{}`)})
	if err != nil {
		return err
	}
	body, err := server.PostTelemetryCommand(context.Background(), socket, config.TelemetryPath, request, config.HTTPClientTimeout)
	if err == nil {
		var report struct {
			Version    int      `json:"version"`
			Migrations []string `json:"migrations"`
		}
		if len(body) != 0 {
			if err := json.Unmarshal(body, &report); err != nil {
				return err
			}
		}
		if len(report.Migrations) > 0 {
			fmt.Printf("  telemetry schema: v%d (migrations: %s)\n", report.Version, strings.Join(report.Migrations, ", "))
		}
		return nil
	}
	spoolDir, spoolErr := config.TelemetrySpoolDir()
	if spoolErr != nil {
		return spoolErr
	}
	if server.AppendTelemetrySpool(spoolDir, request, config.TelemetrySpoolMaxBytes) != nil {
		return fmt.Errorf("telemetry schema: server unavailable and spool write failed: %w", err)
	}
	return nil
}
