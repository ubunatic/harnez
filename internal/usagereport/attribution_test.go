package usagereport

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// flatFixture: the window starts at t0 (10:00 UTC) and the value is 10 at 11:00
// and 12:00, then rises to 11 at 13:00. Calls at 11:30 and 12:30 build that
// step; a call at 13:30 comes after the last rise, so it is pending.
func flatFixture() ([]usage.QuotaHistoryEntry, []Call) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(3*time.Hour), "Weekly (7-day)", 11, &reset),
	}
	calls := []Call{
		call("A", "a1", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "a2", t0.Add(150*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "a3", t0.Add(210*time.Minute), "claude-sonnet-5-5", 1_000_000),
	}
	return samples, calls
}

// Calls during a flat stretch build the next 1% step, so they belong to the
// rise that ends it: the interval runs from the window start (or the previous
// rise) to the sample where the value rises, not from the sample just before.
func TestBuildFlatStretchCallsGetTheRise(t *testing.T) {
	samples, calls := flatFixture()
	calls = calls[:2]
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	if len(rep.Intervals) != 1 {
		t.Fatalf("intervals = %d, want 1", len(rep.Intervals))
	}
	iv := rep.Intervals[0]
	// The first sample (10% at 11:00) is the baseline, so the interval starts there.
	if !iv.Start.Equal(t0.Add(time.Hour)) || !iv.End.Equal(t0.Add(3*time.Hour)) {
		t.Fatalf("interval %v..%v, want first sample %v to the rise at 3h", iv.Start, iv.End, t0.Add(time.Hour))
	}
	if got := iv.CostMicros["A"]; got != 2_000_000 {
		t.Fatalf("attributed cost = %d micro-USD, want both flat-stretch calls (2000000)", got)
	}
	if got := iv.Share["A"]; got != 1 {
		t.Fatalf("share = %v pp, want the whole 1 pp rise", got)
	}
}

// attributedFactor is API micros over the plan micros of pct, with the plan's
// net rounded to whole micros as Build rounds it.
func attributedFactor(api int64, pct float64) float64 {
	return float64(api) / math.Round(pct/100*float64(baseWeeklyPlan))
}

// A call after the last rise has no quota step yet: it is pending, gets no
// share, and stays out of the factor. Attributed + pending = all API cost.
func TestBuildPendingCallsGetNoShare(t *testing.T) {
	samples, calls := flatFixture()
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	a := rowFor(t, rep, "A")
	if a.QuotaPct == nil || math.Abs(*a.QuotaPct-1) > 1e-9 {
		t.Fatalf("A quota = %v, want 1 pp", a.QuotaPct)
	}
	if a.AttributedCostMicros == nil || *a.AttributedCostMicros != 2_000_000 {
		t.Fatalf("A attributed = %v, want 2000000", a.AttributedCostMicros)
	}
	if a.UnattributedCostMicros == nil || *a.UnattributedCostMicros != 1_000_000 {
		t.Fatalf("A unattributed = %v, want 1000000", a.UnattributedCostMicros)
	}
	if a.APICostMicros == nil || *a.APICostMicros != 3_000_000 {
		t.Fatalf("A api cost = %v, want 3000000", a.APICostMicros)
	}
	if a.Factor == nil || math.Abs(*a.Factor-attributedFactor(2_000_000, 1)) > 1e-9 {
		t.Fatalf("A factor = %v, want attributed 2 USD over 1 pp of the plan", a.Factor)
	}
	tot := rep.Totals
	if tot.AttributedCostMicros == nil || tot.UnattributedCostMicros == nil || tot.APICostMicros == nil ||
		*tot.AttributedCostMicros+*tot.UnattributedCostMicros != *tot.APICostMicros {
		t.Fatalf("totals attributed+pending != api cost: %+v", tot)
	}
}

