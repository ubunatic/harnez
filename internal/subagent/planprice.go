package subagent

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PlanPriceSpec defines a monthly subscription price for one provider (ticket 1215).
// The price is given in USD (monthly_usd_micros) or in EUR (monthly_eur_micros,
// converted once at load with plan_fx, ticket 1301). USD wins when both are set.
// A nil MonthlyUSDMicros after resolution marks the price as unknown.
type PlanPriceSpec struct {
	Plan             string `yaml:"plan"`
	Currency         string `yaml:"currency"`
	MonthlyUSDMicros *int64 `yaml:"monthly_usd_micros"`
	MonthlyEURMicros *int64 `yaml:"monthly_eur_micros"`
}

// PlanFXSpec is the spec's USD-per-EUR rate with its source and date.
type PlanFXSpec struct {
	UsdPerEUR float64 `yaml:"usd_per_eur"`
	Source    string  `yaml:"source"`
	Date      string  `yaml:"date"`
}

type planPricesConfig struct {
	Agent struct {
		PlanPrices map[string]*PlanPriceSpec `yaml:"plan_prices"`
	} `yaml:"agent"`
}

// usdMicrosFromEUR converts a EUR plan price to micro-USD. A missing amount or a
// missing or zero FX rate gives nil: the price is unknown, never 0.
func usdMicrosFromEUR(eur *int64, usdPerEUR float64) *int64 {
	if eur == nil || *eur <= 0 || usdPerEUR <= 0 {
		return nil
	}
	v := int64(math.Round(float64(*eur) * usdPerEUR))
	return &v
}

// resolvePlanUSD returns the entry's price in micro-USD, or nil when unknown.
func resolvePlanUSD(entry *PlanPriceSpec, fx *PlanFXSpec) *int64 {
	if entry == nil {
		return nil
	}
	if entry.MonthlyUSDMicros != nil {
		return entry.MonthlyUSDMicros
	}
	if entry.MonthlyEURMicros != nil {
		rate := 0.0
		if fx != nil {
			rate = fx.UsdPerEUR
		}
		return usdMicrosFromEUR(entry.MonthlyEURMicros, rate)
	}
	return nil
}

// PlanVATRate returns plan_vat_rate from spec/agent.yaml, the VAT fraction for
// the gross line of the usage report (ticket 1302 M1). A missing value is an
// error: nothing here defaults it.
func PlanVATRate() (float64, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	if spec.PlanVATRate == nil {
		return 0, fmt.Errorf("plan_vat_rate is missing from spec/agent.yaml")
	}
	return *spec.PlanVATRate, nil
}

// EffectivePlanPrices returns spec/agent.yaml plan_prices overlaid with any
// agent.plan_prices entries from ~/.harnez/config.yaml. Setting an entry or
// its price to null clears the price to unknown (nil). Results are in USD.
func EffectivePlanPrices() (map[string]PlanPriceSpec, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	out := make(map[string]PlanPriceSpec, len(spec.PlanPrices))
	for provider, entry := range spec.PlanPrices {
		usd := resolvePlanUSD(entry, spec.PlanFX)
		if usd == nil || *usd <= 0 {
			continue
		}
		v := *usd
		out[provider] = PlanPriceSpec{Plan: entry.Plan, Currency: "USD", MonthlyUSDMicros: &v}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return out, nil
	}
	b, readErr := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return out, nil
		}
		return nil, fmt.Errorf("read agent plan_prices config: %w", readErr)
	}
	var cfg planPricesConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent plan_prices config: %w", err)
	}
	for provider, override := range cfg.Agent.PlanPrices {
		if override != nil && override.MonthlyUSDMicros != nil && *override.MonthlyUSDMicros < 0 {
			return nil, fmt.Errorf("agent.plan_prices.%s.monthly_usd_micros must be >= 0", provider)
		}
		usd := resolvePlanUSD(override, spec.PlanFX)
		if usd == nil || *usd == 0 {
			delete(out, provider)
			continue
		}
		v := *usd
		plan := override.Plan
		if plan == "" {
			if existing, ok := out[provider]; ok {
				plan = existing.Plan
			}
		}
		out[provider] = PlanPriceSpec{Plan: plan, Currency: "USD", MonthlyUSDMicros: &v}
	}
	return out, nil
}

// PlanPriceFor returns the effective monthly subscription price in micro-USD for
// provider, or nil when unset/unknown.
func PlanPriceFor(provider string) (*int64, error) {
	prices, err := EffectivePlanPrices()
	if err != nil {
		return nil, err
	}
	entry, ok := prices[provider]
	if !ok || entry.MonthlyUSDMicros == nil || *entry.MonthlyUSDMicros <= 0 {
		return nil, nil
	}
	v := *entry.MonthlyUSDMicros
	return &v, nil
}
