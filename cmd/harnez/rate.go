// rate implements `harnez rate`, an ultra-compact positional command an
// agent fires after every internal tool call to record a 1-5 quality
// rating. See issues/117-harnez-rate-command.md.
//
// This is the most latency-sensitive command in the tool-telemetry
// feature (issues 116/117/118/120) — it can fire many times per agent
// turn — so it does the minimum work: parse args, resolve session/ticket/
// agent, open the DB, insert one row, exit. No extra I/O, no network.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// rateAgentEnvVars maps agent-identifying environment variables (the same
// signals resolve.SessionEnvVars uses to detect *which* agent set a
// session id) to the agent_id auto-detect should report when --agent and
// $HARNEZ_AGENT are both unset. Order matters: first match wins.
var rateAgentEnvVars = []struct {
	Env   string
	Agent string
}{
	{"CLAUDE_CODE_SESSION_ID", "claude"},
	{"CLAUDE_SESSION_ID", "claude"},
	{"ANTIGRAVITY_SESSION_ID", "antigravity"},
	{"CODEX_SESSION_ID", "codex"},
}

// detectAgent resolves the agent_id to record: --agent flag, then
// $HARNEZ_AGENT, then the first matching entry in rateAgentEnvVars, else
// "unknown". getenv is injected for tests; nil means os.Getenv.
func detectAgent(flagValue string, getenv func(string) string) string {
	if flagValue != "" {
		return flagValue
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if v := getenv("HARNEZ_AGENT"); v != "" {
		return v
	}
	for _, d := range rateAgentEnvVars {
		if getenv(d.Env) != "" {
			return d.Agent
		}
	}
	return "unknown"
}

func newRateCmd() *cobra.Command {
	var agentFlag string
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   `rate <tool_name> <score> "<description>" [<ticket_id>]`,
		Short: "Record a 1-5 quality rating for an internal tool call",
		Long: `rate writes a single tool_calls row (call_type=internal, exit_code=NULL)
recording how well an internal tool call (file read, edit, semantic scan,
web search, ...) served the agent's purpose. Designed to fire immediately
after every such tool call without MCP/JSON-RPC schema overhead:

  harnez rate <tool_name> <score> "<description>" [<ticket_id>]

  tool_name    free-form tool identifier, e.g. "Read" or "semantic-scan"
  score        integer 1-5
  description  one-line quoted summary
  ticket_id    optional "<project_folder>/<ticket_name>"; when omitted it
               is resolved from the current git branch (or the session's
               most recently used ticket) instead of being written as NULL

Sub-20ms end-to-end target: this fires many times per agent turn.`,
		Args:         cobra.RangeArgs(3, 4),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRate(args, rateOptions{AgentFlag: agentFlag, SessionFlag: sessionFlag})
		},
	}
	cmd.Flags().StringVar(&agentFlag, "agent", "", "agent identifier override (default: $HARNEZ_AGENT or auto-detect)")
	cmd.Flags().StringVar(&sessionFlag, "session", "", "session_id override (default: resolve.Session())")
	return cmd
}

// rateOptions bundles runRate's inputs. The zero value matches production
// behavior (real env, real cwd, default DB path); tests override the
// Getenv/TicketDir/StateDir/DBPath fields to isolate from the caller's
// real environment and the user's real telemetry DB.
type rateOptions struct {
	AgentFlag   string
	SessionFlag string
	Getenv      func(string) string // nil means os.Getenv
	TicketDir   string              // resolve.TicketOptions.Dir override
	StateDir    string              // resolve.Session/Ticket state/lock dir override
	DBPath      string              // telemetry DB path override; empty means telemetry.DefaultDBPath()
}

// runRate does the actual parse/resolve/insert work.
func runRate(args []string, opts rateOptions) error {
	toolName := args[0]
	scoreStr := args[1]
	description := args[2]
	var explicitTicket string
	if len(args) == 4 {
		explicitTicket = args[3]
	}

	score, err := strconv.Atoi(scoreStr)
	if err != nil {
		return fmt.Errorf("rate: score must be an integer between 1 and 5, got %q", scoreStr)
	}

	agent := detectAgent(opts.AgentFlag, opts.Getenv)

	sessionID, err := resolve.Session(resolve.SessionOptions{
		Explicit: opts.SessionFlag,
		Getenv:   opts.Getenv,
		LockDir:  opts.StateDir,
	})
	if err != nil {
		return fmt.Errorf("rate: resolve session: %w", err)
	}

	ticketID, err := resolve.Ticket(resolve.TicketOptions{
		Explicit:  explicitTicket,
		Dir:       opts.TicketDir,
		SessionID: sessionID,
		StateDir:  opts.StateDir,
	})
	if err != nil {
		return fmt.Errorf("rate: resolve ticket: %w", err)
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("rate: %w", err)
		}
		dbPath = p
	}
	return insertRateRow(dbPath, toolName, description, agent, sessionID, ticketID, score)
}

// insertRateRow opens the telemetry DB at dbPath and writes one row. Split
// out from runRate so tests can point it at a throwaway DB path via
// TELEMETRY_DB_PATH-style plumbing without touching the user's real DB.
func insertRateRow(dbPath, toolName, description, agent, sessionID, ticketID string, score int) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("rate: open telemetry db: %w", err)
	}
	defer db.Close()

	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	call := telemetry.ToolCall{
		SessionID:   sessionID,
		TicketID:    ticketID,
		ProjectName: filepath.Base(wd),
		WorkingDir:  wd,
		AgentID:     agent,
		ToolName:    toolName,
		CallType:    "internal",
		Score:       &score,
		Note:        description,
		ExitCode:    nil,
	}

	if err := db.Insert(call); err != nil {
		// Insert deliberately doesn't duplicate the DB's own score-range
		// CHECK constraint as a second hardcoded validation layer (see
		// internal/telemetry/insert.go's doc comment, per docs/other/Spec.md).
		// Detect that specific constraint failure here only to give the
		// range in the message text, not to re-enforce it.
		if strings.Contains(err.Error(), "constraint") && strings.Contains(err.Error(), "score") {
			return fmt.Errorf("rate: score must be between 1 and 5, got %d", score)
		}
		return fmt.Errorf("rate: %w", err)
	}
	return nil
}
