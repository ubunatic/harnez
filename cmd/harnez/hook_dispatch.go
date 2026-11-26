package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/hookdispatch"
	"ubunatic.com/harnez/internal/telemetry"
)

// hookDeps are the side effects the Claude handlers reach. Zero values give the
// production behaviour; tests inject stand-ins so no handler touches the
// telemetry server, the sound device or a hosted backend.
type hookDeps struct {
	// PreEdit is passed to runPreEditHook. Its zero value picks the default backend.
	PreEdit preEditOptions
	// SkillInsert records a skill tool call. Nil uses the default insert.
	SkillInsert func(dbPath string, call telemetry.ToolCall) error
	// Telemetry receives codex telemetry records. Nil uses sendTelemetryRecord.
	Telemetry func(kind string, payload any)
	// PlaySound plays the Stop sound. Nil starts the detached player.
	PlaySound func(ctx context.Context) error
	// StateDir holds the native-read markers. Empty uses defaultReadHookStateDir().
	StateDir string
}

// claudeHookRegistry binds the run functions to hookdispatch.ClaudeManifest.
// Each function wraps the same code its old subcommand runs, so the output
// contract is unchanged. The manifest holds the events and tool routing.
func claudeHookRegistry(deps hookDeps) (hookdispatch.Registry, error) {
	if deps.Telemetry == nil {
		deps.Telemetry = func(kind string, payload any) { sendTelemetryRecord(kind, payload) }
	}
	if deps.PlaySound == nil {
		deps.PlaySound = func(ctx context.Context) error { return runHookSound(ctx, hookSoundOptions{}) }
	}
	if deps.StateDir == "" {
		deps.StateDir = defaultReadHookStateDir()
	}
	skill := func(_ context.Context, in io.Reader, _ io.Writer) error {
		runClaudeSkillHook(in, skillHookOptions{Insert: deps.SkillInsert})
		return nil
	}
	codexTelemetry := func(_ context.Context, in io.Reader, _ io.Writer) error {
		return runCodexTelemetryWith(in, deps.Telemetry)
	}
	return hookdispatch.ClaudeManifest.Bind(map[string]hookdispatch.Func{
		"PreToolUse/exec": func(_ context.Context, in io.Reader, out io.Writer) error {
			return runExecHook(in, out)
		},
		"PreToolUse/read": func(_ context.Context, in io.Reader, out io.Writer) error {
			return runClaudeReadHook(in, out, readHookOptions{})
		},
		"PreToolUse/pre-edit": func(ctx context.Context, in io.Reader, out io.Writer) error {
			return runPreEditHook(ctx, in, out, "", "", deps.PreEdit)
		},
		"PostToolUse/skill":        skill,
		"PostToolUseFailure/skill": skill,
		"UserPromptSubmit/skill":   skill,
		"SessionStart/claude-instructions": func(_ context.Context, _ io.Reader, out io.Writer) error {
			return runClaudeInstructionsHook(out)
		},
		"PreCompact/codex-telemetry": codexTelemetry,
		"PreCompact/compact-reset": func(_ context.Context, in io.Reader, _ io.Writer) error {
			return runCompactResetHook(in, deps.StateDir)
		},
		"PostCompact/codex-telemetry": codexTelemetry,
		"Stop/sound": func(ctx context.Context, _ io.Reader, _ io.Writer) error {
			return deps.PlaySound(ctx)
		},
	})
}

// addHookEventCmds adds one hidden `harnez hook <Event>` command per manifest event.
func addHookEventCmds(parent *cobra.Command) {
	for _, event := range hookdispatch.ClaudeManifest.Events() {
		parent.AddCommand(newHookEventCmd(event))
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
			reg, err := claudeHookRegistry(hookDeps{})
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return hookdispatch.Dispatch(ctx, reg, event, raw, cmd.OutOrStdout(),
				hookdispatch.Options{Budget: budget, Stderr: cmd.ErrOrStderr()})
		},
	}
}
