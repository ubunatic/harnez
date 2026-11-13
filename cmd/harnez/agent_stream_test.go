package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

func TestResumeNoteRequiresIdleSessionAboveSpecThreshold(t *testing.T) {
	threshold, note, err := subagent.ResumeNote()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		idle time.Duration
		want bool
	}{
		{name: "new session", idle: 0},
		{name: "at threshold", idle: threshold},
		{name: "above threshold", idle: threshold + time.Nanosecond, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := withResumeNote("prompt", tc.idle)
			wantCount := 0
			if tc.want {
				wantCount = 1
			}
			if count := strings.Count(got, note); count != wantCount {
				t.Fatalf("note count = %d, want %d in %q", count, wantCount, got)
			}
		})
	}
	if strings.Contains(withProtocol("prompt", false, "developer", false, ""), note) {
		t.Fatal("new session protocol contains a resume note")
	}
	codexPre, err := subagent.FamilyPreamble("codex")
	if err != nil || codexPre == "" {
		t.Fatalf("FamilyPreamble(codex) = %q, %v", codexPre, err)
	}
	if start := withProtocol("prompt", false, "developer", false, "codex"); !strings.Contains(start, codexPre) {
		t.Fatalf("start protocol missing codex family preamble: %q", start)
	}
	if resumed := withProtocol("prompt", false, "developer", false, ""); strings.Contains(resumed, codexPre) {
		t.Fatalf("resume protocol repeated family preamble: %q", resumed)
	}
}

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
	skipShort(t, "waits for a heartbeat in the real stream watchdog")
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--role", "advisor", "--model", "codex:luna", "prompt", "--name", "w", "--store-dir", t.TempDir()})
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
	skipShort(t, "waits for a heartbeat in the real stream watchdog")
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--role", "advisor", "--model", "codex:luna", "prompt", "--stream", "stats", "--store-dir", t.TempDir()})
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
	cmd.SetArgs([]string{"start", "--role", "advisor", "--model", "codex:luna", "prompt", "--stream", "loud", "--store-dir", t.TempDir()})
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
	if got, want := tokenSummary(r, nil), "tokens: turn 33.2k new (1200 out), 1.2M cached; session 33.2k new, 1.2M cached"; got != want {
		t.Fatalf("tokenSummary = %q, want %q", got, want)
	}
	sess := &subagent.Session{TokenTotalsKnown: true, TokensCumulative: 55000000, CachedTokensTotal: 47100000}
	if got, want := tokenSummary(r, sess), "tokens: turn 33.2k new (1200 out), 1.2M cached; session 7.9M new, 47.1M cached"; got != want {
		t.Fatalf("tokenSummary(sess) = %q, want %q", got, want)
	}
	unknownTurn := &subagent.TurnResult{TurnTokensUnknown: true}
	if got, want := tokenSummary(unknownTurn, sess), "tokens: turn unknown; session 7.9M new, 47.1M cached"; got != want {
		t.Fatalf("tokenSummary(unknown turn) = %q, want %q", got, want)
	}
	if got, want := tokenSummary(unknownTurn, nil), "tokens: turn unknown; session unknown"; got != want {
		t.Fatalf("tokenSummary(unknown both) = %q, want %q", got, want)
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

func TestStatsModeRepeatsAnAlreadyShownFinalMessage(t *testing.T) {
	for _, model := range []string{"codex:luna", "claude:sonnet"} {
		t.Run(model, func(t *testing.T) {
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will do it")}}}
			got := zeroElapsed(runScripted(t, d, "start", "--model", model, "task", "--name", "w", "--stream", "stats"))
			if !strings.Contains(got, "[reply: 0s]\nCONFIRM: will do it\n") {
				t.Fatalf("missing reply marker:\n%s", got)
			}
		})
	}
}

type batchReplyDriver struct{ recordingAgentDriver }

func (*batchReplyDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "batch reply"}, nil
}

