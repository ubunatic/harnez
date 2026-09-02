package telemetry

// Tests for issue 142's overhead-measurement additions: RateCallOverhead
// (real, measured call_type="internal" byte/count aggregation) and
// EstimateTokens (the explicitly-labeled heuristic conversion on top of
// it — never presented as provider-reported).

import "testing"

func TestRateCallOverhead_CountsOnlyInternalCallType(t *testing.T) {
	db := openTestDB(t)

	rateCall := sampleCall("sess-1", "Read", 5, 0) // CallType "internal" by default
	rateCall.RawBytes = 80
	shellCall := sampleCall("sess-1", "Read", 4, 0)
	shellCall.CallType = "shell"
	shellCall.RawBytes = 99999 // must not leak into the rate-only aggregate

	for _, c := range []ToolCall{rateCall, shellCall} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	o, err := db.RateCallOverhead(Filter{})
	if err != nil {
		t.Fatalf("RateCallOverhead: %v", err)
	}
	if o.Count != 1 {
		t.Fatalf("Count = %d, want 1 (only the internal/rate call)", o.Count)
	}
	if o.TotalCallBytes != 80 {
		t.Errorf("TotalCallBytes = %d, want 80", o.TotalCallBytes)
	}
	if o.AvgCallBytes != 80 {
		t.Errorf("AvgCallBytes = %v, want 80", o.AvgCallBytes)
	}
}

func TestRateCallOverhead_FilterCallTypeOverridden(t *testing.T) {
	db := openTestDB(t)
	rateCall := sampleCall("sess-1", "Read", 5, 0)
	rateCall.RawBytes = 50
	if err := db.Insert(rateCall); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Even a caller-supplied CallType="shell" filter must not exclude the
	// internal rows this report exists to measure — RateCallOverhead owns
	// that dimension of the filter.
	o, err := db.RateCallOverhead(Filter{CallType: "shell"})
	if err != nil {
		t.Fatalf("RateCallOverhead: %v", err)
	}
	if o.Count != 1 {
		t.Errorf("Count = %d, want 1 (CallType filter should be overridden to internal)", o.Count)
	}
}

func TestRateCallOverhead_EmptyIsZeroNotError(t *testing.T) {
	db := openTestDB(t)
	o, err := db.RateCallOverhead(Filter{})
	if err != nil {
		t.Fatalf("RateCallOverhead: %v", err)
	}
	if o.Count != 0 || o.TotalCallBytes != 0 || o.AvgCallBytes != 0 {
		t.Errorf("empty DB overhead = %+v, want all-zero", o)
	}
}

func TestEstimateTokens_IsLabeledHeuristicNotExact(t *testing.T) {
	cases := []struct {
		bytes int64
		want  int64
	}{
		{0, 0},
		{4, 1},
		{100, 25},
		{7, 1}, // integer division, deliberately rounds down
	}
	for _, c := range cases {
		if got := EstimateTokens(c.bytes); got != c.want {
			t.Errorf("EstimateTokens(%d) = %d, want %d", c.bytes, got, c.want)
		}
	}
}
