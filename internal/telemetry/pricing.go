package telemetry

import (
	"encoding/json"
	"os"
)

// RecordedPricingRevision is the checked-in revision of the fallback catalog.
const RecordedPricingRevision = "harnez-2026-09-18"

// LoadPricingCatalog reads an explicit JSON pricing file when configured, or
// uses the recorded catalog. Rates are micro-USD per million tokens.
func LoadPricingCatalog(path string) (PricingCatalog, error) {
	data := []byte(`{"gpt-5":{"model":"gpt-5","revision":"harnez-2026-09-18","rates":{"cached_input_micros_per_million":125000,"uncached_input_micros_per_million":1250000,"output_micros_per_million":10000000,"reasoning_micros_per_million":10000000}}}`)
	if path == "" {
		path = os.Getenv("HARNEZ_PRICING_FILE")
	}
	if path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	var raw map[string]ModelPricing
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return PricingCatalog(raw), nil
}
