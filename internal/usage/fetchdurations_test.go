package usage

import (
	"testing"
	"time"
)

// TestFetchDurationKindForHost checks the local/remote key split issue 168
// relies on so a fast local CollectAll and a slower SSH-tunneled
// CollectRemote (per distinct host) never share -- and cross-contaminate --
// an estimate.
func TestFetchDurationKindForHost(t *testing.T) {
	if got := fetchDurationKindForHost(""); got != fetchDurationKindLocal {
		t.Errorf("fetchDurationKindForHost(\"\") = %q, want %q", got, fetchDurationKindLocal)
	}
	a := fetchDurationKindForHost("box-a")
	b := fetchDurationKindForHost("box-b")
	if a == b {
		t.Errorf("expected distinct hosts to get distinct keys, got %q for both", a)
	}
	if a == fetchDurationKindLocal || b == fetchDurationKindLocal {
		t.Errorf("expected remote keys to differ from the local key, got %q/%q", a, b)
	}
}

// TestLoadFetchDurationEstimateColdStart checks a homeDir with no persisted
// cache reports ok=false rather than a fabricated zero estimate -- the
// signal splashBarPercent's fallback-to-sweep branch depends on.
func TestLoadFetchDurationEstimateColdStart(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	dir := t.TempDir()
	if est, ok := loadFetchDurationEstimate(dir, fetchDurationKindLocal); ok {
		t.Fatalf("expected no estimate on a cold cache, got %v (ok=%v)", est, ok)
	}
}

// TestRecordFetchDurationRoundTrip checks a single recorded sample becomes
// the estimate for its own kind, and leaves an unrelated kind untouched.
func TestRecordFetchDurationRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	dir := t.TempDir()

	recordFetchDuration(dir, fetchDurationKindLocal, 2*time.Second)

	got, ok := loadFetchDurationEstimate(dir, fetchDurationKindLocal)
	if !ok {
		t.Fatal("expected an estimate to exist after recording one sample")
	}
	if got != 2*time.Second {
		t.Errorf("estimate = %v, want %v after a single sample", got, 2*time.Second)
	}

	if _, ok := loadFetchDurationEstimate(dir, fetchDurationKindForHost("box-a")); ok {
		t.Error("expected an unrelated kind to still report no estimate")
	}
}

// TestRecordFetchDurationBlendsRollingEstimate checks repeated samples move
// the estimate toward newly observed durations (EWMA) rather than either
// staying pinned to the first sample or snapping straight to the latest one.
func TestRecordFetchDurationBlendsRollingEstimate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	dir := t.TempDir()

	recordFetchDuration(dir, fetchDurationKindLocal, 2*time.Second)
	recordFetchDuration(dir, fetchDurationKindLocal, 10*time.Second)

	got, ok := loadFetchDurationEstimate(dir, fetchDurationKindLocal)
	if !ok {
		t.Fatal("expected an estimate to exist after recording samples")
	}
	if got <= 2*time.Second || got >= 10*time.Second {
		t.Errorf("expected the blended estimate strictly between the two samples, got %v", got)
	}
}

// TestRecordFetchDurationIgnoresNonPositive checks a zero/negative duration
// (e.g. a canceled-context fetch that RunWatchWithOptions's own guard
// already tries to filter out) never gets persisted -- it would otherwise
// drag a real rolling estimate down to a value that reflects an aborted
// fetch, not a completed one.
func TestRecordFetchDurationIgnoresNonPositive(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	dir := t.TempDir()

	recordFetchDuration(dir, fetchDurationKindLocal, 0)
	recordFetchDuration(dir, fetchDurationKindLocal, -5*time.Second)

	if _, ok := loadFetchDurationEstimate(dir, fetchDurationKindLocal); ok {
		t.Error("expected no estimate to be persisted from non-positive durations")
	}
}
