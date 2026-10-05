package subagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCodexCheckResumable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	tests := []struct {
		name     string
		sessions bool
		file     bool
		want     bool
	}{
		{"missing sessions directory", false, false, true},
		{"missing rollout", true, false, false},
		{"matching rollout", true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sessions && os.MkdirAll(filepath.Join(home, "sessions", "2026", "01", "02"), 0700) != nil {
				t.Fatal("mkdir")
			}
			if tt.file && os.WriteFile(filepath.Join(home, "sessions", "2026", "01", "02", "rollout-prefix-thread.jsonl"), nil, 0600) != nil {
				t.Fatal("write")
			}
			ok, _ := (CodexDriver{}).CheckResumable("thread")
			if ok != tt.want {
				t.Fatalf("ok=%v, want %v", ok, tt.want)
			}
		})
	}
}

func TestResolveModel(t *testing.T) {
	for _, tc := range []struct{ spec, provider, name, tier string }{{"codex:luna:low", "codex", "gpt-6-luna", "low"}, {"codex:terra", "codex", "gpt-5.6-terra", "low"}, {"terra:low", "codex", "gpt-5.6-terra", "low"}, {"terra", "codex", "gpt-5.6-terra", "low"}, {"luna:low", "codex", "gpt-6-luna", "low"}, {"opus", "claude", "opus", "low"}, {"opus:med", "claude", "opus", "med"}, {"claude:haiku", "claude", "haiku", "low"}, {"claude:sonnet", "claude", "sonnet", "low"}, {"claude:opus", "claude", "opus", "low"}, {"claude:haiku:latest", "claude", "haiku", "low"}, {"agy:flash37:low", "agy", "gemini-3.7-flash", "low"}, {"agy:gemini-3.7-flash:low", "agy", "gemini-3.7-flash", "low"}, {"flash38", "agy", "gemini-3.8-flash", "low"}} {
		m, err := ResolveModel(tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		if m.Provider != tc.provider || m.Name != tc.name || m.Tier != tc.tier {
			t.Fatalf("%s resolved to %#v", tc.spec, m)
		}
	}
}

func TestResolveModelThreePartProviderSpec(t *testing.T) {
	m, err := ResolveModel("agy:flash37:low")
	if err != nil || m.Provider != "agy" || m.Name != "gemini-3.7-flash" || m.Tier != "low" {
		t.Fatalf("resolved model = %#v, %v", m, err)
	}
}

func TestCodexPassesReasoningEffort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var got []string
	d := CodexDriver{Command: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}` + "\n"), nil
	}}
	if _, err := d.Run(context.Background(), RunOptions{Model: Model{Name: "gpt-5.6-luna", Tier: "med"}, Prompt: "go"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox", "-m", "gpt-5.6-luna", "-c", "model_reasoning_effort=medium", "-c", "model_auto_compact_token_limit=200000", "go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("run args = %q, want %q", got, want)
	}

	if _, err := d.Resume(context.Background(), "t1", "continue", Model{Tier: "low"}); err != nil {
		t.Fatal(err)
	}
	want = []string{"exec", "resume", "t1", "--json", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-c", "model_reasoning_effort=low", "-c", "model_auto_compact_token_limit=200000", "continue"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resume args = %q, want %q", got, want)
	}
}

func TestCodexExecArgsUseConfiguredAutoCompactLimit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  compact_threshold_tokens: 12345\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		codexRunArgs(Model{Name: "gpt"}, "prompt"),
		codexResumeArgs("thread", "prompt", Model{Name: "gpt"}),
	} {
		if !strings.Contains(strings.Join(args, " "), "model_auto_compact_token_limit=12345") {
			t.Fatalf("args = %q, want configured auto-compact limit", args)
		}
	}
}

func TestCodexResumeEffortTiersAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		tier   string
		want   bool
		effort string
	}{
		{"low", true, "low"}, {"med", true, "medium"}, {"high", true, "high"}, {"", false, ""},
	} {
		var got []string
		d := CodexDriver{Command: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			got = args
			return []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}` + "\n"), nil
		}}
		if _, err := d.Resume(context.Background(), "t1", "continue", Model{Tier: tc.tier}); err != nil {
			t.Fatal(err)
		}
		has := strings.Contains(strings.Join(got, " "), "model_reasoning_effort=")
		if has != tc.want || has && !strings.Contains(strings.Join(got, " "), "model_reasoning_effort="+tc.effort) {
			t.Fatalf("tier %q args=%q", tc.tier, got)
		}
	}
}

