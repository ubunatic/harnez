package main

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newExecCommand() *cobra.Command {
	var toolFlag string
	var ticketFlag string
	var distillFlag string
	var expectFailureFlag bool
	var quota1Flag bool
	var timeoutFlag time.Duration

	cmd := &cobra.Command{
		Use:     "exec [--tool <tool_name>] [--ticket <ticket_id>] [--distill[=<mode>]] [--expect-failure] [--quota-1] -- <command...>",
		Aliases: []string{"⚙", "⚙️"},
		Short:   "Run a command, proxy its stdio unbuffered, and record shell-call telemetry",
		Long: `exec wraps an arbitrary command: it spawns <command...> as a subprocess,
proxies its stdin/stdout/stderr to the caller with no added buffering
latency, preserves and re-exits with the child's exact exit code
(including signal-terminated cases), and writes one call_type='shell'
tool_calls row recording duration_ms, raw_bytes, distilled_bytes (when
distillation is active), and synthetic quality score (1-5).

  harnez exec --tool git -- git status
  harnez exec --tool npm --ticket harnez/118-harnez-exec-shell-interceptor -- npm test
  harnez exec --tool Bash --distill -- go test ./...
  ⚙ echo "hello from gear"
  ⚙ --tool test -- true

The telemetry write is best-effort and bounded: it never delays the
wrapped command's own execution, and gives up waiting on a slow/hung DB
write after a short bound rather than hanging the caller.

Timeout precedence is an explicit HARNEZ_TIMEOUT=<duration|0> or HTO=<duration|0> prefix, then
the Bash tool's native timeout/background intent, then repo config
exec.timeout, then the built-in 60s default. HTO=0 disables the limit;
for example, HTO=10m make build. HARNEZ_TIMEOUT remains supported.

See 'harnez exec hook' for the separate PreToolUse rewrite stage that
points an agent's Bash tool calls at this command.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := toolFlag
			if tool == "" {
				tool = inferToolFromArgs(args, "Bash")
			} else if err := validateToolName(tool); err != nil {
				return err
			}
			exitCode, err := runExecWrapper(args, execOptions{
				Tool:          tool,
				Ticket:        ticketFlag,
				Distill:       distillFlag,
				ExpectFailure: expectFailureFlag,
				Quota1:        quota1Flag,
				Timeout:       timeoutFlag,
				TimeoutSet:    cmd.Flags().Changed("timeout"),
			}, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if exitCode != 0 {
				return silenceIfExitCode(cmd, &exitCodeError{Code: exitCode})
			}
			return nil
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&toolFlag, "tool", "", "tool identifier the wrapped command belongs to, e.g. git, npm, Bash (default: inferred from command or Bash)")
	cmd.Flags().StringVar(&ticketFlag, "ticket", "", "ticket_id override (default: resolve.Ticket())")
	cmd.Flags().StringVar(&distillFlag, "distill", "", "distillation filter mode (auto, gotest, git, raw)")
	cmd.Flags().BoolVar(&expectFailureFlag, "expect-failure", false, "record telemetry row with call_type=shell-expected")
	cmd.Flags().BoolVar(&quota1Flag, "quota-1", false, "enforce Quota-1 single-test boundary (require code edits between runs)")
	cmd.Flags().DurationVar(&timeoutFlag, "timeout", 0, "maximum command runtime (default: HTO/HARNEZ_TIMEOUT, tool intent, repo config, then 60s)")
	cmd.Flags().Lookup("distill").NoOptDefVal = "auto"

	cmd.AddCommand(newExecHookCmd())
	return cmd
}

// newExecHookCmd implements `harnez exec hook`, the PreToolUse handshake
// stage described in docs/HookRewritePattern.md. It reuses distill.go's
// hookInput/hookOutput/hookSpecificOutput types (same package, same
// Claude Code hook payload shape) rather than redefining them.
func newExecHookCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "PreToolUse hook: rewrite Bash commands to route through 'harnez exec'",
		Long: `hook implements a Claude Code PreToolUse hook for the Bash matcher.

It reads the tool-call JSON payload from stdin and, for Bash commands not
already routed through 'harnez exec', emits a rewrite envelope pointing
the command at:

  harnez exec --tool <tool_name> -- <original command>

This is the endpoint issues/119's apply-managed hook entries invoke; the
hook itself never spawns the command or writes telemetry — see
docs/HookRewritePattern.md for why the rewrite and execution stages are
kept as separate commands. Installing this hook into a user's Claude Code
settings is issues/119's job, out of this command's scope.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecHook(os.Stdin, os.Stdout)
		},
	}
}
