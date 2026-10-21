package issues

import (
	"encoding/json"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestEmbeddedLifecycleSpecAndSchema(t *testing.T) {
	spec, err := currentLifecycleSpec()
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(spec.Lifecycle.Unresolved, "backlog") {
		t.Fatal("embedded unresolved lifecycle does not include backlog")
	}
	if !StatusFilterMatches("Backlog", "backlog") || !StatusFilterMatches("Backlog — later", "unresolved") {
		t.Fatal("embedded lifecycle filters do not match Backlog")
	}
	if StatusFilterMatches("Backlog:", "backlog") || StatusFilterMatches("Backlogged", "backlog") {
		t.Fatal("Backlog filter accepted a near-miss")
	}

	data, err := harnez.DefaultFS.ReadFile("spec/schemas/issues.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Type                 string   `json:"type"`
		AdditionalProperties bool     `json:"additionalProperties"`
		Required             []string `json:"required"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode lifecycle schema: %v", err)
	}
	if schema.Type != "object" || schema.AdditionalProperties || !containsString(schema.Required, "lifecycle") {
		t.Fatalf("lifecycle schema is not strict at the root: %+v", schema)
	}

	values, err := harnez.DefaultFS.ReadFile("spec/issues.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(values), "match_mode: lead_token\n      match_terms: [done]") {
		t.Fatal("Done matcher is not declared as an exact leading token in the spec")
	}
}
