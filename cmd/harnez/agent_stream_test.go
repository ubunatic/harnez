package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/subagent"
)

// elapsedField matches the elapsed time in stream labels ("[reply: 1s]").
var elapsedField = regexp.MustCompile(`\[([a-z ]+): [0-9hms]+([],])`)

// zeroElapsed sets stream label times to 0s: a slow -race runner can cross a
// second, and these tests check labels and order, not timing.
func zeroElapsed(s string) string { return elapsedField.ReplaceAllString(s, "[$1: 0s$2") }

type streamDriver struct{ recordingAgentDriver }

func (*streamDriver) emit(fn subagent.EventFunc) *subagent.TurnResult {
	fn(subagent.Event{Kind: "session", Text: "thread-1", Bytes: 40})
	fn(subagent.Event{Kind: "message", Text: "on it", Bytes: 80})
	fn(subagent.Event{Kind: "activity", Text: "running sleep", Bytes: 80})
	time.Sleep(60 * time.Millisecond)
	fn(subagent.Event{Kind: "message", Text: "all done", Bytes: 80})
	return &subagent.TurnResult{SessionID: "thread-1", Response: "all done", Messages: []string{"on it", "all done"}, TokensTurn: 500, ContextTokens: 100000, CompactionObserved: true}
}

func (d *streamDriver) RunStream(_ context.Context, _ subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.emit(fn), nil
}

func (d *streamDriver) ResumeStream(_ context.Context, _, _ string, _ subagent.Model, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.emit(fn), nil
}

func TestAgentStartStreamsLabeledBlocks(t *testing.T) {
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--name", "w", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := zeroElapsed(out.String())
	order := []string{"[session info: id=thread-1 agent=codex:gpt-6-luna action=start resolved=new]", "name=w", "reconnect: harnez agent resume thread-1", "[wait:", "[message: 0s]\non it", "[heartbeat: 0s, ~", "last: running sleep]", "[message: 0s]\nall done", "[done: 2 messages, last message is the reply"}
	pos := 0
	for _, want := range order {
		i := strings.Index(got[pos:], want)
		if i < 0 {
			t.Fatalf("missing %q (in order) in:\n%s", want, got)
		}
		pos += i + len(want)
	}
	if strings.Contains(got, "[agent messages]") || strings.Contains(got, "[msg ") {
		t.Fatalf("old labels present:\n%s", got)
	}
}

func TestAgentResumeStreamsCompactionAckLabel(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "claude", Model: "luna", Status: "completed", TokensSinceCompact: 250000, ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := zeroElapsed(out.String())
	if !strings.HasPrefix(got, "[session info: id=sid agent=claude:luna action=resume resolved=name]\n[wait: ") || !strings.Contains(got, "[compact: verified /compact at 250.0k context tokens") || !strings.Contains(got, "[compaction ack: 0s]\non it") || !strings.Contains(got, "[message: 0s]\nall done") {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestHeartbeatSchedule(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 10 * time.Minute, 15 * time.Minute, 20 * time.Minute}
	for i, w := range want {
		if got := heartbeatAt(i); got != w {
			t.Fatalf("heartbeatAt(%d) = %s, want %s", i, got, w)
		}
	}
}

func TestAgentStartStatsModeShowsOnlyHeartbeatsAndReply(t *testing.T) {
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--stream", "stats", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := zeroElapsed(out.String())
	if strings.Contains(got, "on it") || strings.Contains(got, "[message:") {
		t.Fatalf("intermediate message leaked in stats mode:\n%s", got)
	}
	for _, want := range []string{"[heartbeat: ", "1 messages, 1 commands", "[reply: 0s]\nall done\n", "[done: 2 messages (only the reply is shown)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestAgentRejectsUnknownStreamMode(t *testing.T) {
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--stream", "loud", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --stream") {
		t.Fatalf("err = %v", err)
	}
}

func TestShortDur(t *testing.T) {
	for in, want := range map[time.Duration]string{0: "0s", 30 * time.Second: "30s", time.Minute: "1m", 150 * time.Second: "2m30s", time.Hour: "1h", time.Hour + 5*time.Minute: "1h5m"} {
		if got := shortDur(in); got != want {
			t.Fatalf("shortDur(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestTokenSummarySeparatesNewFromCached(t *testing.T) {
	r := &subagent.TurnResult{InputTokens: 1230000, OutputTokens: 1200, CachedTokens: 1198000, TokensTurn: 1231200}
	if got, want := tokenSummary(r), "tokens: 33.2k new (1200 out), 1.2M cached"; got != want {
		t.Fatalf("tokenSummary = %q, want %q", got, want)
	}
	for in, want := range map[int]string{999: "999", 12300: "12.3k", 1230742: "1.2M"} {
		if got := humanCount(in); got != want {
			t.Fatalf("humanCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestStreamingResumeKeepsStderrQuiet(t *testing.T) {
	// Isolate the caller's Harnez session state so a pending session tip
	// cannot leak into this command's deliberately quiet stderr.
	t.Setenv("HOME", t.TempDir())
	for _, name := range resolve.SessionEnvVars {
		t.Setenv(name, "")
	}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", WorkingDir: t.TempDir(), Status: "completed", TokensSinceCompact: 250000, ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty while streaming", errOut.String())
	}
}

func TestStatsModeDoesNotRepeatAnAlreadyShownFinalMessage(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will do it")}}} // the only message is also the reply
	got := zeroElapsed(runScripted(t, d, "start", "--model", "luna", "task", "--name", "w", "--stream", "stats"))
	if strings.Count(got, "will do it") != 1 || strings.Contains(got, "CONFIRM:") {
		t.Fatalf("final message repeated or raw tag shown:\n%s", got)
	}
	if !strings.Contains(got, "[reply: 0s]\n(the final message was already shown above)\n") {
		t.Fatalf("missing reply marker:\n%s", got)
	}
}

func TestZeroElapsed(t *testing.T) {
	in := "[heartbeat: 1s, ~70 tokens]\n[reply: 1m5s]\nall done\n[session info: id=sid]\n[done: 2 messages]"
	want := "[heartbeat: 0s, ~70 tokens]\n[reply: 0s]\nall done\n[session info: id=sid]\n[done: 2 messages]"
	if got := zeroElapsed(in); got != want {
		t.Fatalf("zeroElapsed = %q, want %q", got, want)
	}
}