func TestAgyStatsStartAndResume(t *testing.T) {
	d := &batchReplyDriver{}
	storeDir := t.TempDir()
	got, err := runWithStore(t, d, storeDir, "start", "--role", "advisor", "--model", "agy:argon:low", "--name", "worker", "--stream", "stats", "task")
	if err != nil || !strings.Contains(zeroElapsed(got), "[reply: 0s]\nok\n") {
		t.Fatalf("start: err=%v output=%s", err, got)
	}
	got, err = runWithStore(t, d, storeDir, "resume", "--name", "worker", "--stream", "stats", "task")
	if err != nil || !strings.Contains(zeroElapsed(got), "[reply: 0s]\nbatch reply\n") {
		t.Fatalf("resume: err=%v output=%s", err, got)
	}
}

func TestTurnStreamReturnedReply(t *testing.T) {
	for _, tc := range []struct {
		name, mode, event, response string
		wantReply                   bool
	}{
		{"codex stats confirmation", streamStats, "CONFIRM: hello", "CONFIRM: hello", true},
		{"claude stats plan", streamStats, "PLAN: hello", "PLAN: hello", true},
		{"full same", streamFull, "hello", "hello", false},
		{"full different", streamFull, "draft", "final", true},
		{"full absent", streamFull, "", "final", true},
		{"full transformed confirmation", streamFull, "CONFIRM: hello", "CONFIRM: hello", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := newAgentCmd()
			cmd.SetOut(&out)
			ts := newTurnStream(cmd, tc.mode, false)
			if tc.event != "" {
				ts.onEvent(msg(tc.event))
			}
			ts.finish(&subagent.TurnResult{Response: tc.response})
			got := zeroElapsed(out.String())
			marker := "[reply: 0s]\n" + tc.response + "\n"
			if strings.Contains(got, marker) != tc.wantReply {
				t.Fatalf("reply marker presence = %v, want %v:\n%s", strings.Contains(got, marker), tc.wantReply, got)
			}
			if tc.name == "full same" && strings.Count(got, tc.response) != 1 {
				t.Fatalf("expected reply exactly once:\n%s", got)
			}
		})
	}
}

func TestZeroElapsed(t *testing.T) {
	in := "[heartbeat: 1s, ~70 tokens]\n[reply: 1m5s]\nall done\n[session info: id=sid]\n[done: 2 messages]"
	want := "[heartbeat: 0s, ~70 tokens]\n[reply: 0s]\nall done\n[session info: id=sid]\n[done: 2 messages]"
	if got := zeroElapsed(in); got != want {
		t.Fatalf("zeroElapsed = %q, want %q", got, want)
	}
}

type doneSummaryDriver struct {
	recordingAgentDriver
	res *subagent.TurnResult
}

func (d *doneSummaryDriver) Run(_ context.Context, _ subagent.RunOptions) (*subagent.TurnResult, error) {
	return d.res, nil
}
func (d *doneSummaryDriver) RunStream(_ context.Context, _ subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	fn(subagent.Event{Kind: "session", Text: d.res.SessionID})
	fn(subagent.Event{Kind: "message", Text: d.res.Response})
	return d.res, nil
}
func (d *doneSummaryDriver) ResumeStream(_ context.Context, _, _ string, _ subagent.Model, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	fn(subagent.Event{Kind: "message", Text: d.res.Response})
	return d.res, nil
}

