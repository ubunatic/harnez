package subagent

import (
	"context"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestAgyRunArgOrderAndAddDir(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/work/dir", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"status":"SUCCESS","response":"PONG\n"}`), nil
	}}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "ping", Model: Model{Name: "gemini-3.7-flash", Tier: "low"}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"--add-dir", "/work/dir", "--model", "gemini-3.7-flash", "--effort", "low", "--output-format", "json", "-p", "ping"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if args[len(args)-2] != "-p" {
		t.Fatalf("-p must be the last flag, directly followed by the prompt: %#v", args)
	}
}

func TestAgyEffortMapping(t *testing.T) {
	for _, tc := range []struct{ tier, want string }{{"low", "low"}, {"med", "medium"}, {"high", "high"}} {
		var args []string
		d := AgyDriver{Dir: "/d", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
			args = gotArgs
			return []byte(`{"status":"SUCCESS","response":"ok"}`), nil
		}}
		if _, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m", Tier: tc.tier}}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(args[4:6], []string{"--effort", tc.want}) {
			t.Fatalf("tier %q: args = %#v, want effort %q", tc.tier, args, tc.want)
		}
	}
}

func TestAgyResumeArgs(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/work/dir", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"conversation_id":"abc","status":"SUCCESS","response":"done"}`), nil
	}}
	r, err := d.Resume(context.Background(), "abc", "continue", Model{Name: "claude-sonnet-4-6", Tier: "high"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--conversation", "abc", "--add-dir", "/work/dir", "--model", "claude-sonnet-4-6", "--effort", "high", "--output-format", "json", "-p", "continue"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if r.SessionID != "abc" || r.Response != "done" {
		t.Fatalf("unexpected result %#v", r)
	}
}

func TestAgyParseJSONWithTokens(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"conversation_id":"c1","status":"SUCCESS","response":"PONG\n","duration_seconds":1.7,"num_turns":1,"usage":{"input_tokens":11810,"output_tokens":23,"thinking_tokens":21,"cache_read_tokens":0,"total_tokens":11833}}`), nil
	}}
	r, err := d.Run(context.Background(), RunOptions{Prompt: "ping", Model: Model{Name: "m", Tier: "low"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "c1" || r.Response != "PONG\n" {
		t.Fatalf("unexpected result %#v", r)
	}
	if r.InputTokens != 11810 {
		t.Fatalf("InputTokens = %d, want 11810", r.InputTokens)
	}
	if r.OutputTokens != 23+21 {
		t.Fatalf("OutputTokens = %d, want %d (output+thinking)", r.OutputTokens, 23+21)
	}
	if r.CachedTokens != 0 {
		t.Fatalf("CachedTokens = %d, want 0", r.CachedTokens)
	}
	if r.TokensTurn != r.InputTokens+r.OutputTokens+r.CachedTokens {
		t.Fatalf("TokensTurn = %d, want sum %d", r.TokensTurn, r.InputTokens+r.OutputTokens+r.CachedTokens)
	}
}

func TestAgyNonSuccessStatusIsError(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"status":"ERROR","response":"tool call failed"}`), nil
	}}
	_, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m"}})
	if err == nil || !strings.Contains(err.Error(), "tool call failed") {
		t.Fatalf("error = %v, want status/response included", err)
	}
}

func TestAgyMalformedJSONIsError(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`not json`), nil
	}}
	_, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m"}})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestAgyCompactUsesResumeWithSlashCompact(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/d", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"status":"SUCCESS","response":"ok"}`), nil
	}}
	if _, err := d.Compact(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if args[len(args)-1] != "/compact" {
		t.Fatalf("args = %#v, want last element /compact", args)
	}
	for _, a := range args {
		if a == "--model" {
			t.Fatalf("args = %#v, --model must be omitted for an empty Model", args)
		}
	}
}

func TestAgyOmitsEffortWhenUnsupported(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/d", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"status":"SUCCESS","response":"ok"}`), nil
	}}
	m := Model{Name: "claude-sonnet-4-6", Tier: "low", Effort: boolPtr(false)}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: m}); err != nil {
		t.Fatal(err)
	}
	want := []string{"--add-dir", "/d", "--model", "claude-sonnet-4-6", "--output-format", "json", "-p", "p"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestAgyExitOneParsesStdoutErrorJSON(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"status":"ERROR","error":"--effort is not supported for model claude-sonnet-4-6"}`), &exec.ExitError{}
	}}
	_, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "claude-sonnet-4-6"}})
	if err == nil || !strings.Contains(err.Error(), "--effort is not supported") {
		t.Fatalf("error = %v, want the stdout error field surfaced", err)
	}
}

func TestAgyRunRefusesEmptyDir(t *testing.T) {
	d := AgyDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("command must not run with an empty Dir")
		return nil, nil
	}}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m"}}); err == nil {
		t.Fatal("expected error for empty Dir")
	}
}

func TestAgyResumeRefusesEmptyDir(t *testing.T) {
	d := AgyDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("command must not run with an empty Dir")
		return nil, nil
	}}
	if _, err := d.Resume(context.Background(), "id", "p", Model{Name: "m"}); err == nil {
		t.Fatal("expected error for empty Dir")
	}
}

func TestAgyStopAndDeleteAreNoop(t *testing.T) {
	d := AgyDriver{}
	if err := d.Stop(context.Background(), "id"); err != nil {
		t.Fatalf("Stop returned %v, want nil", err)
	}
	if err := d.Delete(context.Background(), "id"); err != nil {
		t.Fatalf("Delete returned %v, want nil", err)
	}
}

var _ Driver = AgyDriver{}
