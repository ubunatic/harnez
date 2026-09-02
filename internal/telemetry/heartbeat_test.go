package telemetry

// Tests for issue 179's heartbeat additions: HeartbeatCallType's
// distinctness from failure ratings in storage/reporting, and
// HeartbeatStats' last-heartbeat/calls-since reporting.

import (
	"testing"
	"time"
)

func heartbeatCall(session string, at time.Time) ToolCall {
	return ToolCall{
		CreatedAt:   at,
		SessionID:   session,
		TicketID:    "179",
		ProjectName: "harnez",
		WorkingDir:  "/home/uwe/projects/harnez",
		AgentID:     "claude",
		ToolName:    "heartbeat",
		CallType:    HeartbeatCallType,
		Score:       nil,
		Note:        "ok",
		ExitCode:    nil,
		RawBytes:    20,
	}
}

func TestHeartbeat_DoesNotDiluteFailureAggregates(t *testing.T) {
	db := openTestDB(t)

	// One real failure rating (score 1) and several heartbeats. If
	// heartbeats leaked into the score/failure aggregates, AvgScore and
	// FailureCount below would be diluted/wrong.
	failing := sampleCall("sess-1", "Read", 1, 0)
	if err := db.Insert(failing); err != nil {
		t.Fatalf("Insert failing: %v", err)
	}
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := db.Insert(heartbeatCall("sess-1", now.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("Insert heartbeat: %v", err)
		}
	}

	byTool, err := db.AggregateByTool(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("AggregateByTool: %v", err)
	}
	var readGroup, heartbeatGroup *GroupStats
	for i := range byTool {
		switch byTool[i].Key {
		case "Read":
			readGroup = &byTool[i]
		case "heartbeat":
			heartbeatGroup = &byTool[i]
		}
	}
	if readGroup == nil {
		t.Fatalf("expected a Read group in %+v", byTool)
	}
	if readGroup.ScoredCount != 1 || readGroup.AvgScore != 1 {
		t.Errorf("Read group ScoredCount/AvgScore = %d/%v, want 1/1 (heartbeats must not dilute it)", readGroup.ScoredCount, readGroup.AvgScore)
	}
	if readGroup.FailureCount != 1 {
		t.Errorf("Read group FailureCount = %d, want 1", readGroup.FailureCount)
	}
	if heartbeatGroup == nil {
		t.Fatalf("expected a heartbeat group in %+v", byTool)
	}
	if heartbeatGroup.ScoredCount != 0 {
		t.Errorf("heartbeat group ScoredCount = %d, want 0 (score is always NULL)", heartbeatGroup.ScoredCount)
	}
	if heartbeatGroup.FailureCount != 0 {
		t.Errorf("heartbeat group FailureCount = %d, want 0 (NULL score/exit_code never counts as a failure)", heartbeatGroup.FailureCount)
	}
	if heartbeatGroup.Count != 5 {
		t.Errorf("heartbeat group Count = %d, want 5", heartbeatGroup.Count)
	}
}

func TestRateCallOverhead_IncludesBothRatingsAndHeartbeats(t *testing.T) {
	db := openTestDB(t)

	rateCall := sampleCall("sess-1", "Read", 5, 0)
	rateCall.RawBytes = 80
	hb := heartbeatCall("sess-1", time.Now().UTC())
	hb.RawBytes = 20

	for _, c := range []ToolCall{rateCall, hb} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	o, err := db.RateCallOverhead(Filter{})
	if err != nil {
		t.Fatalf("RateCallOverhead: %v", err)
	}
	if o.Count != 2 {
		t.Errorf("Count = %d, want 2 (one rating + one heartbeat)", o.Count)
	}
	if o.TotalCallBytes != 100 {
		t.Errorf("TotalCallBytes = %d, want 100", o.TotalCallBytes)
	}
}

func TestHeartbeatStats_EmptyIsZeroNotError(t *testing.T) {
	db := openTestDB(t)
	info, err := db.HeartbeatStats(Filter{})
	if err != nil {
		t.Fatalf("HeartbeatStats: %v", err)
	}
	if info.Count != 0 || !info.LastAt.IsZero() || info.CallsSince != 0 {
		t.Errorf("empty DB heartbeat stats = %+v, want all-zero", info)
	}
}

func TestHeartbeatStats_ReportsLastHeartbeatAndCallsSince(t *testing.T) {
	db := openTestDB(t)

	base := time.Now().UTC().Add(-time.Hour)
	first := heartbeatCall("sess-1", base)
	if err := db.Insert(first); err != nil {
		t.Fatalf("Insert first heartbeat: %v", err)
	}

	// Some ordinary tool calls after the first heartbeat.
	for i := 1; i <= 3; i++ {
		c := sampleCall("sess-1", "Read", 5, 0)
		c.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert call %d: %v", i, err)
		}
	}

	// A second, more recent heartbeat.
	second := heartbeatCall("sess-1", base.Add(10*time.Minute))
	if err := db.Insert(second); err != nil {
		t.Fatalf("Insert second heartbeat: %v", err)
	}

	// One more call after the second heartbeat.
	last := sampleCall("sess-1", "Edit", 5, 0)
	last.CreatedAt = base.Add(20 * time.Minute)
	if err := db.Insert(last); err != nil {
		t.Fatalf("Insert last call: %v", err)
	}

	info, err := db.HeartbeatStats(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("HeartbeatStats: %v", err)
	}
	if info.Count != 2 {
		t.Fatalf("Count = %d, want 2", info.Count)
	}
	if !info.LastAt.Equal(second.CreatedAt) {
		t.Errorf("LastAt = %v, want %v (the most recent heartbeat)", info.LastAt, second.CreatedAt)
	}
	if info.CallsSince != 1 {
		t.Errorf("CallsSince = %d, want 1 (only the Edit call after the second heartbeat)", info.CallsSince)
	}
}
