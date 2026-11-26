package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/decide"
	"ubunatic.com/harnez/internal/hookdispatch"
)

// claudeHookRegistry maps each Claude hook event to the harnez handlers that
// run for it, in run order. Each handler wraps the same function its old
// subcommand runs, so the output contract is unchanged. preEditBackend is nil
// in production, which lets runPreEditHook pick its default backend.
func claudeHookRegistry(preEditBackend decide.Backend) hookdispatch.Registry {
	skill := func(_ context.Context, in io.Reader, _ io.Writer) error {
		runClaudeSkillHook(in, skillHookOptions{})
		return nil
	}
	codexTelemetry := func(_ context.Context, in io.Reader, _ io.Writer) error {
		return runCodexTelemetry(in)
	}
	return hookdispatch.Registry{
		"PreToolUse": {
			{Name: "exec", Kind: hookdispatch.JSON, Tools: []string{"Bash"},
				Run: func(_ context.Context, in io.Reader, out io.Writer) error {
					return runExecHook(in, out)
				}},
			{Name: "read", Kind: hookdispatch.JSON, Tools: []string{"View", "Read", "ReadMultipleFiles"},
				Run: func(_ context.Context, in io.Reader, out io.Writer) error {
					return runClaudeReadHook(in, out, readHookOptions{})
				}},
			{Name: "pre-edit", Kind: hookdispatch.JSON, Tools: []string{"Edit", "Write"},
				Run: func(ctx context.Context, in io.Reader, out io.Writer) error {
					return runPreEditHook(ctx, in, out, "", "", preEditOptions{Backend: preEditBackend})
				}},
		},
		"PostToolUse":        {{Name: "skill", Kind: hookdispatch.Silent, Tools: []string{"Skill"}, Run: skill}},
		"PostToolUseFailure": {{Name: "skill", Kind: hookdispatch.Silent, Tools: []string{"Skill"}, Run: skill}},
		"UserPromptSubmit":   {{Name: "skill", Kind: hookdispatch.Silent, Run: skill}},
		"SessionStart": {{Name: "claude-instructions", Kind: hookdispatch.Text,
			Run: func(_ context.Context, _ io.Reader, out io.Writer) error {
				return runClaudeInstructionsHook(out)
			}}},
		"PreCompact": {
			{Name: "codex-telemetry", Kind: hookdispatch.Silent, Run: codexTelemetry},
			{Name: "compact-reset", Kind: hookdispatch.Silent,
				Run: func(_ context.Context, in io.Reader, _ io.Writer) error {
					return runCompactResetHook(in, defaultReadHookStateDir())
				}},
		},
		"PostCompact": {{Name: "codex-telemetry", Kind: hookdispatch.Silent, Run: codexTelemetry}},
		"Stop": {{Name: "sound", Kind: hookdispatch.Silent,
			Run: func(ctx context.Context, _ io.Reader, _ io.Writer) error {
				return runHookSound(ctx, hookSoundOptions{})
			}}},
	}
}

// newHookEventCmd is `harnez hook <event>`: one hidden command per Claude hook
// event. It reads the payload once and runs every registered handler.
func newHookEventCmd(event string) *cobra.Command {
	return &cobra.Command{
		Use:          event,
		Short:        "Run every harnez handler for the Claude " + event + " hook event",
		Hidden:       true,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read %s hook payload: %w", event, err)
			}
			budget, err := hookdispatch.LoadBudget()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return hookdispatch.Dispatch(ctx, claudeHookRegistry(nil), event, raw, cmd.OutOrStdout(),
				hookdispatch.Options{Budget: budget, Stderr: cmd.ErrOrStderr()})
		},
	}
}
