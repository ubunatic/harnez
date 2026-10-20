package server

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// PrepareCodexTelemetry captures client-local evidence before crossing the socket.
func PrepareCodexTelemetry(raw []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, nil
	}
	e := codex.ParseEvent(raw)
	var hook struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
	}
	_ = json.Unmarshal(raw, &hook)
	if e.SessionID == "" {
		e.SessionID = hook.SessionID
	}
	wd, _ := os.Getwd()
	ticket, _ := resolve.Ticket(resolve.TicketOptions{SessionID: e.SessionID})
	evidence := codexEvidence{WorkingDir: wd, TicketID: ticket, CreatedAt: time.Now().UTC()}
	if hook.TranscriptPath != "" {
		evidence.Transcript = latestCodexTranscriptTokens(hook.TranscriptPath, e.SessionID)
	}
	payload["harnez_context"], _ = json.Marshal(evidence)
	return json.Marshal(payload)
}

type codexEvidence struct {
	WorkingDir string
	TicketID   string
	CreatedAt  time.Time
	Transcript codex.Event
}

func ProcessCodexTelemetry(dbPath string, raw []byte) error {
	e := codex.ParseEvent(raw)
	var hook struct {
		SessionID string        `json:"session_id"`
		Context   codexEvidence `json:"harnez_context"`
	}
	_ = json.Unmarshal(raw, &hook)
	if e.SessionID == "" {
		e.SessionID = hook.SessionID
	}
	if e.SessionID == "" {
		return nil
	}
	wd, ticket := hook.Context.WorkingDir, hook.Context.TicketID
	createdAt := hook.Context.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	transcriptUsage := hook.Context.Transcript
	if e.Kind == "precompact" || e.Kind == "postcompact" {
		id, err := db.InsertCompactionEvent(telemetry.CompactionEvent{CreatedAt: createdAt, SessionID: e.SessionID, EventType: e.Kind, TurnID: e.TurnID, Trigger: e.CompactionTrigger, Reason: e.CompactionReason, InputTokens: e.InputTokens, CachedInputTokens: e.CachedInputTokens, OutputTokens: e.OutputTokens, ReasoningTokens: e.ReasoningTokens, TotalTokens: e.TotalTokens, Model: e.Model})
		if err != nil {
			return err
		}
		boundaryID, err := db.InsertSessionBoundary(telemetry.SessionBoundary{CreatedAt: createdAt, SessionID: e.SessionID, BoundaryType: e.Kind, CompactionEventID: &id})
		if err != nil {
			return err
		}
		if err := insertCodexTokenSnapshot(db, e.SessionID, e.Kind, &boundaryID, e, transcriptUsage); err != nil {
			return err
		}
		if e.Kind == "postcompact" {
			catalog, err := telemetry.LoadPricingCatalog("")
			if err != nil {
				return err
			}
			return db.ReconcileCompactionEconomics(e.SessionID, e.Model, catalog)
		}
		return nil
	}
	if e.Kind == "sessionstart" || e.Kind == "session_end" || e.Kind == "sessionend" {
		boundaryID, err := db.InsertSessionBoundary(telemetry.SessionBoundary{CreatedAt: createdAt, SessionID: e.SessionID, BoundaryType: e.Kind})
		if err != nil {
			return err
		}
		source := e.Kind
		if source == "session_end" {
			source = "sessionend"
		}
		if err := insertCodexTokenSnapshot(db, e.SessionID, source, &boundaryID, e, transcriptUsage); err != nil {
			return err
		}
		if e.Kind == "sessionend" || e.Kind == "session_end" {
			catalog, err := telemetry.LoadPricingCatalog("")
			if err != nil {
				return err
			}
			return db.ReconcileCompactionEconomics(e.SessionID, e.Model, catalog)
		}
		return nil
	}
	if transcriptUsage.TotalTokens != nil {
		_ = db.UpdateLatestProviderUsage(e.SessionID, transcriptUsage.LastTotalTokens, transcriptUsage.LastInputTokens, transcriptUsage.LastCachedInputTokens, transcriptUsage.LastOutputTokens, transcriptUsage.LastReasoningTokens, transcriptUsage.TotalTokens)
	}
	if err := insertCodexTokenSnapshot(db, e.SessionID, "tool", nil, e, transcriptUsage); err != nil {
		return err
	}
	if e.ToolName == "" {
		return nil
	}
	note := "codex:" + e.ToolCallID
	if e.ToolCallID != "" {
		if calls, err := db.Query(telemetry.Filter{SessionID: e.SessionID}); err == nil {
			for _, prior := range calls {
				if prior.Note == note {
					return nil
				}
			}
		}
	}
	callType := "hook:post"
	if e.Success != nil && !*e.Success {
		callType = "hook:failure"
	}
	call := telemetry.ToolCall{CreatedAt: createdAt, SessionID: e.SessionID, TicketID: ticket, ProjectName: filepath.Base(wd), WorkingDir: wd, AgentID: "codex", ToolName: e.ToolName, CallType: callType, Note: note, DurationMs: e.DurationMs, ExitCode: e.ExitCode, OutputBytes: e.OutputBytes, ActualTokens: e.LastTotalTokens, InputTokens: e.LastInputTokens, CachedInputTokens: e.LastCachedInputTokens, OutputTokens: e.LastOutputTokens, ReasoningTokens: e.LastReasoningTokens, TotalTokens: e.TotalTokens}
	if err := db.Insert(call); err != nil {
		return err
	}
	if skill := telemetry.SkillFromFileRef(e.Command); skill != "" {
		_ = db.Insert(telemetry.ToolCall{CreatedAt: createdAt, SessionID: e.SessionID, TicketID: ticket, ProjectName: filepath.Base(wd), WorkingDir: wd, AgentID: "codex", ToolName: telemetry.SkillToolName, CallType: telemetry.CallTypeSkillInferred, SkillName: skill})
	}
	return nil
}

