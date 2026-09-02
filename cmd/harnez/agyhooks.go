// agyhooks implements `harnez agy-hooks`, the agy-side counterpart to
// `harnez exec hook`: instead of a quiet PATH-shim (issues/195), this
// writes agy's own hooks.json (issue 193's Findings) so agy's PreToolUse
// contract routes run_command calls through `harnez exec`, the same way
// internal/claude's PreToolUse/Bash wiring does for Claude Code. See
// issues/196-agy-native-hooks-plan-alongside-claude-hooks.md.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agy"
)

func newAgyHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agy-hooks",
		Short: "Manage agy's native hooks.json PreToolUse wiring (harnez exec routing)",
		Long: `agy-hooks manages the "harnez" named hook entry inside agy's own
hooks.json (global, ~/.gemini/config/hooks.json) — the "tell agy" native
hook mechanism found by issue 193's research, complementary to issue 195's
quiet PATH-shim. Installing it is an explicit, agy-side opt-in: agy will
run 'harnez agy-hooks hook' before every run_command tool call and, if
that call isn't already routed through 'harnez exec', rewrite it to be.`,
	}
	cmd.AddCommand(newAgyHooksApplyCmd(), newAgyHooksStatusCmd(), newAgyHooksHookCmd())
	return cmd
}

func newAgyHooksApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "apply",
		Short:        "Install/update the harnez PreToolUse entry in agy's hooks.json",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path := agy.HooksPath(home)
			changed, err := agy.Apply(path)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "up to date: %s\n", path)
			}
			return nil
		},
	}
}

func newAgyHooksStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Short:        "Report whether agy's hooks.json has the harnez entry, and whether it has drifted",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path := agy.HooksPath(home)
			installed, drifted := agy.Status(path)
			switch {
			case !installed:
				fmt.Fprintf(cmd.OutOrStdout(), "not installed: %s\n", path)
			case drifted:
				fmt.Fprintf(cmd.OutOrStdout(), "drifted: %s (run 'harnez agy-hooks apply' to repair)\n", path)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "up to date: %s\n", path)
			}
			return nil
		},
	}
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

	tool := "agy"
	if fields := strings.Fields(command); len(fields) > 0 {
		tool = fields[0]
	}

	var resp agyPreToolUseOutput
	resp.Decision = "allow"
	resp.Overwrite.CommandLine = fmt.Sprintf("harnez exec --tool %s -- bash -c %s", tool, shellQuote(command))
	return json.NewEncoder(out).Encode(resp)
}
