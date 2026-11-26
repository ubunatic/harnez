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

// Lookup returns the entry whose key or alias equals model. An exact key wins
// over an alias, so a bare alias such as "opus" maps to its sourced entry.
func (c PricingCatalog) Lookup(model string) (ModelPricing, bool) {
	if pricing, ok := c[model]; ok {
		return pricing, true
	}
	for _, pricing := range c {
		for _, alias := range pricing.Aliases {
			if alias == model {
				return pricing, true
			}
		}
	}
	return ModelPricing{}, false
}

// RequestTokens is one provider API call's token counts. Cache writes are
// split by TTL because the two TTLs have different rates.
type RequestTokens struct {
	Uncached     int64
	CacheRead    int64
	CacheWrite5m int64
	CacheWrite1h int64
	Output       int64
}

// PromptTokens is the prompt size that selects a pricing tier.
func (r RequestTokens) PromptTokens() int64 {
	return r.Uncached + r.CacheRead + r.CacheWrite5m + r.CacheWrite1h
}

// RequestCostMicros prices one API call in micro-USD. The tier is chosen per
// call from its total prompt, the highest tier whose threshold is exceeded.
// It returns (nil, false) when any non-zero count has no rate: unknown is
// never reported as 0.
func RequestCostMicros(pricing ModelPricing, req RequestTokens) (*int64, bool) {
	rates := tierRates(pricing, req.PromptTokens())
	parts := []struct {
		tokens int64
		rate   *int64
	}{
		{req.Uncached, rates.UncachedInputMicrosPerMillion},
		{req.CacheRead, rates.CachedInputMicrosPerMillion},
		{req.CacheWrite5m, rates.CacheWrite5mMicrosPerMillion},
		{req.CacheWrite1h, rates.CacheWrite1hMicrosPerMillion},
		{req.Output, rates.OutputMicrosPerMillion},
	}
	var total int64
	for _, part := range parts {
		if part.tokens == 0 {
			continue
		}
		if part.rate == nil {
			return nil, false
		}
		total += roundedMicros(part.tokens, *part.rate)
	}
	return &total, true
}

// tierRates returns the rates for a prompt of promptTokens: the highest tier whose
// over_input_tokens the prompt exceeds, else the base rates. Compaction economics
// and per-call request cost both use this rule (issue 1303 M2).
func tierRates(pricing ModelPricing, promptTokens int64) PricingRates {
	rates := pricing.Rates
	var threshold int64 = -1
	for _, tier := range pricing.Tiers {
		if promptTokens > tier.OverInputTokens && tier.OverInputTokens > threshold {
			rates = tier.Rates
			threshold = tier.OverInputTokens
		}
	}
	return rates
}

// promptTokens is the prompt size of a snapshot: its total input when reported,
// else uncached plus cached input.
func (u TokenUsage) promptTokens() int64 {
	if u.InputTokens != nil {
		return *u.InputTokens
	}
	var total int64
	if u.UncachedInputTokens != nil {
		total += *u.UncachedInputTokens
	}
	if u.CachedInputTokens != nil {
		total += *u.CachedInputTokens
	}
	return total
}

// CalculateTurnCost estimates one turn's API cost in micro-USD using pricing rates.
// Returns (nil, false) unless every reported component has a rate.
func CalculateTurnCost(rates PricingRates, usage TokenUsage) (*int64, bool) {
	for _, part := range []struct {
		tokens, rate *int64
	}{
		{usage.CachedInputTokens, rates.CachedInputMicrosPerMillion},
		{usage.UncachedInputTokens, rates.UncachedInputMicrosPerMillion},
		{usage.OutputTokens, rates.OutputMicrosPerMillion},
		{usage.ReasoningTokens, rates.ReasoningMicrosPerMillion},
	} {
		if part.tokens != nil && *part.tokens > 0 && part.rate == nil {
			return nil, false
		}
	}
	cost, _ := estimateCost(usage, rates)
	if cost == nil {
		return nil, false
	}
	total := cost.TotalMicros
	return &total, true
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
	result.CompactionCost, partial = estimateCost(*input.Compaction, tierRates(pricing, input.Compaction.promptTokens()))
	if result.CompactionCost == nil {
		result.Status = EconomicsInsufficient
		result.Note = "no priced compaction components"
		return result
	}
	if partial {
		result.Status = EconomicsPartial
	}
	postPartial := false
	result.PostCompactionCost, postPartial = estimateCost(*input.PostCompaction, tierRates(pricing, input.PostCompaction.promptTokens()))
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
	result.BaselineCost, baselinePartial = estimateCost(*input.NoCompactionBase, tierRates(pricing, input.NoCompactionBase.promptTokens()))
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
