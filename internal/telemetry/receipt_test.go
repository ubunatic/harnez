package telemetry

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestReceiptAtomicRollbackAndDedup(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	call := ToolCall{SessionID: "receipt", AgentID: "test", ToolName: "read", CallType: "hook:post"}
	fail := errors.New("after insert")
	if err := db.WithReceipt("record-1", func(tx *DB) error {
		if err := tx.Insert(call); err != nil {
			return err
		}
		if _, err := tx.InsertCompactionEvent(CompactionEvent{SessionID: "receipt", EventType: "precompact", Trigger: "manual"}); err != nil {
			return err
		}
		return fail
	}); !errors.Is(err, fail) {
		t.Fatalf("rollback: %v", err)
	}
	if rows, err := db.Query(Filter{SessionID: "receipt"}); err != nil || len(rows) != 0 {
		t.Fatalf("after rollback tool_calls = %d, %v, want 0", len(rows), err)
	}
	var events int
	if err := db.sql.QueryRow("SELECT count(*) FROM compaction_events WHERE session_id = ?", "receipt").Scan(&events); err != nil || events != 0 {
		t.Fatalf("after rollback compaction_events = %d, %v, want 0", events, err)
	}
	for i := 0; i < 2; i++ {
		if err := db.WithReceipt("record-1", func(tx *DB) error {
			if err := tx.Insert(call); err != nil {
				return err
			}
			_, err := tx.InsertCompactionEvent(CompactionEvent{SessionID: "receipt", EventType: "precompact", Trigger: "manual"})
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Query(Filter{SessionID: "receipt"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %d, %v", len(rows), err)
	}
	if err := db.sql.QueryRow("SELECT count(*) FROM compaction_events WHERE session_id = ?", "receipt").Scan(&events); err != nil || events != 1 {
		t.Fatalf("after retry compaction_events = %d, %v, want 1", events, err)
	}
}
