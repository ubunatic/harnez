package subagent

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Helpers that read expected values from the shipped spec/agent.yaml, so tests
// follow catalog updates instead of pinning model names, tiers, costs or picks
// (issue 1252). Tests that build a synthetic spec or policy keep their literals.

func shippedSpec(t *testing.T) agentSpec {
	t.Helper()
	spec, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

// shippedModels returns the spec model entries that are not disabled in the shipped spec.
func shippedModels(t *testing.T) map[string]modelAlias {
	t.Helper()
	if err := ensureModelCatalog(); err != nil {
		t.Fatal(err)
	}
	models := map[string]modelAlias{}
	for key, entry := range shippedSpec(t).Models {
		if !mergedModelCatalog.ShippedDisabled(entry.Model.Provider, entry.Model.Name) {
			models[key] = entry
		}
	}
	return models
}

func shippedGuides(t *testing.T) map[string]ModelGuide {
	t.Helper()
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/agent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Models map[string]ModelGuide `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	return spec.Models
}

// expectedCost is the spec cost for a tier: its cost_by_tier entry, else the base cost.
func expectedCost(guide ModelGuide, tier string) int {
	if cost, ok := guide.CostByTier[tier]; ok {
		return cost
	}
	return guide.Cost
}

// effortArg is the provider effort flag value for a tier.
func effortArg(tier string) string {
	if tier == "med" {
		return "medium"
	}
	return tier
}

// aliasTier returns the tier an alias pins ("neon:med" pins med) or def when it pins none.
func aliasTier(alias, def string) string {
	if i := strings.LastIndex(alias, ":"); i >= 0 {
		return alias[i+1:]
	}
	return def
}

// shippedChainHead follows trait references in a role's picks to the first model spec.
func shippedChainHead(t *testing.T, role, trait string) string {
	t.Helper()
	models := shippedSpec(t).Roles[role].Models
	for depth := 0; depth <= maxTraitDepth; depth++ {
		chain := models[trait]
		if len(chain) == 0 {
			t.Fatalf("role %s trait %s has no picks", role, trait)
		}
		if _, isTrait := models[chain[0]]; !isTrait {
			return chain[0]
		}
		trait = chain[0]
	}
	t.Fatalf("role %s trait %s exceeds trait depth", role, trait)
	return ""
}

func mustResolve(t *testing.T, spec string) Model {
	t.Helper()
	m, err := ResolveModel(spec)
	if err != nil {
		t.Fatalf("ResolveModel(%q): %v", spec, err)
	}
	return m
}