func TestKnownModelsAndExplicitSpecsFailClosed(t *testing.T) {
	models := KnownModels()
	if len(models) == 0 || models[0].Spec() == "" {
		t.Fatal("known model registry is empty")
	}
	specs := strings.Join(KnownModelSpecs(), " ")
	for _, want := range []string{"codex:luna:med", "codex:luna:high", "agy:flash38:med", "agy:flash38:high"} {
		if !strings.Contains(specs, want) {
			t.Fatalf("known specs %q missing %q", specs, want)
		}
	}
	for _, want := range []string{"claude:haiku:med", "claude:haiku:high", "claude:sonnet:med", "claude:sonnet:high", "claude:opus:med", "claude:opus:high"} {
		if !strings.Contains(specs, want) {
			t.Fatalf("known specs %q missing %q", specs, want)
		}
	}
	for _, unwanted := range []string{"agy:opus:med", "agy:opus:high", "agy:sonnet:med", "agy:sonnet:high"} {
		if strings.Contains(specs, unwanted) {
			t.Fatalf("known specs %q list %s for a model without effort support", specs, unwanted)
		}
	}
	for _, spec := range []string{"codex:luna:invalid", "codex:missing:low", "unknown:model:high"} {
		_, err := ResolveModel(spec)
		if err == nil || !strings.Contains(err.Error(), spec) {
			t.Fatalf("spec %q unexpectedly resolved: %v", spec, err)
		}
		if !strings.Contains(err.Error(), "codex:luna:low") {
			t.Fatalf("spec %q error = %v, want known specs", spec, err)
		}
	}
}

func TestModelGuideCostByTier(t *testing.T) {
	var spec struct {
		Models map[string]ModelGuide `yaml:"models"`
	}
	if err := yaml.Unmarshal([]byte("models:\n  single: {cost: 9}\n  tiered: {cost: 1, cost_by_tier: {med: 4, high: 7}}\n"), &spec); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		model string
		tier  string
		want  int
	}{
		{"single", "low", 9}, {"single", "med", 9}, {"single", "unknown", 9},
		{"tiered", "low", 1}, {"tiered", "med", 4}, {"tiered", "high", 7}, {"tiered", "unknown", 1},
	} {
		if got := spec.Models[tc.model].CostForTier(tc.tier); got != tc.want {
			t.Errorf("%s cost for %s = %d, want %d", tc.model, tc.tier, got, tc.want)
		}
	}
}

func TestCodexDeleteSkipsNonUUIDIDs(t *testing.T) {
	callCount := 0
	d := CodexDriver{Start: func(_ context.Context, _ string, _ ...string) (io.Reader, func() error, error) {
		callCount++
		return nil, nil, nil
	}}
	if err := d.Delete(context.Background(), "thread-1"); err != nil {
		t.Fatal(err)
	}
	if callCount != 0 {
		t.Fatalf("expected no codex call for non-UUID ID, but was called %d times", callCount)
	}
}

func TestCodexDeleteMissingRolloutSkipsCodex(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	called := false
	d := CodexDriver{Start: func(_ context.Context, _ string, _ ...string) (io.Reader, func() error, error) {
		called = true
		return strings.NewReader(""), func() error { return nil }, nil
	}}
	if err := d.Delete(context.Background(), "550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("codex was started for a missing rollout")
	}
}

