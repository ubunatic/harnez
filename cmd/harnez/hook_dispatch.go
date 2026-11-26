package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/hookdispatch"
	"ubunatic.com/harnez/internal/telemetry"
)

// hookDeps are the side effects the hook handlers reach. Zero values give the
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

// withDefaults fills the nil and empty fields with the production behaviour.
func (d hookDeps) withDefaults() hookDeps {
	if d.Telemetry == nil {
		d.Telemetry = func(kind string, payload any) { sendTelemetryRecord(kind, payload) }
	}
	if d.PlaySound == nil {
		d.PlaySound = func(ctx context.Context) error { return runHookSound(ctx, hookSoundOptions{}) }
	}
	if d.StateDir == "" {
		d.StateDir = defaultReadHookStateDir()
	}
	return d
}

// claudeHookRegistry binds the run functions to the claude agent in
// spec/hooks.yaml. Each function wraps the same code its old subcommand runs,
// so the output contract is unchanged. The manifest holds the events and routing.
func claudeHookRegistry(deps hookDeps) (hookdispatch.Registry, error) {
	deps = deps.withDefaults()
	skill := func(_ context.Context, in io.Reader, _ io.Writer) error {
		runClaudeSkillHook(in, skillHookOptions{Insert: deps.SkillInsert})
		return nil
	}
	codexTelemetry := func(_ context.Context, in io.Reader, _ io.Writer) error {
		return runCodexTelemetryWith(in, deps.Telemetry)
	}
	m, err := hookdispatch.LoadManifest("claude")
	if err != nil {
		return nil, err
	}
	return m.Bind(map[string]hookdispatch.Func{
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

// codexHookRegistry binds the run functions to the codex agent in
// spec/hooks.yaml. codex-hook is the PreToolUse Bash handshake; codex-telemetry
// records every other event. Both wrap the code the old codex-hook and
// codex-telemetry subcommands run (issue 1304 M3).
func codexHookRegistry(deps hookDeps) (hookdispatch.Registry, error) {
	deps = deps.withDefaults()
	telemetry := func(_ context.Context, in io.Reader, _ io.Writer) error {
		return runCodexTelemetryWith(in, deps.Telemetry)
	}
	m, err := hookdispatch.LoadManifest("codex")
	if err != nil {
		return nil, err
	}
	return m.Bind(map[string]hookdispatch.Func{
		"PreToolUse/codex-hook": func(_ context.Context, in io.Reader, out io.Writer) error {
			return runCodexHooksHook(in, out)
		},
		"PostToolUse/codex-telemetry":  telemetry,
		"PreCompact/codex-telemetry":   telemetry,
		"PostCompact/codex-telemetry":  telemetry,
		"SessionStart/codex-telemetry": telemetry,
		"Stop/codex-telemetry":         telemetry,
		"SessionEnd/codex-telemetry":   telemetry,
	})
}

// addHookEventCmds adds one hidden command per event of each agent's manifest:
// `harnez hook <Event>` for Claude and `harnez hook codex <Event>` for Codex.
// The embedded spec is validated by tests, so a load failure here is a build
// defect and panics.
func addHookEventCmds(parent *cobra.Command) {
	claude := mustManifest("claude")
	for _, event := range claude.EventNames() {
		parent.AddCommand(newHookEventCmd(event, "Claude", claudeHookRegistry))
	}
	codexParent := &cobra.Command{
		Use:    "codex",
		Short:  "Codex hook events: harnez hook codex <Event>",
		Hidden: true,
		Args:   cobra.NoArgs,
	}
	codex := mustManifest("codex")
	for _, event := range codex.EventNames() {
		codexParent.AddCommand(newHookEventCmd(event, "Codex", codexHookRegistry))
	}
	parent.AddCommand(codexParent)
}

func mustManifest(agent string) hookdispatch.Manifest {
	m, err := hookdispatch.LoadManifest(agent)
	if err != nil {
		panic(fmt.Sprintf("hook manifest %s: %v", agent, err))
	}
	return m
}

// newHookEventCmd is `harnez hook <event>` for one agent: one hidden command
// per hook event. It reads the payload once and runs every registered handler.
func newHookEventCmd(event, agentLabel string, registry func(hookDeps) (hookdispatch.Registry, error)) *cobra.Command {
	return &cobra.Command{
		Use:          event,
		Short:        "Run every harnez handler for the " + agentLabel + " " + event + " hook event",
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
			reg, err := registry(hookDeps{})
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
