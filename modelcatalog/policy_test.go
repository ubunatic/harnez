package modelcatalog

import (
	"strings"
	"testing"
)

func TestValidatePolicyPins(t *testing.T) {
	tests := []struct {
		name string
		doc  any
		want string
	}{
		{"matching pin", map[string]any{"models": map[string]any{"m": map[string]any{"version_pin": "1", "identity": map[string]any{"version": "1"}}}}, ""},
		{"mismatching pin", map[string]any{"models": map[string]any{"m": map[string]any{"version_pin": "2", "identity": map[string]any{"version": "1"}}}}, "does not match identity.version"},
		{"pin without identity version", map[string]any{"models": map[string]any{"m": map[string]any{"version_pin": "2"}}}, ""},
		{"no models", map[string]any{}, ""},
		{"invalid root", []any{}, "policy must be an object"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePolicyPins(tc.doc)
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestIsEnabledPreservesExactPiModelID(t *testing.T) {
	identity := Identity{Harness: "pi", Provider: "anthropic", Model: "anthropic/Claude-Opus-4.6", Tier: "low"}
	model := Model{Identity: identity, Key: identity.Model}
	policy := &Policy{DefaultEnabled: false, Enabled: []string{"pi:anthropic/Claude-Opus-4.6"}}
	catalog := &Catalog{models: map[string]Model{modelID(identity): model}, policy: policy}

	if !catalog.IsEnabled("pi", identity.Model, "low") {
		t.Fatal("explicit policy ID should enable the exact case-sensitive Pi ID")
	}
	if catalog.IsEnabled("pi", "anthropic/claude-opus-4.6", "low") {
		t.Fatal("policy ID must not match a Pi model ID with different case")
	}
}

func TestShippedDisabledNeedsExplicitUserEnable(t *testing.T) {
	identity := Identity{Harness: "agy", Provider: "agy", Model: "gemini-3.7-flash", Tier: "low"}
	model := Model{Identity: identity, Key: "flash37", Aliases: []string{"flash37", "flash37:low"}, Disabled: true}
	catalog := func(policy *Policy) *Catalog {
		return &Catalog{models: map[string]Model{modelID(identity): model}, policy: policy}
	}
	if catalog(nil).IsEnabled("agy", "gemini-3.7-flash", "low") {
		t.Fatal("shipped-disabled model enabled without user policy")
	}
	if catalog(&Policy{DefaultEnabled: true}).IsEnabled("agy", "gemini-3.7-flash", "low") {
		t.Fatal("default_enabled must not re-enable a shipped-disabled model")
	}
	if !catalog(&Policy{Enabled: []string{"agy:flash37"}}).IsEnabled("agy", "gemini-3.7-flash", "low") {
		t.Fatal("explicit user enable lost")
	}
}
