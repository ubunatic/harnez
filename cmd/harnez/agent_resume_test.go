package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestAgentResumeFailureRecordedAndCleared(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("provider failed\nwith detail")}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	for want := 1; want <= 2; want++ {
		cmd := newAgentCmd()
		cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
		if err := cmd.Execute(); err == nil {
			t.Fatal("resume unexpectedly succeeded")
		}
		sess, _ := store.Get("sid")
		if sess.ResumeFailures != want || sess.LastError != "provider failed" || sess.Status != "failed" || sess.ProcessPID != 0 {
			t.Fatalf("session = %#v", sess)
		}
		if len([]rune(sess.LastError)) > 300 {
			t.Fatal("last error too long")
		}
	}
	d.err = nil
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	sess, _ := store.Get("sid")
	if sess.LastError != "" || sess.ResumeFailures != 0 {
		t.Fatalf("success did not clear failure: %#v", sess)
	}
}

func TestAgentListResumeColumnStates(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model, _ string) subagent.Driver {
		d := &resumeOutcomeDriver{}
		if m.Provider == "terminal" {
			d.terminal = true
		}
		return d
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	for _, sess := range []*subagent.Session{
		{ID: "ok", Name: "okay", Provider: "fake", Model: "model", Status: "completed"},
		{ID: "bad", Name: "failed", Provider: "fake", Model: "model", LastError: "provider failed", Status: "completed"},
		{ID: "term", Name: "terminal", Provider: "terminal", Model: "model", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"okay\tfake\tcompleted\tok", "failed\tfake\tcompleted\tfailed", "terminal\tterminal\tcompleted\tterminal"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list = %q, missing %q", out.String(), want)
		}
	}
}

func TestAgentTerminalResumeRefusal(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{terminal: true}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be resumed") || !strings.Contains(err.Error(), "harnez agent start --name") {
		t.Fatalf("error = %v", err)
	}
	sess, _ := store.Get("sid")
	if d.resumes != 0 || sess.ResumeFailures != 0 || d.prompt != "" {
		t.Fatalf("terminal attempt changed state: resumes=%d fresh prompt=%q session=%#v", d.resumes, d.prompt, sess)
	}
}

