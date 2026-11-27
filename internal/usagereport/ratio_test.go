package usagereport

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// ratioOpts is baseOpts with the M2 thresholds: a 5-hour window counts in the ratio
// from 4 pp of weekly rise, and the quiet check is on at 1x.
func ratioOpts() Options {
	opts := baseOpts(t0, t0.Add(12*time.Hour))
	opts.MinRatioWeeklyPct = 4
	opts.MinSessionFiveHourPct = 1
	opts.MinLocalFactor = 1
	return opts
}

// twoWindowSamples has two 5-hour windows (resets at +5h and +10h), each with its own
// 5-hour rise, and weekly readings whose rises are 4 pp in window 1 and the given
// amount in window 2 (the weekly peak climbs 10 -> 14 -> 14+w2).
func twoWindowSamples(fiveH1, fiveH2 int, weekly2 int) []usage.QuotaHistoryEntry {
	r1 := t0.Add(5 * time.Hour)
	r2 := t0.Add(10 * time.Hour)
	return []usage.QuotaHistoryEntry{
		fiveHourRow(t0.Add(time.Hour), 0, &r1),
		fiveHourRow(t0.Add(4*time.Hour), fiveH1, &r1),
		fiveHourRow(t0.Add(6*time.Hour), 0, &r2),
		fiveHourRow(t0.Add(9*time.Hour), fiveH2, &r2),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(4*time.Hour), "Weekly (7-day)", 14, &reset),
		claudeWindow(t0.Add(9*time.Hour), "Weekly (7-day)", 14+weekly2, &reset),
	}
}

// The range ratio is the sum of 5-hour pp over the sum of weekly pp of the windows
// that qualify, and the weekly-equivalent total is the 5-hour total over the ratio.
func TestFiveHourRatioSumsQualifyingWindows(t *testing.T) {
	// Each window has a priced call large enough to pass the quiet check at its
	// weekly-equivalent pp, so the attributed 5-hour total is the whole 5 pp.
	calls := []Call{
		call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 2_000_000),
		call("A", "a2", t0.Add(7*time.Hour), "claude-sonnet-5-5", 2_000_000),
	}
	rep := Build(calls, twoWindowSamples(2, 3, 6), ratioOpts())
	if rep.FiveHourRatio == nil || !samePP(*rep.FiveHourRatio, 0.5) {
		t.Fatalf("ratio = %v, want 0.5 (5 pp over 10 weekly pp)", rep.FiveHourRatio)
	}
	if rep.FiveHourRatioWindows != 2 {
		t.Fatalf("ratio windows = %d, want 2", rep.FiveHourRatioWindows)
	}
	if rep.Totals.FiveHourQuotaPct == nil || !samePP(*rep.Totals.FiveHourQuotaPct, 5) {
		t.Fatalf("attributed 5h total = %v, want 5 pp", rep.Totals.FiveHourQuotaPct)
	}
	if rep.Totals.FiveHourWeeklyEqPct == nil || !samePP(*rep.Totals.FiveHourWeeklyEqPct, 10) {
		t.Fatalf("weekly-equivalent total = %v, want 10 pp", rep.Totals.FiveHourWeeklyEqPct)
	}
}

// A 5-hour window whose weekly rise is under min_ratio_weekly_pct is noise and is
// left out of the ratio, as are its 5-hour pp.
func TestFiveHourRatioSkipsSmallWeeklyWindows(t *testing.T) {
	rep := Build(nil, twoWindowSamples(2, 3, 2), ratioOpts())
	if rep.FiveHourRatio == nil || !samePP(*rep.FiveHourRatio, 0.5) {
		t.Fatalf("ratio = %v, want 0.5 from window 1 alone (2 pp over 4 weekly pp)", rep.FiveHourRatio)
	}
	if rep.FiveHourRatioWindows != 1 {
		t.Fatalf("ratio windows = %d, want 1", rep.FiveHourRatioWindows)
	}
}

