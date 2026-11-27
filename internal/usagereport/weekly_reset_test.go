package usagereport

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// resetSpanSamples has one 5-hour window (rows at +1h and +3h) that spans a weekly
// reset at +2.5h. Old group A reads 10 at +1h and 12 at +2h, so it rises 2 pp before
// the reset. New group B starts at 30 at +2.5h and reads 33 at +3h, so it rises 3 pp.
func resetSpanSamples() []usage.QuotaHistoryEntry {
	r := t0.Add(6 * time.Hour)
	wkB := t0.Add(14 * 24 * time.Hour)
	return []usage.QuotaHistoryEntry{
		fiveHourRow(t0.Add(time.Hour), 0, &r),
		fiveHourRow(t0.Add(3*time.Hour), 1, &r),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &reset),
		claudeWindow(t0.Add(150*time.Minute), "Weekly (7-day)", 30, &wkB),
		claudeWindow(t0.Add(3*time.Hour), "Weekly (7-day)", 33, &wkB),
	}
}

// Reproduction (ticket 1308 M3 item 1): a 5-hour window that spans a weekly reset
// counts the old group's rise before the reset as well as the new group's rise.
func TestFiveHourWindowWeeklyRiseCountsRiseBeforeReset(t *testing.T) {
	opts := ratioOpts()
	opts.Until = t0.Add(12 * time.Hour)
	samples := resetSpanSamples()
	tl := buildTimeline(samples, opts.ResetTolerance, fiveHourSel)
	wins := fiveHourWindows(samples, tl.rises, opts)
	if len(wins) != 1 {
		t.Fatalf("5h windows = %d, want 1", len(wins))
	}
	if !wins[0].weeklyOK || !samePP(wins[0].weekly, 5) {
		t.Fatalf("window weekly rise = %v (ok %v), want 5: 2 pp before the reset and 3 pp after", wins[0].weekly, wins[0].weeklyOK)
	}
	var sum float64
	for _, w := range wins {
		sum += w.weekly
	}
	if rep := Build(nil, samples, opts); !samePP(sum, rep.RisePct) {
		t.Fatalf("Σ window weekly rises = %v pp, weekly rise_pct = %v pp; want equal", sum, rep.RisePct)
	}
}

// Reproduction (ticket 1308 M3 item 2): when the first weekly sample comes after the
// first 5-hour window starts, its weekly group's first reading is the baseline, so
// the window still has a weekly rise.
func TestFiveHourWindowWeeklyRiseWithLateFirstWeeklySample(t *testing.T) {
	opts := ratioOpts()
	opts.Until = t0.Add(12 * time.Hour)
	r := t0.Add(6 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		fiveHourRow(t0.Add(time.Hour), 0, &r),
		fiveHourRow(t0.Add(3*time.Hour), 1, &r),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(3*time.Hour), "Weekly (7-day)", 12, &reset),
	}
	tl := buildTimeline(samples, opts.ResetTolerance, fiveHourSel)
	wins := fiveHourWindows(samples, tl.rises, opts)
	if len(wins) != 1 {
		t.Fatalf("5h windows = %d, want 1", len(wins))
	}
	if !wins[0].weeklyOK || !samePP(wins[0].weekly, 2) {
		t.Fatalf("window weekly rise = %v (ok %v), want 2 from the group's first reading 10", wins[0].weekly, wins[0].weeklyOK)
	}
}
