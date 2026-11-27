package usagereport

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// min_session_five_hour_pct at exactly the threshold: session A's 5-hour share is
// 1 pp, the threshold is 1, so the factor is shown (ticket 1308 M3 item 3).
func TestFiveHourFactorShownAtExactThreshold(t *testing.T) {
	samples := twoWindowSamples(1, 94, 4)
	calls := []Call{call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 40_000)}
	rep := Build(calls, samples, ratioOpts())
	row := rowFor(t, rep, "A")
	if row.FiveHourQuotaPct == nil || !samePP(*row.FiveHourQuotaPct, 1) {
		t.Fatalf("session A 5h share = %v, want 1 pp", row.FiveHourQuotaPct)
	}
	if row.FiveHourFactor == nil {
		t.Fatalf("5h factor nil at exactly min_session_five_hour_pct = %v, want shown", rep.MinSessionFiveHourPct)
	}
}

// Just below the threshold (1 pp against 1.01): no factor, and the text says why.
func TestFiveHourFactorHiddenBelowThresholdSaysWhy(t *testing.T) {
	samples := twoWindowSamples(1, 94, 4)
	calls := []Call{call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 40_000)}
	opts := ratioOpts()
	opts.MinSessionFiveHourPct = 1.01
	rep := Build(calls, samples, opts)
	row := rowFor(t, rep, "A")
	if row.FiveHourFactor != nil {
		t.Fatalf("5h factor = %v below the threshold, want nil", *row.FiveHourFactor)
	}
	if row.FiveHourWeeklyEqPct == nil {
		t.Fatalf("weekly-equivalent nil; it is still shown below the threshold")
	}
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "n/a (< 1.01 pp)") {
		t.Fatalf("render does not say why the factor is n/a:\n%s", buf.String())
	}
}

// End to end with two sessions in one 5-hour rise and a model that only ever shares a
// rise (ticket 1308 M3 item 4). Window 1 (2 pp, weekly 4) has A and B with equal
// calls, both models in it: B has no solo rise, so its 5-hour factor is split and its
// model row has no factor. Window 2 (3 pp, weekly 6) is A's alone, so A's weekly
// factor is measured and sonnet's factor comes from that solo rise.
func TestBuildSplitBasisEndToEnd(t *testing.T) {
	samples := twoWindowSamples(2, 3, 6)
	calls := []Call{
		call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 2_000_000),
		call("B", "b1", t0.Add(2*time.Hour), "claude-haiku-5-5", 1_000_000),
		call("A", "a2", t0.Add(7*time.Hour), "claude-sonnet-5-5", 2_000_000),
	}
	rep := Build(calls, samples, ratioOpts())
	a, b := rowFor(t, rep, "A"), rowFor(t, rep, "B")
	if b.FiveHourFactor == nil || b.FiveHourFactorBasis != "split" {
		t.Fatalf("B 5h factor %v basis %q, want a factor with basis split (no solo rise)", b.FiveHourFactor, b.FiveHourFactorBasis)
	}
	if b.Factor == nil || b.FactorBasis != "split" {
		t.Fatalf("B weekly factor %v basis %q, want a factor with basis split (its weekly share is all shared)", b.Factor, b.FactorBasis)
	}
	if a.FactorBasis != "measured" {
		t.Fatalf("A weekly factor basis %q, want measured (most of its weekly share is solo)", a.FactorBasis)
	}
	if a.FiveHourFactorBasis != "measured" {
		t.Fatalf("A 5h factor basis %q, want measured", a.FiveHourFactorBasis)
	}
	var haiku, sonnet *ModelRow
	for i := range rep.Models {
		switch rep.Models[i].Model {
		case "claude-haiku-5-5":
			haiku = &rep.Models[i]
		case "claude-sonnet-5-5":
			sonnet = &rep.Models[i]
		}
	}
	if haiku == nil || sonnet == nil {
		t.Fatalf("models = %+v, want haiku and sonnet rows", rep.Models)
	}
	if haiku.Factor != nil || haiku.SoloRises != 0 {
		t.Fatalf("haiku factor %v solo rises %d, want none: it only shares rises", haiku.Factor, haiku.SoloRises)
	}
	if sonnet.Factor == nil || sonnet.SoloRises != 1 {
		t.Fatalf("sonnet factor %v solo rises %d, want a factor from one solo rise", sonnet.Factor, sonnet.SoloRises)
	}
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"split", "measured", "claude-haiku-5-5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render lacks %q:\n%s", want, out)
		}
	}
}
