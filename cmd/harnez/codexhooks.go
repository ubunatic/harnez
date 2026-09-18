// codexhooks implements `harnez codex-hook`, the Codex-side counterpart
// to `harnez exec hook`/`harnez agy-hooks hook`: Codex CLI ships a
// stable, always-on native hooks system (unlike agy's), so `harnez apply`
// writes Codex's own ~/.codex/config.toml [hooks.harnez] table (see
// internal/codex) so Codex's PreToolUse contract routes Bash tool calls
// through `harnez exec`, the same way internal/claude's PreToolUse/Bash
// wiring does for Claude Code. This file only implements the handshake
// command that config entry points at — installing it is `harnez
// apply`'s job (internal/claude/apply.go), not a separate management
// command, since Codex's hooks require no more user-facing ceremony than
// Claude's settings.json hooks already get. See
// issues/199-research-codex-hook-surface-for-transparent-exec-distill.md
// (research) and issues/200-codex-native-hooks-preTooluse-wiring.md (plan).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// codexPreToolUseInput mirrors Codex's documented PreToolUse stdin
// contract (issue 199's Q2): hookEventName, tool_name, tool_input,
// tool_use_id, session_id, turn_id, cwd, transcript_path,
// permission_mode, model. Only the fields this handshake needs are
// decoded; the rest pass through unread.
//
// tool_input.command is an assumption, not a confirmed field name: 199's
// Q2 found Codex's PreToolUse schema is a near-exact mirror of Claude
// Code's own (down to shared struct/field naming patterns in the
// binary's strings), and Claude Code's own Bash tool_input carries the
// command string under "command" — so this follows that naming rather
// than agy's "args.CommandLine" shape. If a real Codex Bash tool_input
// turns out to use a different field name, this is the one place to fix.
type codexPreToolUseInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// codexPreToolUseOutput mirrors Codex's documented PreToolUse stdout
// contract (issue 199's Q2): a "permissionDecision" field
// ("allow"/"deny", "ask" unsupported per binary error strings) plus an
// "updatedInput" field, valid only alongside permissionDecision:"allow".
// Only the allow+rewrite shape is ever emitted here — this hook never
// denies or asks, it only routes.
type codexPreToolUseOutput struct {
	PermissionDecision string            `json:"permissionDecision"`
	UpdatedInput       map[string]string `json:"updatedInput,omitempty"`
}

// newCodexHookCmd implements the PreToolUse command handler Codex's
// config.toml [hooks.harnez] table (internal/codex.BuildHooksDoc) points
// at. It's hidden from --help: nothing about it is meant for direct,
// interactive use — Codex invokes it, and installing/removing it is done
// via `harnez apply` (see internal/claude/apply.go), not this command.
func newCodexHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "codex-hook",
		Short:  "Codex PreToolUse handler: rewrite Bash calls to route through 'harnez exec'",
		Hidden: true,
		Long: `codex-hook implements Codex's PreToolUse command-handler contract for
the "Bash" matcher (see issue 199's Findings). It reads the PreToolUse
JSON payload from stdin and, for a non-empty command not already routed
through 'harnez exec', emits a permissionDecision:"allow" envelope whose
updatedInput.command points at:

  harnez exec --tool <tool_name> -- bash -c '<original command>'

This is the handshake stage only: it never spawns the command or writes
telemetry itself (see docs/HookRewritePattern.md, the same split used by
'harnez exec hook' for Claude Code and 'harnez agy-hooks hook' for agy).
'harnez apply' installs the config.toml entry that invokes this command;
there is no separate 'codex-hooks apply/status' command group.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCodexHooksHook(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	return cmd
}

func newCodexTelemetryCmd() *cobra.Command {
	return &cobra.Command{Use: "codex-telemetry", Hidden: true, SilenceUsage: true, RunE: func(cmd *cobra.Command, _ []string) error {
		return runCodexTelemetry(cmd.InOrStdin())
	}}
}

func runCodexTelemetry(in io.Reader) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil
	}
	e := codex.ParseEvent(raw)
	if e.ToolName == "" || e.SessionID == "" {
		return nil
	}
	wd, _ := os.Getwd()
	ticket, _ := resolve.Ticket(resolve.TicketOptions{SessionID: e.SessionID})
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return nil
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	note := "codex:" + e.ToolCallID
	if e.ToolCallID != "" {
		if calls, queryErr := db.Query(telemetry.Filter{SessionID: e.SessionID}); queryErr == nil {
			for _, prior := range calls {
				if prior.Note == note {
					return nil
				}
			}
		}
	}
	callType := "hook:post"
	if e.Success != nil && !*e.Success {
		callType = "hook:failure"
	}
	call := telemetry.ToolCall{CreatedAt: time.Now().UTC(), SessionID: e.SessionID, TicketID: ticket, ProjectName: filepath.Base(wd), WorkingDir: wd, AgentID: "codex", ToolName: e.ToolName, CallType: callType, Note: note, DurationMs: e.DurationMs, ExitCode: e.ExitCode}
	return db.Insert(call)
}

func runCodexHooksHook(in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("decode codex hook payload: %w", err)
	}

	var payload codexPreToolUseInput
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode codex hook payload: %w", err)
	}

	command := payload.ToolInput.Command
	if command == "" || alreadyRoutedThroughExec(command) {
		fmt.Fprintln(out, `{"permissionDecision":"allow"}`)
		return nil
	}

	resp := codexPreToolUseOutput{
		PermissionDecision: "allow",
		UpdatedInput: map[string]string{
			"command": formatGearRewrite(command, ""),
		},
	}
	return json.NewEncoder(out).Encode(resp)
}
