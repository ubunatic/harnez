package usage

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRenderTextDimsStaleQuotaLineAndAnnotatesUpdated verifies issue 107's
// full/verbose-view treatment: a stale agent's quota line is wrapped in the
// dim-grey convention and the "Updated:" caption gets a terminal-independent
// "· stale" suffix, while an otherwise-identical fresh agent gets neither.
func TestRenderTextDimsStaleQuotaLineAndAnnotatesUpdated(t *testing.T) {
	now := time.Now()
	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID: "agy", Name: "Antigravity (AGY)", Installed: true, Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: now.Add(-3 * time.Hour),
			},
			{
				AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: now,
			},
		},
	}

	text := RenderText(summary)

	// Isolate each agent's box so an assertion about one can't accidentally
	// match text that belongs to the other.
	agyIdx := strings.Index(text, "Antigravity (AGY)")
	claudeIdx := strings.Index(text, "Claude Code")
	if agyIdx < 0 || claudeIdx < 0 {
		t.Fatalf("expected both agent boxes in output:\n%s", text)
	}
	var agyBox, claudeBox string
	if agyIdx < claudeIdx {
		agyBox, claudeBox = text[agyIdx:claudeIdx], text[claudeIdx:]
	} else {
		claudeBox, agyBox = text[claudeIdx:agyIdx], text[agyIdx:]
	}

	if !strings.Contains(agyBox, ansiOpen("dim-grey")) {
		t.Errorf("expected the stale agent's box to contain a dim-grey wrap, got:\n%s", stripANSI(agyBox))
	}
	if !strings.Contains(agyBox, "· stale") {
		t.Errorf("expected the stale agent's Updated caption to say '· stale', got:\n%s", stripANSI(agyBox))
	}
	if strings.Contains(claudeBox, "· stale") {
		t.Errorf("expected the fresh agent's Updated caption to NOT say '· stale', got:\n%s", stripANSI(claudeBox))
	}
}

func TestRenderSummary(t *testing.T) {
	now := time.Date(2026, 8, 17, 22, 0, 0, 0, time.UTC)
	resetTime := now.Add(4 * time.Hour)

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Account:       "u***@example.com",
				PlanTier:      "Max",
				ActiveModel:   "claude-sonnet-5",
				Session: &QuotaWindow{
					Name:             "Session (5-hour)",
					UsedPercent:      60.0,
					RemainingPercent: 40.0,
					ResetAt:          &resetTime,
					DurationLeft:     4 * time.Hour,
				},
				Tokens: &TokenBreakdown{
					InputTokens:      1000,
					OutputTokens:     2000,
					CacheReadTokens:  3000,
					CacheWriteTokens: 4000,
					TotalTokens:      10000,
				},
				Details: map[string]string{
					"total_sessions": "12",
				},
			},
			{
				AgentID:       "codex",
				Name:          "OpenAI Codex",
				Installed:     false,
				Authenticated: false,
			},
		},
	}

	// Test JSON rendering
	jsonStr, err := RenderJSON(summary)
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	if !strings.Contains(jsonStr, `"agent_id": "claude"`) {
		t.Errorf("expected JSON to contain agent_id claude, got:\n%s", jsonStr)
	}

	// Test Text rendering
	textStr := RenderText(summary)
	if !strings.Contains(textStr, "Claude Code") {
		t.Errorf("expected Text to contain 'Claude Code', got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "60.0% used") {
		t.Errorf("expected Text to contain '60.0%% used', got:\n%s", textStr)
	}
	// codex has no real usage data (Installed: false) — issue 083 says
	// RenderText should self-hide it rather than print an empty/not-installed
	// box for it.
	if strings.Contains(textStr, "OpenAI Codex") {
		t.Errorf("expected Text to omit codex box (no usage data), got:\n%s", textStr)
	}
}

