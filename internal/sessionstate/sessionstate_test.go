package sessionstate

import (
	"strings"
	"testing"
	"time"
)

func TestGapTip_FreshSessionNoTip(t *testing.T) {
	s := State{Calls: map[string]Invocation{}}
	if _, ok := GapTip(s, false, time.Now(), 0); ok {
		t.Errorf("expected a fresh session (Total=0) to produce no tip")
	}
}

func TestGapTip_RateGapTriggersReminder(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	// 25 calls, never rated: exceeds rateGapThreshold (20) and clears
	// tipCooldown (10) since TotalAtLastTip starts at 0.
	for i := 0; i < 25; i++ {
		Record(&s, "distill", now)
	}

	tip, ok := GapTip(s, false, now, 0)
	if !ok {
		t.Fatalf("expected a rate-gap tip to fire after %d unrated calls", s.Total)
	}
	if !strings.Contains(tip, "harnez rate") {
		t.Errorf("expected tip to mention harnez rate, got: %q", tip)
	}
}

func TestGapTip_RecentRateSuppressesReminder(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 25; i++ {
		Record(&s, "distill", now)
	}
	Record(&s, "find", now) // also clear the find-underuse heuristic
	Record(&s, "rate", now) // resets the rate-gap counter

	if tip, ok := GapTip(s, false, now, 0); ok {
		t.Errorf("expected no tip once both rate and find gaps are cleared, got: %q", tip)
	}
}

func TestGapTip_FindUnderuseFiresWhenNoRateGap(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	// Rate recently (no rate-gap), but never call find, and clear
	// findMinCalls (8) and tipCooldown (10).
	for i := 0; i < 12; i++ {
		Record(&s, "distill", now)
	}
	Record(&s, "rate", now)

	tip, ok := GapTip(s, false, now, 0)
	if !ok {
		t.Fatalf("expected a find-underuse tip to fire")
	}
	if !strings.Contains(tip, "harnez find") {
		t.Errorf("expected tip to mention harnez find, got: %q", tip)
	}
}

func TestGapTip_UsingFindSuppressesUnderuseTip(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 12; i++ {
		Record(&s, "distill", now)
	}
	Record(&s, "find", now)
	Record(&s, "rate", now)

	if _, ok := GapTip(s, false, now, 0); ok {
		t.Errorf("expected no tip once both rate and find have been used")
	}
}

func TestGapTip_CooldownSuppressesRepeatTip(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 25; i++ {
		Record(&s, "distill", now)
	}
	// Simulate a tip having just fired.
	s.TotalAtLastTip = s.Total

	if _, ok := GapTip(s, false, now, 0); ok {
		t.Errorf("expected cooldown to suppress an immediate repeat tip")
	}

	// After tipCooldown more calls, it should be eligible again.
	for i := 0; i < tipCooldown; i++ {
		Record(&s, "distill", now)
	}
	if _, ok := GapTip(s, false, now, 0); !ok {
		t.Errorf("expected the tip to become eligible again after the cooldown elapses")
	}
}

func TestRecord_TracksCountsAndLastRate(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "find", now)
	Record(&s, "find", now)
	Record(&s, "rate", now)

	if s.Total != 3 {
		t.Errorf("expected Total=3, got %d", s.Total)
	}
	if s.Calls["find"].Count != 2 {
		t.Errorf("expected find count=2, got %d", s.Calls["find"].Count)
	}
	if s.LastRateAt.IsZero() {
		t.Errorf("expected LastRateAt to be set after a rate call")
	}
	if s.TotalAtLastRate != 3 {
		t.Errorf("expected TotalAtLastRate=3, got %d", s.TotalAtLastRate)
	}
	if s.FirstCallAt.IsZero() {
		t.Errorf("expected FirstCallAt to be set after the first call")
	}
}

func TestLoadSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	sessionID := "test-session-123"

	s, err := Load(dir, sessionID)
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if s.Total != 0 {
		t.Fatalf("expected fresh state for missing file, got Total=%d", s.Total)
	}

	Record(&s, "distill", time.Now())
	Record(&s, "rate", time.Now())
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir, sessionID)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.Total != 2 {
		t.Errorf("expected Total=2 after round trip, got %d", loaded.Total)
	}
	if loaded.Calls["distill"].Count != 1 {
		t.Errorf("expected distill count=1 after round trip, got %d", loaded.Calls["distill"].Count)
	}
}

// Issue 179: the heartbeat-specific gap tip and its interaction with issue
// 142's rate-feedback opt-out.

