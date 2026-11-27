package usagereport

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// fiveHourRow is one Claude 5-hour window row; its reset_at ends the window.
func fiveHourRow(at time.Time, used int, resetAt *time.Time) usage.QuotaHistoryEntry {
	return claudeWindow(at, usage.ClaudeFiveHourWindowName, used, resetAt)
}

func samePP(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

// The 5-hour share splits a rise across sessions by API cost, as the weekly share
// does, and the session shares plus the not-explained part add up to the rise.
func TestFiveHourShareSplitsBySessionCost(t *testing.T) {
	r := t0.Add(5 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		fiveHourRow(t0, 0, &r),
		fiveHourRow(t0.Add(2*time.Hour), 1, &r),
	}
	calls := []Call{
		call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("B", "b1", t0.Add(90*time.Minute), "claude-sonnet-5-5", 3_000_000),
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	if rep.FiveHourIntervals != 1 || !samePP(rep.FiveHourRisePct, 1) || rep.FiveHourNotExplainedPct != 0 {
		t.Fatalf("5h rises = %d, rise %v pp, not explained %v pp; want 1, 1, 0",
			rep.FiveHourIntervals, rep.FiveHourRisePct, rep.FiveHourNotExplainedPct)
	}
	if got := rowFor(t, rep, "A").FiveHourQuotaPct; got == nil || !samePP(*got, 0.25) {
		t.Fatalf("session A 5h share = %v, want 0.25 pp (1 of 4 cost units)", got)
	}
	if got := rowFor(t, rep, "B").FiveHourQuotaPct; got == nil || !samePP(*got, 0.75) {
		t.Fatalf("session B 5h share = %v, want 0.75 pp", got)
	}
	if rep.Totals.FiveHourQuotaPct == nil || !samePP(*rep.Totals.FiveHourQuotaPct, 1) {
		t.Fatalf("totals 5h share = %v, want 1 pp", rep.Totals.FiveHourQuotaPct)
	}
	// The weekly report has no weekly rows here: its figures stay zero.
	if rep.RisePct != 0 || len(rep.Intervals) != 0 {
		t.Fatalf("weekly rise %v pp, %d intervals from 5-hour rows; want none", rep.RisePct, len(rep.Intervals))
	}
}

// Rises are measured per 5-hour window with the weekly path's rules: a reset is a
// new window, the first non-zero sample is a baseline, a dip and recovery is one
// rise, and a 100% cap still counts.
func TestFiveHourRiseMeasurement(t *testing.T) {
	r1 := t0.Add(5 * time.Hour)
	r2 := t0.Add(10 * time.Hour)
	tests := []struct {
		name          string
		samples       []usage.QuotaHistoryEntry
		wantPct       float64
		wantIntervals int
	}{
		{"reset inside range", []usage.QuotaHistoryEntry{
			fiveHourRow(t0.Add(time.Hour), 40, &r1),
			fiveHourRow(t0.Add(4*time.Hour), 60, &r1),
			fiveHourRow(t0.Add(6*time.Hour), 0, &r2),
			fiveHourRow(t0.Add(7*time.Hour), 5, &r2),
		}, 25, 2},
		{"first sample non-zero is the baseline", []usage.QuotaHistoryEntry{
			fiveHourRow(t0.Add(time.Hour), 30, &r1),
			fiveHourRow(t0.Add(2*time.Hour), 32, &r1),
		}, 2, 1},
		{"dip and recovery is one rise, then a new one", []usage.QuotaHistoryEntry{
			fiveHourRow(t0.Add(time.Hour), 10, &r1),
			fiveHourRow(t0.Add(2*time.Hour), 12, &r1),
			fiveHourRow(t0.Add(150*time.Minute), 11, &r1),
			fiveHourRow(t0.Add(3*time.Hour), 13, &r1),
		}, 3, 2},
		{"100 percent cap counts", []usage.QuotaHistoryEntry{
			fiveHourRow(t0, 98, &r1),
			fiveHourRow(t0.Add(2*time.Hour), 100, &r1),
		}, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No calls: every rise is then not explained, so the sums are the rises.
			rep := Build(nil, tt.samples, baseOpts(t0, t0.Add(8*time.Hour)))
			if rep.FiveHourIntervals != tt.wantIntervals || !samePP(rep.FiveHourRisePct, tt.wantPct) {
				t.Fatalf("5h intervals %d, rise %v pp; want %d, %v pp",
					rep.FiveHourIntervals, rep.FiveHourRisePct, tt.wantIntervals, tt.wantPct)
			}
			if !samePP(rep.FiveHourNotExplainedPct, tt.wantPct) {
				t.Fatalf("not explained %v pp, want the whole rise %v pp", rep.FiveHourNotExplainedPct, tt.wantPct)
			}
		})
	}
}

// The 5-hour window has no weekly-equivalent ratio yet (M2), so its quiet check is
// off: a rise whose cost is far below its plan price still gets its share.
func TestFiveHourQuietCheckIsOff(t *testing.T) {
	r := t0.Add(5 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		fiveHourRow(t0, 0, &r),
		fiveHourRow(t0.Add(2*time.Hour), 1, &r),
	}
	calls := []Call{call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1000)}
	opts := baseOpts(t0, t0.Add(4*time.Hour))
	opts.MinLocalFactor = 0.5
	rep := Build(calls, samples, opts)
	if rep.FiveHourNotExplainedPct != 0 {
		t.Fatalf("5h not explained %v pp, want 0: the quiet check must be off for 5h", rep.FiveHourNotExplainedPct)
	}
	if got := rowFor(t, rep, "A").FiveHourQuotaPct; got == nil || !samePP(*got, 1) {
		t.Fatalf("session A 5h share = %v, want the whole 1 pp", got)
	}
}

// Without 5-hour history the new fields stay nil or zero, and the weekly figures
// are unchanged.
func TestFiveHourAbsentWithoutHistory(t *testing.T) {
	samples, calls := flatFixture()
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	if got := rowFor(t, rep, "A").FiveHourQuotaPct; got != nil {
		t.Fatalf("session A 5h share = %v, want nil without 5-hour rows", *got)
	}
	if rep.Totals.FiveHourQuotaPct != nil || rep.FiveHourIntervals != 0 {
		t.Fatalf("totals 5h share %v, %d intervals; want nil and 0", rep.Totals.FiveHourQuotaPct, rep.FiveHourIntervals)
	}
	if rep.Totals.QuotaPct == nil || !samePP(*rep.Totals.QuotaPct, 1) {
		t.Fatalf("weekly totals quota %v, want the unchanged 1 pp", rep.Totals.QuotaPct)
	}
}

// The text report gets a 5-hour column and a summary line when there are rises.
func TestRenderShowsFiveHourColumn(t *testing.T) {
	r := t0.Add(5 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		fiveHourRow(t0, 0, &r),
		fiveHourRow(t0.Add(2*time.Hour), 1, &r),
	}
	calls := []Call{call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "QUOTA 5H %") {
		t.Fatalf("render has no QUOTA 5H %% column:\n%s", out)
	}
	if !strings.Contains(out, "5-hour window: 1 rise(s), 1.00 pp in range, 0.00 pp not explained locally") {
		t.Fatalf("render has no 5-hour summary line:\n%s", out)
	}
}
