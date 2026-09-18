package telemetry

import (
	"math/big"
)

const microsPerMillion int64 = 1_000_000

// PricingFor returns an exact catalog entry. Unknown models intentionally
// return an empty pricing record; callers must report insufficient data.
func (c PricingCatalog) PricingFor(model string) ModelPricing {
	if pricing, ok := c[model]; ok {
		return pricing
	}
	return ModelPricing{Model: model, Revision: "unknown"}
}

// UsageFromSnapshot extracts cumulative provider counters without modifying
// the append-only evidence row.
func UsageFromSnapshot(snapshot TokenSnapshot) TokenUsage {
	uncached := snapshot.UncachedInputTokens
	if uncached == nil && snapshot.InputTokens != nil && snapshot.CachedInputTokens != nil {
		value := *snapshot.InputTokens - *snapshot.CachedInputTokens
		if value >= 0 {
			uncached = &value
		}
	}
	return TokenUsage{InputTokens: snapshot.InputTokens, CachedInputTokens: snapshot.CachedInputTokens, UncachedInputTokens: uncached, OutputTokens: snapshot.OutputTokens, ReasoningTokens: snapshot.ReasoningTokens}
}

// DeltaSnapshots computes non-negative counter deltas. A reset is never
// treated as zero usage because that would fabricate savings evidence.
func DeltaSnapshots(before, after TokenSnapshot) (TokenUsage, bool) {
	return deltaUsage(UsageFromSnapshot(before), UsageFromSnapshot(after))
}

func deltaUsage(before, after TokenUsage) (TokenUsage, bool) {
	result := TokenUsage{}
	reset := false
	result.InputTokens, reset = delta(before.InputTokens, after.InputTokens)
	if reset {
		return TokenUsage{}, true
	}
	result.CachedInputTokens, reset = delta(before.CachedInputTokens, after.CachedInputTokens)
	if reset {
		return TokenUsage{}, true
	}
	result.UncachedInputTokens, reset = delta(before.UncachedInputTokens, after.UncachedInputTokens)
	if reset {
		return TokenUsage{}, true
	}
	result.OutputTokens, reset = delta(before.OutputTokens, after.OutputTokens)
	if reset {
		return TokenUsage{}, true
	}
	result.ReasoningTokens, reset = delta(before.ReasoningTokens, after.ReasoningTokens)
	return result, reset
}

func delta(before, after *int64) (*int64, bool) {
	if before == nil || after == nil {
		return nil, false
	}
	if *after < *before {
		return nil, true
	}
	value := *after - *before
	return &value, false
}

// CalculateCompactionEconomics calculates costs from already-normalized
// immutable snapshot deltas. Savings are emitted only when a no-compaction
// baseline exists and all its component counters are usable.
func CalculateCompactionEconomics(model string, pricing ModelPricing, input EconomicsInput) CompactionEconomics {
	result := CompactionEconomics{Model: model, PricingRevision: pricing.Revision, Status: EconomicsComplete}
	if input.CounterReset {
		result.Status = EconomicsInsufficient
		result.CounterReset = true
		result.Note = "provider counter reset; savings baseline is not comparable"
		return result
	}
	if input.Compaction == nil || input.PostCompaction == nil {
		result.Status = EconomicsInsufficient
		result.Note = "missing compaction or post-compaction snapshot"
		return result
	}
	var partial bool
	result.CompactionCost, partial = estimateCost(*input.Compaction, pricing.Rates)
	if result.CompactionCost == nil {
		result.Status = EconomicsInsufficient
		result.Note = "no priced compaction components"
		return result
	}
	if partial {
		result.Status = EconomicsPartial
	}
	postPartial := false
	result.PostCompactionCost, postPartial = estimateCost(*input.PostCompaction, pricing.Rates)
	if result.PostCompactionCost == nil {
		result.Status = EconomicsInsufficient
		result.Note = "no priced post-compaction components"
		return result
	}
	if postPartial {
		result.Status = EconomicsPartial
	}
	if input.NoCompactionBase == nil {
		result.Status = EconomicsInsufficient
		result.Note = "missing no-compaction baseline"
		return result
	}
	baselinePartial := false
	result.BaselineCost, baselinePartial = estimateCost(*input.NoCompactionBase, pricing.Rates)
	if result.BaselineCost == nil {
		result.Status = EconomicsInsufficient
		result.Note = "no priced baseline components"
		return result
	}
	if baselinePartial {
		result.Status = EconomicsPartial
	}
	if postPartial || baselinePartial {
		result.Note = "partial post-compaction or baseline pricing; savings withheld"
		return result
	}
	savings := result.BaselineCost.TotalMicros - result.PostCompactionCost.TotalMicros
	result.SavingsMicros = &savings
	return result
}

func estimateCost(usage TokenUsage, rates PricingRates) (*CostEstimate, bool) {
	components := []struct {
		tokens, rate *int64
	}{
		{usage.CachedInputTokens, rates.CachedInputMicrosPerMillion},
		{usage.UncachedInputTokens, rates.UncachedInputMicrosPerMillion},
		{usage.OutputTokens, rates.OutputMicrosPerMillion},
		{usage.ReasoningTokens, rates.ReasoningMicrosPerMillion},
	}
	result := CostEstimate{}
	known := 0
	partial := false
	for i, component := range components {
		if component.tokens == nil {
			partial = true
			continue
		}
		if component.rate == nil {
			partial = true
			continue
		}
		value := roundedMicros(*component.tokens, *component.rate)
		switch i {
		case 0:
			result.CachedInputMicros = value
		case 1:
			result.UncachedInputMicros = value
		case 2:
			result.OutputMicros = value
		case 3:
			result.ReasoningMicros = value
		}
		result.TotalMicros += value
		known++
	}
	if known == 0 {
		return nil, partial
	}
	return &result, partial
}

func roundedMicros(tokens, rate int64) int64 {
	if tokens <= 0 || rate <= 0 {
		return 0
	}
	// Use big.Int so malformed or future large provider counters cannot wrap.
	numerator := new(big.Int).Mul(big.NewInt(tokens), big.NewInt(rate))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, big.NewInt(microsPerMillion), remainder)
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(big.NewInt(microsPerMillion)) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient.Int64()
}
