package telemetry

import (
	"testing"
)

func TestQualityChecksEmptyStorePass(t *testing.T) {
	got, err := openTestDB(t).QualityChecks()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.Checked != 0 || r.Offending != 0 || r.Warn {
			t.Errorf("%s = %+v, want empty pass", r.Name, r)
		}
	}
}

func TestQualityChecksCleanAndOddFixtures(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.sql.Exec("INSERT INTO tool_calls (created_at,session_id,agent_id,tool_name,call_type,input_tokens,output_tokens,total_tokens) VALUES ('2026-01-01T00:00:00Z','s1','a1','Read','internal',2,3,5)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec("INSERT INTO cli_invocations (created_at,session_id,agent_id,command) VALUES ('2026-01-01T00:00:00Z','s1','a1','stats')"); err != nil {
		t.Fatal(err)
	}
	id, err := db.InsertCompactionEvent(CompactionEvent{SessionID: "s1", EventType: "pre", TurnID: "1", Model: "gpt-5"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertSessionBoundary(SessionBoundary{SessionID: "s1", BoundaryType: "compact", CompactionEventID: &id}); err != nil {
		t.Fatal(err)
	}
	clean, err := db.QualityChecks()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range clean {
		if r.Warn {
			t.Errorf("clean %s warned: %+v", r.Name, r)
		}
	}
	if _, err := db.sql.Exec("INSERT INTO tool_calls (created_at,session_id,agent_id,tool_name,call_type,input_tokens,output_tokens,total_tokens) VALUES ('2026-01-01T00:00:00Z','','a1','Read','internal',-1,3,1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec("INSERT INTO cli_invocations (created_at,session_id,agent_id,command) VALUES ('2026-01-01T00:00:00Z','','a1','')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertCompactionEvent(CompactionEvent{SessionID: "s1", EventType: "pre", TurnID: "1"}); err != nil {
		t.Fatal(err)
	}
	missing := int64(999999)
	if _, err := db.InsertSessionBoundary(SessionBoundary{SessionID: "s1", BoundaryType: "compact", CompactionEventID: &missing}); err != nil {
		t.Fatal(err)
	}
	odds, err := db.QualityChecks()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"tool_calls_required_fields": true, "cli_invocations_required_fields": true, "compaction_models_present": true, "orphaned_session_boundaries": true, "duplicate_compaction_events": true, "invalid_tool_call_tokens": true}
	for _, r := range odds {
		if r.Warn != want[r.Name] {
			t.Errorf("%s warn=%v want %v", r.Name, r.Warn, want[r.Name])
		}
	}
}
