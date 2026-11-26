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
	if e.Kind == "postcompact" {
		// The PostCompact hook reads the transcript once and never waits. Claude
		// writes its compact boundary after PostCompact returns, so the collector
		// fills the row later (issue 1303 M1e). The diagnostic records what this read
		// saw (issue 1303 M1c). Its keys are taken before harnez_context is added.
		_, evidence.Claude = payload["compact_summary"]
		evidence.Diag = &compactDiag{Side: "hook", PID: os.Getpid(), TranscriptSet: hook.TranscriptPath != "", Keys: payloadKeys(payload)}
		if hook.TranscriptPath == "" {
			evidence.Diag.Reason = reasonNoTranscript
		} else {
			evidence.Compact, evidence.Diag.Reason = singleClaudeCompactRead(hook.TranscriptPath, e.SessionID)
		}
		// The compact summary and custom instructions are user text. They are dropped
		// here, before the payload crosses the socket or is spooled (issue 1303 M3).
		// Diag.Keys above still names them.
		delete(payload, "compact_summary")
		delete(payload, "custom_instructions")
	}
	payload["harnez_context"], _ = json.Marshal(evidence)
	return json.Marshal(payload)
}

type codexEvidence struct {
	WorkingDir string
	TicketID   string
	CreatedAt  time.Time
	Transcript codex.Event
	Compact    *claudeCompact
	// Claude is whether the payload is Claude's (it carries compact_summary). Only
	// Claude's PostCompact row waits for a boundary (issue 1303 M1e).
	Claude bool
	// Diag is the hook's view of a PostCompact (issue 1303 M1c). Only the
	// collector's note reads it; it is omitted for every other event.
	Diag *compactDiag `json:",omitempty"`
}

// applyClaudeCompact decides a PostCompact against the session's PreCompact row
// (issue 1303 M1, M1c, M1d). It writes nothing: the caller applies an applied
// boundary with applyClaudeBoundary. The PreCompact time is read even when no
// boundary is found, because a pending fill needs it (issue 1303 M1e).
func applyClaudeCompact(db *telemetry.DB, sessionID string, compact *claudeCompact) (compactOutcome, error) {
	preAt, ok, err := db.LatestCompactionEventTime(sessionID, "precompact")
	if err != nil {
		return compactOutcome{}, err
	}
	var out compactOutcome
	if ok {
		out.preAt = &preAt
	}
	if compact != nil {
		boundaryAt := compact.Timestamp
		out.boundaryAt = &boundaryAt
	}
	if compact != nil && !ok {
		out.reason = reasonNoPrecompact
	} else {
		out.reason = claudeBoundaryReason(compact, preAt)
	}
	return out, nil
}

// applyClaudeBoundary puts an applied boundary's tokens on the PostCompact event and
// writes the PreCompact snapshot that ReconcileCompactionEconomics reads. Claude
// gives no cache split, so the whole context is stored as uncached input: an upper
// bound, reported as partial pricing.
func applyClaudeBoundary(db *telemetry.DB, e *codex.Event, compact *claudeCompact) error {
	if err := insertClaudeSnapshot(db, e.SessionID, "precompact", nil, compact.PreTokens); err != nil {
		return err
	}
	post := compact.PostTokens
	e.InputTokens, e.TotalTokens = &post, &post
	e.Model = compact.Model
	return nil
}

// compactOutcome is the collector's decision for one PostCompact: its reason and
// the timestamps behind it. Nil times are unknown (issue 1303 M1d).
type compactOutcome struct {
	reason     string
	preAt      *time.Time
	boundaryAt *time.Time
}

// compactNote is the collector's note for one PostCompact attempt (issue 1303 M1c,
// M1e). filledAfter is set only on an applied fill.
func compactNote(sessionID string, outcome compactOutcome, hook *compactDiag, filledAfter string) compactDiagNote {
	return compactDiagNote{At: time.Now().UTC(), SessionID: sessionID, Kind: "postcompact", Reason: outcome.reason, PreCompactAt: outcome.preAt, BoundaryAt: outcome.boundaryAt, FilledAfter: filledAfter, Hook: hook, CollectorPID: os.Getpid()}
}

func insertClaudeSnapshot(db *telemetry.DB, sessionID, source string, boundaryID *int64, tokens int64) error {
	return db.InsertTokenSnapshot(telemetry.TokenSnapshot{CreatedAt: time.Now().UTC(), SessionID: sessionID, Source: source, BoundaryID: boundaryID, InputTokens: &tokens, UncachedInputTokens: &tokens, TotalTokens: &tokens})
}

