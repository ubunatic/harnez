package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/quota1"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestRunStartRecordsQuotaBoundariesForTurn(t *testing.T) {
	storeDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var calls []bool
	var captureOnce sync.Once
	captureStarted := make(chan struct{})
	continueCapture := make(chan struct{})
	driver.runHook = func() {
		<-captureStarted
		close(continueCapture)
	}
	capture := func(_ context.Context, _ string, force bool) usage.TurnQuotaReading {
		captureOnce.Do(func() {
			close(captureStarted)
			<-continueCapture
		})
		calls = append(calls, force)
		return usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), HasCache: true, CacheAgeMS: 123}
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = runStart(t.Context(), cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, quota: capture, dbPath: dbPath}, startRequest{Role: "advisor", Prompt: "p", StoredPrompt: "p", ModelSpec: "codex:luna", Dir: ".", StreamMode: streamFull, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []bool{false, false}) {
		t.Fatalf("quota capture force flags=%v, want [false false]", calls)
	}
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	usageStore, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer usageStore.Close()
	var boundaries int
	if err := usageStore.QueryRow(context.Background(), `SELECT count(*) FROM turn_quota_boundaries WHERE session_id=? AND turn=1`, sessions[0].ID).Scan(&boundaries); err != nil {
		t.Fatal(err)
	}
	if boundaries != 2 {
		t.Fatalf("stored boundary count=%d, want 2", boundaries)
	}
	tokens, err := usageStore.TurnTokenUsages(context.Background(), sessions[0].ID)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("stored token rows=%+v err=%v, want delta and cumulative", tokens, err)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "quota-readings.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("legacy quota spool exists after turn: stat err=%v", err)
	}
}

func TestTurnQuotaBaselineUsesBeforeCaptureCompletion(t *testing.T) {
	turnStarted := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	capturedAt := turnStarted.Add(5 * time.Second)
	before := usage.TurnQuotaReading{CapturedAt: capturedAt, HasCache: true, CacheAgeMS: 40}

	got := turnQuotaBaseline(turnStarted, before)
	want := capturedAt.Add(-40 * time.Millisecond)
	if !got.Equal(want) || !got.After(turnStarted) {
		t.Fatalf("turnQuotaBaseline = %s, want %s after turn start", got, want)
	}
}

