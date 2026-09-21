package subagent

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestResolveModel(t *testing.T) {
	for _, tc := range []struct{ spec, provider, name, tier string }{{"codex:luna:low", "codex", "gpt-5.6-luna", "low"}, {"claude:haiku", "claude", "haiku", "low"}, {"claude:haiku:latest", "claude", "haiku", "low"}, {"agy:flash:low", "agy", "gemini-3.7-flash", "low"}} {
		m, err := ResolveModel(tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		if m.Provider != tc.provider || m.Name != tc.name || m.Tier != tc.tier {
			t.Fatalf("%s resolved to %#v", tc.spec, m)
		}
	}
}

func TestKnownModelsAndExplicitSpecsFailClosed(t *testing.T) {
	models := KnownModels()
	if len(models) == 0 || models[0].Spec() == "" {
		t.Fatal("known model registry is empty")
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

func TestCodexDeleteUsesNonInteractiveCommand(t *testing.T) {
	var command string
	var args []string
	d := CodexDriver{Command: func(_ context.Context, gotCommand string, gotArgs ...string) ([]byte, error) {
		command, args = gotCommand, gotArgs
		return nil, nil
	}}
	if err := d.Delete(context.Background(), "thread-id"); err != nil {
		t.Fatal(err)
	}
	if command != "codex" || !reflect.DeepEqual(args, []string{"delete", "--force", "thread-id"}) {
		t.Fatalf("command = %q %#v", command, args)
	}
}

func TestClaudeResumeUsesProviderSessionID(t *testing.T) {
	var args []string
	d := ClaudeDriver{Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"result":"done"}`), nil
	}}
	if _, err := d.Resume(context.Background(), "provider-id", "continue"); err != nil {
		t.Fatal(err)
	}
	want := []string{"-p", "--resume", "provider-id", "--output-format", "json", "continue"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestUnsupportedDriverCapabilityError(t *testing.T) {
	_, err := (UnsupportedDriver{Provider: "agy"}).Run(context.Background(), RunOptions{})
	if err == nil || !strings.Contains(err.Error(), `not supported for provider "agy"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexDriver(t *testing.T) {
	d := CodexDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"type":"thread.started","thread_id":"s1"}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":4,"cached_input_tokens":3}}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}`), nil
	}}
	r, err := d.Run(context.Background(), RunOptions{Prompt: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "s1" || r.Response != "done" || r.CachedTokens != 3 {
		t.Fatalf("unexpected result %#v", r)
	}
}

func TestClaudeDriver(t *testing.T) {
	d := ClaudeDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"result":"done","usage":{"input_tokens":10,"output_tokens":4,"cache_read_input_tokens":3,"cache_creation_input_tokens":2}}`), nil
	}}
	r, err := d.Run(context.Background(), RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Response != "done" || r.CachedTokens != 5 || r.TokensTurn != 19 {
		t.Fatalf("unexpected result %#v", r)
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
	r, err := d.ResumeStream(context.Background(), "t1", "p", func(e Event) { kinds = append(kinds, e.Kind+":"+e.Text) })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"session:t1", "message:hi", "activity:running sleep 3", "other:"}
	if !reflect.DeepEqual(kinds, want) || r.Response != "hi" || r.TokensTurn != 15 || r.CachedTokens != 4 {
		t.Fatalf("events=%q result=%+v", kinds, r)
	}
}
