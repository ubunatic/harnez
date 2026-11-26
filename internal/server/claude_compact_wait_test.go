package server

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/hookdispatch"
)

// Issue 1303 M1b: Claude may append the compact boundary after the PostCompact
// hook has read the transcript. The hook must poll for a fresh boundary within
// the spec bound (spec/hooks.yaml claude_compact), not read once. The clock is
// faked, so these tests never sleep.

const claudeBoundaryLineFmt = `{"type":"system","subtype":"compact_boundary","timestamp":%q,"compactMetadata":{"trigger":"manual","preTokens":%d,"postTokens":%d}}`

const claudeAssistantLineFixture = `{"type":"assistant","timestamp":"2026-11-26T10:00:00Z","message":{"model":"claude-opus-5-5"}}`

// fakeCompactClock replaces the wait clock. Each sleep advances the fake time by
// the requested pause and then calls onSleep with the sleep count, so a test can
// append a boundary at a chosen poll. It returns the sleep counter.
func fakeCompactClock(t *testing.T, start time.Time, onSleep func(call int)) *int {
	t.Helper()
	now := start
	sleeps := 0
	prevNow, prevSleep := claudeCompactNow, claudeCompactSleep
	claudeCompactNow = func() time.Time { return now }
	claudeCompactSleep = func(d time.Duration) {
		sleeps++
		now = now.Add(d)
		if onSleep != nil {
			onSleep(sleeps)
		}
	}
	t.Cleanup(func() { claudeCompactNow, claudeCompactSleep = prevNow, prevSleep })
	return &sleeps
}

func appendTranscriptLine(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

// claudePostCompactRaw is a PostCompact payload. Claude's payload carries
// compact_summary; a Codex payload does not, and the wait applies to Claude only.
func claudePostCompactRaw(path string, claude bool) []byte {
	extra := ""
	if claude {
		extra = `,"compact_summary":"summary"`
	}
	return []byte(fmt.Sprintf(`{"session_id":"claude-wait","transcript_path":%q,"hook_event_name":"PostCompact","trigger":"manual"%s}`, path, extra))
}

func compactFromEvidence(t *testing.T, prepared []byte) *claudeCompact {
	t.Helper()
	var hook struct {
		Context codexEvidence `json:"harnez_context"`
	}
	if err := json.Unmarshal(prepared, &hook); err != nil {
		t.Fatal(err)
	}
	return hook.Context.Compact
}

func loadWaitSpec(t *testing.T) hookdispatch.ClaudeCompactWait {
	t.Helper()
	wait, err := hookdispatch.LoadClaudeCompactWait()
	if err != nil {
		t.Fatal(err)
	}
	return wait
}

// A boundary that lands after the PostCompact hook starts reading is found by
// polling within the spec wait. Before the fix the hook reads once and returns nil.
func TestPrepareClaudePostCompactWaitsForBoundaryFlush(t *testing.T) {
	path := writeTranscript(t, claudeAssistantLineFixture)
	start := time.Now().UTC()
	sleeps := fakeCompactClock(t, start, func(call int) {
		if call == 2 {
			appendTranscriptLine(t, path, fmt.Sprintf(claudeBoundaryLineFmt, start.Format(time.RFC3339Nano), 500, 50))
		}
	})
	prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(path, true))
	if err != nil {
		t.Fatal(err)
	}
	got := compactFromEvidence(t, prepared)
	if got == nil || got.PreTokens != 500 || got.PostTokens != 50 {
		t.Fatalf("Compact = %+v, want the boundary that flushed after two polls", got)
	}
	if *sleeps != 2 {
		t.Errorf("polls = %d, want 2", *sleeps)
	}
}

// With no boundary ever written, the hook gives up exactly at the spec wait.
func TestPrepareClaudePostCompactGivesUpAtSpecBound(t *testing.T) {
	path := writeTranscript(t, claudeAssistantLineFixture)
	sleeps := fakeCompactClock(t, time.Now().UTC(), nil)
	prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(path, true))
	if err != nil {
		t.Fatal(err)
	}
	if got := compactFromEvidence(t, prepared); got != nil {
		t.Errorf("Compact = %+v, want nil when no boundary is written", got)
	}
	wait := loadWaitSpec(t)
	if want := int(wait.Wait / wait.Poll); *sleeps != want {
		t.Errorf("polls = %d, want %d (boundary_wait / boundary_poll)", *sleeps, want)
	}
}

// A complete boundary older than boundary_window is a previous compaction. The
// hook keeps polling and takes the fresh boundary that follows it.
func TestPrepareClaudePostCompactSkipsStaleBoundary(t *testing.T) {
	start := time.Now().UTC()
	stale := start.Add(-2 * time.Hour)
	path := writeTranscript(t,
		fmt.Sprintf(claudeBoundaryLineFmt, stale.Format(time.RFC3339Nano), 900, 90),
		claudeAssistantLineFixture,
	)
	sleeps := fakeCompactClock(t, start, func(call int) {
		if call == 1 {
			appendTranscriptLine(t, path, fmt.Sprintf(claudeBoundaryLineFmt, start.Format(time.RFC3339Nano), 500, 50))
		}
	})
	prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(path, true))
	if err != nil {
		t.Fatal(err)
	}
	got := compactFromEvidence(t, prepared)
	if got == nil || got.PreTokens != 500 {
		t.Fatalf("Compact = %+v, want the fresh boundary (500), not the stale one (900)", got)
	}
	if *sleeps != 1 {
		t.Errorf("polls = %d, want 1", *sleeps)
	}
}

// A payload without compact_summary is not Claude: one read, no wait. Codex
// PostCompact rows must not slow down or change.
func TestPrepareCodexPostCompactNeverWaits(t *testing.T) {
	path := writeTranscript(t, claudeAssistantLineFixture)
	sleeps := fakeCompactClock(t, time.Now().UTC(), nil)
	prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(path, false))
	if err != nil {
		t.Fatal(err)
	}
	if got := compactFromEvidence(t, prepared); got != nil {
		t.Errorf("Compact = %+v, want nil", got)
	}
	if *sleeps != 0 {
		t.Errorf("polls = %d, want 0 for a non-Claude payload", *sleeps)
	}
}