func TestWarnQuota1Changes(t *testing.T) {
	dir := t.TempDir()
	state, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	runAt := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if err := os.MkdirAll(filepath.Dir(state), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte(runAt.Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%d.go", i))
		if err := os.WriteFile(path, []byte("package test\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, runAt.Add(time.Minute), runAt.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	warnQuota1Changes(cmd, dir, runAt)
	got := errOut.String()
	if !strings.Contains(got, "7 file(s) changed after the last quota-1 run at "+runAt.Format(time.RFC3339)+":") || !strings.Contains(got, "— code is untested, run make test-q1") {
		t.Fatalf("warning = %q", got)
	}
	if strings.Count(got, "file-") != 5 || out.Len() != 0 {
		t.Fatalf("warning should list 5 files once on stderr; stderr=%q stdout=%q", got, out.String())
	}
}

func TestAgentTurnsWarnAboutQuota1Changes(t *testing.T) {
	for _, mode := range []string{"start", "resume"} {
		t.Run(mode, func(t *testing.T) {
			repoDir := t.TempDir()
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			runAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
			state, _, err := quota1.ResolveStateFile(repoDir)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(state), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(state, []byte(runAt.Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(repoDir, "changed.go")
			if err := os.WriteFile(source, []byte("package changed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			future := runAt.Add(time.Minute)
			if err := os.Chtimes(source, future, future); err != nil {
				t.Fatal(err)
			}
			driver := &recordingAgentDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			cmd := &cobra.Command{}
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
			if mode == "start" {
				err = runStart(t.Context(), cmd, deps, startRequest{Role: "advisor", Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna", Dir: repoDir, StreamMode: streamFull, JSON: true})
			} else {
				if err := store.Save(&subagent.Session{ID: "resume-id", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: repoDir, Status: "completed", ContextTokens: 100}); err != nil {
					t.Fatal(err)
				}
				deps.find = func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
					sessions, err := s.List("", true)
					if err != nil {
						return nil, err
					}
					for _, session := range sessions {
						if session.Name == name || session.ID == name {
							return session, nil
						}
					}
					return nil, fmt.Errorf("session %q not found", name)
				}
				err = runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", StreamMode: streamFull, JSON: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr.String(), "1 file(s) changed after the last quota-1 run at "+runAt.Format(time.RFC3339)+": changed.go — code is untested, run make test-q1") {
				t.Fatalf("stderr warning = %q", stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), "{") {
				t.Fatalf("stdout should remain JSON, got %q", stdout.String())
			}
		})
	}
}

type fixtureCostDriver struct {
	results []*subagent.TurnResult
}

func (d *fixtureCostDriver) Run(_ context.Context, _ subagent.RunOptions) (*subagent.TurnResult, error) {
	r := d.results[0]
	d.results = d.results[1:]
	return r, nil
}
func (d *fixtureCostDriver) Resume(_ context.Context, _, _ string, _ subagent.Model) (*subagent.TurnResult, error) {
	return d.Run(context.Background(), subagent.RunOptions{})
}
func (d *fixtureCostDriver) Compact(context.Context, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}
func (d *fixtureCostDriver) Stop(context.Context, string) error   { return nil }
func (d *fixtureCostDriver) Delete(context.Context, string) error { return nil }

func TestRunStartRecordsFullCostRecordForPricedAndUnpricedModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	pricingFile := filepath.Join(t.TempDir(), "pricing.json")
	if err := os.WriteFile(pricingFile, []byte(`{
  "gpt-6-luna": {
    "model": "gpt-6-luna",
    "revision": "test-2026-11",
    "rates": {
      "cached_input_micros_per_million": 100000,
      "uncached_input_micros_per_million": 1000000,
      "output_micros_per_million": 8000000,
      "reasoning_micros_per_million": 8000000
    }
  }
}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARNEZ_PRICING_FILE", pricingFile)

	storeDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}

	reset := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	runAt := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	mkReadings := func(provider string, beforeUsed, afterUsed float64) func(context.Context, string, bool) usage.TurnQuotaReading {
		idx := 0
		return func(_ context.Context, _ string, _ bool) usage.TurnQuotaReading {
			idx++
			used := beforeUsed
			at := runAt
			if idx > 1 {
				used = afterUsed
				at = runAt.Add(30 * time.Second)
			}
			return usage.TurnQuotaReading{
				CapturedAt: at, Source: "test-fixture", HasCache: true,
				StoreWindows: []usagestore.Window{{Provider: provider, Key: "weekly", UsedFraction: used, ResetAt: &reset, ObservedAt: at}},
			}
		}
	}

	// Case 1: Codex gpt-6-luna (priced in HARNEZ_PRICING_FILE) + reasoning tokens + weekly quota delta ($20/mo => $4,615,384/wk).
	codexDriver := &fixtureCostDriver{results: []*subagent.TurnResult{{
		SessionID: "codex-1", Response: "ok", Messages: []string{"ok"},
		InputTokens: 1000, CachedTokens: 5000, OutputTokens: 200,
		ReasoningTokens: 100, ReasoningTokensKnown: true, TokensTurn: 6300,
	}}}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return codexDriver }
	defer func() { agentDriver = old }()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = runStart(t.Context(), cmd, agentDeps{
		store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" },
		quota: mkReadings("codex", 0.10, 0.12), dbPath: dbPath,
	}, startRequest{Role: "advisor", Prompt: "p", StoredPrompt: "p", ModelSpec: "codex:luna:low", Dir: ".", StreamMode: streamFull, JSON: true})
	if err != nil {
		t.Fatal(err)
	}

	// Case 2: Claude haiku (unpriced in pricingFile -> CostMicros nil!) + cache_creation_input_tokens + $100/mo weekly delta.
	claudeDriver := &fixtureCostDriver{results: []*subagent.TurnResult{{
		SessionID: "claude-1", Response: "ok", Messages: []string{"ok"},
		InputTokens: 34, CachedTokens: 66409, CacheWriteTokens: 5269, CacheWriteTokensKnown: true, OutputTokens: 393,
		TokensTurn: 66836,
	}}}
	agentDriver = func(subagent.Model, string) subagent.Driver { return claudeDriver }
	out.Reset()
	err = runStart(t.Context(), cmd, agentDeps{
		store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" },
		quota: mkReadings("claude", 0.20, 0.21), dbPath: dbPath,
	}, startRequest{Role: "advisor", Prompt: "p", StoredPrompt: "p", ModelSpec: "claude:haiku:high", Dir: ".", StreamMode: streamFull, JSON: true})
	if err != nil {
		t.Fatal(err)
	}

	codexSess, err := store.Get("codex-1")
	if err != nil {
		t.Fatal(err)
	}
	// API cost = 1000*1 + 5000*0.1 + 200*8 + 100*8 = 1000 + 500 + 1600 + 800 = 3900 micros.
	if len(codexSess.TurnRecords) != 1 || codexSess.TurnRecords[0].CostMicros == nil || *codexSess.TurnRecords[0].CostMicros != 3900 || codexSess.TurnRecords[0].PricingRevision != "test-2026-11" {
		t.Fatalf("codex turn record cost = %+v, want CostMicros=3900", codexSess.TurnRecords)
	}
	if codexSess.TurnRecords[0].CacheWriteTokens != nil {
		t.Fatalf("codex CacheWriteTokens = %v, want nil (unknown)", codexSess.TurnRecords[0].CacheWriteTokens)
	}
	// Share = 0.02 * ((20_000_000 * 12) / 52) = 0.02 * 4_615_384 = 92_308 micros.
	if codexSess.TurnRecords[0].SubscriptionPriceShareMicros == nil || *codexSess.TurnRecords[0].SubscriptionPriceShareMicros != 92308 || codexSess.TotalSubscriptionShareMicros == nil || *codexSess.TotalSubscriptionShareMicros != 92308 {
		t.Fatalf("codex share = %+v, session total=%v, want 92308", codexSess.TurnRecords[0].SubscriptionPriceShareMicros, codexSess.TotalSubscriptionShareMicros)
	}

	claudeSess, err := store.Get("claude-1")
	if err != nil {
		t.Fatal(err)
	}
	if claudeSess.TurnRecords[0].CostMicros != nil || claudeSess.TotalCostMicros != nil || claudeSess.TurnRecords[0].PricingRevision != "" {
		t.Fatalf("unpriced claude must omit API cost (nil), got rec=%+v total=%v", claudeSess.TurnRecords[0], claudeSess.TotalCostMicros)
	}
	if claudeSess.TurnRecords[0].CacheWriteTokens == nil || *claudeSess.TurnRecords[0].CacheWriteTokens != 5269 {
		t.Fatalf("claude CacheWriteTokens = %v, want 5269", claudeSess.TurnRecords[0].CacheWriteTokens)
	}
	// Share = 0.01 * ((100_000_000 * 12) / 52) = 230_769 micros.
	if claudeSess.TurnRecords[0].SubscriptionPriceShareMicros == nil || *claudeSess.TurnRecords[0].SubscriptionPriceShareMicros != 230769 {
		t.Fatalf("claude share = %v, want 230769", claudeSess.TurnRecords[0].SubscriptionPriceShareMicros)
	}

	usageStore, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer usageStore.Close()
	codexRows, err := usageStore.TurnTokenUsages(context.Background(), "codex-1")
	if err != nil || len(codexRows) != 2 || codexRows[1].CounterKind != "delta" || codexRows[1].CostMicros == nil || *codexRows[1].CostMicros != 3900 || codexRows[1].Effort != "low" {
		t.Fatalf("codex DB token rows = %+v, err=%v", codexRows, err)
	}
	claudeRows, err := usageStore.TurnTokenUsages(context.Background(), "claude-1")
	if err != nil || len(claudeRows) != 2 || claudeRows[1].CounterKind != "delta" || claudeRows[1].CostMicros != nil || claudeRows[1].CacheWriteTokens == nil || *claudeRows[1].CacheWriteTokens != 5269 {
		t.Fatalf("claude DB token rows = %+v, err=%v", claudeRows, err)
	}
	claudeDeltas, err := usageStore.TurnQuotaDeltasForSession(context.Background(), "claude-1")
	if err != nil || len(claudeDeltas) != 1 || claudeDeltas[0].SubscriptionPriceShareMicros == nil || *claudeDeltas[0].SubscriptionPriceShareMicros != 230769 {
		t.Fatalf("claude DB quota deltas = %+v, err=%v", claudeDeltas, err)
	}

	// Case 3: Resume codex-1 on turn 2 with another priced turn and quota delta -> accumulates Session totals.
	codexResumeDriver := &fixtureCostDriver{results: []*subagent.TurnResult{{
		SessionID: "codex-1", Response: "ok2", Messages: []string{"ok2"},
		InputTokens: 1000, CachedTokens: 0, OutputTokens: 100,
		ReasoningTokens: 0, ReasoningTokensKnown: true, TokensTurn: 1100,
	}}}
	agentDriver = func(subagent.Model, string) subagent.Driver { return codexResumeDriver }
	err = runResume(t.Context(), cmd, agentDeps{
		store: func() (*subagent.FileSessionStore, error) { return store, nil },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
			return s.Get(name)
		},
		parent: func() string { return "" },
		quota:  mkReadings("codex", 0.12, 0.13), dbPath: dbPath,
	}, resumeRequest{Name: "codex-1", Prompt: "next", StreamMode: streamFull, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
	resumedSess, err := store.Get("codex-1")
	if err != nil {
		t.Fatal(err)
	}
	// Turn 2 API cost = 1000*1 + 100*8 = 1800 micros; total = 3900 + 1800 = 5700 micros.
	if len(resumedSess.TurnRecords) != 2 || resumedSess.TurnRecords[1].CostMicros == nil || *resumedSess.TurnRecords[1].CostMicros != 1800 || resumedSess.TotalCostMicros == nil || *resumedSess.TotalCostMicros != 5700 {
		t.Fatalf("resumed codex totals = %v, turn2 = %+v", resumedSess.TotalCostMicros, resumedSess.TurnRecords)
	}
	// Turn 2 share = 0.01 * 4_615_384 = 46_154; total share = 92_308 + 46_154 = 138_462.
	if resumedSess.TotalSubscriptionShareMicros == nil || *resumedSess.TotalSubscriptionShareMicros != 138462 {
		t.Fatalf("resumed TotalSubscriptionShareMicros = %v, want 138462", resumedSess.TotalSubscriptionShareMicros)
	}
}
