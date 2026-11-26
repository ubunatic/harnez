package main

import (
	"context"
	"encoding/json"
	"time"

	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

// compactQuotaCapture reads the quota for a compaction sample. Tests replace it
// so no test reaches the live usage endpoint (issue 1303 M2).
var compactQuotaCapture = usage.CaptureCompactQuota

// compactQuotaAsync runs a sample after the record's transaction has committed,
// so a slow fetch never holds a write lock or delays the record. Tests run it
// inline.
var compactQuotaAsync = func(fn func()) { go fn() }

// scheduleClaudeCompactQuota starts one quota sample for a Claude PostCompact
// record the collector has just stored. Other records are ignored. It runs in
// the collector only, never in the hook process (issue 1303 M2).
func scheduleClaudeCompactQuota(dbPath string, record server.TelemetryRecord) bool {
	e := codex.ParseEvent(record.Payload)
	var hook struct {
		SessionID string `json:"session_id"`
		Context   struct {
			Claude bool `json:"claude"`
		} `json:"harnez_context"`
	}
	_ = json.Unmarshal(record.Payload, &hook)
	sessionID := e.SessionID
	if sessionID == "" {
		sessionID = hook.SessionID
	}
	if e.Kind != "postcompact" || !hook.Context.Claude || sessionID == "" || record.RecordID == "" {
		return false
	}
	compactQuotaAsync(func() {
		_ = usage.RecordCompactQuota(context.Background(), dbPath, record.RecordID, sessionID, time.Now().UTC(), compactQuotaCapture)
	})
	return true
}