// TestRenderText_StaleWithinSevenDaysStillShown covers issue 101's core
// requirement: an agent whose last known data is older than the 30-minute
// live-recollect window but still well within the 7-day display-hide
// threshold keeps showing its last-known snapshot, annotated with a "last
// updated" timestamp, rather than vanishing or blanking.
func TestRenderText_StaleWithinSevenDaysStillShown(t *testing.T) {
	// IsStale/FormatAgo compare LastRefreshed against actual wall-clock time
	// (not the summary's own Timestamp), so both must be anchored to
	// time.Now() rather than a fixed historical date.
	now := time.Now()
	lastRefreshed := now.Add(-3 * time.Hour) // >> 30min live-recollect window, << 7d display window

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity (AGY)",
				Installed:     true,
				Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: lastRefreshed,
			},
		},
	}

	textStr := RenderText(summary)

	if !strings.Contains(textStr, "Antigravity (AGY)") {
		t.Errorf("expected Text to still show the stale-but-within-7d agent box, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "42.0% used") {
		t.Errorf("expected Text to still show the last-known quota data, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "Updated:") || !strings.Contains(textStr, "3h") {
		t.Errorf("expected Text to annotate the box with a 'last updated ~3h ago' line, got:\n%s", textStr)
	}
}

// TestRenderText_SevenDayStaleAgentHidden covers issue 101's auto-hide gate:
// once an agent's last known data is 7+ days stale, RenderText hides it even
// though HasUsageData() would still report true (a genuinely
// abandoned/uninstalled agent, not one that's merely not running right now).
func TestRenderText_SevenDayStaleAgentHidden(t *testing.T) {
	now := time.Now()
	lastRefreshed := now.Add(-8 * 24 * time.Hour)

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity (AGY)",
				Installed:     true,
				Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: lastRefreshed,
			},
		},
	}

	textStr := RenderText(summary)

	if strings.Contains(textStr, "── Antigravity (AGY) ") {
		t.Errorf("expected Text to hide an agent 8 days stale, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "No supported agent") {
		t.Errorf("expected the all-agents-absent fallback message, got:\n%s", textStr)
	}
}

// TestRenderText_AllAgentsAbsent covers issue 083's "don't render a silently
// empty screen" requirement: when no supported agent has any real recorded
// usage, RenderText must say so explicitly instead of printing nothing but
// the header.
func TestRenderText_AllAgentsAbsent(t *testing.T) {
	summary := UsageSummary{
		Timestamp: time.Date(2026, 8, 17, 22, 0, 0, 0, time.UTC),
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: false},
			{AgentID: "agy", Name: "Antigravity (AGY)", Installed: false},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: false},
		},
	}

	textStr := RenderText(summary)

	// Box titles render as "── <Name> ──"; check for that rather than the
	// bare name, since the explanatory fallback message legitimately mentions
	// the agent names in prose.
	for _, name := range []string{"Claude Code", "Antigravity (AGY)", "OpenAI Codex"} {
		if strings.Contains(textStr, "── "+name+" ") {
			t.Errorf("expected Text to omit %s box (no usage data anywhere), got:\n%s", name, textStr)
		}
	}
	if !strings.Contains(textStr, "No supported agent") {
		t.Errorf("expected Text to explain that no agent has recorded usage, got:\n%s", textStr)
	}
}

// sleepyRoundTripper is an http.RoundTripper that sleeps a fixed delay
// before returning a trivial 200 response, regardless of the request. It
// stands in for the real Claude/Codex quota endpoints in
// TestCollectAllRunsCollectorsConcurrently below, so that a real
// client.Do call (issue 129) takes an observable, controlled amount of
// wall time.
type sleepyRoundTripper struct {
	delay time.Duration
}

func (rt *sleepyRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	time.Sleep(rt.delay)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}, nil
}

