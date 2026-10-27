package modelcatalog

import "testing"

func TestPurposeAliasSchema(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		valid  bool
	}{
		{"embedded", func(map[string]any) {}, true},
		{"unset allowed", func(d map[string]any) {
			d["roles"].(map[string]any)["developer"].(map[string]any)["models"].(map[string]any)["ai"] = []any{}
		}, true},
		{"duplicate", func(d map[string]any) {
			d["roles"].(map[string]any)["developer"].(map[string]any)["models"].(map[string]any)["smart"] = []any{"sol:med", "sol:med"}
		}, false},
		{"invalid trait", func(d map[string]any) {
			d["roles"].(map[string]any)["developer"].(map[string]any)["models"].(map[string]any)["Bad"] = []any{"sol:med"}
		}, false},
		{"old key", func(d map[string]any) { d["purpose_roles"] = map[string]any{} }, false},
		{"missing traits", func(d map[string]any) { delete(d, "model_traits") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := readYAML(t, "spec/agent.yaml").(map[string]any)
			tc.change(doc)
			if err := validate(t, "agent.schema.json", doc); (err == nil) != tc.valid {
				t.Fatalf("schema error = %v, valid = %v", err, tc.valid)
			}
		})
	}
}
