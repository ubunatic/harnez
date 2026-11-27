package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

// meterRecord is the subset of a turn record the metering tests check. It is
// read through JSON so the tests fail on behaviour, not on a missing field.
type meterRecord struct {
	Turn              int `json:"turn"`
	NewInputTokens    int `json:"new_input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	PeakContextTokens int `json:"peak_context_tokens"`
}

func meterRecords(t *testing.T, sess *subagent.Session) []meterRecord {
	t.Helper()
	b, err := json.Marshal(sess)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		TurnRecords []meterRecord `json:"turn_records"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	return got.TurnRecords
}

// decodeOneJSON requires data to hold exactly one JSON document and nothing else.
func decodeOneJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, data)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout has content after the JSON document: %q", data)
	}
	return doc
}

// countingDeps wraps toolLimitTestDeps so the test can see how often the
// turn quota is read (before and after a turn).
func countingDeps(store *subagent.FileSessionStore, calls *atomic.Int32) agentDeps {
	deps := toolLimitTestDeps(store)
	deps.quota = func(context.Context, string, bool) usage.TurnQuotaReading {
		calls.Add(1)
		return usage.TurnQuotaReading{}
	}
	return deps
}

// live tiny-turn numbers at 134000 prompt tokens: new input 2+4 minus the cached
// part that covers the rest, output 4 (see claudeLiveAssistant).
var liveTurn134k = meterRecord{Turn: 1, NewInputTokens: 6, CachedInputTokens: 133998, OutputTokens: 4, PeakContextTokens: 134000}

func TestClaudeMeterStoppedStartTurn(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var quotaCalls atomic.Int32
	err = runStart(context.Background(), claudeLiveCmd(), countingDeps(store, &quotaCalls), startRequest{
		Name: "claude-live", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
		ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull,
	})
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold at 134000 tokens") {
		t.Fatalf("runStart error = %v, want a watchdog stop", err)
	}
	sess, err := resolveSession(store, "claude-live", "")
	if err != nil {
		t.Fatal(err)
	}
	recs := meterRecords(t, sess)
	if len(recs) != 1 || recs[0] != liveTurn134k {
		t.Fatalf("turn records = %+v, want one record %+v", recs, liveTurn134k)
	}
	if n := quotaCalls.Load(); n != 2 {
		t.Fatalf("turn quota read %d times on a stopped turn, want exactly 2 (before and after)", n)
	}
	assertStoppedTurnTotals(t, sess)
}

// assertStoppedTurnTotals checks the session totals a stopped 134k turn leaves:
// the calls it finished, metered once (ticket 1264 M3).
func assertStoppedTurnTotals(t *testing.T, sess *subagent.Session) {
	t.Helper()
	if !sess.TokenTotalsKnown || sess.InputTokensTotal != 2 || sess.OutputTokensTotal != 4 || sess.CachedTokensTotal != 133998 || sess.CacheWriteTokensTotal != 4694 {
		t.Fatalf("totals input=%d output=%d cached=%d cache_write=%d known=%v, want 2, 4, 133998, 4694, true",
			sess.InputTokensTotal, sess.OutputTokensTotal, sess.CachedTokensTotal, sess.CacheWriteTokensTotal, sess.TokenTotalsKnown)
	}
	if sess.TokensCumulative != 134004 {
		t.Fatalf("TokensCumulative = %d, want 134004 (input + output + cached)", sess.TokensCumulative)
	}
	if sess.TokensSinceCompact != 6 {
		t.Fatalf("TokensSinceCompact = %d, want 6 (the turn's tokens that were not cache reads)", sess.TokensSinceCompact)
	}
}

