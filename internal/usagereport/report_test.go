package usagereport

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

// Fixture prices in micro-USD per million tokens. Haiku has a higher tier over
// 100k prompt tokens (ticket 1301).
func fixtureCatalog() telemetry.PricingCatalog {
	p := func(v int64) *int64 { return &v }
	base := telemetry.PricingRates{
		UncachedInputMicrosPerMillion: p(1_000_000),
		CachedInputMicrosPerMillion:   p(100_000),
		CacheWrite5mMicrosPerMillion:  p(1_250_000),
		CacheWrite1hMicrosPerMillion:  p(2_000_000),
		OutputMicrosPerMillion:        p(5_000_000),
	}
	high := telemetry.PricingRates{
		UncachedInputMicrosPerMillion: p(2_000_000),
		CachedInputMicrosPerMillion:   p(200_000),
		CacheWrite5mMicrosPerMillion:  p(2_500_000),
		CacheWrite1hMicrosPerMillion:  p(4_000_000),
		OutputMicrosPerMillion:        p(10_000_000),
	}
	haiku := telemetry.ModelPricing{Model: "claude-haiku-5-5", Revision: "fixture", Provider: "claude", Rates: base,
		Tiers: []telemetry.PricingTier{{OverInputTokens: 100_000, Rates: high}}}
	// Sonnet has base rates only, so multi-million-token fixtures stay at base
	// price; Haiku is reserved for the tier tests.
	sonnet := telemetry.ModelPricing{Model: "claude-sonnet-5-5", Revision: "fixture", Provider: "claude", Rates: base}
	return telemetry.PricingCatalog{"claude-haiku-5-5": haiku, "claude-sonnet-5-5": sonnet}
}

var (
	t0    = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	reset = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
)

const testTol = time.Minute

func rp(t time.Time) *time.Time { return &t }

func weekly(at time.Time, used int, resetAt *time.Time) usage.QuotaHistoryEntry {
	return usage.QuotaHistoryEntry{Timestamp: at, Agent: "claude", Window: "Weekly (7-day)", UsedPercent: used, ResetAt: resetAt}
}

// call builds a call of `input` uncached tokens on model at time at.
func call(session, id string, at time.Time, model string, input int) Call {
	return Call{Session: session, MessageID: id, At: at, Usage: subagent.RequestUsage{Model: model, Input: input, CacheWriteSplitKnown: true}}
}

func baseOpts(since, until time.Time) Options {
	weekly := int64(23_076_923) // EUR-free fixture: monthly USD 100 * 12 / 52
	return Options{
		Since: since, Until: until, Catalog: fixtureCatalog(), ResetTolerance: testTol,
		WeeklyPlanMicros: &weekly, VATRate: 0.19, UnexplainedTolerancePct: 1,
	}
}

func rowFor(t *testing.T, rep Report, session string) SessionRow {
	t.Helper()
	for _, r := range rep.Sessions {
		if r.Session == session {
			return r
		}
	}
	t.Fatalf("no row for session %q in %+v", session, rep.Sessions)
	return SessionRow{}
}

// Two sessions share one 3-point rise; the rise splits by API cost (3:1).
func TestBuildSplitsOverlapBySessionCost(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset)), weekly(t0.Add(time.Hour), 13, rp(reset))}
	calls := []Call{
		call("A", "a1", t0.Add(10*time.Minute), "claude-sonnet-5-5", 3_000_000),
		call("B", "b1", t0.Add(20*time.Minute), "claude-sonnet-5-5", 1_000_000),
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	a, b := rowFor(t, rep, "A"), rowFor(t, rep, "B")
	if a.QuotaPct == nil || math.Abs(*a.QuotaPct-2.25) > 1e-9 {
		t.Fatalf("A quota pct = %v, want 2.25", a.QuotaPct)
	}
	if b.QuotaPct == nil || math.Abs(*b.QuotaPct-0.75) > 1e-9 {
		t.Fatalf("B quota pct = %v, want 0.75", b.QuotaPct)
	}
	if rep.Totals.QuotaPct == nil || math.Abs(*rep.Totals.QuotaPct-3) > 1e-9 {
		t.Fatalf("total quota pct = %v, want 3", rep.Totals.QuotaPct)
	}
	if a.APICostMicros == nil || *a.APICostMicros != 3_000_000 {
		t.Fatalf("A api cost = %v, want 3000000 micro-USD", a.APICostMicros)
	}
	if len(rep.Intervals) != 1 || !rep.Intervals[0].Attributed {
		t.Fatalf("intervals = %+v, want one attributed interval", rep.Intervals)
	}
}

