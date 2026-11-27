package usagereport

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// gapSamples has two weekly groups with a gap between them. Group A (reset +7d)
// reads 10 then 12. Group B (reset +14d) first reads 40 at +20h, then 41. Each 5-hour
// window reads 0 then 1 (window 1) and 39 then 40 (window 2, whose first sample sits
// at the pre-existing level).
func gapSamples() []usage.QuotaHistoryEntry {
	r1 := t0.Add(5 * time.Hour)
	r2 := t0.Add(25 * time.Hour)
	wkB := t0.Add(14 * 24 * time.Hour)
	return []usage.QuotaHistoryEntry{
		fiveHourRow(t0.Add(time.Hour), 0, &r1),
		fiveHourRow(t0.Add(2*time.Hour), 1, &r1),
		fiveHourRow(t0.Add(20*time.Hour), 39, &r2),
		fiveHourRow(t0.Add(21*time.Hour), 40, &r2),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &reset),
		claudeWindow(t0.Add(20*time.Hour), "Weekly (7-day)", 40, &wkB),
		claudeWindow(t0.Add(21*time.Hour), "Weekly (7-day)", 41, &wkB),
	}
}

// Reproduction (ticket 1308 M2b item 1): after a weekly reset the new group's first
// reading is its baseline, so the 40 that was there before the gap is not a rise. The
// second window's weekly rise is 41 - 40 = 1, not 41, and the ratio is 2 / 3.
func TestFiveHourWindowWeeklyRiseSkipsGapBaseline(t *testing.T) {
	opts := ratioOpts()
	opts.MinRatioWeeklyPct = 0.5
	opts.Until = t0.Add(48 * time.Hour)
	samples := gapSamples()
	tl := buildTimeline(samples, opts.ResetTolerance, fiveHourSel)
	wins := fiveHourWindows(samples, tl.rises, opts)
	if len(wins) != 2 {
		t.Fatalf("5h windows = %d, want 2", len(wins))
	}
	if !wins[1].weeklyOK || !samePP(wins[1].weekly, 1) {
		t.Fatalf("window 2 weekly rise = %v (ok %v), want 1 pp: the 40 is the new group's baseline", wins[1].weekly, wins[1].weeklyOK)
	}
	ratio, n, ok := fiveHourRatio(wins, opts)
	if !ok || n != 2 || !samePP(ratio, 2.0/3) {
		t.Fatalf("ratio = %v over %d windows (ok %v), want 2/3 over 2", ratio, n, ok)
	}
}

// Partition sum (ticket 1308 M2b item 2): over the in-range 5-hour windows, the
// weekly rises add up to the weekly report's own rise, the gap fixture included.
func TestFiveHourWindowWeeklyRisesSumToWeeklyRise(t *testing.T) {
	opts := ratioOpts()
	opts.MinRatioWeeklyPct = 0.5
	opts.Until = t0.Add(48 * time.Hour)
	samples := gapSamples()
	tl := buildTimeline(samples, opts.ResetTolerance, fiveHourSel)
	var sum float64
	for _, w := range fiveHourWindows(samples, tl.rises, opts) {
		if w.weeklyOK {
			sum += w.weekly
		}
	}
	rep := Build(nil, samples, opts)
	if !samePP(sum, rep.RisePct) {
		t.Fatalf("Σ window weekly rises = %v pp, weekly rise_pct = %v pp; want equal", sum, rep.RisePct)
	}
}