func ProcessCodexTelemetry(dbPath string, raw []byte) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return ProcessCodexTelemetryTx(db, raw)
}
func ProcessCodexTelemetryTx(db *telemetry.DB, raw []byte) error {
	e := codex.ParseEvent(raw)
	var hook struct {
		SessionID      string        `json:"session_id"`
		TranscriptPath string        `json:"transcript_path"`
		Context        codexEvidence `json:"harnez_context"`
	}
	_ = json.Unmarshal(raw, &hook)
	if e.SessionID == "" {
		e.SessionID = hook.SessionID
	}
	if e.SessionID == "" {
		return nil
	}
	// Every session event gives a pending Claude fill one attempt (issue 1303 M1e).
	// Best effort: a failed attempt leaves the record for the next event, and must
	// not drop this event's own telemetry.
	_ = claudeFillOnEvent(db, e.SessionID)
	wd, ticket := hook.Context.WorkingDir, hook.Context.TicketID
	createdAt := hook.Context.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	transcriptUsage := hook.Context.Transcript
	if e.Kind == "precompact" || e.Kind == "postcompact" {
		claudeApplied := false
		var outcome compactOutcome
		if e.Kind == "postcompact" {
			var err error
			outcome, err = applyClaudeCompact(db, e.SessionID, hook.Context.Compact)
			if err != nil {
				return err
			}
			if outcome.reason == reasonApplied {
				claudeApplied = true
				if err := applyClaudeBoundary(db, &e, hook.Context.Compact); err != nil {
					return err
				}
			}
		}
		id, err := db.InsertCompactionEvent(telemetry.CompactionEvent{CreatedAt: createdAt, SessionID: e.SessionID, EventType: e.Kind, TurnID: e.TurnID, Trigger: e.CompactionTrigger, Reason: e.CompactionReason, InputTokens: e.InputTokens, CachedInputTokens: e.CachedInputTokens, OutputTokens: e.OutputTokens, ReasoningTokens: e.ReasoningTokens, TotalTokens: e.TotalTokens, Model: e.Model})
		if err != nil {
			return err
		}
		boundaryID, err := db.InsertSessionBoundary(telemetry.SessionBoundary{CreatedAt: createdAt, SessionID: e.SessionID, BoundaryType: e.Kind, CompactionEventID: &id})
		if err != nil {
			return err
		}
		if claudeApplied {
			if err := insertClaudeSnapshot(db, e.SessionID, e.Kind, &boundaryID, *e.TotalTokens); err != nil {
				return err
			}
		} else if err := insertCodexTokenSnapshot(db, e.SessionID, e.Kind, &boundaryID, e, transcriptUsage); err != nil {
			return err
		}
		if e.Kind == "postcompact" {
			catalog, err := telemetry.LoadPricingCatalog("")
			if err != nil {
				return err
			}
			if err := db.ReconcileCompactionEconomics(e.SessionID, e.Model, catalog); err != nil {
				return err
			}
			if hook.Context.Claude && !claudeApplied && outcome.preAt != nil {
				// The boundary is not written yet: Claude writes it after PostCompact
				// returns. Fill the row later (issue 1303 M1e).
				p := claudeFillPending{SessionID: e.SessionID, TranscriptPath: hook.TranscriptPath, PreCompactAt: *outcome.preAt, RowID: id, BoundaryID: boundaryID, Reason: outcome.reason, ArmedAt: time.Now().UTC(), Hook: hook.Context.Diag}
				if err := armClaudeFill(db.Path(), p); err != nil {
					return err
				}
			}
			// The hook's reason wins when the hook found no boundary (issue 1303 M1c).
			if hook.Context.Diag != nil && hook.Context.Diag.Reason != "" {
				outcome.reason = hook.Context.Diag.Reason
			}
			return writeCompactDiagnostic(db.Path(), compactNote(e.SessionID, outcome, hook.Context.Diag, ""))
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
		if err := db.UpdateLatestProviderUsage(e.SessionID, transcriptUsage.LastTotalTokens, transcriptUsage.LastInputTokens, transcriptUsage.LastCachedInputTokens, transcriptUsage.LastOutputTokens, transcriptUsage.LastReasoningTokens, transcriptUsage.TotalTokens); err != nil {
			return err
		}
	}
	if err := insertCodexTokenSnapshot(db, e.SessionID, "tool", nil, e, transcriptUsage); err != nil {
		return err
	}
	if e.ToolName == "" {
		return nil
	}
	note := "codex:" + e.ToolCallID
	if e.ToolCallID != "" {
		calls, err := db.Query(telemetry.Filter{SessionID: e.SessionID})
		if err != nil {
			return err
		}
		for _, prior := range calls {
			if prior.Note == note {
				return nil
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
		if err := db.Insert(telemetry.ToolCall{CreatedAt: createdAt, SessionID: e.SessionID, TicketID: ticket, ProjectName: filepath.Base(wd), WorkingDir: wd, AgentID: "codex", ToolName: telemetry.SkillToolName, CallType: telemetry.CallTypeSkillInferred, SkillName: skill}); err != nil {
			return err
		}
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