func TestDoneLineContextUsageCostCheckAndQuotaUnknown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}

	// Turn 1 (cumulative provider): defaults must not trigger [check:], quota unknown note appears.
	drv := &doneSummaryDriver{res: &subagent.TurnResult{
		SessionID: "cum-1", Response: "ok1", Messages: []string{"ok1"},
		InputTokens: 20000, CachedTokens: 10000, OutputTokens: 1000, TokensTurn: 21000, TokensCumulative: 21000,
		CumulativeTotalsKnown: true, CumulativeInputTokens: 20000, CumulativeCachedTokens: 10000, CumulativeOutputTokens: 1000,
		ContextTokens: 20000, ContextWindowSize: 400000,
	}}
	oldDrv := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return drv }
	defer func() { agentDriver = oldDrv }()

	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--role", "developer", "--model", "codex:luna", "--name", "w985", "--store-dir", storeDir, "step 1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got1 := out.String()
	for _, want := range []string{
		"tokens: turn 11.0k new (1000 out), 10.0k cached; session 11.0k new, 10.0k cached",
		"context usage 20.0k of 400.0k window, compaction limit 100.0k",
		"quota unknown: cost check is the only signal",
	} {
		if !strings.Contains(got1, want) {
			t.Fatalf("turn 1 missing %q in:\n%s", want, got1)
		}
	}
	if strings.Contains(got1, "[check:") {
		t.Fatalf("short session unexpectedly emitted check:\n%s", got1)
	}

	// Lower thresholds in ~/.harnez/config.yaml; resume cumulative turn triggers [check:] naming role and model.
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  cost_check:\n    cached_to_new_ratio: 5\n    min_turns: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	drv.res = &subagent.TurnResult{
		SessionID: "cum-1", Response: "ok2", Messages: []string{"ok2"},
		InputTokens: 230000, CachedTokens: 210000, OutputTokens: 2000, TokensTurn: 232000, TokensCumulative: 232000,
		CumulativeTotalsKnown: true, CumulativeInputTokens: 230000, CumulativeCachedTokens: 210000, CumulativeOutputTokens: 2000,
		ContextTokens: 80000, ContextWindowSize: 400000,
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "w985", "--store-dir", storeDir, "step 2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got2 := out.String()
	for _, want := range []string{
		"tokens: turn 11.0k new (1000 out), 200.0k cached; session 22.0k new, 210.0k cached",
		"context usage 80.0k of 400.0k window, compaction limit 100.0k",
		"[check: turn re-read 200.0k cached for 11.0k new (18x); context usage 80.0k; fresh start ~20.0k; consider: harnez agent start --role developer --model codex:",
	} {
		if !strings.Contains(got2, want) {
			t.Fatalf("turn 2 missing %q in:\n%s", want, got2)
		}
	}

	// Compaction drop (cumulative counters smaller than stored totals): must print turn unknown, no negative delta.
	drv.res = &subagent.TurnResult{
		SessionID: "cum-1", Response: "ok3", Messages: []string{"ok3"},
		InputTokens: 50000, CachedTokens: 40000, OutputTokens: 500, TokensTurn: 50500, TokensCumulative: 50500,
		CumulativeTotalsKnown: true, CumulativeInputTokens: 50000, CumulativeCachedTokens: 40000, CumulativeOutputTokens: 500,
		ContextTokens: 15000, ContextWindowSize: 400000,
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "w985", "--store-dir", storeDir, "step 3"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got3 := out.String()
	if !strings.Contains(got3, "tokens: turn unknown; session 10.5k new, 40.0k cached") {
		t.Fatalf("compaction drop missing turn unknown in:\n%s", got3)
	}

	// agy-like unknown context & missing usage: no false ratio check.
	if err := store.Save(&subagent.Session{ID: "agy-1", Name: "agy985", Provider: "agy", Model: "gemini-4-argon", Role: "developer", WorkingDir: t.TempDir(), Status: "completed", HarnessType: "harnez", Turn: 3}); err != nil {
		t.Fatal(err)
	}
	drv.res = &subagent.TurnResult{SessionID: "agy-1", Response: "agy ok", Messages: []string{"agy ok"}, ContextTokens: -1, TurnTokensUnknown: true}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "agy985", "--store-dir", storeDir, "step 4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got4 := out.String()
	if !strings.Contains(got4, "context usage unknown") || !strings.Contains(got4, "turn unknown") || strings.Contains(got4, "[check:") {
		t.Fatalf("agy output invalid:\n%s", got4)
	}

	// agent list headers use CONTEXT_USAGE and CONTEXT_WINDOW.
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CONTEXT_USAGE\tCONTEXT_WINDOW") {
		t.Fatalf("agent list missing renamed headers:\n%s", out.String())
	}
}
