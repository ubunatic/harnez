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
	if spec.Scale.Min != 1.0 || spec.Scale.Max != 5.0 {
		t.Fatalf("unexpected scale: %+v", spec.Scale)
	}

	// Verify subscription multipliers
	if m := spec.Multiplier("codex"); m != 0.25 {
		t.Fatalf("expected codex multiplier 0.25, got %v", m)
	}
	if m := spec.Multiplier("claude"); m != 1.0 {
		t.Fatalf("expected claude multiplier 1.0, got %v", m)
	}
	if m := spec.Multiplier("agy"); m != 0.33 {
		t.Fatalf("expected agy multiplier 0.33, got %v", m)
	}
	if m := spec.Multiplier("unknown_provider"); m != 1.0 {
		t.Fatalf("expected default multiplier 1.0, got %v", m)
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