func TestGapTip_HeartbeatGapSupersedesPlainRateGap(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	// heartbeatGapThreshold (40) unrated calls: past both thresholds, so the
	// more specific heartbeat nudge must win over the plain rate-gap tip.
	for i := 0; i < heartbeatGapThreshold; i++ {
		Record(&s, "distill", now)
	}

	tip, ok := GapTip(s, false, now, 0)
	if !ok {
		t.Fatalf("expected a heartbeat-gap tip to fire after %d unrated calls", s.Total)
	}
	if !strings.Contains(tip, "--ok") {
		t.Errorf("expected tip to suggest `harnez rate --ok`, got: %q", tip)
	}
}

func TestGapTip_BelowHeartbeatThresholdUsesPlainReminder(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	// One call short of heartbeatGapThreshold: still the plain rate-gap tip,
	// not the heartbeat-specific one.
	for i := 0; i < heartbeatGapThreshold-1; i++ {
		Record(&s, "distill", now)
	}

	tip, ok := GapTip(s, false, now, 0)
	if !ok {
		t.Fatalf("expected a rate-gap tip to fire after %d unrated calls", s.Total)
	}
	if strings.Contains(tip, "--ok") {
		t.Errorf("expected the plain rate-gap tip (no --ok mention) below heartbeatGapThreshold, got: %q", tip)
	}
}

func TestGapTip_HeartbeatCallClearsRateGap(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < heartbeatGapThreshold; i++ {
		Record(&s, "distill", now)
	}
	Record(&s, "find", now) // clear the find-underuse heuristic too
	Record(&s, "rate", now) // an --ok heartbeat is still subcommand "rate"

	if tip, ok := GapTip(s, false, now, 0); ok {
		t.Errorf("expected a heartbeat/rate call to clear the gap, got tip: %q", tip)
	}
}

func TestGapTip_FeedbackDisabledSuppressesRateAndHeartbeatTips(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < heartbeatGapThreshold; i++ {
		Record(&s, "distill", now)
	}
	Record(&s, "find", now) // also clear the find-underuse heuristic

	if tip, ok := GapTip(s, true, now, 0); ok {
		t.Errorf("expected feedbackDisabled=true to suppress both rate-gap and heartbeat tips, got: %q", tip)
	}

	// Sanity check: the same state without the opt-out still produces a tip.
	if _, ok := GapTip(s, false, now, 0); !ok {
		t.Errorf("expected feedbackDisabled=false to still surface a tip for the same state")
	}
}

// Issue 186: the wall-clock counterpart to the call-count gap checks above.

func TestGapTip_TimeOnlyTriggersRateGap(t *testing.T) {
	start := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "rate", start) // establishes LastRateAt/TotalAtLastRate

	// More calls than tipCooldown (10) but well under rateGapThreshold (20)
	// and heartbeatGapThreshold (40) — spaced far enough apart in wall time
	// to cross rateGapIdle (20m default) since the last rate call.
	later := start.Add(25 * time.Minute)
	for i := 0; i < 12; i++ {
		Record(&s, "distill", later)
	}

	tip, ok := GapTip(s, false, later, 0)
	if !ok {
		t.Fatalf("expected a time-based rate-gap tip after %v idle with only %d calls since rate",
			later.Sub(start), s.Total-s.TotalAtLastRate)
	}
	if !strings.Contains(tip, "harnez rate") || strings.Contains(tip, "--ok") {
		t.Errorf("expected the plain (non-heartbeat) rate-gap tip, got: %q", tip)
	}
}

func TestGapTip_TimeOnlyTriggersHeartbeatGap(t *testing.T) {
	start := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "rate", start)

	// Under both count thresholds (but past tipCooldown), and past
	// heartbeatGapIdle (2x rateGapIdle = 40m default): the more specific
	// heartbeat nudge should win, exactly as it does for the count-based
	// case.
	later := start.Add(45 * time.Minute)
	for i := 0; i < 12; i++ {
		Record(&s, "distill", later)
	}

	tip, ok := GapTip(s, false, later, 0)
	if !ok {
		t.Fatalf("expected a time-based heartbeat-gap tip after %v idle", later.Sub(start))
	}
	if !strings.Contains(tip, "--ok") {
		t.Errorf("expected the heartbeat-specific tip, got: %q", tip)
	}
}

func TestGapTip_CountOnlyTriggerUnchangedByTimeCheck(t *testing.T) {
	// Existing count-based behavior (issue 179/183) must be unaffected: many
	// calls in quick succession (no measurable elapsed time) still trigger
	// on count alone.
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 25; i++ {
		Record(&s, "distill", now)
	}

	tip, ok := GapTip(s, false, now, 0)
	if !ok {
		t.Fatalf("expected the count-based rate-gap tip to still fire with zero elapsed time")
	}
	if !strings.Contains(tip, "harnez rate") {
		t.Errorf("expected tip to mention harnez rate, got: %q", tip)
	}
}

