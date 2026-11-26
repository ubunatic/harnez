package usagereport

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// hourlyFixture: weekly rises 10->12 over 10:00-11:00 UTC (calls 10:30 cost 1M
// and 10:40 cost 3M, all attributed) and 12->13 over 13:00-14:00 UTC (call
// 13:30 cost 1M). The plan price is the baseOpts weekly price.
func hourlyFixture() ([]usage.QuotaHistoryEntry, []Call) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 12, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &reset),
		claudeWindow(t0.Add(3*time.Hour), "Weekly (7-day)", 12, &reset),
		claudeWindow(t0.Add(4*time.Hour), "Weekly (7-day)", 13, &reset),
	}
	calls := []Call{
		call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("B", "b1", t0.Add(40*time.Minute), "claude-sonnet-5-5", 3_000_000),
		call("A", "a2", t0.Add(3*time.Hour+30*time.Minute), "claude-sonnet-5-5", 1_000_000),
	}
	return samples, calls
}

// hourFactor is API micro-USD over (quota pp / 100 x weekly plan micro-USD).
func hourFactor(cost int64, pct float64) float64 {
	return float64(cost) / (pct / 100 * float64(baseWeeklyPlan))
}

const baseWeeklyPlan = int64(23_076_923)

func TestBuildHourlyFactorByLocalHour(t *testing.T) {
	samples, calls := hourlyFixture()
	opts := baseOpts(t0, t0.Add(5*time.Hour))
	opts.Location = time.UTC
	rep := Build(calls, samples, opts)
	if rep.UnexplainedPct != 0 {
		t.Fatalf("unexplained = %v, want 0 for the fixture", rep.UnexplainedPct)
	}
	if len(rep.Hours) != 24 {
		t.Fatalf("hours = %d rows, want 24", len(rep.Hours))
	}
	h10 := rep.Hours[10]
	if h10.Samples != 1 || h10.APICostMicros == nil || *h10.APICostMicros != 4_000_000 ||
		h10.QuotaPct == nil || math.Abs(*h10.QuotaPct-2) > 1e-9 ||
		h10.Factor == nil || math.Abs(*h10.Factor-hourFactor(4_000_000, 2)) > 1e-9 {
		t.Fatalf("hour 10 = %+v, want 1 sample, 4 USD, 2 pp, factor %v", h10, hourFactor(4_000_000, 2))
	}
	h13 := rep.Hours[13]
	if h13.Samples != 1 || h13.APICostMicros == nil || *h13.APICostMicros != 1_000_000 ||
		h13.Factor == nil || math.Abs(*h13.Factor-hourFactor(1_000_000, 1)) > 1e-9 {
		t.Fatalf("hour 13 = %+v, want 1 sample, 1 USD and factor %v", h13, hourFactor(1_000_000, 1))
	}
	// No rise or no cost in hour 11: n/a, never zero.
	h11 := rep.Hours[11]
	if h11.Samples != 0 || h11.APICostMicros != nil || h11.QuotaPct != nil || h11.Factor != nil {
		t.Fatalf("hour 11 = %+v, want all n/a", h11)
	}
}

func TestBuildHourlyUsesLocalTime(t *testing.T) {
	samples, calls := hourlyFixture()
	opts := baseOpts(t0, t0.Add(5*time.Hour))
	opts.Location = time.FixedZone("UTC+2", 2*3600)
	rep := Build(calls, samples, opts)
	// 10:30-10:40 UTC is 12:30-12:40 at UTC+2.
	if rep.Hours[12].APICostMicros == nil || *rep.Hours[12].APICostMicros != 4_000_000 {
		t.Fatalf("local hour 12 = %+v, want the 4 USD of the 10:xx UTC calls", rep.Hours[12])
	}
	if rep.Hours[10].APICostMicros != nil {
		t.Fatalf("local hour 10 = %+v, want n/a (calls were not in local hour 10)", rep.Hours[10])
	}
}

func TestBuildSeriesHasOnePointPerAttributedRise(t *testing.T) {
	samples, calls := hourlyFixture()
	opts := baseOpts(t0, t0.Add(5*time.Hour))
	opts.Location = time.UTC
	rep := Build(calls, samples, opts)
	if len(rep.Series) != 2 {
		t.Fatalf("series = %d points, want 2", len(rep.Series))
	}
	first := rep.Series[0]
	if !first.Start.Equal(t0) || !first.End.Equal(t0.Add(time.Hour)) ||
		first.APICostMicros != 4_000_000 || math.Abs(first.QuotaPct-2) > 1e-9 ||
		first.Factor == nil || math.Abs(*first.Factor-hourFactor(4_000_000, 2)) > 1e-9 {
		t.Fatalf("first series point = %+v", first)
	}
}

func TestBuildHourlyFactorNilWithoutPlanPrice(t *testing.T) {
	samples, calls := hourlyFixture()
	opts := baseOpts(t0, t0.Add(5*time.Hour))
	opts.Location = time.UTC
	opts.WeeklyPlanMicros = nil
	rep := Build(calls, samples, opts)
	h10 := rep.Hours[10]
	if h10.APICostMicros == nil || h10.QuotaPct == nil || h10.Factor != nil {
		t.Fatalf("hour 10 without plan price = %+v, want cost and quota, factor n/a", h10)
	}
	if rep.Series[0].Factor != nil {
		t.Fatalf("series factor without plan price = %v, want nil", *rep.Series[0].Factor)
	}
}

// The hours and the series are two views of the same attributed rises: their
// quota and cost sums must agree with the report totals.
func TestBuildHourlySumsMatchAttributed(t *testing.T) {
	samples, calls := hourlyFixture()
	opts := baseOpts(t0, t0.Add(5*time.Hour))
	opts.Location = time.UTC
	rep := Build(calls, samples, opts)
	var hourPct float64
	var hourCost, seriesCost int64
	for _, h := range rep.Hours {
		if h.QuotaPct != nil {
			hourPct += *h.QuotaPct
		}
		if h.APICostMicros != nil {
			hourCost += *h.APICostMicros
		}
	}
	for _, p := range rep.Series {
		seriesCost += p.APICostMicros
	}
	if math.Abs(hourPct-(rep.RisePct-rep.UnexplainedPct)) > 1e-9 {
		t.Fatalf("hour quota sum = %v, want attributed rise %v", hourPct, rep.RisePct-rep.UnexplainedPct)
	}
	if hourCost != seriesCost {
		t.Fatalf("hour cost sum = %d, series cost sum = %d", hourCost, seriesCost)
	}
}

func TestSparkline(t *testing.T) {
	cases := map[string]struct {
		in   []float64
		want string
	}{
		"empty":    {nil, ""},
		"ramp":     {[]float64{1, 2, 3, 4, 5, 6, 7, 8}, "▁▂▃▄▅▆▇█"},
		"constant": {[]float64{5, 5}, "▄▄"},
	}
	for name, c := range cases {
		if got := sparkline(c.in); got != c.want {
			t.Fatalf("%s: sparkline = %q, want %q", name, got, c.want)
		}
	}
}

func TestRenderTextShowsHoursAndSeries(t *testing.T) {
	samples, calls := hourlyFixture()
	opts := baseOpts(t0, t0.Add(5*time.Hour))
	opts.Location = time.UTC
	rep := Build(calls, samples, opts)
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"factor by local hour", "10:00", "23:00", "factor over time", "n/a"} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output misses %q:\n%s", want, out)
		}
	}
}