func TestClaudeMeterStoppedResumeTurn(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-live-meter", Name: "claude-live-meter", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-live", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	var quotaCalls atomic.Int32
	err = runResume(context.Background(), claudeLiveCmd(), countingDeps(store, &quotaCalls), resumeRequest{Name: "claude-live-meter", Prompt: "continue", StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold at 134000 tokens") {
		t.Fatalf("runResume error = %v, want a watchdog stop", err)
	}
	sess, err := resolveSession(store, "claude-live-meter", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Turn != 2 {
		t.Fatalf("sess.Turn = %d, want 2", sess.Turn)
	}
	want := liveTurn134k
	want.Turn = 2
	recs := meterRecords(t, sess)
	if len(recs) != 1 || recs[0] != want {
		t.Fatalf("turn records = %+v, want one record %+v", recs, want)
	}
	if n := quotaCalls.Load(); n != 2 {
		t.Fatalf("turn quota read %d times on a stopped resume, want exactly 2 (before and after)", n)
	}
	assertStoppedTurnTotals(t, sess)
}

// A resume after a watchdog stop compacts before it sends the new prompt. The
// fake claude logs each argv and answers /compact and the prompt differently.
func TestClaudeResumeCompactsBeforePromptAfterStop(t *testing.T) {
	dir := t.TempDir()
	compactPath := filepath.Join(dir, "compact.jsonl")
	mainPath := filepath.Join(dir, "main.jsonl")
	logPath := filepath.Join(dir, "argv.log")
	compactBoundary := `{"type":"system","subtype":"compact_boundary","session_id":"sess-live","compact_metadata":{"pre_tokens":134000,"post_tokens":1000}}`
	for path, stream := range map[string]string{
		compactPath: claudeLiveStream(claudeLiveInit(), compactBoundary, claudeLiveAssistant("msg_c", 1000, "null"), claudeLiveResult(1000)),
		mainPath:    claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_m", 5000, "null"), claudeLiveResult(5000)),
	} {
		if err := os.WriteFile(path, []byte(stream), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The prompt argv can span lines (the protocol preamble), so each call is logged on one line.
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$(printf '%%s' \"$*\" | tr '\\n' ' ')\" >> %s\ncase \"$*\" in\n  *\"/compact\"*) cat %s ;;\n  *) cat %s ;;\nesac\n", logPath, compactPath, mainPath)
	setupFakeProviderEnv(t, map[string]string{"claude": script})

	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-live-order", Name: "claude-live-order", Provider: "claude", Model: "haiku", Tier: "low", Status: "stopped", ProviderSessionID: "sess-live", ContextTokens: 134000, WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	if err := runResume(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), resumeRequest{Name: "claude-live-order", Prompt: "continue", StreamMode: streamFull}); err != nil {
		t.Fatalf("runResume: %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "/compact") || !strings.Contains(lines[1], "continue") {
		t.Fatalf("argv log = %q, want /compact first and the prompt second", logData)
	}
	sess, err := resolveSession(store, "claude-live-order", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "completed" {
		t.Fatalf("status = %q, want completed", sess.Status)
	}
}

func TestClaudeWatchdogJSONStartStopsWithOneDocument(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cmd := claudeLiveCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
		Name: "claude-live", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
		ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull, JSON: true,
	})
	want := "crossed live token threshold at 134000 tokens (limit 100000)"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("runStart error = %v, want %q", err, want)
	}
	doc := decodeOneJSON(t, stdout.Bytes())
	if doc["status"] != "stopped" {
		t.Fatalf("status = %v, want stopped", doc["status"])
	}
	if doc["peak_context_tokens"] != float64(134000) {
		t.Fatalf("peak_context_tokens = %v, want 134000", doc["peak_context_tokens"])
	}
}

// A completed turn reports its largest main-chain prompt. Repeated lines for
// one message id are one call, so the peak is 40000, not 120000.
func TestClaudeMeterPeakOnCompletedTurn(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stream   string
		wantPeak int
	}{
		{
			name:     "just under threshold",
			stream:   claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_a", 99999, "null"), claudeLiveResult(99999)),
			wantPeak: 99999,
		},
		{
			name: "one message id repeated",
			stream: claudeLiveStream(claudeLiveInit(),
				claudeLiveAssistant("msg_rep", 40000, "null"),
				claudeLiveAssistant("msg_rep", 40000, "null"),
				claudeLiveAssistant("msg_rep", 40000, "null"),
				claudeLiveResult(40000)),
			wantPeak: 40000,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeClaudeLive(t, tc.stream, "")
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			cmd := claudeLiveCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
				Name: "claude-live", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
				ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull, JSON: true,
			})
			if err != nil {
				t.Fatalf("runStart: %v", err)
			}
			doc := decodeOneJSON(t, stdout.Bytes())
			if doc["status"] != "completed" {
				t.Fatalf("status = %v, want completed", doc["status"])
			}
			if doc["peak_context_tokens"] != float64(tc.wantPeak) {
				t.Fatalf("peak_context_tokens = %v, want %d", doc["peak_context_tokens"], tc.wantPeak)
			}
			sess, err := resolveSession(store, "claude-live", "")
			if err != nil {
				t.Fatal(err)
			}
			recs := meterRecords(t, sess)
			if len(recs) != 1 || recs[0].PeakContextTokens != tc.wantPeak {
				t.Fatalf("turn records = %+v, want one record with peak %d", recs, tc.wantPeak)
			}
		})
	}
}

// claudeLiveNoSession strips the session_id field from a live stream line, for
// turns whose provider never reports a session ID (ticket 1264 M3 item 2).
func claudeLiveNoSession(line string) string {
	return strings.Replace(line, `,"session_id":"sess-live"`, "", 1)
}