// Regression (25bf9316 shape): 2 pp of 5-hour rise at a ratio of 12 is 0.1667
// weekly-equivalent pp. Its $0.0019 of local API cost is far below the plan price of
// that quota, so the rise is quiet: no share and no factor.
func TestFiveHourQuietUsesWeeklyEquivalent(t *testing.T) {
	samples := twoWindowSamples(2, 94, 4)
	calls := []Call{call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 1900)}
	rep := Build(calls, samples, ratioOpts())
	if rep.FiveHourRatio == nil || !samePP(*rep.FiveHourRatio, 12) {
		t.Fatalf("ratio = %v, want 12", rep.FiveHourRatio)
	}
	if got := rowFor(t, rep, "A").FiveHourQuotaPct; got != nil && *got != 0 {
		t.Fatalf("session A 5h share = %v, want none: the rise is quiet", *got)
	}
	if !samePP(rep.FiveHourNotExplainedPct, 96) {
		t.Fatalf("5h not explained = %v pp, want 96 (the quiet 2 pp and the uncalled 94 pp)", rep.FiveHourNotExplainedPct)
	}
}

// Near miss of the same rise: $0.0400 of local API cost is above the plan price of
// its 0.1667 weekly-equivalent pp (factor about 1.04), so it is attributed, measured
// from the session's own calls, with the weekly-equivalent pp and the factor.
func TestFiveHourFactorOnNearMiss(t *testing.T) {
	samples := twoWindowSamples(2, 94, 4)
	calls := []Call{call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 40_000)}
	rep := Build(calls, samples, ratioOpts())
	row := rowFor(t, rep, "A")
	if row.FiveHourQuotaPct == nil || !samePP(*row.FiveHourQuotaPct, 2) {
		t.Fatalf("session A 5h share = %v, want 2 pp", row.FiveHourQuotaPct)
	}
	if row.FiveHourWeeklyEqPct == nil || !samePP(*row.FiveHourWeeklyEqPct, 2.0/12) {
		t.Fatalf("session A weekly-equivalent = %v, want 0.1667", row.FiveHourWeeklyEqPct)
	}
	if row.FiveHourFactor == nil || *row.FiveHourFactor < 1.03 || *row.FiveHourFactor > 1.05 {
		t.Fatalf("session A 5h factor = %v, want about 1.04", row.FiveHourFactor)
	}
	if row.FiveHourFactorBasis != "measured" {
		t.Fatalf("5h factor basis = %q, want measured (A was the only session in the rise)", row.FiveHourFactorBasis)
	}
	if len(rep.Models) != 1 || rep.Models[0].Model != "claude-sonnet-5-5" || rep.Models[0].Factor == nil {
		t.Fatalf("models = %+v, want one sonnet row with a factor", rep.Models)
	}
}

// Without weekly history there is no ratio: the weekly-equivalent and factor stay
// n/a, and the text says why.
func TestFiveHourNoRatioSaysWhy(t *testing.T) {
	r := t0.Add(5 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		fiveHourRow(t0, 0, &r),
		fiveHourRow(t0.Add(2*time.Hour), 1, &r),
	}
	calls := []Call{call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, ratioOpts())
	if rep.FiveHourRatio != nil || rep.Totals.FiveHourWeeklyEqPct != nil || rowFor(t, rep, "A").FiveHourFactor != nil {
		t.Fatalf("ratio %v, totals eq %v; want both nil without weekly history", rep.FiveHourRatio, rep.Totals.FiveHourWeeklyEqPct)
	}
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no range ratio") {
		t.Fatalf("render does not say there is no ratio:\n%s", buf.String())
	}
}

// The basis is measured when at least half of the session's 5-hour pp comes from
// rises where it was the only session, and split otherwise.
func TestBasisOf(t *testing.T) {
	if got := basisOf(2, 2); got != "measured" {
		t.Fatalf("basisOf(2, 2) = %q, want measured", got)
	}
	if got := basisOf(1, 2); got != "measured" {
		t.Fatalf("basisOf(1, 2) = %q, want measured at exactly half", got)
	}
	if got := basisOf(0.5, 2); got != "split" {
		t.Fatalf("basisOf(0.5, 2) = %q, want split", got)
	}
}
