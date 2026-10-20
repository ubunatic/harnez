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

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/server"
)

// codexPreToolUseInput mirrors Codex's documented PreToolUse stdin
// contract (issue 199's Q2): hookEventName, tool_name, tool_input,
// tool_use_id, session_id, turn_id, cwd, transcript_path,
// permission_mode, model. Only the fields this handshake needs are
// decoded; the rest pass through unread.
//
// Codex's documented Bash hook input uses tool_input.command.
type codexPreToolUseInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	ToolName       string `json:"tool_name"`
	ToolInput      struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// codexPreToolUseOutput mirrors Codex's documented PreToolUse stdout
// contract (issue 199's Q2): a "permissionDecision" field
// ("allow"/"deny", "ask" unsupported per binary error strings) plus an
// "updatedInput" field, valid only alongside permissionDecision:"allow".
// Only the allow+rewrite shape is ever emitted here — this hook never
// denies or asks, it only routes. No-op calls exit successfully without
// output; a bare allow without updatedInput is not a supported rewrite.
type codexPreToolUseOutput struct {
	HookSpecificOutput codexHookSpecificOutput `json:"hookSpecificOutput"`
}

type codexHookSpecificOutput struct {
	HookEventName      string            `json:"hookEventName"`
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
	prepared, err := server.PrepareCodexTelemetry(raw)
	if err == nil {
		sendTelemetryRecord("codex", json.RawMessage(prepared))
	}
	return nil
}

func runCodexTelemetryAt(in io.Reader, dbPath string) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil
	}
	prepared, err := server.PrepareCodexTelemetry(raw)
	if err != nil {
		return nil
	}
	return server.ProcessCodexTelemetry(dbPath, prepared)
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
		return nil
	}

	resp := codexPreToolUseOutput{
		HookSpecificOutput: codexHookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput: map[string]string{
				"command": formatGearRewrite(command, ""),
			},
		},
	}
	return json.NewEncoder(out).Encode(resp)
}
