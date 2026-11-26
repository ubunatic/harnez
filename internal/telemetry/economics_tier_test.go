package telemetry

import (
	"errors"
	"testing"
)

// tieredPricing is fullPricing with one input tier above 100000 prompt tokens at
// five times the base uncached rate (issue 1303 M2).
func tieredPricing() ModelPricing {
	pricing := fullPricing()
	pricing.Tiers = []PricingTier{{OverInputTokens: 100000, Rates: PricingRates{
		CachedInputMicrosPerMillion:   economicsPtr(5_000_000),
		UncachedInputMicrosPerMillion: economicsPtr(10_000_000),
		OutputMicrosPerMillion:        economicsPtr(15_000_000),
		ReasoningMicrosPerMillion:     economicsPtr(20_000_000),
	}}}
	return pricing
}

// TestCompactionEconomicsAppliesInputTier prices a compaction snapshot the way
// the #1302 report prices an API call: the tier whose threshold the prompt
// exceeds. The near-miss pair 100000 and 100001 is the boundary check.
func TestCompactionEconomicsAppliesInputTier(t *testing.T) {
	tests := []struct {
		name   string
		prompt int64
		want   int64
	}{
		{name: "at threshold keeps base rate", prompt: 100000, want: 200000},
		{name: "one over threshold uses tier rate", prompt: 100001, want: 1000010},
		{name: "large precompact uses tier rate", prompt: 150777, want: 1507770},
		{name: "small compaction keeps base rate", prompt: 90000, want: 180000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCompactionEconomics("gpt-test", tieredPricing(), EconomicsInput{
				Compaction:       fullUsage(0, tt.prompt, 0, 0).ptr(),
				PostCompaction:   fullUsage(0, 1, 0, 0).ptr(),
				NoCompactionBase: fullUsage(0, 1, 0, 0).ptr(),
			})
			if got.CompactionCost == nil || got.CompactionCost.TotalMicros != tt.want {
				t.Fatalf("compaction cost = %+v, want %d micros", got.CompactionCost, tt.want)
			}
		})
	}
}

// TestRequestAndCompactionShareTierRule proves the compaction economics and the
// per-call request cost choose the same tier for the same prompt.
func TestRequestAndCompactionShareTierRule(t *testing.T) {
	pricing := tieredPricing()
	for _, prompt := range []int64{90000, 100000, 100001, 150777} {
		req, ok := RequestCostMicros(pricing, RequestTokens{Uncached: prompt})
		if !ok || req == nil {
			t.Fatalf("RequestCostMicros(%d) unpriced", prompt)
		}
		got := CalculateCompactionEconomics("gpt-test", pricing, EconomicsInput{
			Compaction:       fullUsage(0, prompt, 0, 0).ptr(),
			PostCompaction:   fullUsage(0, 1, 0, 0).ptr(),
			NoCompactionBase: fullUsage(0, 1, 0, 0).ptr(),
		})
		if got.CompactionCost == nil || got.CompactionCost.TotalMicros != *req {
			t.Fatalf("prompt %d: compaction %+v, request %d", prompt, got.CompactionCost, *req)
		}
	}
}

// TestCompactionEconomicsSpecHaikuTier loads the real spec: a 150777-token Haiku
// precompact is priced at the tier, and the persisted rates are the tier's.
func TestCompactionEconomicsSpecHaikuTier(t *testing.T) {
	catalog, err := LoadPricingCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	pricing := catalog.PricingFor("claude-haiku-5-5")
	got := CalculateCompactionEconomics("claude-haiku-5-5", pricing, EconomicsInput{
		Compaction:       fullUsage(0, 150777, 0, 0).ptr(),
		PostCompaction:   fullUsage(0, 12495, 0, 0).ptr(),
		NoCompactionBase: fullUsage(0, 1000, 0, 0).ptr(),
	})
	// 150777 tokens at the tier's 500000 micros per million (see spec/pricing.yaml).
	if got.CompactionCost == nil || got.CompactionCost.TotalMicros != 75389 {
		t.Fatalf("compaction cost = %+v, want 75389 micros at the tier rate", got.CompactionCost)
	}
	if got.PostCompactionCost == nil || got.PostCompactionCost.TotalMicros != 1250 {
		t.Fatalf("post cost = %+v, want 1250 micros at the base rate", got.PostCompactionCost)
	}
}

// TestInTransactionRollsBackFailedFill checks the M1 follow-up: a failure after
// one write inside the fill leaves no partial state.
func TestInTransactionRollsBackFailedFill(t *testing.T) {
	db := openTestDB(t)
	boom := errors.New("injected failure after row update")
	err := db.InTransaction(func(tx *DB) error {
		if _, err := tx.executor().Exec("INSERT INTO telemetry_receipts(record_id) VALUES ('fill-1')"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTransaction error = %v, want the injected failure", err)
	}
	var n int
	if err := db.executor().QueryRow("SELECT COUNT(*) FROM telemetry_receipts WHERE record_id='fill-1'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("failed transaction left %d receipt rows, want 0", n)
	}
}

// TestInTransactionSavepointInsideRecordKeepsRecordWrites checks the nested case
// the collector hits: a fill inside a record's transaction. Its failure undoes
// only its own writes; the record's other writes still commit.
func TestInTransactionSavepointInsideRecordKeepsRecordWrites(t *testing.T) {
	db := openTestDB(t)
	err := db.WithReceipt("record-1", func(tx *DB) error {
		if err := tx.InTransaction(func(inner *DB) error {
			if _, err := inner.executor().Exec("INSERT INTO telemetry_receipts(record_id) VALUES ('fill-nested')"); err != nil {
				return err
			}
			return errors.New("fill failed")
		}); err == nil {
			t.Fatal("nested failure was not returned")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var fill, record int
	if err := db.executor().QueryRow("SELECT COUNT(*) FROM telemetry_receipts WHERE record_id='fill-nested'").Scan(&fill); err != nil {
		t.Fatal(err)
	}
	if err := db.executor().QueryRow("SELECT COUNT(*) FROM telemetry_receipts WHERE record_id='record-1'").Scan(&record); err != nil {
		t.Fatal(err)
	}
	if fill != 0 || record != 1 {
		t.Fatalf("nested fill rows = %d, record rows = %d; want 0 and 1", fill, record)
	}
}
