// codexhooks implements `harnez codex-hooks`, the Codex-side counterpart
// to `harnez exec hook`/`harnez agy-hooks hook`: Codex CLI ships a
// stable, always-on native hooks system (unlike agy's), so this writes
// Codex's own ~/.codex/config.toml [hooks.harnez] table so Codex's
// PreToolUse contract routes Bash tool calls through `harnez exec`, the
// same way internal/claude's PreToolUse/Bash wiring does for Claude Code.
// See issues/199-research-codex-hook-surface-for-transparent-exec-distill.md
// (research) and issues/200-codex-native-hooks-preTooluse-wiring.md (plan).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/codex"
)

func newCodexHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex-hooks",
		Short: "Manage Codex's native config.toml PreToolUse wiring (harnez exec routing)",
		Long: `codex-hooks manages the "harnez" named hook entry inside Codex CLI's
own hooks system (global, ~/.codex/config.toml's [hooks.harnez] table) —
a native, always-on mechanism (issue 199's research), unlike agy's, which
required opting into a comparable hooks.json feature. Installing it is an
explicit, Codex-side opt-in: Codex will run 'harnez codex-hooks hook'
before every Bash tool call and, if that call isn't already routed
through 'harnez exec', rewrite it to be.

Note: Codex gates new or modified hooks behind its own hook-trust review
(issue 199's Q5) independently of the "enabled" flag written here — after
'apply', Codex will prompt for trust review (its /hooks TUI panel, or
"New hook - review required") before this hook actually starts running.
'--dangerously-bypass-hook-trust' exists on the codex CLI itself to skip
that gate for automation; harnez does not set it for you.`,
	}
	cmd.AddCommand(newCodexHooksApplyCmd(), newCodexHooksStatusCmd(), newCodexHooksHookCmd())
	return cmd
}

func newCodexHooksApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "apply",
		Short:        "Install/update the harnez PreToolUse entry in Codex's config.toml",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path := codex.HooksPath(home)
			changed, err := codex.Apply(path)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "up to date: %s\n", path)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "note: Codex will prompt for hook-trust review before this hook becomes active (see its /hooks panel).")
			return nil
		},
	}
}

func newCodexHooksStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Short:        "Report whether Codex's config.toml has the harnez entry, and whether it has drifted",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path := codex.HooksPath(home)
			installed, drifted := codex.Status(path)
			switch {
			case !installed:
				fmt.Fprintf(cmd.OutOrStdout(), "not installed: %s\n", path)
			case drifted:
				fmt.Fprintf(cmd.OutOrStdout(), "drifted: %s (run 'harnez codex-hooks apply' to repair)\n", path)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "up to date: %s\n", path)
			}
			return nil
		},
	}
}

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

func newCodexHooksHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "Codex PreToolUse handler: rewrite Bash calls to route through 'harnez exec'",
		Long: `hook implements Codex's PreToolUse command-handler contract for the
"Bash" matcher (see issue 199's Findings). It reads the PreToolUse JSON
payload from stdin and, for a non-empty command not already routed
through 'harnez exec', emits a permissionDecision:"allow" envelope whose
updatedInput.command points at:

  harnez exec --tool <tool_name> -- bash -c '<original command>'

This is the handshake stage only: it never spawns the command or writes
telemetry itself (see docs/HookRewritePattern.md, the same split used by
'harnez exec hook' for Claude Code and 'harnez agy-hooks hook' for agy).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCodexHooksHook(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
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

	tool := "codex"
	if fields := strings.Fields(command); len(fields) > 0 {
		tool = fields[0]
	}

	resp := codexPreToolUseOutput{
		PermissionDecision: "allow",
		UpdatedInput: map[string]string{
			"command": fmt.Sprintf("harnez exec --tool %s -- bash -c %s", tool, shellQuote(command)),
		},
	}
	return json.NewEncoder(out).Encode(resp)
}
