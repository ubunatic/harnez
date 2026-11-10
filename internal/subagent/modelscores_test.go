package subagent

import (
	"testing"
)

func TestLoadModelScores(t *testing.T) {
	spec, err := LoadModelScores()
	if err != nil {
		t.Fatalf("LoadModelScores failed: %v", err)
	}

	if spec.Version != 1 {
		t.Fatalf("expected version 1, got %d", spec.Version)
	}
	if spec.Scale.Min != 1.0 || spec.Scale.Max != 5.0 || spec.Scale.Step != 0.5 {
		t.Fatalf("unexpected scale: %+v", spec.Scale)
	}

	// Verify cost ticks from spec
	if len(spec.CostTicks) == 0 {
		t.Fatal("expected non-empty cost ticks from spec")
	}

	// Verify subscription multipliers
	m, err := spec.Multiplier("codex")
	if err != nil || m != 0.25 {
		t.Fatalf("expected codex multiplier 0.25, got %v (err: %v)", m, err)
	}
	m, err = spec.Multiplier("claude")
	if err != nil || m != 1.0 {
		t.Fatalf("expected claude multiplier 1.0, got %v (err: %v)", m, err)
	}
	m, err = spec.Multiplier("agy")
	if err != nil || m != 0.33 {
		t.Fatalf("expected agy multiplier 0.33, got %v (err: %v)", m, err)
	}

	// Item 3: missing multiplier MUST be an error, not fallback to 1.0
	_, err = spec.Multiplier("non_existent_provider_xyz")
	if err == nil {
		t.Fatal("expected error for missing provider multiplier, got nil")
	}

	// Verify provider colors resolved from spec/colors.yaml
	codexColor, err := spec.ProviderColor("codex")
	if err != nil || codexColor != "32" {
		t.Fatalf("expected codex color 32 from spec/colors.yaml, got %q (err: %v)", codexColor, err)
	}
	claudeColor, err := spec.ProviderColor("claude")
	if err != nil || claudeColor != "33" {
		t.Fatalf("expected claude color 33 from spec/colors.yaml, got %q (err: %v)", claudeColor, err)
	}
	agyColor, err := spec.ProviderColor("agy")
	if err != nil || agyColor != "36" {
		t.Fatalf("expected agy color 36 from spec/colors.yaml, got %q (err: %v)", agyColor, err)
	}

	// Verify frontier color from spec/colors.yaml
	frontierColor, err := spec.FrontierColor()
	if err != nil || frontierColor != "1;37" {
		t.Fatalf("expected frontier color 1;37 from spec/colors.yaml, got %q (err: %v)", frontierColor, err)
	}

	// Verify ratings for every known model
	entries, err := KnownModelEntriesE()
	if err != nil {
		t.Fatalf("KnownModelEntriesE failed: %v", err)
	}

	for _, e := range entries {
		alias := modelAliasName(e.Model)
		rating, source, ok := spec.Rating(e.Model.Provider, alias, e.Model.Tier)
		if !ok {
			t.Errorf("model %s (%s:%s:%s) has no rating in spec/model_scores.yaml", e.Spec, e.Model.Provider, alias, e.Model.Tier)
			continue
		}
		if rating < spec.Scale.Min || rating > spec.Scale.Max {
			t.Errorf("rating %v for %s out of scale range [%v, %v]", rating, e.Spec, spec.Scale.Min, spec.Scale.Max)
		}
		if source != "owner estimate, unverified" {
			t.Errorf("source for %s is %q, expected 'owner estimate, unverified'", e.Spec, source)
		}
	}
}