func insertCodexTokenSnapshot(db *telemetry.DB, sessionID, source string, boundaryID *int64, event, transcript codex.Event) error {
	if event.TotalTokens == nil && transcript.TotalTokens == nil && event.LastTotalTokens == nil && transcript.LastTotalTokens == nil {
		return nil
	}
	if event.TotalTokens == nil {
		event.InputTokens, event.CachedInputTokens, event.OutputTokens, event.ReasoningTokens, event.TotalTokens = transcript.InputTokens, transcript.CachedInputTokens, transcript.OutputTokens, transcript.ReasoningTokens, transcript.TotalTokens
	}
	if event.LastTotalTokens == nil {
		event.LastInputTokens, event.LastCachedInputTokens, event.LastOutputTokens, event.LastReasoningTokens, event.LastTotalTokens = transcript.LastInputTokens, transcript.LastCachedInputTokens, transcript.LastOutputTokens, transcript.LastReasoningTokens, transcript.LastTotalTokens
	}
	return db.InsertTokenSnapshot(telemetry.TokenSnapshot{CreatedAt: time.Now().UTC(), SessionID: sessionID, Source: source, BoundaryID: boundaryID, InputTokens: event.InputTokens, CachedInputTokens: event.CachedInputTokens, OutputTokens: event.OutputTokens, ReasoningTokens: event.ReasoningTokens, TotalTokens: event.TotalTokens, LastInputTokens: event.LastInputTokens, LastCachedInputTokens: event.LastCachedInputTokens, LastOutputTokens: event.LastOutputTokens, LastReasoningTokens: event.LastReasoningTokens, LastTotalTokens: event.LastTotalTokens})
}

func latestCodexTranscriptTokens(path, sessionID string) codex.Event {
	file, err := os.Open(path)
	if err != nil {
		return codex.Event{}
	}
	defer file.Close()
	var latest codex.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		e := codex.ParseEvent(scanner.Bytes())
		if e.SessionID != "" && e.SessionID != sessionID || e.TotalTokens == nil {
			continue
		}
		latest = e
	}
	return latest
}
