package usagereport

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// Item 1: a window's first interval starts at its first sample. Calls before a
// non-zero baseline are "before first quota sample": cost shown, no share.
func TestBuildFirstRiseStartsAtFirstSample(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 37, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 38, &reset),
	}
	calls := []Call{
		call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "a2", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000),
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(3*time.Hour)))
	if len(rep.Intervals) != 1 || !rep.Intervals[0].Start.Equal(t0.Add(time.Hour)) {
		t.Fatalf("intervals = %+v, want one rise starting at the 37%% sample", rep.Intervals)
	}
	a := rowFor(t, rep, "A")
	if a.QuotaPct == nil || math.Abs(*a.QuotaPct-1) > 1e-9 {
		t.Fatalf("A quota = %v, want 1 pp", a.QuotaPct)
	}
	if a.AttributedCostMicros == nil || *a.AttributedCostMicros != 1_000_000 {
		t.Fatalf("A attributed = %v, want only the call after the baseline (1000000)", a.AttributedCostMicros)
	}
	if rep.Buckets.BeforeFirst != 1_000_000 {
		t.Fatalf("before-first bucket = %d, want the 00:30 call (1000000)", rep.Buckets.BeforeFirst)
	}
}

// Item 1 exception: a first sample at 0% means nothing was used before it, so the
// first interval starts at the window start and the early call is attributed.
func TestBuildZeroBaselineStartsAtWindowStart(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 0, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 1, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(3*time.Hour)))
	if len(rep.Intervals) != 1 || !rep.Intervals[0].Start.Equal(t0) {
		t.Fatalf("intervals = %+v, want one rise from the window start %v", rep.Intervals, t0)
	}
	if a := rowFor(t, rep, "A"); a.AttributedCostMicros == nil || *a.AttributedCostMicros != 1_000_000 {
		t.Fatalf("A attributed = %v, want 1000000", a.AttributedCostMicros)
	}
	if rep.Buckets.BeforeFirst != 0 {
		t.Fatalf("before-first bucket = %d, want 0 after a 0%% baseline", rep.Buckets.BeforeFirst)
	}
}

// Item 2: a Claude weekly row with no reset_at never merges or splits a window.
// It is skipped and counted.
func TestBuildNilResetSamplesNeverMergeOrSplit(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 12, nil),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 13, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(3*time.Hour)))
	if rep.RisePct != 3 || len(rep.Intervals) != 1 {
		t.Fatalf("rise=%v intervals=%d, want one 3 pp rise from 10%% to 13%%", rep.RisePct, len(rep.Intervals))
	}
	if rep.SkippedSamples != 1 {
		t.Fatalf("skipped samples = %d, want 1 (the nil reset_at row)", rep.SkippedSamples)
	}
}

// Item 3: every in-range priced call lands in exactly one bucket, and the buckets
// sum to the total API cost. Two windows: the first has a call before its first
// sample, a call after its last rise (older window), and the call inside the
// rise; the second window has a rise and a call after its last rise (newest).
func TestBuildEveryCallInExactlyOneBucket(t *testing.T) {
	r1 := t0.Add(7 * 24 * time.Hour)
	r2 := t0.Add(14 * 24 * time.Hour)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &r1),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &r1),
		claudeWindow(t0.Add(8*24*time.Hour), "Weekly (7-day)", 0, &r2),
		claudeWindow(t0.Add(8*24*time.Hour+time.Hour), "Weekly (7-day)", 3, &r2),
	}
	calls := []Call{
		call("A", "c1", t0.Add(30*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "c2", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "c3", t0.Add(150*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "c4", t0.Add(8*24*time.Hour+30*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "c5", t0.Add(8*24*time.Hour+2*time.Hour), "claude-sonnet-5-5", 1_000_000),
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(9*24*time.Hour)))
	b := rep.Buckets
	if b.BeforeFirst != 1_000_000 || b.AfterOlder != 1_000_000 || b.Pending != 1_000_000 {
		t.Fatalf("buckets = %+v, want 1 USD each in before-first, after-older and pending", b)
	}
	if b.Attributed != 2_000_000 || b.Quiet != 0 {
		t.Fatalf("buckets = %+v, want 2 USD attributed and nothing quiet", b)
	}
	if sum := b.Attributed + b.Quiet + b.BeforeFirst + b.AfterOlder + b.Pending; sum != 5_000_000 {
		t.Fatalf("bucket sum = %d, want the total API cost 5000000", sum)
	}
}

// Item 3 boundaries: intervals are [Start, End). A call at exactly the rising
// sample belongs to the bucket after the rise, not to the rise.
func TestBuildHalfOpenBoundaryGoesToNextBucket(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(2*time.Hour), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(3*time.Hour)))
	if rep.Buckets.Attributed != 0 || rep.Buckets.Pending != 1_000_000 {
		t.Fatalf("buckets = %+v, want the call at the rising sample in pending, not attributed", rep.Buckets)
	}
	if rep.UnexplainedPct != 2 {
		t.Fatalf("not explained = %v, want the whole 2 pp rise", rep.UnexplainedPct)
	}
}

