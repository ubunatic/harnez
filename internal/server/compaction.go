package server

import (
	"context"
	"encoding/json"
	"fmt"

	"ubunatic.com/harnez/internal/telemetry"
)

// SendCompactionPair submits a successful agy checkpoint to the collector.
func SendCompactionPair(ctx context.Context, sessionID string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(sessionID)
	if err != nil {
		return err
	}
	record, err := NewTelemetryRecord("agy_compaction", payload)
	if err != nil {
		return err
	}
	if DispatchTelemetry(ctx, config, record) == UpdateDropped {
		return fmt.Errorf("agy compact: telemetry record dropped")
	}
	return nil
}

func InsertAgyCompactionPair(path, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("agy compact: missing session ID")
	}
	db, err := telemetry.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return InsertAgyCompactionPairTx(db, sessionID)
}

// InsertAgyCompactionPairTx writes both events through the supplied transaction.
func InsertAgyCompactionPairTx(db *telemetry.DB, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("agy compact: missing session ID")
	}
	for _, kind := range []string{"precompact", "postcompact"} {
		if _, err := db.InsertCompactionEvent(telemetry.CompactionEvent{SessionID: sessionID, EventType: kind, Trigger: "manual", Source: "stream_fallback"}); err != nil {
			return err
		}
	}
	return nil
}
