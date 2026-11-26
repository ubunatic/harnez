package telemetry

import "strings"

// ClaudeCallPricing returns the catalog entry whose key is exactly the call's
// model id. Aliases such as "opus" never match, so a newer Opus cannot be
// priced with old rates by accident. A dated stream id (claude-haiku-5-5-20261101)
// falls back to the id without its date suffix. Moved from cmd/harnez for the
// usage report (ticket 1302 M1); the per-call turn cost uses the same rule.
func ClaudeCallPricing(catalog PricingCatalog, model string) (ModelPricing, bool) {
	if model == "" {
		return ModelPricing{}, false
	}
	for _, id := range []string{model, stripDateSuffix(model)} {
		if p, ok := catalog.Lookup(id); ok && p.Model == id && p.Provider == "claude" {
			return p, true
		}
	}
	return ModelPricing{}, false
}

// stripDateSuffix removes a trailing "-YYYYMMDD" from a model id.
func stripDateSuffix(model string) string {
	i := strings.LastIndex(model, "-")
	if i < 0 || len(model)-i-1 != 8 {
		return model
	}
	for _, c := range model[i+1:] {
		if c < '0' || c > '9' {
			return model
		}
	}
	return model[:i]
}
