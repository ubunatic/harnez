package telemetry

import (
	"fmt"
	"strings"
	"time"
)

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

	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO tool_calls (
			created_at, session_id, ticket_id, project_name, working_dir,
			agent_id, tool_name, call_type, score, note, exit_code,
			duration_ms, raw_bytes, distilled_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		createdAt.Format(time.RFC3339Nano),
		tc.SessionID, tc.TicketID, tc.ProjectName, tc.WorkingDir,
		tc.AgentID, tc.ToolName, tc.CallType, tc.Score, tc.Note, tc.ExitCode,
		tc.DurationMs, tc.RawBytes, tc.DistilledBytes,
	)
	if err != nil {
		if isConstraintErr(err) {
			return fmt.Errorf("telemetry: insert violates schema constraint: %w", err)
		}
		return fmt.Errorf("telemetry: insert: %w", err)
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
