package telemetry

// Tests for issue 188's UnratedFailureCount: the session-window heuristic
// that flags genuinely-failed tool calls with no `harnez rate` call since.

import (
	"testing"
	"time"
)

// execCall builds a regular (non-rate) tool_calls row, as cmd/harnez/exec.go
// would insert one — call_type "" — with the given score/exit code and
// timestamp.
func execCall(session, tool string, score, exitCode *int, at time.Time) ToolCall {
	return ToolCall{
		CreatedAt:   at,
		SessionID:   session,
		TicketID:    "188",
		ProjectName: "harnez",
		WorkingDir:  "/home/uwe/projects/harnez",
		AgentID:     "claude",
		ToolName:    tool,
		CallType:    "",
		Score:       score,
		Note:        "",
		ExitCode:    exitCode,
		DurationMs:  5,
		RawBytes:    100,
	}
}

// rateCall builds a `harnez rate` failure-rating row (call_type "internal").
func rateCall(session string, at time.Time) ToolCall {
	return ToolCall{
		CreatedAt:   at,
		SessionID:   session,
		TicketID:    "188",
		ProjectName: "harnez",
		WorkingDir:  "/home/uwe/projects/harnez",
		AgentID:     "claude",
		ToolName:    "Read",
		CallType:    rateCallType,
		Score:       intPtr(1),
		Note:        "rated",
		ExitCode:    nil,
		RawBytes:    50,
	}
}

func TestUnratedFailureCount_UnratedFailureTriggers(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()

	// A genuine failure (non-zero exit code) with no rate call afterward.
	if err := db.Insert(execCall("sess-1", "Bash", nil, intPtr(1), now)); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 1 {
		t.Errorf("UnratedFailureCount = %d, want 1", n)
	}
}

func TestUnratedFailureCount_RatedFailureDoesNotCount(t *testing.T) {
	db := openTestDB(t)
	start := time.Now().UTC()

	if err := db.Insert(execCall("sess-1", "Bash", nil, intPtr(1), start)); err != nil {
		t.Fatalf("Insert failing call: %v", err)
	}
	if err := db.Insert(rateCall("sess-1", start.Add(time.Second))); err != nil {
		t.Fatalf("Insert rate call: %v", err)
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 0 {
		t.Errorf("UnratedFailureCount = %d, want 0 (failure was rated)", n)
	}
}

func TestUnratedFailureCount_ZeroFailuresNoTrigger(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()

	// All successful calls, never rated.
	for i := 0; i < 3; i++ {
		if err := db.Insert(execCall("sess-1", "Read", nil, intPtr(0), now.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 0 {
		t.Errorf("UnratedFailureCount = %d, want 0 (no failures)", n)
	}
}

func TestUnratedFailureCount_LowScoreCountsAsFailure(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()

	// score <= 2 counts as a failure per GroupStats.FailureCount's
	// definition, even with exit_code 0/nil.
	if err := db.Insert(execCall("sess-1", "Grep", intPtr(2), nil, now)); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 1 {
		t.Errorf("UnratedFailureCount = %d, want 1", n)
	}
}

func TestUnratedFailureCount_OnlyCountsFailuresAfterLastRate(t *testing.T) {
	db := openTestDB(t)
	start := time.Now().UTC()

	// Failure #1, then a rate call addressing it, then a second,
	// still-unrated failure.
	if err := db.Insert(execCall("sess-1", "Bash", nil, intPtr(1), start)); err != nil {
		t.Fatalf("Insert failure 1: %v", err)
	}
	if err := db.Insert(rateCall("sess-1", start.Add(time.Second))); err != nil {
		t.Fatalf("Insert rate call: %v", err)
	}
	if err := db.Insert(execCall("sess-1", "Bash", nil, intPtr(1), start.Add(2*time.Second))); err != nil {
		t.Fatalf("Insert failure 2: %v", err)
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 1 {
		t.Errorf("UnratedFailureCount = %d, want 1 (only the post-rate failure)", n)
	}
}

// TestUnratedFailureCount_ExpectedFailureExcluded covers issue 226: a
// shell row marked call_type=ExpectedFailureCallType (the classification
// cmd/harnez/exec.go writes for a HARNEZ_EXPECT_FAILURE=1-prefixed
// command) is not counted, even though it carries a real non-zero
// exit_code that would otherwise trip UnratedFailureCount.
func TestUnratedFailureCount_ExpectedFailureExcluded(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()

	expected := execCall("sess-1", "Bash", nil, intPtr(1), now)
	expected.CallType = ExpectedFailureCallType
	if err := db.Insert(expected); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 0 {
		t.Errorf("UnratedFailureCount = %d, want 0 (expected failure excluded)", n)
	}
}

// TestUnratedFailureCount_MixedExpectedAndRealFailures ensures the
// exclusion is per-row: an expected-failure row sitting alongside a real,
// unmarked failure must not mask the real one.
func TestUnratedFailureCount_MixedExpectedAndRealFailures(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()

	expected := execCall("sess-1", "Bash", nil, intPtr(1), now)
	expected.CallType = ExpectedFailureCallType
	real := execCall("sess-1", "Bash", nil, intPtr(1), now.Add(time.Second))
	for _, c := range []ToolCall{expected, real} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 1 {
		t.Errorf("UnratedFailureCount = %d, want 1 (only the unmarked real failure)", n)
	}
}

func TestUnratedFailureCount_ScopedToSession(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()

	if err := db.Insert(execCall("sess-1", "Bash", nil, intPtr(1), now)); err != nil {
		t.Fatalf("Insert sess-1 failure: %v", err)
	}
	if err := db.Insert(execCall("sess-2", "Bash", nil, intPtr(1), now)); err != nil {
		t.Fatalf("Insert sess-2 failure: %v", err)
	}

	n, err := db.UnratedFailureCount(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("UnratedFailureCount: %v", err)
	}
	if n != 1 {
		t.Errorf("UnratedFailureCount = %d, want 1 (scoped to sess-1 only)", n)
	}
}
