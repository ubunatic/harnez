package sessionstate

import (
	"strings"
	"testing"
	"time"
)

func TestGapTip_FreshSessionNoTip(t *testing.T) {
	s := State{Calls: map[string]Invocation{}}
	if _, ok := GapTip(s); ok {
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

	tip, ok := GapTip(s)
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

	if tip, ok := GapTip(s); ok {
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

	tip, ok := GapTip(s)
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

	if _, ok := GapTip(s); ok {
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

	if _, ok := GapTip(s); ok {
		t.Errorf("expected cooldown to suppress an immediate repeat tip")
	}

	// After tipCooldown more calls, it should be eligible again.
	for i := 0; i < tipCooldown; i++ {
		Record(&s, "distill", now)
	}
	if _, ok := GapTip(s); !ok {
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
