package telemetry

import (
	"fmt"
	"strings"
)

func (d *DB) CompactionEconomicsStats(f Filter) (CompactionEconomicsStats, error) {
	var clauses []string
	var args []any
	if f.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, f.SessionID)
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.Since.Format("2006-01-02T15:04:05.999999999Z07:00"))
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "created_at < ?")
		args = append(args, f.Until.Format("2006-01-02T15:04:05.999999999Z07:00"))
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	query := `SELECT COUNT(*), COALESCE(MAX(model),''), COALESCE(MAX(pricing_revision),''), MAX(status), MAX(compaction_cost_micros), MAX(post_compaction_cost_micros), MAX(baseline_cost_micros), MAX(savings_micros), COALESCE(MAX(note),'') FROM compaction_economics` + where
	var s CompactionEconomicsStats
	var status *string
	if err := d.sql.QueryRow(query, args...).Scan(&s.CompactionCount, &s.Model, &s.PricingRevision, &status, &s.CompactionCostMicros, &s.PostCompactionCostMicros, &s.BaselineCostMicros, &s.SavingsMicros, &s.Note); err != nil {
		return s, fmt.Errorf("telemetry: compaction economics stats: %w", err)
	}
	if status != nil {
		s.Status = *status
	} else {
		s.Status = string(EconomicsInsufficient)
		s.Note = "no compaction economics recorded"
	}
	return s, nil
}
