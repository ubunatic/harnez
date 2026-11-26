package usage

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

func TestComputeTurnQuotaShare(t *testing.T) {
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Second)
	reset := t0.Add(72 * time.Hour)
	reset2 := reset.Add(168 * time.Hour)
	monthly100 := int64(100_000_000)

	before := TurnQuotaReading{
		CapturedAt: t0,
		StoreWindows: []usagestore.Window{
			{Provider: "claude", Key: "five_hour", UsedFraction: 0.10, ResetAt: &reset, ObservedAt: t0},
			{Provider: "claude", Key: "weekly", UsedFraction: 0.20, ResetAt: &reset, ObservedAt: t0},
		},
	}
	after := TurnQuotaReading{
		CapturedAt: t1,
		StoreWindows: []usagestore.Window{
			{Provider: "claude", Key: "five_hour", UsedFraction: 0.15, ResetAt: &reset, ObservedAt: t1},
			{Provider: "claude", Key: "weekly", UsedFraction: 0.22, ResetAt: &reset, ObservedAt: t1},
		},
	}
	share, window, beforePct, afterPct := ComputeTurnQuotaShare(before, after, &monthly100, testResetTol)
	if window != "weekly" || beforePct == nil || *beforePct != 20 || afterPct == nil || *afterPct != 22 {
		t.Fatalf("window=%q before=%v after=%v", window, beforePct, afterPct)
	}
	// weekly = (100_000_000 * 12) / 52 = 23_076_923; 0.02 * 23_076_923 = 461_538
	if share == nil || *share != 461538 {
		t.Fatalf("share = %v, want 461538", share)
	}

	// Missing plan price keeps share nil while retaining before/after %
	share, _, beforePct, afterPct = ComputeTurnQuotaShare(before, after, nil, testResetTol)
	if share != nil || beforePct == nil || afterPct == nil {
		t.Fatalf("unset plan price: share=%v before=%v after=%v", share, beforePct, afterPct)
	}

	// Reset crossed leaves share nil (never 0)
	afterReset := TurnQuotaReading{
		CapturedAt:   t1,
		StoreWindows: []usagestore.Window{{Provider: "claude", Key: "weekly", UsedFraction: 0.25, ResetAt: &reset2, ObservedAt: t1}},
	}
	if share, _, _, _ := ComputeTurnQuotaShare(before, afterReset, &monthly100, testResetTol); share != nil {
		t.Fatalf("reset crossed share = %v, want nil", share)
	}

	// Capture error leaves everything nil
	afterErr := TurnQuotaReading{CapturedAt: t1, Error: "rate limited"}
	if share, win, bp, ap := ComputeTurnQuotaShare(before, afterErr, &monthly100, testResetTol); share != nil || win != "" || bp != nil || ap != nil {
		t.Fatalf("error reading returned share=%v win=%q bp=%v ap=%v, want all nil/empty", share, win, bp, ap)
	}
}

// testResetTol is the reset tolerance the spec ships (spec/usage.yaml).
const testResetTol = time.Minute

// Ticket 1302 M1: reset_at values that drift by about 0.2 s are the same
// window, and a share must pair across that jitter.
func TestSameResetMarkerTolerance(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	jittered := base.Add(180 * time.Millisecond)
	far := base.Add(2 * time.Minute)
	if !sameResetMarker(&base, &jittered, testResetTol) {
		t.Fatalf("180ms jitter must be the same window under %v", testResetTol)
	}
	if sameResetMarker(&base, &far, testResetTol) {
		t.Fatalf("2m apart must be a different window under %v", testResetTol)
	}
	if sameResetMarker(&base, &jittered, 0) {
		t.Fatalf("zero tolerance must keep exact equality")
	}
	if !sameResetMarker(nil, nil, testResetTol) || sameResetMarker(&base, nil, testResetTol) {
		t.Fatalf("nil reset markers: both nil is the same, one nil is not")
	}
}

func TestComputeTurnQuotaShareResetJitter(t *testing.T) {
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Second)
	reset := t0.Add(72 * time.Hour)
	jittered := reset.Add(180 * time.Millisecond)
	monthly100 := int64(100_000_000)
	before := TurnQuotaReading{CapturedAt: t0, StoreWindows: []usagestore.Window{{Provider: "claude", Key: "weekly", UsedFraction: 0.20, ResetAt: &reset, ObservedAt: t0}}}
	after := TurnQuotaReading{CapturedAt: t1, StoreWindows: []usagestore.Window{{Provider: "claude", Key: "weekly", UsedFraction: 0.22, ResetAt: &jittered, ObservedAt: t1}}}
	if share, _, _, _ := ComputeTurnQuotaShare(before, after, &monthly100, testResetTol); share == nil || *share != 461538 {
		t.Fatalf("jittered reset under tolerance: share=%v, want 461538", share)
	}
	if share, _, _, _ := ComputeTurnQuotaShare(before, after, &monthly100, 0); share != nil {
		t.Fatalf("jittered reset with zero tolerance: share=%v, want nil", share)
	}
}
