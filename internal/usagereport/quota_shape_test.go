package usagereport

import (
	"math"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// Rows in the live quota-history shape (checked 2026-11-26): Claude's weekly
// row is agent "claude", no group, window "Weekly (7-day)", and its reset_at
// drifts by a few hundred ms between samples. agy rows carry a group, window
// "weekly" and no reset_at. Only field shapes are copied, never file content.
func claudeWindow(at time.Time, window string, used int, resetAt *time.Time) usage.QuotaHistoryEntry {
	return usage.QuotaHistoryEntry{Timestamp: at, Agent: "claude", Window: window, UsedPercent: used, ResetAt: resetAt}
}

func agyRow(at time.Time, group string, used int) usage.QuotaHistoryEntry {
	return usage.QuotaHistoryEntry{Timestamp: at, Agent: "agy", Group: group, Window: "weekly", UsedPercent: used}
}

// The report must read only Claude's weekly window: interleaved session, tokens
// and agy rows (two groups, no reset_at) must not add or mix any rise.
func TestBuildReadsOnlyClaudeWeeklyWindow(t *testing.T) {
	reset := t0.Add(7 * 24 * time.Hour)
	drift := reset.Add(-302 * time.Millisecond)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 66, &drift),
		claudeWindow(t0, "Session (5-hour)", 20, &drift),
		claudeWindow(t0, "tokens", 0, nil),
		agyRow(t0, "Gemini Models", 0),
		agyRow(t0, "Claude and GPT models", 12),
		claudeWindow(t0.Add(30*time.Minute), "Weekly (7-day)", 68, &reset),
		claudeWindow(t0.Add(30*time.Minute), "Session (5-hour)", 25, &reset),
		agyRow(t0.Add(30*time.Minute), "Gemini Models", 0),
		agyRow(t0.Add(30*time.Minute), "Claude and GPT models", 12),
	}
	calls := []Call{call("A", "a1", t0.Add(10*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(time.Hour)))
	if len(rep.Intervals) != 1 || rep.RisePct != 2 {
		t.Fatalf("intervals=%d rise=%v, want one 2 pp rise from the Claude weekly window only", len(rep.Intervals), rep.RisePct)
	}
	a := rowFor(t, rep, "A")
	if rep.UnexplainedPct != 0 || a.QuotaPct == nil || math.Abs(*a.QuotaPct-2) > 1e-9 {
		t.Fatalf("unexplained=%v session A quota=%v, want 0 and 2 pp", rep.UnexplainedPct, a.QuotaPct)
	}
}

// 70 -> 69 -> 71 is a 1 pp rise measured from the 70% peak, not a 2 pp rise
// from the 69% dip.
func TestBuildFallIsNotCountedTwice(t *testing.T) {
	reset := t0.Add(7 * 24 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 70, &reset),
		claudeWindow(t0.Add(10*time.Minute), "Weekly (7-day)", 69, &reset),
		claudeWindow(t0.Add(20*time.Minute), "Weekly (7-day)", 71, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(15*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(time.Hour)))
	if rep.RisePct != 1 || len(rep.Intervals) != 1 || !rep.Intervals[0].Start.Equal(t0) {
		t.Fatalf("rise=%v intervals=%+v, want one 1 pp rise starting at the 70%% peak", rep.RisePct, rep.Intervals)
	}
}

// Invariants on a two-window history with a fall: every rise is split exactly
// between sessions and the not-explained remainder, no window exceeds 100 pp,
// and no session carries more than the total rise.
func TestBuildInvariantsPerWindow(t *testing.T) {
	r1 := t0.Add(7 * 24 * time.Hour)
	r2 := t0.Add(14 * 24 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &r1),
		claudeWindow(t0.Add(10*time.Minute), "Weekly (7-day)", 14, &r1),
		claudeWindow(t0.Add(20*time.Minute), "Weekly (7-day)", 13, &r1),
		claudeWindow(t0.Add(30*time.Minute), "Weekly (7-day)", 16, &r1),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 0, &r2),
		claudeWindow(t0.Add(90*time.Minute), "Weekly (7-day)", 5, &r2),
	}
	calls := []Call{
		call("A", "a1", t0.Add(5*time.Minute), "claude-sonnet-5-5", 2_000_000),
		call("B", "b1", t0.Add(25*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "a2", t0.Add(70*time.Minute), "claude-sonnet-5-5", 3_000_000),
		call("B", "b2", t0.Add(80*time.Minute), "claude-sonnet-5-5", 1_000_000),
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	// Peak-based rises: 10->14 (+4), 14->16 (+2; the 13 dip is skipped), 0->5 (+5).
	if rep.RisePct != 11 || rep.UnexplainedPct != 0 {
		t.Fatalf("rise=%v unexplained=%v, want 11 and 0", rep.RisePct, rep.UnexplainedPct)
	}
	// Each window's rises stay within 100 pp: the first window (before r2) sums
	// to 6 pp, the second to 5 pp, and neither may exceed 100.
	var w1, w2 float64
	for _, iv := range rep.Intervals {
		if iv.End.Before(t0.Add(time.Hour)) {
			w1 += iv.DeltaPct
		} else {
			w2 += iv.DeltaPct
		}
	}
	if w1 != 6 || w2 != 5 || w1 > 100 || w2 > 100 {
		t.Fatalf("per-window rises = %v and %v, want 6 and 5, each at most 100 pp", w1, w2)
	}
	total := rep.UnexplainedPct
	for _, r := range rep.Sessions {
		if r.QuotaPct == nil {
			continue
		}
		total += *r.QuotaPct
		if *r.QuotaPct > rep.RisePct+1e-9 {
			t.Fatalf("session %s quota %v exceeds the rise %v", r.Session, *r.QuotaPct, rep.RisePct)
		}
	}
	if math.Abs(total-rep.RisePct) > 1e-9 {
		t.Fatalf("sessions plus not-explained = %v, want the rise %v", total, rep.RisePct)
	}
	for _, iv := range rep.Intervals {
		var sum float64
		for _, s := range iv.Share {
			sum += s
		}
		if iv.Attributed && math.Abs(sum-iv.DeltaPct) > 1e-9 {
			t.Fatalf("interval %v..%v shares sum to %v, want %v", iv.Start, iv.End, sum, iv.DeltaPct)
		}
	}
	// Hand split: A 4 (first rise) + 3.75 (second window), B 2 + 1.25.
	if a := rowFor(t, rep, "A"); a.QuotaPct == nil || math.Abs(*a.QuotaPct-7.75) > 1e-9 {
		t.Fatalf("A quota = %v, want 7.75", a.QuotaPct)
	}
	if b := rowFor(t, rep, "B"); b.QuotaPct == nil || math.Abs(*b.QuotaPct-3.25) > 1e-9 {
		t.Fatalf("B quota = %v, want 3.25", b.QuotaPct)
	}
}
