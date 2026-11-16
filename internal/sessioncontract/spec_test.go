package sessioncontract

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestEmbeddedSchema(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/session_isolation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	data, err = harnez.DefaultFS.ReadFile("spec/schemas/session_isolation.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatal(err)
	}
	entries := doc["ownership"].([]any)
	entries[0].(map[string]any)["class"] = "unknown"
	if err := resolved.Validate(doc); err == nil {
		t.Fatal("invalid ownership class accepted")
	}
}

func TestContractValidation(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/shutdown", "merge_legacy", "arbitrary_paths", "arbitrary_pids", "cross_session_stop", "cross_session_resume"} {
		bad := c
		bad.RunnerOperations = append(append([]string{}, c.RunnerOperations...), route)
		if err := bad.Validate(); err == nil {
			t.Errorf("runner operation %q accepted", route)
		}
	}
	bad := c
	bad.Ownership = append([]Ownership{}, c.Ownership...)
	bad.Ownership[0].Class = "unknown"
	if err := bad.Validate(); err == nil {
		t.Fatal("invalid ownership accepted")
	}
}

func TestPromoterRoots(t *testing.T) {
	for _, tc := range []struct {
		root, approved, checkout string
		ok                       bool
	}{
		{"/home/u/go/bin", "/home/u/go/bin", "/work/repo", true},
		{"relative/bin", "relative/bin", "/work/repo", false},
		{"/tmp/arbitrary", "/home/u/go/bin", "/work/repo", false},
		{"/work/repo/bin", "/work/repo/bin", "/work/repo", false},
		{"/work/repo/../repo/bin", "/work/repo/../repo/bin", "/work/repo", false},
	} {
		if err := ValidateRoot(tc.root, tc.approved, tc.checkout); (err == nil) != tc.ok {
			t.Errorf("root %q: %v", tc.root, err)
		}
	}
}
