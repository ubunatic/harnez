package usage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUsageDebugKeysAndFrames(t *testing.T) {
	seen := map[byte]string{}
	for name, action := range mustWatchActions().spec.Actions {
		for _, key := range action.Keys {
			b, _ := decodeSpecKey(key)
			if old, ok := seen[b]; ok {
				t.Fatalf("key %s used by %s and %s", key, old, name)
			}
			seen[b] = name
		}
	}
	key := mustWatchActions().spec.Actions["toggle_debug_box"].Keys[0][0]
	if strings.ContainsRune("jdku", rune(key)) {
		t.Fatal("debug key conflicts with diagnostics scrolling")
	}
	state, eff := dispatchWatchKey(watchKeyState{}, '!', false)
	if !state.debugOverlay || !state.debugBox || !eff.redraw {
		t.Fatalf("! did not enable gauge and box: %+v", state)
	}
	state, _ = dispatchWatchKey(state, key, false)
	if state.debugBox || !state.debugOverlay {
		t.Fatal("general toggle must close box independently of gauge")
	}
	state, _ = dispatchWatchKey(state, key, false)
	now := time.Now()
	summary := UsageSummary{Timestamp: now, Agents: []AgentUsage{
		{AgentID: "claude", LastRefreshed: now, Session: &QuotaWindow{UsedPercent: 10}},
		{AgentID: "agy", LastRefreshed: now.Add(-20 * time.Minute), QuotaFetchError: "signal: killed", QuotaFetchCause: "our context timeout"},
		{AgentID: "codex"},
	}}
	for _, mode := range []string{"normal", "compact", "minimal"} {
		opts := WatchOptions{Mode: mode, DebugBox: state.debugBox, DebugOverlay: state.debugOverlay}
		frame := buildWatchFrameAt(summary, nil, DefaultWatchInterval, initialWatchSections(opts), 120, 40, true, now, opts)
		text := stripANSI(strings.Join(frame.lines, "\n"))
		wants := []string{"Debug", "claude  last_fetch:", "agy  last_fetch:", "codex  last_fetch: unknown", "stale_reason: fetch failed: signal: killed", "cause: our context timeout"}
		if mode != "minimal" {
			wants = append(wants, "[b]debug")
		}
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s frame missing %q: %s", mode, want, text)
			}
		}
		for _, line := range frame.lines {
			if visLen(line) > frame.cols {
				t.Fatalf("%s frame line too wide", mode)
			}
		}
	}
	if got := debugText("a\u200d\x1b[2J\r\n"); strings.ContainsAny(got, "\x1b\r\n") {
		t.Fatalf("terminal controls survived: %q", got)
	}
}

func TestCatchupLogAndFetchCause(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	now := time.Now()
	registry, err := NewRegistry(
		Collector{ID: "fresh", Provider: "claude", Cadence: time.Minute, Timeout: time.Second, Collect: func(context.Context) (AgentUsage, error) {
			t.Error("fresh provider was fetched")
			return AgentUsage{}, nil
		}},
		Collector{ID: "old", Provider: "agy", Cadence: time.Minute, Timeout: time.Second, Collect: func(context.Context) (AgentUsage, error) {
			return AgentUsage{QuotaFetchError: "signal: killed"}, nil
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	registry.lastGood["fresh"] = AgentUsage{AgentID: "claude", LastRefreshed: now}
	var logs []string
	registry.logFetchFailure = func(line string) { logs = append(logs, line) }
	agents := registry.CollectCatchup(context.Background(), time.Minute, nil, nil)
	registry.waitKillDiagnostics()
	if len(logs) != 2 || !strings.Contains(logs[0], "providers fetched: [agy]; providers skipped as fresh: [claude]") {
		t.Fatalf("catch-up log: %v", logs)
	}
	if agents[1].QuotaFetchCause != "unknown" {
		t.Fatalf("missing stored cause: %+v", agents[1])
	}
	if got := fetchFailureCause(registry.collectors[1], time.Second, errors.New("signal: killed"), context.DeadlineExceeded); got != "our context timeout" {
		t.Fatalf("cause = %s", got)
	}
}