// TestCollectAllRunsCollectorsConcurrently is issue 129's verification that
// collectAll's three per-agent collectors (claude, agy, codex) run
// concurrently rather than sequentially. It wires up real credentials/auth
// files for all three agents so each collector actually reaches its live
// fetch path, then makes each of those live fetches artificially slow
// (~50ms): Claude and Codex via a sleepy http.RoundTripper standing in for
// their HTTP quota endpoints, and AGY via a stubbed runAGYUsageCmdFn
// standing in for its `agy -p "/usage"` subprocess call. If the three
// collectors ran sequentially, three ~50ms calls would sum to ~150ms; run
// concurrently, total wall time should stay close to a single ~50ms call.
//
// Before this change (sequential collectClaude/collectAGY/collectCodex
// calls in collectAll), an equivalent live cold-cache `harnez usage` run
// measured roughly 10-12s wall time (three real subprocess/HTTP calls of
// ~3-4s each in series); after switching to goroutines + sync.WaitGroup,
// the equivalent run drops to roughly the slowest single collector's time
// (~3-4s), matching the ~50ms-vs-150ms ratio this test asserts at a much
// smaller, CI-friendly scale.
func TestCollectAllRunsCollectorsConcurrently(t *testing.T) {
	const delay = 50 * time.Millisecond

	homeDir := t.TempDir()
	claudeDir := filepath.Join(homeDir, ".claude")
	agyDir := filepath.Join(homeDir, ".gemini", "antigravity-cli")
	codexDir := filepath.Join(homeDir, ".codex")
	for _, d := range []string{claudeDir, agyDir, codexDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", d, err)
		}
	}

	claudeCreds := `{"claudeAiOauth":{"accessToken":"tok","subscriptionType":"pro"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(claudeCreds), 0o600); err != nil {
		t.Fatalf("write claude credentials: %v", err)
	}

	codexAuth := `{"auth_mode":"chatgpt","tokens":{"access_token":"tok","id_token":"tok"}}`
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(codexAuth), 0o600); err != nil {
		t.Fatalf("write codex auth: %v", err)
	}

	prevAGYFn := runAGYUsageCmdFn
	runAGYUsageCmdFn = func(ctx context.Context) ([]byte, error) {
		time.Sleep(delay)
		return []byte(agyOKOutput), nil
	}
	defer func() { runAGYUsageCmdFn = prevAGYFn }()

	client := &http.Client{Transport: &sleepyRoundTripper{delay: delay}}

	start := time.Now()
	collectAll(context.Background(), homeDir, client, false, nil)
	elapsed := time.Since(start)

	// Generous upper bound: well under the ~150ms a sequential run of
	// three ~50ms collectors would take, but comfortably above a single
	// ~50ms collector plus scheduling/test overhead.
	const maxElapsed = 120 * time.Millisecond
	if elapsed >= maxElapsed {
		t.Errorf("expected collectAll to run its 3 collectors concurrently (~%s total), took %s (>= %s, looks sequential)", delay, elapsed, maxElapsed)
	}
}

// TestCollectAllProgressReportsStartedAndTerminalStageForEverySource is issue
// 169's core contract for the fetch-stage-reporting mechanism: with no prior
// cached snapshot (so every one of the three per-agent collectors actually
// runs live), CollectAllProgress's callback must see a FetchStarted followed
// by exactly one terminal stage (FetchDone or FetchFailed) for each of
// "claude", "agy", "codex" -- never zero events (the splash would show
// nothing) and never more than one terminal event per source (the splash's
// "only the latest event" contract assumes a clean start->terminal sequence
// per source, not a source flip-flopping).
func TestCollectAllProgressReportsStartedAndTerminalStageForEverySource(t *testing.T) {
	homeDir := t.TempDir() // empty: no creds/auth anywhere, so every collector
	// takes its fast "not installed" path rather than a real network call.
	client := &http.Client{Transport: &sleepyRoundTripper{delay: 0}}

	var mu sync.Mutex
	started := map[string]int{}
	terminal := map[string]int{}

	progress := func(source string, stage FetchStage) {
		mu.Lock()
		defer mu.Unlock()
		switch stage {
		case FetchStarted:
			started[source]++
		case FetchDone, FetchFailed:
			terminal[source]++
		}
	}

	CollectAllProgress(context.Background(), homeDir, client, progress)

	for _, source := range []string{"claude", "agy", "codex"} {
		if started[source] != 1 {
			t.Errorf("expected exactly 1 FetchStarted for %q, got %d", source, started[source])
		}
		if terminal[source] != 1 {
			t.Errorf("expected exactly 1 terminal (done/failed) event for %q, got %d", source, terminal[source])
		}
	}
}

// TestCollectAllProgressNilCallbackIsNoop checks that a nil FetchProgressFunc
// (what every non-watch caller -- CollectAll, CollectAllLive, --summary,
// RenderSummary, the collector daemon -- passes) never panics and produces
// the same UsageSummary shape as before this ticket's change, i.e. the
// progress-reporting mechanism is purely additive.
func TestCollectAllProgressNilCallbackIsNoop(t *testing.T) {
	homeDir := t.TempDir()
	client := &http.Client{Transport: &sleepyRoundTripper{delay: 0}}

	summary := CollectAllProgress(context.Background(), homeDir, client, nil)
	if len(summary.Agents) != 3 {
		t.Fatalf("expected 3 agents in summary, got %d", len(summary.Agents))
	}
}
