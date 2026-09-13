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
	{"ANTIGRAVITY_CONVERSATION_ID", "agy"},
	{"ANTIGRAVITY_AGENT", "agy"},
	{"ANTIGRAVITY_AGENTAPI_EXE", "agy"},
	{"ANTIGRAVITY_SESSION_ID", "agy"},
	{"CODEX_SESSION_ID", "codex"},
}

// detectAgent resolves the agent_id to record: --agent flag, then
// $HARNEZ_AGENT, then the first matching entry in rateAgentEnvVars, else
// "unknown". getenv is injected for tests; nil means os.Getenv.
//
// The "unknown" return here is a shrug, not a classification: it means "no
// agent env var was found", which conflates a human at a terminal with an
// agent that exports nothing harnez recognises (resolve.SessionEnvVars
// documents two of its entries as unconfirmed guesses). tool_calls rows are
// written by an agent deliberately calling `harnez rate`/`harnez exec`, so
// that ambiguity is harmless there. It is NOT harmless for
// cli_invocations.agent_id, which records unattended invocations including
// a human's — see classifyInvoker (cmd/harnez/attribution.go) and issue
// 328, which wraps this function in the documented
// "agent:<id>" / "human" / "unknown" vocabulary that `harnez log --human` /
// `--agent` filter on.
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
	var okFlag bool
	var sinceFlag int

	cmd := &cobra.Command{
		Use:   `rate <tool_name> <score> "<description>" [<ticket_id>]`,
		Short: "Record a 1-5 quality rating for an internal tool call, or a lean --ok heartbeat",
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

Sub-20ms end-to-end target: this fires many times per agent turn.

--ok records a distinct, lighter-weight heartbeat instead of a per-tool
rating (call_type=heartbeat, score=NULL, exit_code=NULL) — issue 179's
answer to "silence is ambiguous": confirm a stretch of tool calls was fine
without inventing a fake per-tool score for it. No tool_name/score
required:

  harnez rate --ok ["<note>"] [<ticket_id>] [--since <n>]

  note       optional one-line note (defaults to "ok")
  ticket_id  optional, same resolution as the default form
  --since n  optional: how many tool calls this heartbeat covers (recorded
             in the note for human reference only; not a queryable column)`,
		Args: func(cmd *cobra.Command, args []string) error {
			if okFlag {
				if len(args) > 2 {
					return fmt.Errorf(`rate --ok: at most 2 args ("<note>" [<ticket_id>]), got %d`, len(args))
				}
				return nil
			}
			return cobra.RangeArgs(3, 4)(cmd, args)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := rateOptions{AgentFlag: agentFlag, SessionFlag: sessionFlag}
			if okFlag {
				return runRateOk(args, opts, sinceFlag)
			}
			return runRate(args, opts)
		},
	}
	cmd.Flags().StringVar(&agentFlag, "agent", "", "agent identifier override (default: $HARNEZ_AGENT or auto-detect)")
	cmd.Flags().StringVar(&sessionFlag, "session", "", "session_id override (default: resolve.Session())")
	cmd.Flags().BoolVar(&okFlag, "ok", false, "record a lean heartbeat confirming recent tool calls were fine, instead of a per-tool rating")
	cmd.Flags().IntVar(&sinceFlag, "since", 0, "with --ok: number of tool calls this heartbeat covers (descriptive only)")
	return cmd
}

// rateOptions bundles runRate's inputs. The zero value matches production
// behavior (real env, real cwd, default DB path); tests override the
// Getenv/StateDir/DBPath fields to isolate from the caller's real
// environment and the user's real telemetry DB.
type rateOptions struct {
	AgentFlag   string
	SessionFlag string
	Getenv      func(string) string // nil means os.Getenv
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

	// An unresolved ticket_id (no explicit arg, nothing recorded yet for
	// this session) is expected, not an error — see resolve.Ticket's doc
	// comment: this repo's own usage never branches per ticket, so nothing
	// beyond an explicit arg or session history can determine one. resolve.Ticket
	// only still returns an error for the rare state-dir I/O failure case.
	ticketID, err := resolve.Ticket(resolve.TicketOptions{
		Explicit:  explicitTicket,
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

// runRateOk implements `harnez rate --ok`: it resolves agent/session/ticket
// the same way runRate does, then writes a heartbeat row instead of a
// per-tool rating. args is [] | ["<note>"] | ["<note>", "<ticket_id>"] —
// validated by newRateCmd's Args func before this runs.
func runRateOk(args []string, opts rateOptions, since int) error {
	note := "ok"
	if len(args) >= 1 && args[0] != "" {
		note = args[0]
	}
	var explicitTicket string
	if len(args) == 2 {
		explicitTicket = args[1]
	}

	agent := detectAgent(opts.AgentFlag, opts.Getenv)

	sessionID, err := resolve.Session(resolve.SessionOptions{
		Explicit: opts.SessionFlag,
		Getenv:   opts.Getenv,
		LockDir:  opts.StateDir,
	})
	if err != nil {
		return fmt.Errorf("rate --ok: resolve session: %w", err)
	}

	ticketID, err := resolve.Ticket(resolve.TicketOptions{
		Explicit:  explicitTicket,
		SessionID: sessionID,
		StateDir:  opts.StateDir,
	})
	if err != nil {
		return fmt.Errorf("rate --ok: resolve ticket: %w", err)
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return fmt.Errorf("rate --ok: %w", err)
		}
		dbPath = p
	}
	return insertHeartbeatRow(dbPath, note, agent, sessionID, ticketID, since)
}

// rateCallPayloadBytes approximates the size of the `harnez rate` command
// line an agent actually issues — tool_name, score, quoted description, and
// ticket_id, roughly matching the Long help text's usage line — as a real,
// measured (not tokenizer-estimated) proxy for per-call overhead. See issue
// 142: this is deliberately the argument bytes, not a token count, since
// harnez has no access to the calling model's tokenizer.
func rateCallPayloadBytes(toolName, description, ticketID string, score int) int64 {
	// `rate <tool_name> <score> "<description>" [<ticket_id>]`
	n := len("rate ") + len(toolName) + len(" ") + len(strconv.Itoa(score)) + len(` "`) + len(description) + len(`"`)
	if ticketID != "" {
		n += len(" ") + len(ticketID)
	}
	return int64(n)
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
		// RawBytes here is the byte length of the rate call's own argument
		// payload (tool_name/score/description/ticket), not any distilled
		// output — there is none for `harnez rate`. It's the measured
		// per-call proxy issue 142's overhead report uses; see
		// telemetry.RateCallOverhead and cmd/harnez/stats.go's --overhead.
		RawBytes: rateCallPayloadBytes(toolName, description, ticketID, score),
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

// heartbeatCallPayloadBytes mirrors rateCallPayloadBytes for the `rate
// --ok` form — a real, measured proxy for this lighter call's own
// argument-payload size, so telemetry.RateCallOverhead's byte totals stay
// meaningful once heartbeats are included alongside failure ratings.
func heartbeatCallPayloadBytes(note, ticketID string, since int) int64 {
	// `rate --ok "<note>" [<ticket_id>] [--since <n>]`
	n := len(`rate --ok "`) + len(note) + len(`"`)
	if ticketID != "" {
		n += len(" ") + len(ticketID)
	}
	if since > 0 {
		n += len(" --since ") + len(strconv.Itoa(since))
	}
	return int64(n)
}

// insertHeartbeatRow opens the telemetry DB at dbPath and writes one
// heartbeat row (call_type=heartbeat, score=NULL, exit_code=NULL) —
// issue 179's lean alternative to a per-tool rating. since, when > 0, is
// folded into the note for human reference; it isn't a queryable column
// (kept lean — see telemetry.HeartbeatStats for the queryable
// last-heartbeat/calls-since data `harnez stats` actually reports).
func insertHeartbeatRow(dbPath, note, agent, sessionID, ticketID string, since int) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("rate --ok: open telemetry db: %w", err)
	}
	defer db.Close()

	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	rawBytes := heartbeatCallPayloadBytes(note, ticketID, since)
	if since > 0 {
		note = fmt.Sprintf("%s (last ~%d calls)", note, since)
	}

	call := telemetry.ToolCall{
		SessionID:   sessionID,
		TicketID:    ticketID,
		ProjectName: filepath.Base(wd),
		WorkingDir:  wd,
		AgentID:     agent,
		ToolName:    "heartbeat",
		CallType:    telemetry.HeartbeatCallType,
		Score:       nil,
		Note:        note,
		ExitCode:    nil,
		RawBytes:    rawBytes,
	}

	if err := db.Insert(call); err != nil {
		return fmt.Errorf("rate --ok: %w", err)
	}
	return nil
}
