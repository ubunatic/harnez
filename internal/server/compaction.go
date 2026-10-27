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
	socket, err := config.Socket()
	if err != nil {
		return err
	}
	spool, err := config.TelemetrySpoolDir()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(sessionID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(TelemetryRecord{Kind: "agy_compaction", Payload: payload})
	if err != nil {
		return err
	}
	if PostTelemetry(ctx, socket, config.TelemetryPath, body, config.UpdateTimeout, spool, config.TelemetrySpoolMaxBytes) == UpdateDropped {
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
	for _, kind := range []string{"precompact", "postcompact"} {
		if _, err := db.InsertCompactionEvent(telemetry.CompactionEvent{SessionID: sessionID, EventType: kind, Trigger: "manual", Source: "stream_fallback"}); err != nil {
			return err
		}
	}
	return nil
}
