package telemetry

import "testing"

func claudeCatalog(t *testing.T) PricingCatalog {
	t.Helper()
	cat, err := LoadPricingCatalog("")
	if err != nil {
		t.Fatalf("LoadPricingCatalog: %v", err)
	}
	return cat
}

func mustPricing(t *testing.T, cat PricingCatalog, model string) ModelPricing {
	t.Helper()
	p, ok := cat.Lookup(model)
	if !ok {
		t.Fatalf("Lookup(%q) missing", model)
	}
	return p
}

func TestClaudeCatalogAliasesResolve(t *testing.T) {
	cat := claudeCatalog(t)
	for _, model := range []string{"haiku", "claude-haiku-5-5", "sonnet", "claude-sonnet-5-5", "opus", "claude-opus-5-5"} {
		p, ok := cat.Lookup(model)
		if !ok || p.Rates.UncachedInputMicrosPerMillion == nil {
			t.Fatalf("Lookup(%q) = %+v, ok=%v; want a priced Claude entry", model, p, ok)
		}
		if p.Provider != "claude" {
			t.Fatalf("Lookup(%q).Provider = %q; want claude", model, p.Provider)
		}
		if p.Source == "" || p.Fetched == "" {
			t.Fatalf("Lookup(%q) lacks source or fetched date: %+v", model, p)
		}
	}
	if _, ok := cat.Lookup("claude-fable-5-2"); ok {
		t.Fatalf("fable has no sourced rates; Lookup must miss, not guess")
	}
	if _, ok := cat.Lookup("no-such-model"); ok {
		t.Fatalf("unknown model must miss")
	}
}

func TestClaudeRequestCostUsesCacheWriteRatesPerTTL(t *testing.T) {
	cat := claudeCatalog(t)
	haiku := mustPricing(t, cat, "haiku")
	// 50,000 prompt tokens stay in the base tier, so only the write rate differs.
	got1h, ok := RequestCostMicros(haiku, RequestTokens{CacheWrite1h: 50_000})
	if !ok || got1h == nil || *got1h != 10_000 {
		t.Fatalf("haiku 1h write of 50k = %v, ok=%v; want 10000 micros (0.20 USD/MTok)", got1h, ok)
	}
	got5m, ok := RequestCostMicros(haiku, RequestTokens{CacheWrite5m: 50_000})
	if !ok || got5m == nil || *got5m != 6_250 {
		t.Fatalf("haiku 5m write of 50k = %v, ok=%v; want 6250 micros (0.125 USD/MTok)", got5m, ok)
	}
}

func TestClaudeRequestTierBoundaryOnTotalPrompt(t *testing.T) {
	cat := claudeCatalog(t)
	haiku := mustPricing(t, cat, "haiku")
	tests := []struct {
		name string
		req  RequestTokens
		want int64
	}{
		// Base rate 0.10 USD/MTok = 100,000 micros per MTok: 99,999 tokens are 9,999.9 micros.
		{"99999 total is base tier", RequestTokens{Uncached: 99_999}, 10_000},
		// "over 100,000" is strict, so exactly 100,000 stays in the base tier.
		{"100000 total is base tier", RequestTokens{Uncached: 100_000}, 10_000},
		// Tier rate 0.50 USD/MTok: 100,001 tokens are 50,000.5 micros, rounded half up.
		{"100001 total is tier", RequestTokens{Uncached: 100_001}, 50_001},
		// The tier is decided on uncached + cache read + cache write, not uncached alone.
		{"cache read counts toward prompt size", RequestTokens{Uncached: 1, CacheRead: 100_000}, 5_001},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RequestCostMicros(haiku, tt.req)
			if !ok || got == nil || *got != tt.want {
				t.Fatalf("RequestCostMicros = %v, ok=%v; want %d", got, ok, tt.want)
			}
		})
	}
}

func TestClaudeRequestCostIsNilWhenAPriceIsMissing(t *testing.T) {
	missing := ModelPricing{Model: "x", Revision: "r", Rates: PricingRates{}}
	if got, ok := RequestCostMicros(missing, RequestTokens{Uncached: 10}); ok || got != nil {
		t.Fatalf("missing input rate = %v, ok=%v; want nil, false (unknown is never 0)", got, ok)
	}
	cat := claudeCatalog(t)
	opus := mustPricing(t, cat, "opus")
	if got, ok := RequestCostMicros(opus, RequestTokens{}); !ok || got == nil || *got != 0 {
		t.Fatalf("zero tokens with rates = %v, ok=%v; want 0, true", got, ok)
	}
}

func TestCalculateTurnCostIsNilWhenAnyTokenHasNoRate(t *testing.T) {
	cached := int64(1_000_000)
	uncached, cachedTokens := int64(10), int64(10)
	rates := PricingRates{CachedInputMicrosPerMillion: &cached}
	if got, ok := CalculateTurnCost(rates, TokenUsage{UncachedInputTokens: &uncached, CachedInputTokens: &cachedTokens}); ok || got != nil {
		t.Fatalf("partial rates = %v, ok=%v; want nil, false", got, ok)
	}
}
