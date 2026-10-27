package telemetry

import (
	"fmt"
)

// CompactionPair describes the most recent observed compaction attempt.
type CompactionPair struct {
	SessionID   string `json:"session_id"`
	TurnID      string `json:"turn_id,omitempty"`
	Trigger     string `json:"trigger,omitempty"`
	Source      string `json:"source"`
	PreAt       string `json:"pre_at"`
	PostAt      string `json:"post_at,omitempty"`
	Interrupted bool   `json:"interrupted"`
}

// LatestCompactionPair matches events within a provider session. Repeated hook
// deliveries for a turn count as one attempt. Events without turn IDs are paired
// in arrival order; a subsequent pre after a post starts a new attempt.
func (d *DB) LatestCompactionPair(sessionID string) (*CompactionPair, error) {
	var hasSource int
	if err := d.sql.QueryRow(mustTelemetrySQL().Statements["compaction_events_has_source"]).Scan(&hasSource); err != nil {
		return nil, err
	}
	query := mustTelemetrySQL().Statements["compaction_pair_events_legacy"]
	if hasSource != 0 {
		query = mustTelemetrySQL().Statements["compaction_pair_events"]
	}
	rows, err := d.sql.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query compaction pair: %w", err)
	}
	defer rows.Close()
	var latest *CompactionPair
	pairs := map[string]*CompactionPair{}
	for rows.Next() {
		var kind, turn, trigger, src, at string
		if err := rows.Scan(&kind, &turn, &trigger, &src, &at); err != nil {
			return nil, err
		}
		if kind == "precompact" {
			if turn != "" {
				if prior := pairs[turn]; prior != nil {
					continue
				}
			} else if latest != nil && latest.TurnID == "" && latest.Interrupted {
				continue
			}
			latest = &CompactionPair{SessionID: sessionID, TurnID: turn, Trigger: trigger, Source: src, PreAt: at, Interrupted: true}
			if turn != "" {
				pairs[turn] = latest
			}
		} else {
			pair := latest
			if turn != "" {
				pair = pairs[turn]
			}
			if pair != nil && pair.TurnID == turn && pair.Interrupted {
				pair.PostAt = at
				pair.Interrupted = false
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return latest, nil
}
