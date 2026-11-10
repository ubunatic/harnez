package telemetry

import (
	"encoding/json"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// RecordedPricingRevision is the checked-in revision of the fallback catalog.
const RecordedPricingRevision = "harnez-2026-09-18"

// LoadPricingCatalog reads an explicit JSON pricing file when configured, or
// uses the recorded catalog. Rates are micro-USD per million tokens.
func LoadPricingCatalog(path string) (PricingCatalog, error) {
	var raw map[string]ModelPricing

	if path == "" {
		path = os.Getenv("HARNEZ_PRICING_FILE")
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		return PricingCatalog(raw), nil
	}

	data, err := fs.ReadFile(harnez.DefaultFS, "spec/pricing.yaml")
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return PricingCatalog(raw), nil
}
