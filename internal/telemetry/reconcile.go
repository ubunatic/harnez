package telemetry

import (
	"database/sql"
	"fmt"
	"time"
)

// ReconcileCompactionEconomics consumes the append-only lifecycle evidence for
// one session. It is intentionally idempotent: each pre-compact event gets one
// economics row, while a session without compaction gets one insufficient row.
func (d *DB) ReconcileCompactionEconomics(sessionID, model string, catalog PricingCatalog) error {
	var eventID int64
	var eventModel string
	err := d.sql.QueryRow(`SELECT id, model FROM compaction_events WHERE session_id = ? AND event_type = 'precompact' ORDER BY id DESC LIMIT 1`, sessionID).Scan(&eventID, &eventModel)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("telemetry: find compaction event: %w", err)
		}
		var count int
		if err := d.sql.QueryRow(`SELECT COUNT(*) FROM compaction_economics WHERE session_id = ?`, sessionID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		pricing := catalog.PricingFor(model)
		economics := CalculateCompactionEconomics(model, pricing, EconomicsInput{})
		return d.InsertCompactionEconomics(persistedEconomics(sessionID, nil, pricing, economics))
	}
	var existing int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM compaction_economics WHERE compaction_event_id = ?`, eventID).Scan(&existing); err != nil {
		return err
	}
	if eventModel != "" {
		model = eventModel
	}
	pre, okPre, err := d.latestSnapshot(sessionID, "precompact")
	if err != nil {
		return err
	}
	post, okPost, err := d.latestSnapshot(sessionID, "postcompact")
	if err != nil {
		return err
	}
	base, okBase, err := d.latestSnapshot(sessionID, "sessionend")
	if err != nil {
		return err
	}
	pricing := catalog.PricingFor(model)
	input := EconomicsInput{}
	if !okPre {
		pre, okPre, err = d.latestEventSnapshot(sessionID, "precompact")
		if err != nil {
			return err
		}
	}
	if !okPost {
		post, okPost, err = d.latestEventSnapshot(sessionID, "postcompact")
		if err != nil {
			return err
		}
	}
	if okPre {
		u := UsageFromSnapshot(pre)
		input.Compaction = &u
	}
	if okPost {
		u := UsageFromSnapshot(post)
		input.PostCompaction = &u
	}
	if okBase {
		u := UsageFromSnapshot(base)
		input.NoCompactionBase = &u
	}
	economics := CalculateCompactionEconomics(model, pricing, input)
	if existing > 0 {
		persisted := persistedEconomics(sessionID, &eventID, pricing, economics)
		_, err := d.sql.Exec(`UPDATE compaction_economics SET model = ?, pricing_revision = ?, cached_input_micros_per_million = ?, uncached_input_micros_per_million = ?, output_micros_per_million = ?, reasoning_micros_per_million = ?, status = ?, compaction_cost_micros = ?, post_compaction_cost_micros = ?, baseline_cost_micros = ?, savings_micros = ?, note = ? WHERE compaction_event_id = ?`, persisted.Model, persisted.PricingRevision, persisted.Rates.CachedInputMicrosPerMillion, persisted.Rates.UncachedInputMicrosPerMillion, persisted.Rates.OutputMicrosPerMillion, persisted.Rates.ReasoningMicrosPerMillion, persisted.Status, persisted.CompactionCostMicros, persisted.PostCompactionCostMicros, persisted.BaselineCostMicros, persisted.SavingsMicros, persisted.Note, eventID)
		return err
	}
	return d.InsertCompactionEconomics(persistedEconomics(sessionID, &eventID, pricing, economics))
}

func (d *DB) latestEventSnapshot(sessionID, eventType string) (TokenSnapshot, bool, error) {
	var s TokenSnapshot
	var input, cached, output, reasoning, total sql.NullInt64
	err := d.sql.QueryRow(`SELECT input_tokens, cached_input_tokens, output_tokens, reasoning_tokens, total_tokens FROM compaction_events WHERE session_id = ? AND event_type = ? ORDER BY id DESC LIMIT 1`, sessionID, eventType).Scan(&input, &cached, &output, &reasoning, &total)
	if err == sql.ErrNoRows {
		return s, false, nil
	}
	if err != nil {
		return s, false, err
	}
	if input.Valid {
		s.InputTokens = &input.Int64
	}
	if cached.Valid {
		s.CachedInputTokens = &cached.Int64
	}
	if output.Valid {
		s.OutputTokens = &output.Int64
	}
	if reasoning.Valid {
		s.ReasoningTokens = &reasoning.Int64
	}
	if total.Valid {
		s.TotalTokens = &total.Int64
	}
	return s, true, nil
}

func (d *DB) latestSnapshot(sessionID, source string) (TokenSnapshot, bool, error) {
	var s TokenSnapshot
	var created string
	var input, cached, uncached, output, reasoning, total sql.NullInt64
	err := d.sql.QueryRow(`SELECT created_at, input_tokens, cached_input_tokens, uncached_input_tokens, output_tokens, reasoning_tokens, total_tokens FROM token_snapshots WHERE session_id = ? AND source = ? ORDER BY id DESC LIMIT 1`, sessionID, source).Scan(&created, &input, &cached, &uncached, &output, &reasoning, &total)
	if err != nil {
		if err == sql.ErrNoRows {
			return s, false, nil
		}
		return s, false, err
	}
	if input.Valid {
		s.InputTokens = &input.Int64
	}
	if cached.Valid {
		s.CachedInputTokens = &cached.Int64
	}
	if uncached.Valid {
		s.UncachedInputTokens = &uncached.Int64
	}
	if output.Valid {
		s.OutputTokens = &output.Int64
	}
	if reasoning.Valid {
		s.ReasoningTokens = &reasoning.Int64
	}
	if total.Valid {
		s.TotalTokens = &total.Int64
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return s, true, nil
}

func persistedEconomics(sessionID string, eventID *int64, pricing ModelPricing, economics CompactionEconomics) PersistedEconomics {
	var compaction, post, baseline, savings *int64
	if economics.CompactionCost != nil {
		compaction = &economics.CompactionCost.TotalMicros
	}
	if economics.PostCompactionCost != nil {
		post = &economics.PostCompactionCost.TotalMicros
	}
	if economics.BaselineCost != nil {
		baseline = &economics.BaselineCost.TotalMicros
	}
	savings = economics.SavingsMicros
	return PersistedEconomics{SessionID: sessionID, CompactionEventID: eventID, Model: economics.Model, PricingRevision: economics.PricingRevision, Rates: pricing.Rates, Status: economics.Status, CompactionCostMicros: compaction, PostCompactionCostMicros: post, BaselineCostMicros: baseline, SavingsMicros: savings, Note: economics.Note}
}