// TestDetachedStartWorkerKeepsWatchdogStop: a detached start worker that the
// token watchdog stops must leave the session stopped, not failed (item 1).
func TestDetachedStartWorkerKeepsWatchdogStop(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&subagent.Session{ID: "det-start-1", Name: "claude-det", StartPrompt: "say ok", Provider: "claude", Model: "haiku", Tier: "low", Status: "running", ProcessPID: os.Getpid(), WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez"}); err != nil {
		t.Fatal(err)
	}
	err = runDetachedWorker(claudeLiveCmd(), startRequest{
		SessionID: "det-start-1", Name: "claude-det", Role: "advisor", Prompt: "say ok",
		ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull,
	}, storeDir)
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold") {
		t.Fatalf("detached worker error = %v, want a watchdog stop", err)
	}
	sess, err := store.Get("det-start-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "stopped" {
		t.Fatalf("status = %q, want stopped (a watchdog stop is not a failure)", sess.Status)
	}
	if !strings.Contains(sess.LastError, "crossed live token threshold at 134000 tokens") {
		t.Fatalf("last error = %q, want the watchdog stop reason", sess.LastError)
	}
	if sess.ContextTokens != 134000 {
		t.Fatalf("ContextTokens = %d, want 134000", sess.ContextTokens)
	}
}

// TestDetachedResumeWorkerKeepsWatchdogStop: the detached resume worker keeps
// the watchdog stop as stopped too (item 1).
func TestDetachedResumeWorkerKeepsWatchdogStop(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&subagent.Session{ID: "det-resume-1", Name: "claude-det-resume", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-live", ProcessPID: os.Getpid(), WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	err = runDetachedResumeWorker(claudeLiveCmd(), "det-resume-1", resumeRequest{Prompt: "continue", StreamMode: streamFull}, storeDir)
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold") {
		t.Fatalf("detached resume worker error = %v, want a watchdog stop", err)
	}
	sess, err := store.Get("det-resume-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "stopped" {
		t.Fatalf("status = %q, want stopped (a watchdog stop is not a failure)", sess.Status)
	}
}

// TestClaudeWatchdogJSONStartWithoutProviderIDStops: a start whose turn never
// reports a provider session ID still meters, stops, and prints one JSON
// document with the quota read before and after (item 2).
func TestClaudeWatchdogJSONStartWithoutProviderIDStops(t *testing.T) {
	stream := claudeLiveStream(claudeLiveNoSession(claudeLiveInit()), claudeLiveNoSession(claudeLiveAssistant("msg_live", 134000, "null")))
	fakeClaudeLive(t, stream, "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var quotaCalls atomic.Int32
	cmd := claudeLiveCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	err = runStart(context.Background(), cmd, countingDeps(store, &quotaCalls), startRequest{
		Name: "claude-nosid", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
		ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull, JSON: true,
	})
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold at 134000 tokens") {
		t.Fatalf("runStart error = %v, want a watchdog stop", err)
	}
	doc := decodeOneJSON(t, stdout.Bytes())
	if doc["status"] != "stopped" {
		t.Fatalf("JSON status = %v, want stopped", doc["status"])
	}
	sess, err := resolveSession(store, "claude-nosid", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "stopped" || sess.ContextTokens != 134000 {
		t.Fatalf("session status=%q context=%d, want stopped at 134000", sess.Status, sess.ContextTokens)
	}
	if recs := meterRecords(t, sess); len(recs) != 1 || recs[0] != liveTurn134k {
		t.Fatalf("turn records = %+v, want one record %+v", recs, liveTurn134k)
	}
	if n := quotaCalls.Load(); n != 2 {
		t.Fatalf("turn quota read %d times on a stopped turn, want exactly 2 (before and after)", n)
	}
}

// TestClaudeWatchdogJSONResumeStopsWithOneDocument: a JSON resume stopped by
// the watchdog prints one stopped document and returns a non-nil error (item 6).
func TestClaudeWatchdogJSONResumeStopsWithOneDocument(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-json-resume", Name: "claude-json-resume", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-live", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	cmd := claudeLiveCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	err = runResume(context.Background(), cmd, toolLimitTestDeps(store), resumeRequest{Name: "claude-json-resume", Prompt: "continue", StreamMode: streamFull, JSON: true})
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold at 134000 tokens") {
		t.Fatalf("runResume error = %v, want a watchdog stop", err)
	}
	doc := decodeOneJSON(t, stdout.Bytes())
	if doc["status"] != "stopped" {
		t.Fatalf("JSON status = %v, want stopped", doc["status"])
	}
}

// TestClaudeResumeStopStoresObservedProviderSessionID: a resume stopped by the
// watchdog keeps the provider session ID the turn reported, not the stale one
// stored before the turn (item 5).
func TestClaudeResumeStopStoresObservedProviderSessionID(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-stale-id", Name: "claude-stale-id", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-stale", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	err = runResume(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), resumeRequest{Name: "claude-stale-id", Prompt: "continue", StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "crossed live token threshold") {
		t.Fatalf("runResume error = %v, want a watchdog stop", err)
	}
	sess, err := resolveSession(store, "claude-stale-id", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.ProviderSessionID != "sess-live" {
		t.Fatalf("ProviderSessionID = %q, want sess-live (the id the turn reported)", sess.ProviderSessionID)
	}
}