// Item 3: the interval start is clamped to --since for display.
func TestBuildClampsIntervalStartToSince(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	rep := Build(calls, samples, baseOpts(t0.Add(time.Hour), t0.Add(3*time.Hour)))
	if len(rep.Intervals) != 1 || !rep.Intervals[0].Start.Equal(t0.Add(time.Hour)) {
		t.Fatalf("intervals = %+v, want the rise to start at --since %v", rep.Intervals, t0.Add(time.Hour))
	}
	if a := rowFor(t, rep, "A"); a.AttributedCostMicros == nil || *a.AttributedCostMicros != 1_000_000 {
		t.Fatalf("A attributed = %v, want 1000000", a.AttributedCostMicros)
	}
}

// Item 4: a session with any unpriced call has no API, attributed or pending
// cost and no factor; the unpriced count is reported.
func TestBuildMixedPricingGivesNoFactor(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 12, &reset),
	}
	calls := []Call{
		call("A", "a1", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000),
		call("A", "a2", t0.Add(100*time.Minute), "claude-unknown-9", 1_000_000),
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(3*time.Hour)))
	a := rowFor(t, rep, "A")
	if a.APICostMicros != nil || a.AttributedCostMicros != nil || a.Factor != nil {
		t.Fatalf("A api=%v attributed=%v factor=%v, want all n/a with an unpriced call", a.APICostMicros, a.AttributedCostMicros, a.Factor)
	}
	if a.UnpricedCalls != 1 || rep.UnpricedCalls != 1 {
		t.Fatalf("unpriced calls: session %d, report %d, want 1 and 1", a.UnpricedCalls, rep.UnpricedCalls)
	}
}

// Item 5, the f8441a87 shape: a 1 pp rise over 14 hours against $0.003 of local
// cost is a quiet rise (factor 0.08x with the real plan). Below min_local_factor
// it is not explained locally and gets no share. A normal rise is still shared.
func TestBuildQuietRiseIsNotExplained(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0, "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(14*time.Hour), "Weekly (7-day)", 11, &reset),
	}
	opts := baseOpts(t0, t0.Add(15*time.Hour))
	opts.MinLocalFactor = 0.1

	quiet := []Call{call("A", "a1", t0.Add(13*time.Hour), "claude-sonnet-5-5", 3_000)}
	rep := Build(quiet, samples, opts)
	if len(rep.Intervals) != 1 || rep.Intervals[0].Attributed {
		t.Fatalf("interval = %+v, want one rise that is not attributed", rep.Intervals)
	}
	if rep.UnexplainedPct != 1 || rep.Buckets.Quiet != 3_000 {
		t.Fatalf("not explained=%v quiet bucket=%d, want 1 pp and 3000", rep.UnexplainedPct, rep.Buckets.Quiet)
	}
	if a := rowFor(t, rep, "A"); a.QuotaPct != nil || a.Factor != nil {
		t.Fatalf("A quota=%v factor=%v, want no share and no factor for a quiet rise", a.QuotaPct, a.Factor)
	}

	normal := []Call{call("A", "a2", t0.Add(13*time.Hour), "claude-sonnet-5-5", 1_000_000)}
	rep = Build(normal, samples, opts)
	if a := rowFor(t, rep, "A"); a.QuotaPct == nil || math.Abs(*a.QuotaPct-1) > 1e-9 {
		t.Fatalf("normal rise: A quota=%v, want 1 pp", a.QuotaPct)
	}
}

