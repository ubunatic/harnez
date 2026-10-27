package modelcatalog

import "testing"

func TestPurposeAliasSchema(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		valid  bool
	}{
		{"embedded", func(map[string]any) {}, true},
		{"unset allowed", func(d map[string]any) { d["purpose_aliases"].(map[string]any)["dev:ai"] = []any{} }, true},
		{"duplicate", func(d map[string]any) {
			d["purpose_aliases"].(map[string]any)["dev:smart"] = []any{"sol:med", "sol:med"}
		}, false},
		{"invalid role", func(d map[string]any) { d["purpose_aliases"].(map[string]any)["other:smart"] = []any{"sol:med"} }, false},
		{"missing roles", func(d map[string]any) { delete(d, "purpose_roles") }, false},
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
