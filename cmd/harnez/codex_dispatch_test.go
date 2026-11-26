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
		{
			// The old entry had no tool filter beyond the host matcher, so a payload
			// without tool_name is still rewritten.
			name:    "codex PreToolUse without tool_name",
			agent:   "codex",
			event:   "PreToolUse",
			payload: `{"hook_event_name":"PreToolUse","tool_input":{"command":"go test ./internal/x"},"session_id":"s1"}`,
			old:     oldHook,
			wantOut: `"permissionDecision":"allow"`,
		},
		{
			// A malformed payload fails with the subcommand's error text, not a silent no-op.
			name:    "codex PreToolUse malformed JSON",
			agent:   "codex",
			event:   "PreToolUse",
			payload: `{"tool_name":`,
			old:     oldHook,
			wantErr: true,
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
	got := reg["PreToolUse"][0]
	if got.Name != "codex-hook" || got.Kind != hookdispatch.JSON {
		t.Fatalf("PreToolUse handler = %+v, want codex-hook JSON", got)
	}
	// codex-hook replaces the subcommand, which ran on every payload, so it has
	// no tool filter and a failing run aborts the dispatch as the subcommand did.
	if len(got.Tools) != 0 || !got.Strict {
		t.Fatalf("codex-hook Tools = %v, Strict = %v; want no tool filter and strict", got.Tools, got.Strict)
	}
}

// The old codex-hook and codex-telemetry subcommands stay registered on the
// real root command for hooks written before the switch.
func TestLegacyCodexSubcommandsStillRegistered(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"codex-hook", "codex-telemetry"} {
		c, _, err := root.Find([]string{name})
		if err != nil || c == nil || c.Name() != name || c.Parent() != root {
			got := ""
			if c != nil {
				got = c.Name()
			}
			t.Errorf("legacy %s resolved to %q, err %v", name, got, err)
		}
	}
}
