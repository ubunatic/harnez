package subagent

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PlanPriceSpec defines a monthly subscription price for one provider (ticket 1215).
// A nil MonthlyUSDMicros marks the subscription price as unknown.
type PlanPriceSpec struct {
	Plan             string `yaml:"plan"`
	MonthlyUSDMicros *int64 `yaml:"monthly_usd_micros"`
}

type planPricesConfig struct {
	Agent struct {
		PlanPrices map[string]*PlanPriceSpec `yaml:"plan_prices"`
	} `yaml:"agent"`
}

// EffectivePlanPrices returns spec/agent.yaml plan_prices overlaid with any
// agent.plan_prices entries from ~/.harnez/config.yaml. Setting an entry or
// its monthly_usd_micros to null clears the price to unknown (nil).
func EffectivePlanPrices() (map[string]PlanPriceSpec, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	out := make(map[string]PlanPriceSpec, len(spec.PlanPrices))
	for provider, entry := range spec.PlanPrices {
		if entry == nil || entry.MonthlyUSDMicros == nil {
			continue
		}
		v := *entry.MonthlyUSDMicros
		if v <= 0 {
			continue
		}
		out[provider] = PlanPriceSpec{Plan: entry.Plan, MonthlyUSDMicros: &v}
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
		if override == nil || override.MonthlyUSDMicros == nil {
			delete(out, provider)
			continue
		}
		if *override.MonthlyUSDMicros < 0 {
			return nil, fmt.Errorf("agent.plan_prices.%s.monthly_usd_micros must be >= 0", provider)
		}
		if *override.MonthlyUSDMicros == 0 {
			delete(out, provider)
			continue
		}
		v := *override.MonthlyUSDMicros
		plan := override.Plan
		if plan == "" {
			if existing, ok := out[provider]; ok {
				plan = existing.Plan
			}
		}
		out[provider] = PlanPriceSpec{Plan: plan, MonthlyUSDMicros: &v}
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
