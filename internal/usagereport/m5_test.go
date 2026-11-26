package usagereport

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// M5 item 2: the real near-miss shape. A 1 pp rise with $0.0038 of local API
// cost on a 3,785,192 µUSD weekly plan has factor 0.1004x. With the spec's
// default min_local_factor it is quiet: not attributed, counted as not
// explained locally.
func TestBuildNearMissShapeIsQuietAtDefault(t *testing.T) {
	min, err := usage.ReportMinLocalFactor()
	if err != nil {
		t.Fatal(err)
	}
	plan := int64(3_785_192)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 11, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 3_800)}
	opts := baseOpts(t0, t0.Add(2*time.Hour))
	opts.WeeklyPlanMicros = &plan
	opts.MinLocalFactor = min
	rep := Build(calls, samples, opts)
	if len(rep.Intervals) != 1 || rep.Intervals[0].Attributed {
		t.Fatalf("interval = %+v, want one quiet rise at factor 0.1004x", rep.Intervals)
	}
	if rep.UnexplainedPct != 1 || rep.Buckets.Quiet != 3_800 || rep.Buckets.Attributed != 0 {
		t.Fatalf("not explained=%v quiet=%d attributed=%d, want 1 pp, 3800, 0", rep.UnexplainedPct, rep.Buckets.Quiet, rep.Buckets.Attributed)
	}
	if a := rowFor(t, rep, "A"); a.QuotaPct != nil || a.Factor != nil {
		t.Fatalf("A quota=%v factor=%v, want no share and no factor", a.QuotaPct, a.Factor)
	}
}

// M5 item 2 near miss: a 1 pp rise at 1.1x (local cost 41,637 µUSD on the same
// plan) stays attributed under the default, so the default does not swallow it.
func TestBuildNearMissAt1p1StaysAttributed(t *testing.T) {
	min, err := usage.ReportMinLocalFactor()
	if err != nil {
		t.Fatal(err)
	}
	plan := int64(3_785_192)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 11, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 41_637)}
	opts := baseOpts(t0, t0.Add(2*time.Hour))
	opts.WeeklyPlanMicros = &plan
	opts.MinLocalFactor = min
	rep := Build(calls, samples, opts)
	a := rowFor(t, rep, "A")
	if a.QuotaPct == nil || math.Abs(*a.QuotaPct-1) > 1e-9 || a.Factor == nil || math.Abs(*a.Factor-1.1) > 0.01 {
		t.Fatalf("A quota=%v factor=%v, want 1 pp and a factor near 1.1x", a.QuotaPct, a.Factor)
	}
	if rep.Buckets.Quiet != 0 || rep.Buckets.Attributed != 41_637 {
		t.Fatalf("buckets = %+v, want 41637 attributed and nothing quiet", rep.Buckets)
	}
}

// M5 item 3: the totals and session column is API − ATTRIB, named UNATTRIB, not
// PENDING: pending is the open tail only (the bucket line keeps that word).
func TestRenderUsesUnattributedColumn(t *testing.T) {
	samples, calls := flatFixture()
	rep := Build(calls, samples, baseOpts(t0, t0.Add(4*time.Hour)))
	var text bytes.Buffer
	if err := RenderText(&text, rep, 1); err != nil {
		t.Fatal(err)
	}
	out := text.String()
	header := strings.SplitN(out, "\n", 3)[1]
	if !strings.Contains(header, "UNATTRIB $") || strings.Contains(header, "PENDING") {
		t.Fatalf("header = %q, want UNATTRIB $ and no PENDING", header)
	}
	if strings.Contains(out, "PENDING $") {
		t.Fatalf("report still names the column PENDING $:\n%s", out)
	}
	var js bytes.Buffer
	if err := RenderJSON(&js, rep); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Totals map[string]json.RawMessage `json:"totals"`
	}
	if err := json.Unmarshal(js.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc.Totals["unattributed_api_cost_micros_usd"]) != "1000000" {
		t.Fatalf("totals unattributed_api_cost_micros_usd = %s, want 1000000", doc.Totals["unattributed_api_cost_micros_usd"])
	}
	if _, ok := doc.Totals["pending_api_cost_micros_usd"]; ok {
		t.Fatalf("totals still carry pending_api_cost_micros_usd")
	}
}