// Item 6: a transcript whose modification time is before --since cannot hold
// newer calls, so it is not read.
func TestScanCallsSkipsFilesOlderThanSince(t *testing.T) {
	projects := filepath.Join(t.TempDir(), "projects")
	old := filepath.Join(projects, "-w", "old.jsonl")
	fresh := filepath.Join(projects, "-w", "fresh.jsonl")
	writeFile(t, old, line("m1", "claude-sonnet-5-5", "2026-11-12T10:00:00Z", 4))
	writeFile(t, fresh, line("m2", "claude-sonnet-5-5", "2026-11-20T10:00:00Z", 4))
	mtime := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(old, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	calls, err := ScanCalls(projects, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].MessageID != "m2" {
		t.Fatalf("calls = %+v, want only m2 from the file modified after --since", calls)
	}
}

// Item 6: a line over the scanner limit is an error, not a silently dropped file.
func TestScanCallsOverlongLineIsAnError(t *testing.T) {
	projects := filepath.Join(t.TempDir(), "projects")
	big := `{"type":"assistant","message":"` + strings.Repeat("x", 17<<20) + `"}` + "\n"
	writeFile(t, filepath.Join(projects, "-w", "s1.jsonl"), line("m1", "claude-sonnet-5-5", "2026-11-13T10:01:00Z", 4)+big)
	if _, err := ScanCalls(projects, time.Time{}); err == nil {
		t.Fatal("ScanCalls with a 17 MiB line returned no error, want one")
	}
}

// Item 7: transcripts → ScanCalls → Build, end to end on a temp directory.
func TestScanCallsBuildEndToEnd(t *testing.T) {
	projects := filepath.Join(t.TempDir(), "projects")
	entry := `{"type":"assistant","timestamp":"2026-11-13T11:30:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","usage":{"input_tokens":1000000,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
	writeFile(t, filepath.Join(projects, "-w", "s1.jsonl"), entry)
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 11, &reset),
	}
	calls, err := ScanCalls(projects, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	rep := Build(calls, samples, baseOpts(t0, t0.Add(3*time.Hour)))
	a := rowFor(t, rep, "s1")
	if a.APICostMicros == nil || *a.APICostMicros != 1_000_000 || a.QuotaPct == nil || math.Abs(*a.QuotaPct-1) > 1e-9 {
		t.Fatalf("s1 api=%v quota=%v, want 1000000 and 1 pp", a.APICostMicros, a.QuotaPct)
	}
}

// Item 7: the cache-write TTL split is read from a transcript and priced.
// Sonnet fixture rates: input 1 USD, 5m write 1.25, 1h write 2, per million.
func TestTranscriptCacheCreationSplitIsPriced(t *testing.T) {
	projects := filepath.Join(t.TempDir(), "projects")
	entry := `{"type":"assistant","timestamp":"2026-11-13T11:30:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","usage":{"input_tokens":1000,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":3000,"cache_creation":{"ephemeral_5m_input_tokens":1000,"ephemeral_1h_input_tokens":2000}}}}` + "\n"
	writeFile(t, filepath.Join(projects, "-w", "s1.jsonl"), entry)
	calls, err := ScanCalls(projects, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	rep := Build(calls, nil, baseOpts(t0, t0.Add(3*time.Hour)))
	// 1000 x 1 + 1000 x 1.25 + 2000 x 2 = 6250 micro-USD.
	if rep.Totals.APICostMicros == nil || *rep.Totals.APICostMicros != 6250 {
		t.Fatalf("api cost = %v, want 6250 with the TTL split priced", rep.Totals.APICostMicros)
	}
}

// Item 8: a plan price that is unknown prints a plain n/a, not the threshold marker.
func TestRenderUnknownPlanPriceIsPlainNA(t *testing.T) {
	samples := []usage.QuotaHistoryEntry{
		claudeWindow(t0.Add(time.Hour), "Weekly (7-day)", 10, &reset),
		claudeWindow(t0.Add(2*time.Hour), "Weekly (7-day)", 11, &reset),
	}
	calls := []Call{call("A", "a1", t0.Add(90*time.Minute), "claude-sonnet-5-5", 1_000_000)}
	opts := baseOpts(t0, t0.Add(3*time.Hour))
	opts.WeeklyPlanMicros = nil
	opts.MinSessionQuotaPct = 1.5
	rep := Build(calls, samples, opts)
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "< 1.5 pp") {
		t.Fatalf("output has the threshold marker with no plan price:\n%s", buf.String())
	}
}

// Item 8: under --session, the window, hours and series are for all sessions,
// and the output says so.
func TestRenderNamesSessionScope(t *testing.T) {
	samples, calls := flatFixture()
	opts := baseOpts(t0, t0.Add(4*time.Hour))
	opts.Session = "A"
	rep := Build(calls, samples, opts)
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "cover all sessions") {
		t.Fatalf("output does not say the hours, series and window cover all sessions:\n%s", buf.String())
	}
}
