package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/decide"
	"ubunatic.com/harnez/internal/hookdispatch"
)

// The dispatcher must register exactly the handlers that today's hook entries
// run, per event, in a fixed order (issue 1304 plan table).
func TestHookRegistryMatchesTodaysHookEntries(t *testing.T) {
	reg := claudeHookRegistry(nil)
	if err := reg.Validate(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PreToolUse":         "exec,read,pre-edit",
		"PostToolUse":        "skill",
		"PostToolUseFailure": "skill",
		"UserPromptSubmit":   "skill",
		"SessionStart":       "claude-instructions",
		"PreCompact":         "codex-telemetry,compact-reset",
		"PostCompact":        "codex-telemetry",
		"Stop":               "sound",
	}
	got := map[string]string{}
	for _, event := range reg.Events() {
		var names []string
		for _, h := range reg[event] {
			names = append(names, h.Name)
		}
		got[event] = strings.Join(names, ",")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry = %v\nwant %v", got, want)
	}
}

func TestHookRegistryToolRoutingMatchesOldMatchers(t *testing.T) {
	reg := claudeHookRegistry(nil)
	wantTools := map[string][]string{
		"exec":     {"Bash"},
		"read":     {"View", "Read", "ReadMultipleFiles"},
		"pre-edit": {"Edit", "Write"},
		"skill":    {"Skill"},
	}
	for _, event := range reg.Events() {
		for _, h := range reg[event] {
			if want, ok := wantTools[h.Name]; ok && event != "UserPromptSubmit" && !reflect.DeepEqual(h.Tools, want) {
				t.Errorf("%s/%s tools = %v, want %v", event, h.Name, h.Tools, want)
			}
		}
	}
}

// goldenMock is a decide backend that answers "no rule violated", so pre-edit
// never reaches the hosted backend during tests.
type goldenMock struct{}

func (goldenMock) Decide(context.Context, *decide.Request) (*decide.Response, error) {
	return nil, fmt.Errorf("golden test: no backend call expected")
}

// Each event's dispatcher output must equal what the old subcommand wrote for
// the same payload. Each side runs under its own temp HOME, so read markers
// and telemetry rows from one side cannot change the other side's output.
func TestHookDispatchMatchesSubcommandOutput(t *testing.T) {
	repo := t.TempDir()
	big := filepath.Join(repo, "big.go")
	var src strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&src, "var v%d = %d\n", i, i)
	}
	if err := os.WriteFile(big, []byte(src.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	edit := filepath.Join(repo, "x.go")
	if err := os.WriteFile(edit, []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldSkill := func(in io.Reader, out io.Writer) error { runClaudeSkillHook(in, skillHookOptions{}); return nil }
	cases := []struct {
		name    string
		event   string
		payload string
		old     func(in io.Reader, out io.Writer) error
	}{
		{
			name:    "PreToolUse Bash rewrite",
			event:   "PreToolUse",
			payload: `{"tool_name":"Bash","tool_input":{"command":"go test ./internal/x"},"session_id":"s1","cwd":"` + repo + `"}`,
			old:     func(in io.Reader, out io.Writer) error { return runExecHook(in, out) },
		},
		{
			name:    "PreToolUse Read large file",
			event:   "PreToolUse",
			payload: `{"tool_name":"Read","tool_input":{"file_path":"` + big + `"},"session_id":"s1","cwd":"` + repo + `"}`,
			old: func(in io.Reader, out io.Writer) error {
				return runClaudeReadHook(in, out, readHookOptions{})
			},
		},
		{
			name:    "PreToolUse Edit",
			event:   "PreToolUse",
			payload: `{"tool_name":"Edit","tool_input":{"file_path":"` + edit + `","new_string":"package x\n"},"session_id":"s1","cwd":"` + repo + `"}`,
			old: func(in io.Reader, out io.Writer) error {
				return runPreEditHook(context.Background(), in, out, "", "", preEditOptions{Backend: goldenMock{}})
			},
		},
		{
			name:    "PostToolUse Skill",
			event:   "PostToolUse",
			payload: `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"x"},"session_id":"s1","cwd":"` + repo + `"}`,
			old:     oldSkill,
		},
		{
			name:    "UserPromptSubmit typed prompt",
			event:   "UserPromptSubmit",
			payload: `{"hook_event_name":"UserPromptSubmit","prompt":"hello","session_id":"s1","cwd":"` + repo + `"}`,
			old:     oldSkill,
		},
		{
			name:    "SessionStart instructions",
			event:   "SessionStart",
			payload: `{"hook_event_name":"SessionStart"}`,
			old:     func(_ io.Reader, out io.Writer) error { return runClaudeInstructionsHook(out) },
		},
		{
			name:    "PreCompact read-marker reset",
			event:   "PreCompact",
			payload: `{"session_id":"s1"}`,
			old: func(in io.Reader, _ io.Writer) error {
				return runCompactResetHook(in, defaultReadHookStateDir())
			},
		},
	}

	budget, err := hookdispatch.LoadBudget()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var want bytes.Buffer
			if err := tc.old(strings.NewReader(tc.payload), &want); err != nil {
				t.Fatalf("old subcommand: %v", err)
			}

			t.Setenv("HOME", t.TempDir())
			var got bytes.Buffer
			reg := claudeHookRegistry(goldenMock{})
			if err := hookdispatch.Dispatch(context.Background(), reg, tc.event, []byte(tc.payload), &got, hookdispatch.Options{Budget: budget, Stderr: io.Discard}); err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if got.String() != want.String() {
				t.Fatalf("dispatcher output changed\n got %q\nwant %q", got.String(), want.String())
			}
		})
	}
}

// The `harnez hook <event>` command reads stdin once and dispatches. A
// UserPromptSubmit payload with no typed skill writes nothing and exits 0.
func TestHookEventCommandDispatches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newHookCmd()
	cmd.SetArgs([]string{"UserPromptSubmit"})
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"UserPromptSubmit","prompt":"just text"}`))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook UserPromptSubmit: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output %q", out.String())
	}
}