// A verified stale provider session (claude "No conversation found") starts
// a fresh provider session with the same prompt (issue 791): the store holds
// the new id, the note goes to stderr and the --json result says so.
func TestAgentResumeStaleSessionStartsFresh(t *testing.T) {
	stale := fmt.Errorf("claude resume: %w", &subagent.ProviderExitError{Err: errors.New("exit status 1"), Stderr: "No conversation found with session ID: sid"})
	wantNote := "could not resume session sid (claude resume: exit status 1: No conversation found with session ID: sid); starting a new session"
	for _, jsonOut := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%v", jsonOut), func(t *testing.T) {
			d := &resumeOutcomeDriver{err: stale}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			storeDir := t.TempDir()
			store := saveResumeSession(t, storeDir, "sid", "worker", "claude")
			var stdout, stderr bytes.Buffer
			cmd := newAgentCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			args := []string{"resume", "--name", "worker", "hi", "--store-dir", storeDir}
			if jsonOut {
				args = append(args, "--json")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("resume: %v (stderr %s)", err, stderr.String())
			}
			sess, _ := store.Get("sid")
			if d.resumes != 1 || d.prompt != "hi" {
				t.Fatalf("resumes=%d fresh prompt=%q", d.resumes, d.prompt)
			}
			if sess.ProviderSessionID != "recorded" || sess.ResumeFailures != 0 || sess.LastError != "" || sess.Status != "completed" {
				t.Fatalf("session after fresh start = %#v", sess)
			}
			if !strings.Contains(stderr.String(), wantNote) {
				t.Fatalf("stderr lacks note %q:\n%s", wantNote, stderr.String())
			}
			if !jsonOut {
				return
			}
			var got struct {
				Resumed           *bool  `json:"resumed"`
				PreviousSessionID string `json:"previous_session_id"`
				Note              string `json:"note"`
				ProviderSessionID string `json:"provider_session_id"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("json: %v\n%s", err, stdout.String())
			}
			if got.Resumed == nil || *got.Resumed || got.PreviousSessionID != "sid" || got.Note != wantNote || got.ProviderSessionID != "recorded" {
				t.Fatalf("json result = %+v", got)
			}
		})
	}
}

// Errors that are not a stale session keep failing the resume, with no
// fresh provider session and the failure recorded.
func TestAgentResumeOtherErrorsDoNotStartFresh(t *testing.T) {
	d := &resumeOutcomeDriver{err: errors.New("usage limit reached")}
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "worker", "hi"); err == nil || !strings.Contains(err.Error(), "usage limit reached") {
		t.Fatalf("error = %v", err)
	}
	sess, _ := store.Get("sid")
	if d.prompt != "" || d.resumes != 1 || sess.ResumeFailures != 1 || sess.ProviderSessionID != "" {
		t.Fatalf("fresh prompt=%q resumes=%d session=%#v", d.prompt, d.resumes, sess)
	}
}

func TestCodexResumeQuotaRecoveryQuarantinesAndExcludesSession(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("usage limit reached")}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "codex")
	availability := usage.ProviderQuotaAvailability{State: "exhausted"}
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		availability: func(string, string) usage.ProviderQuotaAvailability { return availability },
	}
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull}); err == nil {
		t.Fatal("expected exhausted quota failure")
	}
	availability = usage.ProviderQuotaAvailability{State: "available"}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull}); err == nil {
		t.Fatal("expected quarantine failure")
	}
	sess, _ := store.Get("sid")
	if sess.CodexQuarantine == nil || sess.CodexQuarantine.Attempts != 1 {
		t.Fatalf("quarantine = %#v", sess.CodexQuarantine)
	}
	if got := attributable([]*subagent.Session{sess}, ".", ""); len(got) != 0 {
		t.Fatalf("quarantined session selected: %#v", got)
	}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "harnez agent start") || d.resumes != 2 {
		t.Fatalf("repeat resume err=%v resumes=%d", err, d.resumes)
	}
}

func TestCodexTransientRateLimitDoesNotQuarantine(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("429 rate limit exceeded")}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "codex")
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		availability: func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "available"}
		},
	}
	for range 2 {
		cmd := newAgentCmd()
		if err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull}); err == nil {
			t.Fatal("expected rate-limit failure")
		}
	}
	sess, _ := store.Get("sid")
	if sess.CodexQuarantine != nil || sess.CodexQuotaResumePending || sess.ResumeFailures != 2 || d.resumes != 2 {
		t.Fatalf("transient errors changed quarantine state: session=%#v resumes=%d", sess, d.resumes)
	}
}

func TestAgentStatusResumeDiagnostics(t *testing.T) {
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	sess, _ := store.Get("sid")
	sess.LastError = "provider failed"
	sess.ResumeFailures = 2
	_ = store.Save(sess)
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Last error: provider failed") || !strings.Contains(out.String(), "Resume failures: 2") {
		t.Fatalf("status = %q", out.String())
	}
	sess.LastError = ""
	_ = store.Save(sess)
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Last error:") || strings.Contains(out.String(), "Resume failures:") {
		t.Fatalf("clean status = %q", out.String())
	}
}

func TestAgentStatusReplyHistory(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 10, 12, 30, 0, 0, time.UTC)
	sess := &subagent.Session{
		ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed",
		Response: "", Turn: 3,
		ReplyHistory: []subagent.ReplyRecord{
			{Turn: 1, At: at, Text: "ALPHA"},
			{Turn: 2, At: at.Add(time.Minute), Text: "BETA"},
			{Turn: 3, At: at.Add(2 * time.Minute), Text: ""},
		},
	}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want []string
		miss []string
	}{
		{args: []string{"--reply"}, want: []string{"Reply (turn 3", "(empty reply)"}, miss: []string{"ALPHA", "BETA"}},
		{args: []string{"--turn", "1"}, want: []string{"Reply (turn 1", "ALPHA"}, miss: []string{"BETA"}},
		{args: []string{"--all"}, want: []string{"Reply (turn 1", "ALPHA", "Reply (turn 2", "BETA", "Reply (turn 3", "(empty reply)"}},
	} {
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetArgs(append([]string{"status", "--name", "worker", "--store-dir", storeDir}, tc.args...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("status %v: %v", tc.args, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(out.String(), want) {
				t.Errorf("status %v = %q, want %q", tc.args, out.String(), want)
			}
		}
		for _, miss := range tc.miss {
			if strings.Contains(out.String(), miss) {
				t.Errorf("status %v = %q, must not contain %q", tc.args, out.String(), miss)
			}
		}
	}

	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got subagent.Session
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.ReplyHistory) != 3 || got.ReplyHistory[1].Text != "BETA" || got.ReplyHistory[2].Text != "" {
		t.Fatalf("JSON reply_history = %#v", got.ReplyHistory)
	}
}

func TestRecordAgentReplyCapsHistoryAtSpecLimit(t *testing.T) {
	sess := &subagent.Session{}
	for turn := 1; turn <= 20; turn++ {
		sess.ReplyHistory = append(sess.ReplyHistory, subagent.ReplyRecord{Turn: turn, Text: fmt.Sprint(turn)})
	}
	at := time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC)
	if err := recordAgentReply(sess, 21, "21", nil, at); err != nil {
		t.Fatal(err)
	}
	if len(sess.ReplyHistory) != 20 || sess.ReplyHistory[0].Turn != 2 || sess.ReplyHistory[19].Turn != 21 {
		t.Fatalf("reply history turns = %#v", sess.ReplyHistory)
	}
}

func TestAgentResumeMissingIdentifier(t *testing.T) {
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "nope", "hi", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAttributableOrderingAndFilters(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model, _ string) subagent.Driver {
		d := &resumeOutcomeDriver{}
		d.terminal = m.Provider == "terminal"
		return d
	}
	defer func() { agentDriver = old }()
	now := time.Now()
	sessions := []*subagent.Session{
		{ID: "old", Name: "old", Provider: "fake", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(-time.Hour)},
		{ID: "new", Name: "new", Provider: "fake", WorkingDir: ".", Status: "completed", LastActiveAt: now},
		{ID: "other-dir", Name: "other-dir", Provider: "fake", WorkingDir: "/else", Status: "completed", LastActiveAt: now.Add(time.Hour)},
		{ID: "other-caller", Name: "other-caller", Provider: "fake", WorkingDir: ".", ParentSessionID: "else", Status: "completed", LastActiveAt: now.Add(time.Hour)},
		{ID: "terminal", Name: "terminal", Provider: "terminal", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(time.Hour)},
	}
	for _, sess := range sessions[:2] {
		sess.ParentSessionID = "caller"
	}
	got := attributable(sessions, ".", "caller")
	if len(got) != 2 || got[0].Name != "new" || got[1].Name != "old" {
		t.Fatalf("attributable = %#v", got)
	}
}

func TestAgentResumeAttributionAndContinue(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	now := time.Now()
	for _, sess := range []*subagent.Session{{ID: "one", Name: "one", Provider: "fake", Model: "model", WorkingDir: ".", Status: "completed", ContextTokens: 100, LastActiveAt: now.Add(-time.Hour)}, {ID: "two", Name: "two", Provider: "fake", Model: "model", WorkingDir: ".", Status: "completed", ContextTokens: 100, LastActiveAt: now}} {
		_ = store.Save(sess)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--continue", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if d.resumes != 1 {
		t.Fatalf("resumes=%d", d.resumes)
	}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "one", "--continue", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error=%v", err)
	}
}

func (d *recordingAgentDriver) Run(_ context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	if d.runHook != nil {
		d.runHook()
	}
	d.dir = opts.Dir
	d.prompt = opts.Prompt
	return &subagent.TurnResult{SessionID: "recorded", Response: "ok"}, nil
}

func (d *recordingAgentDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}

func (d *recordingAgentDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "Context compacted.", InputTokens: 1000, ContextTokens: 1000}, nil
}

func (d *recordingAgentDriver) Stop(_ context.Context, id string) error {
	d.stopped = append(d.stopped, id)
	return nil
}

func (d *recordingAgentDriver) Delete(_ context.Context, id string) error {
	d.deleted = append(d.deleted, id)
	return nil
}

func TestAgentResumePrintsReplyNotStructDump(t *testing.T) {
	// The reattach notice is asserted below; an inherited HTO=0 would suppress it.
	t.Setenv(execTimeoutShortEnv, "")
	t.Setenv(execTimeoutEnv, "")
	t.Setenv(execTimeoutExplicitEnv, "")
	// The agent command runs sessionTipHook; isolate HOME and the session
	// env so the caller's own session tips cannot leak into stderr.
	t.Setenv("HOME", t.TempDir())
	for _, name := range resolve.SessionEnvVars {
		t.Setenv(name, "")
	}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &replyDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "{0x") || !strings.Contains(got, "[agent messages]\n[msg 1]\nthe reply text") {
		t.Fatalf("stdout = %q, want plain reply without struct dump", got)
	}
	if e := errOut.String(); !strings.HasPrefix(e, "[session timeline]\n") || !strings.Contains(e, "host background job") || !strings.Contains(e, "Do NOT poll") || !strings.Contains(e, " done] ") {
		t.Fatalf("stderr = %q, want timeline with wait notice", e)
	}
	saved, err := store.Get("sid")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Response != "the reply text" || len(saved.ReplyHistory) != 1 || saved.ReplyHistory[0].Turn != 1 || saved.ReplyHistory[0].Text != "the reply text" || saved.ReplyHistory[0].At.IsZero() {
		t.Fatalf("saved response/history = %q / %#v", saved.Response, saved.ReplyHistory)
	}
}

type replyDriver struct{ recordingAgentDriver }

func (*replyDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "the reply text"}, nil
}

type ackDriver struct{ recordingAgentDriver }

func (*ackDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "real reply", Messages: []string{"Context compacted.", "real reply"}, TokensTurn: 900000, CachedTokens: 890000}, nil
}

func TestAgentResumeCompactsOnceAndSeparatesAck(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &ackDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "claude", Model: "luna", Status: "completed", TokensCumulative: 7000000, TokensSinceCompact: 250000, ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[agent messages]\n[msg 1]\nreal reply\n" {
		t.Fatalf("stdout = %q", got)
	}
	if e := errOut.String(); !strings.Contains(e, " compact] verified /compact") || !strings.Contains(e, "agent acknowledged: Context compacted.") {
		t.Fatalf("stderr = %q", e)
	}
	sess, err := store.Get("sid")
	if err != nil || sess.TokensSinceCompact != 10000 || sess.TokensCumulative != 7900000 {
		t.Fatalf("since=%d cumulative=%d err=%v", sess.TokensSinceCompact, sess.TokensCumulative, err)
	}
	if subagent.ShouldCompact(sess.TokensSinceCompact) {
		t.Fatal("next resume must not compact again")
	}
}

type codexAutoCompactDriver struct {
	recordingAgentDriver
	compactCalls int
	resumeCalls  int
	result       subagent.TurnResult
}

func (d *codexAutoCompactDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	d.compactCalls++
	return nil, errors.New("manual compact must not be called")
}

func (d *codexAutoCompactDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumeCalls++
	r := d.result
	r.Response = "resumed"
	r.TokensTurn = 5
	return &r, nil
}

func TestCodexOverThresholdResumeUsesProviderAutoCompact(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  compact_threshold_tokens: 100000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		result      subagent.TurnResult
		wantErr     bool
		wantContext string
	}{
		{name: "compaction verified", result: subagent.TurnResult{ContextTokens: 50000, CompactionObserved: true}},
		{name: "missing compaction record", result: subagent.TurnResult{ContextTokens: 50000}, wantErr: true, wantContext: "context 50000"},
		{name: "still over limit", result: subagent.TurnResult{ContextTokens: 120000, CompactionObserved: true}, wantErr: true, wantContext: "context 120000"},
		{name: "unreadable rollout", result: subagent.TurnResult{ContextTokens: -1, CompactionObserved: true}, wantErr: true, wantContext: "context unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			dir := t.TempDir()
			st, err := subagent.NewSessionStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 250000}); err != nil {
				t.Fatal(err)
			}
			driver := &codexAutoCompactDriver{result: tc.result}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			_, err = runWithStore(t, driver, dir, "resume", "--name", "worker", "prompt")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if !strings.Contains(err.Error(), "Codex did not auto-compact session sid") || !strings.Contains(err.Error(), tc.wantContext) || !strings.Contains(err.Error(), "start a fresh session with harnez agent start") {
					t.Fatalf("error = %v", err)
				}
				stopped, getErr := st.Get("sid")
				if getErr != nil || stopped.ResumeBlockedReason == "" {
					t.Fatalf("resume block=%q err=%v", stopped.ResumeBlockedReason, getErr)
				}
				if _, nextErr := runWithStore(t, driver, dir, "resume", "--name", "worker", "again"); nextErr == nil || !strings.Contains(nextErr.Error(), "cannot be resumed") {
					t.Fatalf("second resume error = %v", nextErr)
				}
			}
			if driver.compactCalls != 0 || driver.resumeCalls != 1 {
				t.Fatalf("compact calls=%d resume calls=%d", driver.compactCalls, driver.resumeCalls)
			}
		})
	}
}

type unverifiedCompactDriver struct {
	recordingAgentDriver
	resumes int
}

func (d *unverifiedCompactDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "Context compacted.", ContextTokens: 200}, nil
}

func (d *unverifiedCompactDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumes++
	return &subagent.TurnResult{Response: "should not be sent"}, nil
}

func TestRunResumeBlocksPromptWhenCompactionIsUnverified(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  compact_threshold_tokens: 100\n"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, interactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "batch", true: "interactive"}[interactive], func(t *testing.T) {
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			provider := "claude"
			if interactive {
				provider = "codex"
			}
			sess := &subagent.Session{ID: "resume", ProviderSessionID: "provider", Name: "worker", Provider: provider, Model: "model", Status: "completed", ContextTokens: 200}
			if interactive {
				sess.HarnessType, sess.Status = "interactive", "active"
			}
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			driver := &unverifiedCompactDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			deps := agentDeps{
				store:  func() (*subagent.FileSessionStore, error) { return store, nil },
				parent: func() string { return "" },
				find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
					return s.Find(id)
				},
			}
			err = runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "must not be sent", StreamMode: streamStats, JSON: true})
			if interactive {
				want := `session "worker" has 200 context tokens, over the limit 100 (setting agent.compact_threshold_tokens in ~/.harnez/config.yaml); an active interactive session cannot be compacted and verified from outside; next step: resume it non-interactively, or start a fresh session with harnez agent start`
				if err == nil || err.Error() != want {
					t.Fatalf("resume error = %v, want %q", err, want)
				}
			} else if err == nil || !strings.Contains(err.Error(), "refusing to send resume prompt") {
				t.Fatalf("resume error = %v, want unverified-compaction error", err)
			}
			if !interactive && driver.resumes != 0 {
				t.Fatalf("resume prompt calls = %d, want none", driver.resumes)
			}
		})
	}
}

type blockingResumeDriver struct {
	recordingAgentDriver
	started chan struct{}
	release chan struct{}
}

func (d *blockingResumeDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	close(d.started)
	<-d.release
	return &subagent.TurnResult{SessionID: "provider-session", Response: "done", ContextTokens: 100}, nil
}

func TestAgentResumeStatusIsRunningDuringTurn(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	driver := &blockingResumeDriver{started: make(chan struct{}), release: make(chan struct{})}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()

	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(driver.release) }) }
	resumeDone := make(chan error, 1)
	go func() {
		_, err := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "prompt")
		resumeDone <- err
	}()
	waitResume := sync.OnceValue(func() error { return <-resumeDone })
	defer func() {
		release()
		_ = waitResume()
	}()
	select {
	case <-driver.started:
	case <-time.After(2 * time.Second):
		release()
		t.Fatal("provider turn did not start")
	}

	var out bytes.Buffer
	status := newAgentCmd()
	status.SetOut(&out)
	status.SetErr(new(bytes.Buffer))
	status.SetArgs([]string{"status", "--name", "worker", "--json", "--store-dir", storeDir})
	if err := status.Execute(); err != nil {
		t.Fatal(err)
	}
	var got subagent.Session
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" {
		t.Errorf("mid-turn status = %q, want running", got.Status)
	}
	activeRole, activeSession := os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	_ = os.Setenv(agentRoleEnv, "")
	_ = os.Setenv(agentSessionEnv, "")
	_, secondErr := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "conflict")
	_ = os.Setenv(agentRoleEnv, activeRole)
	_ = os.Setenv(agentSessionEnv, activeSession)
	if secondErr == nil || !strings.Contains(secondErr.Error(), "already has a resume turn running") {
		t.Errorf("second resume error = %v, want active-turn refusal", secondErr)
	}
	release()
	if err := waitResume(); err != nil {
		t.Fatal(err)
	}
	finished, err := store.Get("sid")
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.ProcessPID != 0 {
		t.Fatalf("final session state = status %q pid %d, want completed/0", finished.Status, finished.ProcessPID)
	}
}

func TestAgentResumeReclaimsStaleRunningSession(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	deadProcess := exec.Command("true")
	if err := deadProcess.Start(); err != nil {
		t.Fatal(err)
	}
	deadPID := deadProcess.Process.Pid
	if err := deadProcess.Wait(); err != nil {
		t.Fatalf("wait for dead PID fixture: %v", err)
	}
	if processExists(deadPID) {
		t.Fatalf("test fixture PID %d unexpectedly exists", deadPID)
	}
	if err := store.Save(&subagent.Session{ID: "stale", Name: "worker", Provider: "claude", Model: "haiku", Tier: "low", Status: "running", ProcessPID: deadPID, ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "prompt"); err != nil {
		t.Fatalf("resume stale session: %v", err)
	}
	finished, err := store.Get("stale")
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.ProcessPID != 0 || finished.LastError != "" {
		t.Fatalf("reclaimed session = status %q pid %d error %q", finished.Status, finished.ProcessPID, finished.LastError)
	}
}

func TestAgentResumeWithoutNameResolution(t *testing.T) {
	now := time.Now()
	older := &subagent.Session{ID: "id-old", Name: "old", LastActiveAt: now.Add(-time.Hour)}
	newer := &subagent.Session{ID: "id-new", Name: "new", LastActiveAt: now}
	other := &subagent.Session{ID: "id-else", Name: "elsewhere", WorkingDir: "/somewhere/else", LastActiveAt: now}
	for _, tc := range []struct {
		name     string
		sessions []*subagent.Session
		args     []string
		wantID   string
		header   string
		wantErr  []string
	}{
		{"unique in dir", []*subagent.Session{older, other}, []string{"resume", "go"}, "id-old", "resolved=dir]", nil},
		{"unique with -c", []*subagent.Session{older}, []string{"resume", "-c", "go"}, "id-old", "resolved=continue]", nil},
		{"-c picks most recent", []*subagent.Session{older, newer, other}, []string{"resume", "-c", "go"}, "id-new", "resolved=continue]", nil},
		{"latest in directory", []*subagent.Session{older, newer}, []string{"resume", "go"}, "id-new", "resolved=dir]", nil},
		{"fuzzy name", []*subagent.Session{{ID: "id-classify", Name: "neus-classify-dev", LastActiveAt: now}}, []string{"resume", "classify", "again"}, "id-classify", "resolved=id]", nil},
		{"id prefix", []*subagent.Session{{ID: "id-prefix-123456", Name: "prefixed", LastActiveAt: now}}, []string{"resume", "id-prefix", "again"}, "id-prefix-123456", "resolved=id]", nil},
		{"ambiguous fuzzy words remain prompt", []*subagent.Session{{ID: "id-a", Name: "neus-classify-dev", LastActiveAt: now}, {ID: "id-b", Name: "classify-api", LastActiveAt: now}}, []string{"resume", "classify", "again"}, "id-b", "resolved=dir]", nil},
		{"none", []*subagent.Session{other}, []string{"resume", "go"}, "", "", []string{"no resumable agent in .", "harnez agent start --name"}},
		{"none with -c", nil, []string{"resume", "-c", "go"}, "", "", []string{"no resumable agent in ."}},
		{"by name", []*subagent.Session{older, newer}, []string{"resume", "--name", "old", "go"}, "id-old", "resolved=name]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			saveSessions(t, storeDir, tc.sessions...)
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			out, err := runWithStore(t, d, storeDir, tc.args...)
			if tc.wantErr != nil {
				for _, w := range tc.wantErr {
					if err == nil || !strings.Contains(err.Error(), w) {
						t.Fatalf("err = %v, want %q", err, w)
					}
				}
				if len(d.resumed) != 0 {
					t.Fatalf("provider resumed despite error: %v", d.resumed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(d.resumed) != 1 || d.resumed[0] != tc.wantID || !strings.Contains(out, tc.header) {
				t.Fatalf("resumed=%v want %s; header %q missing in:\n%s", d.resumed, tc.wantID, tc.header, out)
			}
		})
	}
}

func TestAgentStartFailureKeepsSessionAndResumeSelector(t *testing.T) {
	old := agentDriver
	d := &failedStartStreamDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "other-1", Name: "other-one", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed"},
		{ID: "other-2", Name: "other-two", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
	err = runStart(t.Context(), cmd, deps, startRequest{Name: "crashed", Role: "advisor", Prompt: "do work", StoredPrompt: "do work", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "temporary provider failure") {
		t.Fatalf("start error = %v, want provider failure", err)
	}
	startErr := err
	sessions, err := store.List("", true)
	if err != nil {
		t.Fatal(err)
	}
	var recovered *subagent.Session
	for _, sess := range sessions {
		if sess.Name == "crashed" {
			recovered = sess
		}
	}
	if recovered == nil || recovered.ProviderSessionID != "provider-thread" || recovered.Status == "completed" || !strings.Contains(recovered.LastError, "temporary provider failure") {
		t.Fatalf("failed session record = %#v", recovered)
	}
	if strings.Contains(startErr.Error(), "raw generated source") || strings.Contains(startErr.Error(), "traceback line 2") {
		t.Fatalf("failure exposed raw tool output: %v", startErr)
	}

	for _, selector := range []string{"crashed", recovered.ID[:8], "provider-thread"} {
		t.Run("resume-"+selector, func(t *testing.T) {
			resume := newAgentCmd()
			resume.SetOut(new(bytes.Buffer))
			resume.SetErr(new(bytes.Buffer))
			resume.SetArgs([]string{"--store-dir", storeDir, "resume", selector, "continue work"})
			if err := resume.Execute(); err != nil {
				t.Fatalf("resume by %q: %v", selector, err)
			}
		})
	}
	implicit := newAgentCmd()
	implicit.SetOut(new(bytes.Buffer))
	implicit.SetErr(new(bytes.Buffer))
	implicit.SetArgs([]string{"--store-dir", storeDir, "resume", "ordinary prompt"})
	if err := implicit.Execute(); err != nil {
		t.Fatalf("implicit latest-session resume: %v", err)
	}
}

func TestAgentResumeHexLikeWordsRemainPrompts(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	for _, tc := range []struct {
		prompt string
		ids    []string
	}{
		{prompt: "add", ids: []string{"add12345"}},
		{prompt: "c", ids: []string{"cafe1234"}},
		{prompt: "deface", ids: []string{"deface1234", "deface5678"}},
	} {
		t.Run(tc.prompt, func(t *testing.T) {
			storeDir := t.TempDir()
			sessions := make([]*subagent.Session, 0, len(tc.ids))
			for i, id := range tc.ids {
				sessions = append(sessions, &subagent.Session{ID: id, Name: "worker-" + id, Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed", LastActiveAt: time.Now().Add(time.Duration(i) * time.Second)})
			}
			saveSessions(t, storeDir, sessions...)
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			if _, err := runWithStore(t, d, storeDir, "resume", "--continue", tc.prompt); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(d.prompt, tc.prompt) {
				t.Fatalf("resume prompt = %q, want original prompt suffix %q", d.prompt, tc.prompt)
			}
		})
	}
}

type failedStartStreamDriver struct{ recordingAgentDriver }

func (d *failedStartStreamDriver) Run(ctx context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, errors.New("Run path used")
}

func (d *failedStartStreamDriver) RunStream(_ context.Context, _ subagent.RunOptions, emit subagent.EventFunc) (*subagent.TurnResult, error) {
	emit(subagent.Event{Kind: "session", Text: "provider-thread"})
	return nil, errors.New("temporary provider failure\nraw generated source\ntraceback line 2")
}

func (d *failedStartStreamDriver) ResumeStream(ctx context.Context, id, prompt string, model subagent.Model, emit subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.Resume(ctx, id, prompt, model)
}

func (d *failedStartStreamDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{SessionID: "provider-thread", Response: "resumed", Messages: []string{"resumed"}, ContextTokens: 100}, nil
}

func TestRunResumeRecordsFreshQuotaPairAndAdvancesTurn(t *testing.T) {
	storeDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "resume-1", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", HarnessType: "harnez", Status: "completed", Turn: 4, ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var calls []bool
	capture := func(_ context.Context, _ string, force bool) usage.TurnQuotaReading {
		calls = append(calls, force)
		return usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), HasCache: true, CacheAgeMS: 10}
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, quota: capture, dbPath: dbPath, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return s.Find(id)
	}}
	if err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", JSON: true, StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []bool{false, false}) {
		t.Fatalf("quota capture force flags=%v, want [false false]", calls)
	}
	sess, err := store.Get("resume-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Turn != 5 {
		t.Fatalf("persisted turn=%d, want 5", sess.Turn)
	}
	usageStore, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer usageStore.Close()
	var boundaries int
	if err := usageStore.QueryRow(context.Background(), `SELECT count(*) FROM turn_quota_boundaries WHERE session_id=? AND turn=5`, "resume-1").Scan(&boundaries); err != nil {
		t.Fatal(err)
	}
	if boundaries != 2 {
		t.Fatalf("stored boundary count=%d, want 2", boundaries)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "quota-readings.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("legacy quota spool exists after resume: stat err=%v", err)
	}
}

func TestRunResumeWithoutCobraFlags(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "direct-resume", Name: "direct", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	driver := &scriptDriver{steps: []step{{ev: msg("CONFIRM: resumed")}, {ev: msg("done")}}}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return resolveSession(s, id, "")
	}}
	if err := runResume(t.Context(), cmd, deps, resumeRequest{Prompt: "continue", Name: "direct", Dir: ".", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "resolved=name") {
		t.Fatalf("stream header missing: %s", out.String())
	}
	if len(driver.resumed) != 1 || driver.resumed[0] != "direct-resume" {
		t.Fatalf("resumed = %v", driver.resumed)
	}
}

func TestRunResumeRejectsSessionWithRemovedModel(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "retired-session", Name: "retired", Provider: "codex", Model: "gpt-5.6-terra", Tier: "low", WorkingDir: ".", HarnessType: "harnez", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	called := false
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver {
		called = true
		return &recordingAgentDriver{}
	}
	defer func() { agentDriver = old }()
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return s.Find(id)
	}}
	err = runResume(t.Context(), &cobra.Command{}, deps, resumeRequest{Name: "retired", Prompt: "continue", StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "uses unavailable model") || !strings.Contains(err.Error(), "known specs") {
		t.Fatalf("resume error = %v, want clear unavailable-model error", err)
	}
	if called {
		t.Fatal("resume constructed a provider driver for a removed model")
	}
}

func TestRunResumeAcceptsCurrentCodexModels(t *testing.T) {
	for _, tc := range []struct{ model, tier string }{{"gpt-6-luna", "med"}, {"gpt-6.1-sol", "low"}} {
		t.Run(tc.model+":"+tc.tier, func(t *testing.T) {
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(&subagent.Session{ID: "current-session", Name: "current", Provider: "codex", Model: tc.model, Tier: tc.tier, WorkingDir: ".", HarnessType: "harnez", Status: "completed", ContextTokens: 100}); err != nil {
				t.Fatal(err)
			}
			driver := &resumeOutcomeDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, dbPath: filepath.Join(t.TempDir(), "telemetry.sqlite"), find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
				return s.Find(id)
			}}
			if err := runResume(t.Context(), &cobra.Command{}, deps, resumeRequest{Name: "current", Prompt: "continue", StreamMode: streamFull}); err != nil {
				t.Fatal(err)
			}
			if driver.resumes != 1 {
				t.Fatalf("resume calls=%d, want 1", driver.resumes)
			}
		})
	}
}

func TestRunResumeWarnsAndContinuesWhenPolicyDisablesModel(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "current-session", Name: "current", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", HarnessType: "harnez", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	driver := &resumeOutcomeDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		modelEnabled: func(string, string, string) (bool, error) { return false, nil },
		dbPath:       filepath.Join(t.TempDir(), "telemetry.sqlite"),
	}
	if err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "current", Prompt: "continue", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	if driver.resumes != 1 || !strings.Contains(stderr.String(), "disabled by current policy") {
		t.Fatalf("resume calls=%d stderr=%q", driver.resumes, stderr.String())
	}
}

func TestRunResumePassesStoredAgyModelAndTier(t *testing.T) {
	workDir := t.TempDir()
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "agy-session", Name: "worker", Provider: "agy", Model: "flash37", Tier: "med", WorkingDir: workDir, Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	var args []string
	old := agentDriver
	agentDriver = func(_ subagent.Model, dir string) subagent.Driver {
		return subagent.AgyDriver{Dir: dir, Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
			args = gotArgs
			return []byte(`{"conversation_id":"agy-session","status":"SUCCESS","response":"done"}`), nil
		}}
	}
	defer func() { agentDriver = old }()
	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return s.Find(id)
	}}
	if err := runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	want := []string{"--conversation", "agy-session", "--add-dir", workDir, "--model", "flash37", "--effort", "medium", "--output-format", "json", "-p", "continue"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("agy resume args = %#v, want %#v", args, want)
	}
}