func TestCodexDeleteExistingRolloutWithProviderError(t *testing.T) {
	setCodexDeleteRollout(t)
	d := CodexDriver{Start: func(_ context.Context, _ string, _ ...string) (io.Reader, func() error, error) {
		return strings.NewReader(""), func() error { return errors.New("provider delete failed") }, nil
	}}
	err := d.Delete(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err == nil || !strings.Contains(err.Error(), "provider delete failed") {
		t.Fatalf("error = %v, want provider failure", err)
	}
}

func TestCodexDeleteIncludesStderr(t *testing.T) {
	setCodexDeleteRollout(t)
	d := CodexDriver{Start: func(_ context.Context, _ string, _ ...string) (io.Reader, func() error, error) {
		return strings.NewReader(""), func() error {
			return fmt.Errorf("exit status 1: --force requires a session UUID")
		}, nil
	}}
	err := d.Delete(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err == nil || !strings.Contains(err.Error(), "codex delete:") || !strings.Contains(err.Error(), "--force requires a session UUID") {
		t.Fatalf("error = %v, want 'codex delete:' wrapper with stderr", err)
	}
}

func TestCodexDeleteUsesNonInteractiveCommand(t *testing.T) {
	setCodexDeleteRollout(t)
	var cmd string
	var args []string
	d := CodexDriver{Start: func(_ context.Context, gotCmd string, gotArgs ...string) (io.Reader, func() error, error) {
		cmd = gotCmd
		args = gotArgs
		return strings.NewReader(""), func() error { return nil }, nil
	}}
	if err := d.Delete(context.Background(), "550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatal(err)
	}
	if cmd != "codex" {
		t.Fatalf("cmd = %q, want codex", cmd)
	}
	if !reflect.DeepEqual(args, []string{"delete", "--force", "550e8400-e29b-41d4-a716-446655440000"}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestCodexDeleteHonorsCodexHomeOverride(t *testing.T) {
	setCodexDeleteRollout(t)
	started := false
	d := CodexDriver{Start: func(_ context.Context, _ string, _ ...string) (io.Reader, func() error, error) {
		started = true
		return strings.NewReader(""), func() error { return nil }, nil
	}}
	if err := d.Delete(context.Background(), "550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("codex was not started for rollout under CODEX_HOME")
	}
}

func setCodexDeleteRollout(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	path := filepath.Join(home, "sessions", "2026", "10", "02", "rollout-test-550e8400-e29b-41d4-a716-446655440000.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeResumeUsesProviderSessionID(t *testing.T) {
	var args []string
	d := ClaudeDriver{Dir: "/tmp/session-dir", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"session_id":"provider-session","result":"done"}`), nil
	}}
	r, err := d.Resume(context.Background(), "provider-id", "continue", Model{Name: "haiku"})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "provider-session" {
		t.Fatalf("session ID = %q, want provider-session", r.SessionID)
	}
	want := []string{"-p", "--resume", "provider-id", "--dangerously-skip-permissions", "--model", "haiku", "--output-format", "stream-json", "--verbose", "continue"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestClaudeDriverEffortTiers(t *testing.T) {
	for _, tc := range []struct {
		name, tier, effort string
	}{
		{"low", "low", "low"},
		{"medium", "med", "medium"},
		{"high", "high", "high"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var runArgs, resumeArgs []string
			d := ClaudeDriver{Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
				args := append([]string(nil), gotArgs...)
				if len(args) > 1 && args[1] == "--resume" {
					resumeArgs = args
				} else {
					runArgs = args
				}
				return []byte(`{"session_id":"s1","result":"done"}`), nil
			}}
			model := Model{Name: "sonnet", Tier: tc.tier}
			if _, err := d.Run(context.Background(), RunOptions{Model: model, Prompt: "go"}); err != nil {
				t.Fatal(err)
			}
			if _, err := d.Resume(context.Background(), "s1", "continue", model); err != nil {
				t.Fatal(err)
			}
			wantRun := []string{"-p", "--dangerously-skip-permissions", "--model", "sonnet", "--effort", tc.effort, "--output-format", "stream-json", "--verbose", "go"}
			wantResume := []string{"-p", "--resume", "s1", "--dangerously-skip-permissions", "--model", "sonnet", "--effort", tc.effort, "--output-format", "stream-json", "--verbose", "continue"}
			if !reflect.DeepEqual(runArgs, wantRun) {
				t.Errorf("run args = %q, want %q", runArgs, wantRun)
			}
			if !reflect.DeepEqual(resumeArgs, wantResume) {
				t.Errorf("resume args = %q, want %q", resumeArgs, wantResume)
			}
		})
	}
}

func TestClaudeDriverOmitsEffortWithoutTier(t *testing.T) {
	var runArgs, resumeArgs []string
	d := ClaudeDriver{Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args := append([]string(nil), gotArgs...)
		if len(args) > 1 && args[1] == "--resume" {
			resumeArgs = args
		} else {
			runArgs = args
		}
		return []byte(`{"session_id":"s1","result":"done"}`), nil
	}}
	model := Model{Name: "sonnet"}
	if _, err := d.Run(context.Background(), RunOptions{Model: model, Prompt: "go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Resume(context.Background(), "s1", "continue", model); err != nil {
		t.Fatal(err)
	}
	wantRun := []string{"-p", "--dangerously-skip-permissions", "--model", "sonnet", "--output-format", "stream-json", "--verbose", "go"}
	wantResume := []string{"-p", "--resume", "s1", "--dangerously-skip-permissions", "--model", "sonnet", "--output-format", "stream-json", "--verbose", "continue"}
	if !reflect.DeepEqual(runArgs, wantRun) {
		t.Errorf("run args = %q, want %q", runArgs, wantRun)
	}
	if !reflect.DeepEqual(resumeArgs, wantResume) {
		t.Errorf("resume args = %q, want %q", resumeArgs, wantResume)
	}
}

func TestUnsupportedDriverCapabilityError(t *testing.T) {
	_, err := (UnsupportedDriver{Provider: "agy"}).Run(context.Background(), RunOptions{})
	if err == nil || !strings.Contains(err.Error(), `not supported for provider "agy"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestModelEntriesWithDriverMarksInteractiveOnly(t *testing.T) {
	entries := []ModelEntry{
		{Spec: "fake:chat:low", Model: Model{Provider: "fake", Name: "chat"}},
		{Spec: "codex:luna:low", Model: Model{Provider: "codex", Name: "luna"}},
	}
	got := ModelEntriesWithDriver(entries, func(m Model) Driver {
		if m.Provider == "fake" {
			return UnsupportedDriver{Provider: m.Provider}
		}
		return testBatchDriver{}
	})
	if got[0].Batch {
		t.Fatal("unsupported fake provider reported batch support")
	}
	if !got[1].Batch {
		t.Fatal("supported provider reported interactive-only")
	}
}

type testBatchDriver struct{}

func (testBatchDriver) Run(context.Context, RunOptions) (*TurnResult, error) { return nil, nil }
func (testBatchDriver) Resume(context.Context, string, string, Model) (*TurnResult, error) {
	return nil, nil
}
func (testBatchDriver) Compact(context.Context, string) (*TurnResult, error) { return nil, nil }
func (testBatchDriver) Stop(context.Context, string) error                   { return nil }
func (testBatchDriver) Delete(context.Context, string) error                 { return nil }

func TestCodexDriver(t *testing.T) {
	d := CodexDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"type":"thread.started","thread_id":"s1"}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":13,"cached_input_tokens":3}}}}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":4,"cached_input_tokens":3}}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}`), nil
	}}
	r, err := d.Run(context.Background(), RunOptions{Prompt: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "s1" || r.Response != "done" || r.CachedTokens != 3 || r.ContextTokens != -1 {
		t.Fatalf("unexpected result %#v", r)
	}
}

func TestClaudeDriver(t *testing.T) {
	d := ClaudeDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"session_id":"claude-id","result":"done","usage":{"input_tokens":10,"output_tokens":4,"cache_read_input_tokens":3,"cache_creation_input_tokens":2}}`), nil
	}}
	r, err := d.Run(context.Background(), RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "claude-id" || r.Response != "done" || r.CachedTokens != 5 || r.TokensTurn != 19 {
		t.Fatalf("unexpected result %#v", r)
	}
}