// Plan cost is the attributed share times the weekly plan price, and gross is
// net times (1 + VAT), both rounded to micro-USD.
func TestBuildPlanNetAndGross(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset)), weekly(t0.Add(time.Hour), 13, rp(reset))}
	calls := []Call{call("A", "a1", t0.Add(time.Minute), "claude-sonnet-5-5", 1_000_000)}
	opts := baseOpts(t0, t0.Add(2*time.Hour))
	rep := Build(calls, samples, opts)
	a := rowFor(t, rep, "A")
	wantNet := int64(math.Round(0.03 * 23_076_923))
	if a.PlanNetMicros == nil || *a.PlanNetMicros != wantNet {
		t.Fatalf("plan net = %v, want %d", a.PlanNetMicros, wantNet)
	}
	if a.PlanGrossMicros == nil || *a.PlanGrossMicros != int64(math.Round(float64(wantNet)*1.19)) {
		t.Fatalf("plan gross = %v, want net*1.19", a.PlanGrossMicros)
	}
	if a.Factor == nil || math.Abs(*a.Factor-float64(1_000_000)/float64(wantNet)) > 1e-9 {
		t.Fatalf("factor = %v", a.Factor)
	}
}

// A reset_at that moves by a week is a new window: no delta across it.
func TestBuildWindowResetInsideRangeGivesNoDelta(t *testing.T) {
	next := reset.Add(7 * 24 * time.Hour)
	samples := []usage.QuotaHistoryEntry{weekly(t0, 90, rp(reset)), weekly(t0.Add(time.Hour), 2, rp(next))}
	rep := Build(nil, samples, baseOpts(t0.Add(-time.Hour), t0.Add(2*time.Hour)))
	if len(rep.Intervals) != 0 || rep.RisePct != 0 {
		t.Fatalf("reset crossed: intervals=%d rise=%v, want none", len(rep.Intervals), rep.RisePct)
	}
}

// reset_at drifting by about 0.2 s is one window (turn_quota_pairing tolerance).
func TestBuildGroupsResetJitter(t *testing.T) {
	jittered := reset.Add(180 * time.Millisecond)
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset)), weekly(t0.Add(time.Hour), 13, rp(jittered))}
	rep := Build(nil, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	if len(rep.Intervals) != 1 || rep.RisePct != 3 {
		t.Fatalf("jittered reset: intervals=%d rise=%v, want one interval of 3", len(rep.Intervals), rep.RisePct)
	}
}

