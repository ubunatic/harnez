package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/hookdispatch"
)

// `harnez hook codex <Event>` replaces the old codex-hook and codex-telemetry
// subcommands for Codex. Each event's reply and side effects must equal what
// the old subcommand did for the same payload (issue 1304 M3).
func TestCodexDispatchMatchesSubcommandBehaviour(t *testing.T) {
	repo := t.TempDir()
	oldHook := func(_ context.Context, in io.Reader, out io.Writer, _ goldenEnv) error {
		return runCodexHooksHook(in, out)
	}
	oldTelemetry := func(_ context.Context, in io.Reader, _ io.Writer, e goldenEnv) error {
		return runCodexTelemetryWith(in, e.sendTelemetry)
	}
	cases := []goldenCase{
		{
			name:    "codex PreToolUse Bash rewrite",
			agent:   "codex",
			event:   "PreToolUse",
			payload: `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"go test ./internal/x"},"session_id":"s1","cwd":"` + repo + `"}`,
			old:     oldHook,
			wantOut: `"permissionDecision":"allow"`,
		},
		{
			name:    "codex PreToolUse already routed through harnez",
			agent:   "codex",
			event:   "PreToolUse",
			payload: `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"harnez exec --tool Bash -- bash -c 'go test'"},"session_id":"s1"}`,
			old:     oldHook,
		},
		{
			name:    "codex PreToolUse empty command",
			agent:   "codex",
			event:   "PreToolUse",
			payload: `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":""},"session_id":"s1"}`,
			old:     oldHook,
		},
	}
	for _, event := range []string{"PostToolUse", "PreCompact", "PostCompact", "SessionStart", "Stop", "SessionEnd"} {
		cases = append(cases, goldenCase{
			name:          "codex " + event + " telemetry",
			agent:         "codex",
			event:         event,
			payload:       `{"hook_event_name":"` + event + `","session_id":"s1","model":"gpt-5","token_usage":{"total_tokens":10}}`,
			old:           oldTelemetry,
			wantTelemetry: 1,
		})
	}
	runGoldenCases(t, cases)
}

// Every Codex manifest event is a hidden `harnez hook codex <Event>` command.
func TestCodexHookEventCommandsRegisteredForEveryManifestEvent(t *testing.T) {
	root := newHookCmd()
	for _, event := range manifestEvents(t, "codex") {
		c, _, err := root.Find([]string{"codex", event})
		if err != nil || c.Name() != event || !c.Hidden || c.Parent().Name() != "codex" {
			t.Errorf("hook codex %s: command %v, err %v", event, c, err)
		}
	}
}

// The Codex registry binds exactly the manifest's handlers and validates.
func TestCodexHookRegistryMatchesManifest(t *testing.T) {
	reg, err := codexHookRegistry(hookDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(reg.Events(), ","); got != strings.Join(manifestEvents(t, "codex"), ",") {
		t.Fatalf("registry events = %s", got)
	}
	if got := reg["PreToolUse"][0]; got.Name != "codex-hook" || got.Kind != hookdispatch.JSON {
		t.Fatalf("PreToolUse handler = %+v, want codex-hook JSON", got)
	}
}

// The old codex-hook and codex-telemetry subcommands keep working for hooks
// written before the switch.
func TestLegacyCodexSubcommandsStillRegistered(t *testing.T) {
	if got := newCodexHookCmd().Name(); got != "codex-hook" {
		t.Errorf("legacy codex-hook command name = %s", got)
	}
	if got := newCodexTelemetryCmd().Name(); got != "codex-telemetry" {
		t.Errorf("legacy codex-telemetry command name = %s", got)
	}
}
