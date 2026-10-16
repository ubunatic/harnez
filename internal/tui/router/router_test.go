package router

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

// batchDriver is a fake non-streaming driver that records its calls.
type batchDriver struct {
	subagent.UnsupportedDriver
	calls *[]string
}

func (d batchDriver) Run(_ context.Context, o subagent.RunOptions) (*subagent.TurnResult, error) {
	*d.calls = append(*d.calls, "run "+o.Model.Provider+" "+o.Prompt)
	return &subagent.TurnResult{SessionID: "s-" + o.Model.Provider, Messages: []string{"one", "two"}}, nil
}

func (d batchDriver) Resume(_ context.Context, id, prompt string, m subagent.Model) (*subagent.TurnResult, error) {
	*d.calls = append(*d.calls, "resume "+id+" "+prompt)
	return &subagent.TurnResult{SessionID: id, Messages: []string{"again"}}, nil
}

// streamDriver is a fake streaming driver emitting a fixed event sequence.
type streamDriver struct{ batchDriver }

func (d streamDriver) RunStream(_ context.Context, o subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	*d.calls = append(*d.calls, "runstream "+o.Model.Provider)
	for _, ev := range []subagent.Event{
		{Kind: "session", Text: "t1"},
		{Kind: "message", Text: "first"},
		{Kind: "activity", Text: "running ls"},
		{Kind: "other"},
		{Kind: "message", Text: "second"},
	} {
		fn(ev)
	}
	return &subagent.TurnResult{SessionID: "t1"}, nil
}

func (d streamDriver) ResumeStream(_ context.Context, id, _ string, _ subagent.Model, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	*d.calls = append(*d.calls, "resumestream "+id)
	fn(subagent.Event{Kind: "message", Text: "resumed"})
	return &subagent.TurnResult{SessionID: id}, nil
}

func collect(t *testing.T, r *Router, spec, prompt string) []Chunk {
	t.Helper()
	var got []Chunk
	if err := r.Turn(context.Background(), spec, prompt, func(c Chunk) { got = append(got, c) }); err != nil {
		t.Fatalf("Turn(%s): %v", spec, err)
	}
	return got
}

func TestTurnRoutesPerProviderAndResumes(t *testing.T) {
	var calls []string
	r := &Router{Dir: t.TempDir(), Driver: func(m subagent.Model, _ string) subagent.Driver {
		if m.Provider == "codex" {
			return streamDriver{batchDriver{calls: &calls}}
		}
		return batchDriver{calls: &calls}
	}}

	got := collect(t, r, "claude:sonnet", "hi")
	want := []Chunk{{ChunkText, "one"}, {ChunkText, "two"}}
	if strings.Join(texts(got), "|") != "one|two" || got[0].Kind != want[0].Kind {
		t.Fatalf("claude chunks = %#v, want %#v", got, want)
	}
	collect(t, r, "claude:sonnet:high", "again")
	collect(t, r, "agy:flash38", "x")
	got = collect(t, r, "codex:luna", "c")
	if s := strings.Join(texts(got), "|"); s != "first|running ls|second" {
		t.Fatalf("codex stream order = %q", s)
	}
	if got[1].Kind != ChunkActivity {
		t.Fatalf("codex activity kind = %q", got[1].Kind)
	}
	collect(t, r, "codex:luna", "d")

	wantCalls := []string{
		"run claude hi",
		"resume s-claude again", // a tier change keeps the backend session
		"run agy x",
		"runstream codex",
		"resumestream t1",
	}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(wantCalls, "\n"))
	}
	if id := r.Session("codex:luna"); id != "t1" {
		t.Fatalf("Session(codex:luna) = %q", id)
	}
}

func TestTurnUnsupportedProviderErrors(t *testing.T) {
	r := &Router{Driver: func(m subagent.Model, _ string) subagent.Driver {
		return subagent.UnsupportedDriver{Provider: m.Provider}
	}}
	err := r.Turn(context.Background(), "claude:sonnet", "hi", func(Chunk) { t.Fatal("unexpected chunk") })
	if err == nil || !strings.Contains(err.Error(), "not supported by harnez tui yet") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveSpecValidatesAgainstAgentSpec(t *testing.T) {
	got, err := ResolveSpec("codex:luna")
	if err != nil || !strings.HasPrefix(got, "codex:luna:") {
		t.Fatalf("ResolveSpec(codex:luna) = %q, %v", got, err)
	}
	if _, err := ResolveSpec("nosuch:model"); err == nil {
		t.Fatal("ResolveSpec(nosuch:model) succeeded")
	}
}

func texts(cs []Chunk) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Text)
	}
	return out
}

// fakeCLI installs an executable shell script named name on PATH.
func fakeCLI(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", t.TempDir())
	return dir
}

func TestTurnStreamsFakeCodexCLIAndResumes(t *testing.T) {
	dir := fakeCLI(t, "codex", `echo "$@" >> "$(dirname "$0")/args"
echo '{"type":"thread.started","thread_id":"th-1"}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"alpha"}}'
echo '{"type":"item.started","item":{"type":"command_execution","command":"go test"}}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"beta"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
`)
	r := New(t.TempDir())
	got := collect(t, r, "codex:luna", "p1")
	if s := strings.Join(texts(got), "|"); s != "alpha|running go test|beta" {
		t.Fatalf("chunks = %q", s)
	}
	collect(t, r, "codex:luna", "p2")
	b, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "exec resume th-1 ") {
		t.Fatalf("codex args:\n%s", b)
	}
}

// Cancelling a turn kills the provider's whole process tree, including a
// grandchild that would otherwise keep stdout open.
func TestTurnCancelKillsProviderTree(t *testing.T) {
	dir := fakeCLI(t, "claude", `sleep 300 &
echo $! > "$(dirname "$0")/child.pid"
wait
`)
	pidFile := filepath.Join(dir, "child.pid")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- New(t.TempDir()).Turn(ctx, "claude:sonnet", "hi", func(Chunk) {}) }()

	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0; {
		if b, err := os.ReadFile(pidFile); err == nil && strings.HasSuffix(string(b), "\n") {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		if time.Now().After(deadline) {
			t.Fatal("fake claude never started its child")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Turn err = %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Turn did not return after cancel")
	}
	for deadline := time.Now().Add(3 * time.Second); alive(pid); {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("grandchild %d survived cancel", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// alive reports whether pid runs; a zombie awaiting its reaper counts as dead.
func alive(pid int) bool {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	_, rest, _ := strings.Cut(string(b), ") ")
	return !strings.HasPrefix(rest, "Z")
}
