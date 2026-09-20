package subagent

import (
	"context"
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
