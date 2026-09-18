package telemetry

import "testing"

func economicsPtr(value int64) *int64 { return &value }

func fullPricing() ModelPricing {
	return ModelPricing{
		Model: "gpt-test", Revision: "2026-09-18",
		Rates: PricingRates{
			CachedInputMicrosPerMillion:   economicsPtr(1_000_000),
			UncachedInputMicrosPerMillion: economicsPtr(2_000_000),
			OutputMicrosPerMillion:        economicsPtr(3_000_000),
			ReasoningMicrosPerMillion:     economicsPtr(4_000_000),
		},
	}
}

func fullUsage(cached, uncached, output, reasoning int64) TokenUsage {
	input := cached + uncached
	return TokenUsage{InputTokens: economicsPtr(input), CachedInputTokens: economicsPtr(cached), UncachedInputTokens: economicsPtr(uncached), OutputTokens: economicsPtr(output), ReasoningTokens: economicsPtr(reasoning)}
}

func TestCalculateCompactionEconomicsTable(t *testing.T) {
	tests := []struct {
		name                                string
		pricing                             ModelPricing
		input                               EconomicsInput
		status                              EconomicsStatus
		compaction, post, baseline, savings *int64
	}{
		{
			name:    "all components and baseline",
			pricing: fullPricing(),
			input: EconomicsInput{
				Compaction:       fullUsage(1_000_000, 2_000_000, 3_000_000, 4_000_000).ptr(),
				PostCompaction:   fullUsage(100_000, 200_000, 300_000, 400_000).ptr(),
				NoCompactionBase: fullUsage(200_000, 400_000, 600_000, 800_000).ptr(),
			},
			status: EconomicsComplete, compaction: economicsPtr(30_000_000), post: economicsPtr(3_000_000), baseline: economicsPtr(6_000_000), savings: economicsPtr(3_000_000),
		},
		{
			name:    "missing rate is partial and preserves known cost",
			pricing: ModelPricing{Revision: "r1", Rates: PricingRates{CachedInputMicrosPerMillion: economicsPtr(1_000_000), UncachedInputMicrosPerMillion: economicsPtr(2_000_000)}},
			input:   EconomicsInput{Compaction: fullUsage(1_000_000, 1_000_000, 0, 0).ptr(), PostCompaction: fullUsage(1, 1, 1, 1).ptr(), NoCompactionBase: fullUsage(1, 1, 1, 1).ptr()},
			status:  EconomicsPartial, compaction: economicsPtr(3_000_000), post: economicsPtr(3), baseline: economicsPtr(3),
		},
		{
			name:    "missing baseline is insufficient",
			pricing: fullPricing(),
			input:   EconomicsInput{Compaction: fullUsage(1, 0, 0, 0).ptr(), PostCompaction: fullUsage(1, 0, 0, 0).ptr()},
			status:  EconomicsInsufficient, compaction: economicsPtr(1), post: economicsPtr(1),
		},
		{
			name:    "counter reset is insufficient",
			pricing: fullPricing(),
			input:   EconomicsInput{CounterReset: true, Compaction: fullUsage(1, 1, 1, 1).ptr(), PostCompaction: fullUsage(1, 1, 1, 1).ptr(), NoCompactionBase: fullUsage(1, 1, 1, 1).ptr()},
			status:  EconomicsInsufficient,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CalculateCompactionEconomics(test.pricing.Model, test.pricing, test.input)
			if got.Status != test.status {
				t.Fatalf("status = %q, want %q", got.Status, test.status)
			}
			assertCost := func(name string, got *CostEstimate, want *int64) {
				if want == nil {
					if got != nil {
						t.Fatalf("%s = %+v, want nil", name, got)
					}
					return
				}
				if got == nil || got.TotalMicros != *want {
					t.Fatalf("%s = %+v, want total %d", name, got, *want)
				}
			}
			assertCost("compaction", got.CompactionCost, test.compaction)
			assertCost("post", got.PostCompactionCost, test.post)
			assertCost("baseline", got.BaselineCost, test.baseline)
			if test.savings == nil {
				if got.SavingsMicros != nil {
					t.Fatalf("savings = %d, want nil", *got.SavingsMicros)
				}
			} else if got.SavingsMicros == nil || *got.SavingsMicros != *test.savings {
				t.Fatalf("savings = %v, want %d", got.SavingsMicros, *test.savings)
			}
		})
	}
}

func (u TokenUsage) ptr() *TokenUsage { return &u }

func TestRoundedMicrosAndSnapshotDerivation(t *testing.T) {
	if got := roundedMicros(1, 500_000); got != 1 {
		t.Fatalf("roundedMicros = %d, want 1", got)
	}
	if got := roundedMicros(1, 499_999); got != 0 {
		t.Fatalf("roundedMicros = %d, want 0", got)
	}
	input, cached := int64(10), int64(4)
	usage := UsageFromSnapshot(TokenSnapshot{InputTokens: &input, CachedInputTokens: &cached})
	if usage.UncachedInputTokens == nil || *usage.UncachedInputTokens != 6 {
		t.Fatalf("uncached = %v, want 6", usage.UncachedInputTokens)
	}
}

func TestDeltaSnapshotsRejectsCounterReset(t *testing.T) {
	before := int64(100)
	after := int64(20)
	_, reset := DeltaSnapshots(TokenSnapshot{InputTokens: &before}, TokenSnapshot{InputTokens: &after})
	if !reset {
		t.Fatal("counter reset was not detected")
	}
}

func TestUnknownPricingIsInsufficient(t *testing.T) {
	pricing := (PricingCatalog{}).PricingFor("missing-model")
	got := CalculateCompactionEconomics("missing-model", pricing, EconomicsInput{Compaction: fullUsage(1, 1, 1, 1).ptr(), PostCompaction: fullUsage(1, 1, 1, 1).ptr(), NoCompactionBase: fullUsage(1, 1, 1, 1).ptr()})
	if got.Status != EconomicsInsufficient {
		t.Fatalf("status = %q, want %q", got.Status, EconomicsInsufficient)
	}
}

func TestInsertCompactionEconomicsPersistsPricingRevisionAndRates(t *testing.T) {
	db := openTestDB(t)
	rate := int64(123456)
	eventID := int64(7)
	if err := db.InsertCompactionEconomics(PersistedEconomics{SessionID: "pricing-session", CompactionEventID: &eventID, Model: "gpt-test", PricingRevision: "2026-09-18", Rates: PricingRates{CachedInputMicrosPerMillion: &rate}, Status: EconomicsPartial, Note: "missing output rate"}); err != nil {
		t.Fatal(err)
	}
	var revision, status, note string
	var storedRate int64
	if err := db.sql.QueryRow(`SELECT pricing_revision, cached_input_micros_per_million, status, note FROM compaction_economics WHERE session_id = ?`, "pricing-session").Scan(&revision, &storedRate, &status, &note); err != nil {
		t.Fatal(err)
	}
	if revision != "2026-09-18" || storedRate != rate || status != string(EconomicsPartial) || note != "missing output rate" {
		t.Fatalf("persisted pricing = %q %d %q %q", revision, storedRate, status, note)
	}
}
