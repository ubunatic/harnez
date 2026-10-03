package telemetry

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LatestTokenSnapshots returns the newest recorded token snapshot for each
// requested provider session ID. Snapshot rows are ordered newest first, so
// sessions without captured usage are omitted from the result.
func (d *DB) LatestTokenSnapshots(sessionIDs []string) (map[string]TokenSnapshot, error) {
	ids := make([]string, 0, len(sessionIDs))
	seen := make(map[string]struct{}, len(sessionIDs))
	for _, id := range sessionIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[string]TokenSnapshot{}, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	query := `SELECT session_id, created_at, source, input_tokens, cached_input_tokens,
		uncached_input_tokens, output_tokens, reasoning_tokens, total_tokens,
		last_input_tokens, last_cached_input_tokens, last_output_tokens,
		last_reasoning_tokens, last_total_tokens
		FROM token_snapshots WHERE session_id IN (` + placeholders + `) ORDER BY id DESC`
	ctx, cancel := defaultContext()
	defer cancel()
	rows, err := d.sql.QueryContext(ctx, query, stringArgs(ids)...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query latest token snapshots: %w", err)
	}
	defer rows.Close()

	out := make(map[string]TokenSnapshot, len(ids))
	for rows.Next() {
		var sessionID, createdAt, source string
		var input, cached, uncached, output, reasoning, total sql.NullInt64
		var lastInput, lastCached, lastOutput, lastReasoning, lastTotal sql.NullInt64
		if err := rows.Scan(&sessionID, &createdAt, &source, &input, &cached, &uncached, &output, &reasoning, &total, &lastInput, &lastCached, &lastOutput, &lastReasoning, &lastTotal); err != nil {
			return nil, fmt.Errorf("telemetry: scan latest token snapshot: %w", err)
		}
		if _, ok := out[sessionID]; ok {
			continue
		}
		snapshot := TokenSnapshot{SessionID: sessionID, Source: source}
		snapshot.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		snapshot.InputTokens = nullableInt64(input)
		snapshot.CachedInputTokens = nullableInt64(cached)
		snapshot.UncachedInputTokens = nullableInt64(uncached)
		snapshot.OutputTokens = nullableInt64(output)
		snapshot.ReasoningTokens = nullableInt64(reasoning)
		snapshot.TotalTokens = nullableInt64(total)
		snapshot.LastInputTokens = nullableInt64(lastInput)
		snapshot.LastCachedInputTokens = nullableInt64(lastCached)
		snapshot.LastOutputTokens = nullableInt64(lastOutput)
		snapshot.LastReasoningTokens = nullableInt64(lastReasoning)
		snapshot.LastTotalTokens = nullableInt64(lastTotal)
		out[sessionID] = snapshot
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: iterate latest token snapshots: %w", err)
	}
	return out, nil
}

func nullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func stringArgs(values []string) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value
	}
	return args
}