func TestGapTip_BothUnderThresholdNoTip(t *testing.T) {
	start := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "rate", start)

	// Few calls (under rateGapThreshold) and short elapsed time (under
	// rateGapIdle): neither signal should fire.
	later := start.Add(5 * time.Minute)
	for i := 0; i < 3; i++ {
		Record(&s, "distill", later)
	}

	if tip, ok := GapTip(s, false, later, 0); ok {
		t.Errorf("expected no tip when both count and time are under threshold, got: %q", tip)
	}
}

func TestGapTip_TimeBasedTipRespectsOptOut(t *testing.T) {
	start := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "rate", start)

	later := start.Add(45 * time.Minute)
	for i := 0; i < 12; i++ {
		Record(&s, "distill", later)
	}
	Record(&s, "find", later) // clear the unrelated find-underuse heuristic

	if tip, ok := GapTip(s, true, later, 0); ok {
		t.Errorf("expected feedbackDisabled=true to suppress the time-based tip too, got: %q", tip)
	}
	if _, ok := GapTip(s, false, later, 0); !ok {
		t.Errorf("expected the same state without the opt-out to still produce a time-based tip")
	}
}

func TestGapTip_TimeGapFromFirstCallWhenNeverRated(t *testing.T) {
	// A session that has never called `harnez rate` at all: the time anchor
	// falls back to FirstCallAt, mirroring the count-based fallback to
	// s.Total when LastRateAt is zero.
	start := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "distill", start) // sets FirstCallAt, never rates

	later := start.Add(25 * time.Minute)
	for i := 0; i < 10; i++ {
		Record(&s, "distill", later)
	}

	tip, ok := GapTip(s, false, later, 0)
	if !ok {
		t.Fatalf("expected a time-based tip anchored on FirstCallAt for a never-rated session")
	}
	if !strings.Contains(tip, "harnez rate") {
		t.Errorf("expected tip to mention harnez rate, got: %q", tip)
	}
}

// --- Issue 188: unrated-failure correlation nudge -------------------------

func TestGapTip_UnratedFailuresTriggersDistinctNudge(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 15; i++ {
		Record(&s, "exec", now)
	}

	tip, ok := GapTip(s, false, now, 3)
	if !ok {
		t.Fatalf("expected an unrated-failure tip to fire")
	}
	if !strings.Contains(tip, "3 tool call") {
		t.Errorf("expected tip to name the failure count, got: %q", tip)
	}
	if !strings.Contains(tip, "harnez rate") {
		t.Errorf("expected tip to mention harnez rate, got: %q", tip)
	}
}

func TestGapTip_UnratedFailuresPreemptsOtherTips(t *testing.T) {
	// A session state that would otherwise fire the plain rate-gap tip
	// (>= rateGapThreshold calls since the last rate). With unratedFailures
	// > 0, the distinct nudge must win instead of the generic gap tip.
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < rateGapThreshold+5; i++ {
		Record(&s, "exec", now)
	}

	tip, ok := GapTip(s, false, now, 2)
	if !ok {
		t.Fatalf("expected a tip to fire")
	}
	if !strings.Contains(tip, "2 tool call") {
		t.Errorf("expected the unrated-failure nudge to take priority, got: %q", tip)
	}
}

func TestGapTip_ZeroFailuresNoNudgeRegardlessOfSilence(t *testing.T) {
	// Plenty of calls since the last rate, but zero real failures: the
	// unrated-failure nudge must not fire (that's the plain gap-tip's job,
	// which can still fire on its own).
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < rateGapThreshold+5; i++ {
		Record(&s, "exec", now)
	}

	tip, ok := GapTip(s, false, now, 0)
	if ok && strings.Contains(tip, "tool call(s) failed") {
		t.Errorf("expected no unrated-failure nudge with zero failures, got: %q", tip)
	}
}

func TestGapTip_UnratedFailuresRespectsOptOut(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 15; i++ {
		Record(&s, "exec", now)
	}

	if tip, ok := GapTip(s, true, now, 3); ok && strings.Contains(tip, "tool call(s) failed") {
		t.Errorf("expected feedbackDisabled=true to suppress the unrated-failure nudge, got: %q", tip)
	}
}

func TestGapTip_UnratedFailuresRespectsCooldown(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	for i := 0; i < 15; i++ {
		Record(&s, "exec", now)
	}
	s.TotalAtLastTip = s.Total - (tipCooldown - 1)

	if tip, ok := GapTip(s, false, now, 3); ok {
		t.Errorf("expected the shared tipCooldown gate to suppress a repeat unrated-failure nudge, got: %q", tip)
	}
}