// One base for totals: the total factor is attributed API cost over attributed
// plan cost. The window line is all local calls over the whole rise; the rise
// is attributed plus not explained.
func TestBuildTotalsUseOneBaseAndWindowLineAddsUp(t *testing.T) {
	samples, calls := flatFixture()
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	tot := rep.Totals
	if tot.Factor == nil || math.Abs(*tot.Factor-attributedFactor(2_000_000, 1)) > 1e-9 {
		t.Fatalf("total factor = %v, want attributed cost over attributed plan", tot.Factor)
	}
	if rep.Window == nil {
		t.Fatalf("window line missing")
	}
	if rep.Window.APICostMicros != 3_000_000 || rep.Window.RisePct != 1 {
		t.Fatalf("window = %+v, want all 3 USD over the 1 pp rise", rep.Window)
	}
	if rep.Window.Factor == nil || math.Abs(*rep.Window.Factor-3_000_000/(0.01*float64(baseWeeklyPlan))) > 1e-9 {
		t.Fatalf("window factor = %v", rep.Window.Factor)
	}
	if *tot.AttributedCostMicros+*tot.UnattributedCostMicros != rep.Window.APICostMicros {
		t.Fatalf("attributed + unattributed = %d, window api = %d", *tot.AttributedCostMicros+*tot.UnattributedCostMicros, rep.Window.APICostMicros)
	}
	if math.Abs(rep.RisePct-(*tot.QuotaPct+rep.UnexplainedPct)) > 1e-9 {
		t.Fatalf("attributed %v + not explained %v != rise %v", *tot.QuotaPct, rep.UnexplainedPct, rep.RisePct)
	}
}

// A session with less than the threshold attributed has no factor: its factor
// would be driven by the 1% step. Totals are not thresholded.
func TestBuildSessionBelowThresholdHasNoFactor(t *testing.T) {
	reset := t0.Add(7 * 24 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 12, &reset),
	}
	calls := []Call{
		call("A", "a1", t0.Add(10*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("B", "b1", t0.Add(20*time.Minute), "claude-sonnet-5-5", 3_000_000),
	}
	opts := baseOpts(t0, t0.Add(2*time.Hour))
	opts.MinSessionQuotaPct = 1
	rep := Build(calls, samples, opts)
	a, b := rowFor(t, rep, "A"), rowFor(t, rep, "B")
	if a.QuotaPct == nil || math.Abs(*a.QuotaPct-0.5) > 1e-9 || a.Factor != nil {
		t.Fatalf("A (0.5 pp) quota=%v factor=%v, want factor n/a", a.QuotaPct, a.Factor)
	}
	if b.QuotaPct == nil || b.Factor == nil {
		t.Fatalf("B (1.5 pp) quota=%v factor=%v, want a factor", b.QuotaPct, b.Factor)
	}
	if rep.Totals.Factor == nil {
		t.Fatalf("totals factor is nil; totals are not thresholded")
	}
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "n/a (< 1 pp)") {
		t.Fatalf("text misses the below-threshold marker:\n%s", buf.String())
	}
}

// The hour table covers every attributed call, including those in a flat
// stretch; a call after the last rise appears in no hour (pending).
func TestBuildHourTableCoversFlatStretch(t *testing.T) {
	samples, calls := flatFixture()
	opts := baseOpts(t0, t0.Add(4*time.Hour))
	opts.Location = time.UTC
	rep := Build(calls, samples, opts)
	// a1 at 11:30 UTC and a2 at 12:30 UTC each take half the 1 pp rise.
	for _, h := range []int{11, 12} {
		row := rep.Hours[h]
		if row.Samples != 1 || row.QuotaPct == nil || math.Abs(*row.QuotaPct-0.5) > 1e-9 {
			t.Fatalf("hour %d = %+v, want 1 sample and 0.5 pp", h, row)
		}
	}
	if rep.Hours[13].APICostMicros != nil || rep.Hours[13].Samples != 0 {
		t.Fatalf("hour 13 = %+v, want n/a: the 13:30 call is pending", rep.Hours[13])
	}
}