// A duplicated quota row (about 24% of real lines) counts once.
func TestBuildCollapsesDuplicateQuotaRows(t *testing.T) {
	first := weekly(t0, 10, rp(reset))
	samples := []usage.QuotaHistoryEntry{first, first, weekly(t0.Add(time.Hour), 13, rp(reset))}
	rep := Build(nil, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	if rep.RisePct != 3 {
		t.Fatalf("rise = %v with a duplicate row, want 3", rep.RisePct)
	}
}

// Haiku calls over 100k prompt tokens price at the higher tier.
func TestBuildHaikuOver100kUsesHigherTier(t *testing.T) {
	calls := []Call{call("A", "a1", t0, "claude-haiku-5-5", 150_000)}
	rep := Build(calls, nil, baseOpts(t0.Add(-time.Hour), t0.Add(time.Hour)))
	if got := rowFor(t, rep, "A").APICostMicros; got == nil || *got != 300_000 {
		t.Fatalf("150k haiku cost = %v, want 300000 (higher tier 2/M)", got)
	}
}

// A dated stream id prices as its base model.
func TestBuildDatedHaikuIdPricesAsBase(t *testing.T) {
	calls := []Call{call("A", "a1", t0, "claude-haiku-5-5-20261101", 50_000)}
	rep := Build(calls, nil, baseOpts(t0.Add(-time.Hour), t0.Add(time.Hour)))
	if got := rowFor(t, rep, "A").APICostMicros; got == nil || *got != 50_000 {
		t.Fatalf("dated haiku cost = %v, want 50000", got)
	}
}

// An unknown model is unpriced: API cost n/a, its rise is not explained locally.
func TestBuildUnknownModelIsNotZero(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset)), weekly(t0.Add(time.Hour), 13, rp(reset))}
	calls := []Call{call("A", "a1", t0.Add(time.Minute), "claude-mystery-9", 1_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	a := rowFor(t, rep, "A")
	if a.APICostMicros != nil || a.Factor != nil || a.QuotaPct != nil {
		t.Fatalf("unknown model: cost=%v factor=%v quota=%v, want all n/a", a.APICostMicros, a.Factor, a.QuotaPct)
	}
	if rep.Totals.APICostMicros != nil {
		t.Fatalf("totals cost = %v, want n/a when a call is unpriced", *rep.Totals.APICostMicros)
	}
	if !rep.Unexplained || rep.UnexplainedPct != 3 {
		t.Fatalf("unexplained = %v (%v), want flagged at 3", rep.Unexplained, rep.UnexplainedPct)
	}
}

// A session with priced calls but no quota history shows quota and plan n/a.
func TestBuildSessionWithoutQuotaDataIsNA(t *testing.T) {
	calls := []Call{call("A", "a1", t0, "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, nil, baseOpts(t0.Add(-time.Hour), t0.Add(time.Hour)))
	a := rowFor(t, rep, "A")
	if a.APICostMicros == nil || *a.APICostMicros != 1_000_000 {
		t.Fatalf("api cost = %v, want 1000000", a.APICostMicros)
	}
	if a.QuotaPct != nil || a.PlanNetMicros != nil || a.PlanGrossMicros != nil || a.Factor != nil {
		t.Fatalf("no quota data: quota=%v net=%v factor=%v, want n/a", a.QuotaPct, a.PlanNetMicros, a.Factor)
	}
}

// A rise with no priced local call in its interval is "not explained locally";
// it is flagged only above the tolerance, so 1 point with tolerance 1 is not.
func TestBuildNotExplainedLocallyTolerance(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset)), weekly(t0.Add(time.Hour), 11, rp(reset))}
	rep := Build(nil, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	if rep.Unexplained || rep.UnexplainedPct != 1 {
		t.Fatalf("1 point at tolerance 1: flagged=%v pct=%v, want not flagged", rep.Unexplained, rep.UnexplainedPct)
	}
	samples[1].UsedPercent = 13
	rep = Build(nil, samples, baseOpts(t0, t0.Add(2*time.Hour)))
	if !rep.Unexplained || rep.UnexplainedPct != 3 {
		t.Fatalf("3 points at tolerance 1: flagged=%v pct=%v, want flagged", rep.Unexplained, rep.UnexplainedPct)
	}
}

// Calls outside the range are ignored, and session rows add up to the totals.
func TestBuildRangeAndTotalsAddUp(t *testing.T) {
	calls := []Call{
		call("A", "a1", t0, "claude-sonnet-5-5", 1_000_000),
		call("B", "b1", t0.Add(time.Minute), "claude-sonnet-5-5", 2_000_000),
		call("A", "a2", t0.Add(-48*time.Hour), "claude-sonnet-5-5", 9_000_000),
	}
	rep := Build(calls, nil, baseOpts(t0.Add(-time.Hour), t0.Add(time.Hour)))
	var calls64, cost int64
	for _, r := range rep.Sessions {
		calls64 += int64(r.Calls)
		cost += *r.APICostMicros
	}
	if rep.Totals.Calls != 2 || int64(rep.Totals.Calls) != calls64 {
		t.Fatalf("totals calls = %d, rows = %d, want 2", rep.Totals.Calls, calls64)
	}
	if rep.Totals.APICostMicros == nil || *rep.Totals.APICostMicros != cost || cost != 3_000_000 {
		t.Fatalf("totals cost = %v, rows = %d, want 3000000", rep.Totals.APICostMicros, cost)
	}
}

// An empty range is an empty report: no error, sessions is [] not null.
func TestBuildEmptyRange(t *testing.T) {
	rep := Build(nil, nil, baseOpts(t0, t0.Add(time.Hour)))
	if len(rep.Sessions) != 0 || rep.Totals.Calls != 0 {
		t.Fatalf("empty range: sessions=%d calls=%d", len(rep.Sessions), rep.Totals.Calls)
	}
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatal(err)
	}
	if s, ok := generic["sessions"].([]any); !ok || len(s) != 0 {
		t.Fatalf("sessions JSON = %v, want []", generic["sessions"])
	}
}

// The weekly window starts seven days before the newest weekly reset_at.
func TestWeeklyWindowStartIsResetMinusSevenDays(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset))}
	start, ok := WeeklyWindowStart(samples, testTol)
	if !ok || !start.Equal(reset.Add(-7*24*time.Hour)) {
		t.Fatalf("window start = %v ok=%v, want %v", start, ok, reset.Add(-7*24*time.Hour))
	}
	if _, ok := WeeklyWindowStart(nil, testTol); ok {
		t.Fatalf("no weekly history must report no window")
	}
}

// --session narrows the rows; the attribution still uses every session's calls.
func TestBuildSessionFilterKeepsAttribution(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{weekly(t0, 10, rp(reset)), weekly(t0.Add(time.Hour), 13, rp(reset))}
	calls := []Call{
		call("A", "a1", t0.Add(time.Minute), "claude-sonnet-5-5", 3_000_000),
		call("B", "b1", t0.Add(2*time.Minute), "claude-sonnet-5-5", 1_000_000),
	}
	opts := baseOpts(t0, t0.Add(2*time.Hour))
	opts.Session = "A"
	rep := Build(calls, samples, opts)
	if len(rep.Sessions) != 1 || rep.Sessions[0].Session != "A" {
		t.Fatalf("filtered rows = %+v, want only A", rep.Sessions)
	}
	if a := rowFor(t, rep, "A"); a.QuotaPct == nil || math.Abs(*a.QuotaPct-2.25) > 1e-9 {
		t.Fatalf("A quota pct = %v, want 2.25 with B still attributed", a.QuotaPct)
	}
}
