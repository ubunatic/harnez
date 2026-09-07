// agyhooks implements `harnez agy-hooks`, the agy-side counterpart to
// `harnez exec hook` and `harnez codex-hook`: instead of a quiet PATH-shim
// (issues/195), `harnez apply` writes agy's own hooks.json (issue 193's
// Findings, issues/209) so agy's PreToolUse contract routes run_command calls
// through `harnez exec`, the same way internal/claude's PreToolUse/Bash wiring
// does for Claude Code. This file implements the handshake command that
// config entry points at. Installing it is `harnez apply`'s job
// (internal/claude/apply.go). See issues/196, issues/209, and issues/267.
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newAgyHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "agy-hooks",
		Short:  "agy PreToolUse handler: rewrite run_command calls to route through 'harnez exec'",
		Hidden: true,
		Long: `agy-hooks implements agy's PreToolUse command-handler contract for the
"run_command" matcher (see issue 193's Findings, issue 209). It reads the toolCall
JSON payload from stdin and, for a non-empty CommandLine not already
routed through 'harnez exec', emits a decision:"allow" envelope whose
overwrite.CommandLine points at:

  harnez exec --tool <first-word-of-command> -- bash -c <original command>

This is the handshake stage only: it never spawns the command or writes
telemetry itself (see docs/HookRewritePattern.md). 'harnez apply' installs
the hooks.json entry that invokes this command; there is no separate
'agy-hooks apply/status' command group.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgyHooksHook(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.AddCommand(newAgyHooksHookCmd())
	return cmd
}

// agyPreToolUseInput mirrors agy's documented PreToolUse stdin contract
// (issue 193's Findings): {"toolCall": {"name": "run_command", "args":
// {"CommandLine": "npm test"}}, "stepIdx": N, ...}.
type agyPreToolUseInput struct {
	ToolCall struct {
		Name string `json:"name"`
		Args struct {
			CommandLine string `json:"CommandLine"`
		} `json:"args"`
	} `json:"toolCall"`
}

// agyPreToolUseOutput mirrors agy's documented PreToolUse stdout contract:
// {"decision": "allow"|"deny"|"ask"|"force_ask", "overwrite":
// {"CommandLine": "..."}}. Only "allow" plus an "overwrite" rewrite is
// ever emitted here — this hook never denies or asks, it only routes.
type agyPreToolUseOutput struct {
	Decision  string `json:"decision"`
	Overwrite struct {
		CommandLine string `json:"CommandLine"`
	} `json:"overwrite"`
}

func newAgyHooksHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "agy PreToolUse handler: rewrite run_command calls to route through 'harnez exec'",
		Long: `hook implements agy's PreToolUse command-handler contract for the
"run_command" matcher (see issue 193's Findings). It reads the toolCall
JSON payload from stdin and, for a non-empty CommandLine not already
routed through 'harnez exec', emits a decision:"allow" envelope whose
overwrite.CommandLine points at:

  harnez exec --tool <first-word-of-command> -- <original command>

This is the handshake stage only: it never spawns the command or writes
telemetry itself (see docs/HookRewritePattern.md, the same split used by
'harnez exec hook' for Claude Code).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgyHooksHook(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

func runAgyHooksHook(in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("decode agy hook payload: %w", err)
	}

	var payload agyPreToolUseInput
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode agy hook payload: %w", err)
	}

	command := payload.ToolCall.Args.CommandLine
	if command == "" || alreadyRoutedThroughExec(command) {
		fmt.Fprintln(out, `{"decision":"allow"}`)
		return nil
	}

	var resp agyPreToolUseOutput
	resp.Decision = "allow"
	resp.Overwrite.CommandLine = formatGearRewrite(command, "")
	return json.NewEncoder(out).Encode(resp)
}
