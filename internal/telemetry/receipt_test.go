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
		return fail
	}); !errors.Is(err, fail) {
		t.Fatalf("rollback: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := db.WithReceipt("record-1", func(tx *DB) error { return tx.Insert(call) }); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Query(Filter{SessionID: "receipt"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %d, %v", len(rows), err)
	}
}