func TestClaudeCommandUsesDirAndReturnsStderr(t *testing.T) {
	bin := t.TempDir()
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\npwd\necho deliberate-error >&2\nexit 7\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := ClaudeDriver{Dir: work}
	out, err := d.command(context.Background(), "-p")
	if err == nil || !strings.Contains(err.Error(), "deliberate-error") {
		t.Fatalf("error = %v, want stderr", err)
	}
	if strings.TrimSpace(string(out)) != work {
		t.Fatalf("working directory output = %q, want %q", strings.TrimSpace(string(out)), work)
	}
}

func TestParseCodexKeepsAllAgentMessages(t *testing.T) {
	data := []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"first"}}
{"type":"item.completed","item":{"type":"agent_message","text":"second"}}
`)
	r, err := parseCodex(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.Response != "second" || len(r.Messages) != 2 || r.Messages[0] != "first" {
		t.Fatalf("response=%q messages=%q", r.Response, r.Messages)
	}
}

func TestCodexStreamEmitsEventsInOrder(t *testing.T) {
	lines := `{"type":"thread.started","thread_id":"t1"}
{"type":"item.completed","item":{"type":"agent_message","text":"hi"}}
{"type":"item.started","item":{"type":"command_execution","command":"sleep 3"}}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5,"cached_input_tokens":4}}
`
	d := CodexDriver{Start: func(context.Context, string, ...string) (io.Reader, func() error, error) {
		return strings.NewReader(lines), func() error { return nil }, nil
	}}
	var kinds []string
	r, err := d.ResumeStream(context.Background(), "t1", "p", Model{}, func(e Event) { kinds = append(kinds, e.Kind+":"+e.Text) })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"session:t1", "message:hi", "activity:running sleep 3", "other:"}
	if !reflect.DeepEqual(kinds, want) || r.Response != "hi" || r.TokensTurn != 15 || r.CachedTokens != 4 {
		t.Fatalf("events=%q result=%+v", kinds, r)
	}
}

func TestCodexStreamReportsMidTurnContextTokenCanary(t *testing.T) {
	lines := `{"type":"thread.started","thread_id":"t1"}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":205000}}}}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
