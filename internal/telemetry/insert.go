package telemetry

import (
	"fmt"
	"strings"
	"time"
)

func (d *DB) InsertCompactionEconomics(estimate PersistedEconomics) error {
	createdAt := estimate.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := d.sql.Exec(mustTelemetrySQL().Statements["insert_compaction_economics"], createdAt.Format(time.RFC3339Nano), estimate.SessionID, estimate.CompactionEventID, estimate.Model, estimate.PricingRevision, estimate.Rates.CachedInputMicrosPerMillion, estimate.Rates.UncachedInputMicrosPerMillion, estimate.Rates.OutputMicrosPerMillion, estimate.Rates.ReasoningMicrosPerMillion, estimate.Status, estimate.CompactionCostMicros, estimate.PostCompactionCostMicros, estimate.BaselineCostMicros, estimate.SavingsMicros, estimate.Note)
	if err != nil {
		return fmt.Errorf("telemetry: insert compaction economics: %w", err)
	}
	return nil
}

func (d *DB) InsertCompactionEvent(event CompactionEvent) (int64, error) {
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	result, err := d.sql.Exec(mustTelemetrySQL().Statements["insert_compaction_event"], createdAt.Format(time.RFC3339Nano), event.SessionID, event.EventType, event.TurnID, event.Trigger, event.Reason, event.InputTokens, event.CachedInputTokens, event.OutputTokens, event.ReasoningTokens, event.TotalTokens, event.Model)
	if err != nil {
		return 0, fmt.Errorf("telemetry: insert compaction event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("telemetry: compaction event id: %w", err)
	}
	return id, nil
}

func (d *DB) InsertSessionBoundary(boundary SessionBoundary) (int64, error) {
	createdAt := boundary.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	result, err := d.sql.Exec(mustTelemetrySQL().Statements["insert_session_boundary"], createdAt.Format(time.RFC3339Nano), boundary.SessionID, boundary.BoundaryType, boundary.CompactionEventID)
	if err != nil {
		return 0, fmt.Errorf("telemetry: insert session boundary: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("telemetry: session boundary id: %w", err)
	}
	return id, nil
}

func (d *DB) InsertTokenSnapshot(snapshot TokenSnapshot) error {
	createdAt := snapshot.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	uncached := snapshot.UncachedInputTokens
	if uncached == nil && snapshot.InputTokens != nil && snapshot.CachedInputTokens != nil {
		value := *snapshot.InputTokens - *snapshot.CachedInputTokens
		if value >= 0 {
			uncached = &value
		}
	}
	_, err := d.sql.Exec(mustTelemetrySQL().Statements["insert_token_snapshot"], createdAt.Format(time.RFC3339Nano), snapshot.SessionID, snapshot.Source, snapshot.BoundaryID, snapshot.InputTokens, snapshot.CachedInputTokens, uncached, snapshot.OutputTokens, snapshot.ReasoningTokens, snapshot.TotalTokens, snapshot.LastInputTokens, snapshot.LastCachedInputTokens, snapshot.LastOutputTokens, snapshot.LastReasoningTokens, snapshot.LastTotalTokens)
	if err != nil {
		return fmt.Errorf("telemetry: insert token snapshot: %w", err)
	}
	return nil
}

// Insert writes one ToolCall synchronously. It does not duplicate the
// schema's constraints (e.g. the score 1-5 range) as a parallel Go
// validation list — invalid values are rejected by the DB's own CHECK
// constraints (schema.go) and surfaced here as a wrapped error, per
// docs/other/Spec.md's "don't shadow the spec" rule applied to this DDL.
//
// tc.ID is ignored; the row's id is assigned by SQLite (AUTOINCREMENT).
// tc.CreatedAt defaults to time.Now().UTC() if zero.
func (d *DB) Insert(tc ToolCall) error {
	createdAt := tc.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	ctx, cancel := defaultContext()
	defer cancel()

	_, err := d.sql.ExecContext(ctx, mustTelemetrySQL().Statements["insert_tool_call"],
		createdAt.Format(time.RFC3339Nano),
		tc.SessionID, tc.TicketID, tc.ProjectName, tc.WorkingDir,
		tc.AgentID, tc.ToolName, tc.CallType, tc.Score, tc.Note, tc.ExitCode,
		tc.DurationMs, tc.RawBytes, tc.DistilledBytes, tc.OutputBytes, tc.ActualTokens,
		tc.InputTokens, tc.CachedInputTokens, tc.OutputTokens, tc.ReasoningTokens, tc.TotalTokens,
		tc.PotentialSavingsTokens, tc.PotentialSavingsBytes,
	)
	if err != nil {
		if isConstraintErr(err) {
			return fmt.Errorf("telemetry: insert violates schema constraint: %w", err)
		}
		return fmt.Errorf("telemetry: insert: %w", err)
	}
	return nil
}

// CLIInvocationRowCap is the retention bound for the cli_invocations table
// (issue 326). Unlike tool_calls (written only on a deliberate `harnez
// rate`/`harnez exec` call) and issue_status_snapshots (deduped against the
// previous row), cli_invocations gets a row per invocation forever, so it
// needs a retention story from day one. A flat row cap is used rather than
// an age-based prune because "how much history is worth keeping" is a size
// question, not a calendar one: a heavy multi-project day and a quiet month
// should both leave a bounded, similarly-useful window for `harnez log`.
// 20k rows is roughly a few months of heavy use at a few MB on disk.
const CLIInvocationRowCap = 20000

// InsertCLIInvocation writes one CLIInvocation row and prunes the table
// back to CLIInvocationRowCap. c.ID is ignored (assigned by SQLite);
// c.CreatedAt defaults to time.Now().UTC() if zero.
func (d *DB) InsertCLIInvocation(c CLIInvocation) error {
	return d.insertCLIInvocation(c, CLIInvocationRowCap)
}

// insertCLIInvocation is InsertCLIInvocation with an injectable row cap, so
// the retention behavior is testable without writing CLIInvocationRowCap
// rows.
func (d *DB) insertCLIInvocation(c CLIInvocation, rowCap int64) error {
	createdAt := c.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	ctx, cancel := defaultContext()
	defer cancel()

	_, err := d.sql.ExecContext(ctx, mustTelemetrySQL().Statements["insert_cli_invocation"],
		createdAt.Format(time.RFC3339Nano),
		c.SessionID, c.AgentID, c.Command, c.Args, c.ProjectName,
		c.WorkingDir, c.TicketID, c.ExitCode, c.DurationMs, c.HarnezVersion,
	)
	if err != nil {
		if isConstraintErr(err) {
			return fmt.Errorf("telemetry: insert cli invocation violates schema constraint: %w", err)
		}
		return fmt.Errorf("telemetry: insert cli invocation: %w", err)
	}
	return d.pruneCLIInvocations(rowCap)
}

// pruneCLIInvocations deletes everything older than the newest rowCap rows.
// It leans on the AUTOINCREMENT id as the monotonic insertion order rather
// than counting rows first: MAX(id) - rowCap is the exact cutoff as long as
// the only source of id gaps is this prune itself (which always removes a
// prefix), so this stays a single statement costing O(rows actually
// deleted) on every write instead of a COUNT(*) scan.
func (d *DB) pruneCLIInvocations(rowCap int64) error {
	if rowCap <= 0 {
		return nil
	}
	ctx, cancel := defaultContext()
	defer cancel()

	_, err := d.sql.ExecContext(ctx, `
		DELETE FROM cli_invocations
		WHERE id <= (SELECT MAX(id) - ? FROM cli_invocations)`, rowCap)
	if err != nil {
		return fmt.Errorf("telemetry: prune cli invocations: %w", err)
	}
	return nil
}

// isConstraintErr reports whether err looks like a SQLite CHECK/NOT NULL
// constraint violation, so Insert can give callers a clearer message
// without hardcoding which constraint fired (that list lives only in
// schemaDDL).
func isConstraintErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "constraint")
}
