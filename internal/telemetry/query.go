package telemetry

import (
	"fmt"
	"strings"
	"time"
)

// Filter narrows a Query/Aggregate call. Zero-value fields are ignored
// (unfiltered). This is deliberately just what a WHERE-filtered aggregate
// query needs today (issue 120's harnez stats consumes this) — no
// generic query-builder abstraction.
type Filter struct {
	ToolName  string
	AgentID   string
	TicketID  string
	SessionID string
	CallType  string
	Since     time.Time // rows with created_at >= Since, if non-zero
	Until     time.Time // rows with created_at < Until, if non-zero
}

// whereClause builds a "WHERE ..." SQL fragment (or "" if unfiltered) and
// its bound args, shared by Query and Aggregate.
func (f Filter) whereClause() (string, []any) {
	var clauses []string
	var args []any

	add := func(col, val string) {
		if val != "" {
			clauses = append(clauses, col+" = ?")
			args = append(args, val)
		}
	}
	add("tool_name", f.ToolName)
	add("agent_id", f.AgentID)
	add("ticket_id", f.TicketID)
	add("session_id", f.SessionID)
	add("call_type", f.CallType)
	if !f.Since.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.Since.Format(time.RFC3339Nano))
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "created_at < ?")
		args = append(args, f.Until.Format(time.RFC3339Nano))
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// Query returns the tool_calls rows matching f, newest first.
func (d *DB) Query(f Filter) ([]ToolCall, error) {
	where, args := f.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, created_at, session_id, ticket_id, project_name, working_dir,
		       agent_id, tool_name, call_type, score, note, exit_code,
		       duration_ms, raw_bytes, distilled_bytes
		FROM tool_calls`+where+`
		ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query: %w", err)
	}
	defer rows.Close()

	var out []ToolCall
	for rows.Next() {
		var tc ToolCall
		var createdAt string
		if err := rows.Scan(
			&tc.ID, &createdAt, &tc.SessionID, &tc.TicketID, &tc.ProjectName,
			&tc.WorkingDir, &tc.AgentID, &tc.ToolName, &tc.CallType, &tc.Score,
			&tc.Note, &tc.ExitCode, &tc.DurationMs, &tc.RawBytes, &tc.DistilledBytes,
		); err != nil {
			return nil, fmt.Errorf("telemetry: scan row: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("telemetry: parse created_at %q: %w", createdAt, err)
		}
		tc.CreatedAt = parsed
		out = append(out, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: query rows: %w", err)
	}
	return out, nil
}

// Stats is an aggregate summary over a Filter's matching rows — the shape
// issue 120's `harnez stats` needs: how many calls, how well they scored,
// how often they failed, and how many bytes distillation saved.
type Stats struct {
	Count          int64
	AvgScore       float64 // 0 if no scored rows
	ScoredCount    int64   // rows with a non-NULL score, denominator for AvgScore
	FailureCount   int64   // rows with exit_code != 0
	ExitCodedCount int64   // rows with a non-NULL exit_code, denominator for failure rate
	TotalRawBytes  int64
	TotalDistilled int64
	AvgDurationMs  float64
}

// Aggregate summarizes the tool_calls rows matching f.
func (d *DB) Aggregate(f Filter) (Stats, error) {
	where, args := f.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	var s Stats
	var avgScore, avgDuration *float64
	row := d.sql.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			AVG(score),
			COUNT(score),
			COUNT(CASE WHEN exit_code IS NOT NULL AND exit_code != 0 THEN 1 END),
			COUNT(exit_code),
			COALESCE(SUM(raw_bytes), 0),
			COALESCE(SUM(distilled_bytes), 0),
			AVG(duration_ms)
		FROM tool_calls`+where, args...)
	if err := row.Scan(
		&s.Count, &avgScore, &s.ScoredCount, &s.FailureCount, &s.ExitCodedCount,
		&s.TotalRawBytes, &s.TotalDistilled, &avgDuration,
	); err != nil {
		return Stats{}, fmt.Errorf("telemetry: aggregate: %w", err)
	}
	if avgScore != nil {
		s.AvgScore = *avgScore
	}
	if avgDuration != nil {
		s.AvgDurationMs = *avgDuration
	}
	return s, nil
}
