package usage

import (
	"encoding/json"
	"io/fs"
	"slices"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestEmbeddedCacheFreshnessPolicy(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/usage.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"collector_cadence", "cache_staleness_multiplier", "display_staleness"} {
		if raw[key] == nil || !slices.Contains(schema.Required, key) {
			t.Fatalf("policy key %s must exist in embedded YAML and be required by its schema", key)
		}
	}
	cadence, err := time.ParseDuration(raw["collector_cadence"].(string))
	if err != nil {
		t.Fatal(err)
	}
	display, err := time.ParseDuration(raw["display_staleness"].(string))
	if err != nil {
		t.Fatal(err)
	}
	cacheAge := cadence * time.Duration(raw["cache_staleness_multiplier"].(int))
	if DefaultCollectorInterval() != cadence || DefaultCacheStaleness() != cacheAge || AgentQuotaAvailabilityMaxAge() != cacheAge || DefaultDisplayStaleness() != display {
		t.Fatal("runtime freshness policy differs from embedded spec")
	}
	collectorCadence, _, err := collectorPolicy()
	if err != nil || collectorCadence != cadence {
		t.Fatalf("collector cadence = %s, err=%v; want %s", collectorCadence, err, cadence)
	}
	now := time.Now()
	if fraction := freshnessFraction(now.Add(-cadence/2), now); fraction != .5 {
		t.Fatalf("half a collector tick should leave half the freshness bar, got %v", fraction)
	}
	for _, tc := range []struct {
		name  string
		age   time.Duration
		limit time.Duration
		stale bool
	}{
		{"cache within threshold", cacheAge / 2, DefaultCacheStaleness(), false},
		{"cache beyond threshold", cacheAge * 2, DefaultCacheStaleness(), true},
		{"display within threshold", display / 2, DefaultDisplayStaleness(), false},
		{"display beyond threshold", display * 2, DefaultDisplayStaleness(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := AgentUsage{LastRefreshed: now.Add(-tc.age)}
			if got := agent.IsStale(tc.limit); got != tc.stale {
				t.Fatalf("stale=%v, want %v for age %s and limit %s", got, tc.stale, tc.age, tc.limit)
			}
		})
	}
}

func TestCacheFreshnessPolicyUsesChangedSpecValues(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	spec["collector_cadence"] = "7m"
	spec["cache_staleness_multiplier"] = 3
	spec["display_staleness"] = "49h"
	data, err = yaml.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	_, cadence, _, _, _, err := parseUsageSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	var parsed usageSpec
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	policy, err := parsed.cachePolicy()
	if err != nil {
		t.Fatal(err)
	}
	if cadence != 7*time.Minute || policy.cadence != cadence || policy.cacheStaleness != 21*time.Minute || policy.displayStaleness != 49*time.Hour {
		t.Fatalf("changed spec policy = %+v; collector cadence = %s", policy, cadence)
	}
}
