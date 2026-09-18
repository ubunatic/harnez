package telemetry

import "fmt"

// UpdateLatestToolCallOutput attaches post-tool output metrics to the newest
// tool call for sessionID. It returns an error when no matching row exists.
func (d *DB) UpdateLatestToolCallOutput(sessionID string, outputBytes, durationMs int64) error {
	return d.UpdateLatestToolCallMetrics(sessionID, outputBytes, durationMs, nil, nil)
}

// UpdateLatestToolCallMetrics attaches output and optional opportunity-savings
// metrics to the newest tool call for sessionID.
func (d *DB) UpdateLatestToolCallMetrics(sessionID string, outputBytes, durationMs int64, savingsTokens, savingsBytes *int64) error {
	result, err := d.sql.Exec(`
		UPDATE tool_calls
		SET output_bytes = ?, duration_ms = ?, potential_savings_tokens = ?, potential_savings_bytes = ?
		WHERE id = (
			SELECT id FROM tool_calls
			WHERE session_id = ?
			ORDER BY id DESC LIMIT 1
		)`, outputBytes, durationMs, savingsTokens, savingsBytes, sessionID)
	if err != nil {
		return fmt.Errorf("telemetry: update latest tool call output: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("telemetry: check updated tool call output: %w", err)
	} else if n == 0 {
		return fmt.Errorf("telemetry: no tool call found for session %q", sessionID)
	}
	return nil
}
