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
