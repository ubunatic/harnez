package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

// step is one scripted event, emitted after wait.
type step struct {
	wait time.Duration
	ev   subagent.Event
}

type scriptDriver struct {
	recordingAgentDriver
	steps   []step
	prompt  string
	sid     string   // session id returned by a turn; "thread-1" when empty
	resumed []string // provider ids passed to ResumeStream
	// environment the provider process would inherit during the turn
	envRole, envSession string
}

func (d *scriptDriver) play(fn subagent.EventFunc) *subagent.TurnResult {
	var msgs []string
	for _, s := range d.steps {
		time.Sleep(s.wait)
		fn(s.ev)
		if s.ev.Kind == "message" {
			msgs = append(msgs, s.ev.Text)
		}
	}
	sid := d.sid
	if sid == "" {
		sid = "thread-1"
	}
	return &subagent.TurnResult{SessionID: sid, Response: msgs[len(msgs)-1], Messages: msgs, ContextTokens: 100}
}

func (d *scriptDriver) RunStream(_ context.Context, o subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	d.prompt = o.Prompt
	d.envRole, d.envSession = os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	return d.play(fn), nil
}

func (d *scriptDriver) ResumeStream(_ context.Context, id, p string, _ subagent.Model, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	d.prompt = p
	d.envRole, d.envSession = os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	d.resumed = append(d.resumed, id)
	return d.play(fn), nil
}

func runScripted(t *testing.T, d *scriptDriver, args ...string) string {
	t.Helper()
	old, oldTimeout := agentDriver, confirmTimeout
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	confirmTimeout = 30 * time.Millisecond
	defer func() { agentDriver, confirmTimeout = old, oldTimeout }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append(args, "--store-dir", t.TempDir()))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func msg(text string) subagent.Event { return subagent.Event{Kind: "message", Text: text, Bytes: 10} }

func tool(text string) subagent.Event { return subagent.Event{Kind: "activity", Text: text, Bytes: 10} }

func TestConfirmationAndPlanShowInStatsMode(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will count files")}, {0, tool("running ls")}, {0, msg("PLAN: ls then sort")}, {0, msg("interim chatter")}, {0, msg("9 files")}}}
	got := zeroElapsed(runScripted(t, d, "start", "--model", "codex:luna", "orig task", "--name", "w", "--stream", "stats"))
	for _, want := range []string{"[confirmation: 0s]\nwill count files\n", "[plan: 0s]\nls then sort\n", "[reply: 0s]\n9 files\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "interim chatter") || strings.Contains(got, "violation") || strings.Contains(got, "warning") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestToolBeforeConfirmationIsAViolation(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, tool("running rg foo")}, {0, msg("CONFIRM: late")}, {0, msg("done")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	want := "[violation: running rg foo before any confirmation; caller may stop the agent: harnez agent stop w]"
	if !strings.Contains(got, want) || strings.Count(got, "[violation:") != 1 {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestSilentAgentGetsWarning(t *testing.T) {
	d := &scriptDriver{steps: []step{{100 * time.Millisecond, msg("late reply")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	if !strings.Contains(got, "[warning: no confirmation after 0s; caller may stop the agent: harnez agent stop w]") || strings.Count(got, "[warning:") != 1 {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestPromptGetsProtocolButStoredPromptStaysOriginal(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	storeDir := t.TempDir()
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "orig task", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.prompt, "Harnez dispatch protocol:") || !strings.HasSuffix(d.prompt, "\n\norig task") || !strings.Contains(d.prompt, "CONFIRM:") {
		t.Fatalf("driver prompt = %q", d.prompt)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("thread-1")
	if err != nil || sess.StartPrompt != "orig task" {
		t.Fatalf("stored prompt = %q, err=%v", sess.StartPrompt, err)
	}
}

func TestPlanFirstAddsGateAndPrompt(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will plan")}, {0, msg("PLAN: 1) read 2) count")}}}
	got := zeroElapsed(runScripted(t, d, "start", "--model", "codex:luna", "count files", "--name", "w", "--plan", "yes"))
	if !strings.Contains(d.prompt, "plan-first turn") || !strings.HasSuffix(d.prompt, "\n\ncount files") {
		t.Fatalf("driver prompt = %q", d.prompt)
	}
	for _, want := range []string{"[plan: 0s]\n1) read 2) count\n", `[gate: plan-first turn ended, nothing was executed; to proceed: harnez agent resume w "go ahead"]`, "[done: 2 messages, tokens:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "warning") {
		t.Fatalf("unexpected warning:\n%s", got)
	}
}

func TestPlanFirstWithoutPlanWarns(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("I just did it")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w", "--plan", "yes")
	if !strings.Contains(got, "[warning: plan-first turn ended without a PLAN: message; the last message was: I just did it]") {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestAgentPlanFlagValidationAndCompatibility(t *testing.T) {
	if _, err := runWithStore(t, &scriptDriver{steps: []step{{ev: msg("ok")}}}, t.TempDir(), "start", "--model", "luna", "--plan", "maybe", "task"); err == nil || err.Error() != `invalid --plan "maybe": want "yes" or "no"` {
		t.Fatalf("invalid plan error = %v", err)
	}
	if _, err := runWithStore(t, &scriptDriver{steps: []step{{ev: msg("ok")}}}, t.TempDir(), "start", "--model", "luna", "--plan-first", "task"); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("old flag error = %v", err)
	}
}

func TestAgentPlanNoEqualsDefault(t *testing.T) {
	for _, args := range [][]string{{"task"}, {"--plan", "no", "task"}} {
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		if _, err := runWithStore(t, d, t.TempDir(), append([]string{"start", "--model", "luna"}, args...)...); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(d.prompt, "plan-first") {
			t.Fatalf("unexpected plan preamble: %q", d.prompt)
		}
	}
}

func TestAgentRootPlanYes(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("CONFIRM: ok")}, {ev: msg("PLAN: wait")}}}
	out, err := runWithStore(t, d, t.TempDir(), "--plan", "yes", "-p", "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.prompt, "plan-first turn") || !strings.Contains(out, "[gate: plan-first turn ended") {
		t.Fatalf("plan output=%q prompt=%q", out, d.prompt)
	}
}

func TestAgentHelpUsesNewForms(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--name") || strings.Contains(out.String(), "start codex:") || strings.Contains(out.String(), "resume <session>") {
		t.Fatalf("root help=%q", out.String())
	}
	start, _, err := newAgentCmd().Find([]string{"start"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(start.Example, "--name") || strings.Contains(start.Example, "start codex:") || strings.Contains(start.Example, "resume <session>") {
		t.Fatalf("start example=%q", start.Example)
	}
}

func TestNormalTurnHasNoPlanFirstText(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	if strings.Contains(d.prompt, "plan-first") || strings.Contains(got, "[gate:") {
		t.Fatalf("prompt=%q\nstdout:\n%s", d.prompt, got)
	}
}