`
	d := CodexDriver{Start: func(context.Context, string, ...string) (io.Reader, func() error, error) {
		return strings.NewReader(lines), func() error { return nil }, nil
	}}
	watchdog := &TokenWatchdog{Threshold: 200000}
	var crossed []int
	_, err := d.ResumeStream(context.Background(), "t1", "p", Model{}, func(event Event) {
		if tokens, ok := watchdog.Observe(event); ok {
			crossed = append(crossed, tokens)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(crossed, []int{205000}) {
		t.Fatalf("crossings = %v, want [205000]", crossed)
	}
}

func TestCodexReadsLastTokenUsageFromRollout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	path := filepath.Join(home, "sessions", "2026", "09", "26", "rollout-2026-09-26T00-26-57-session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	const rollout = `{"type":"turn.completed","usage":{"input_tokens":27754,"cached_input_tokens":24064,"output_tokens":10}}` + "\n" +
		`{"type":"compaction","id":"cmp_06e964c86f3f6504016ab6f55168e487d2ad0aa932bea59e8a","encrypted_content":"<redacted>"}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":41691},"last_token_usage":{"input_tokens":13937,"cached_input_tokens":11008,"output_tokens":5}}}}` + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":41691,"cached_input_tokens":35072,"output_tokens":15}}` + "\n"
	if err := os.WriteFile(path, []byte(rollout), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := codexRolloutContextTokens("session")
	if err != nil || got != 13937 {
		t.Fatalf("rollout context tokens = %d, %v", got, err)
	}
	got, compactions, err := codexRolloutState("session")
	if err != nil || got != 13937 || compactions != 1 {
		t.Fatalf("rollout state = context %d compactions %d err %v", got, compactions, err)
	}
}

func TestCodexResumeRolloutReadFailureIsUnknown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	d := CodexDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}` + "\n"), nil
	}}
	r, err := d.Resume(context.Background(), "missing", "continue", Model{Name: "model"})
	if err != nil || r.ContextTokens != -1 {
		t.Fatalf("resume result context=%d err=%v; want unknown context", r.ContextTokens, err)
	}
}

func TestCodexResumeUsesSameSandboxAsRun(t *testing.T) {
	var got []string
	d := CodexDriver{Command: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}` + "\n"), nil
	}}
	if _, err := d.Resume(context.Background(), "t1", "go", Model{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec", "resume", "t1", "--json", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-c", "model_auto_compact_token_limit=200000", "go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resume args = %q, want %q", got, want)
	}
}

func TestCodexCompactDoesNotSendPrompt(t *testing.T) {
	d := CodexDriver{}
	if _, err := d.Compact(context.Background(), "t1"); err == nil || !strings.Contains(err.Error(), "manual /compact is unsupported") {
		t.Fatalf("error = %v, want unsupported manual compact", err)
	}
}
