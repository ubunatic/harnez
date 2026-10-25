package modelcatalog

import (
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestContractFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		schema  string
		valid   bool
	}{
		{"models-valid.yaml", "models.schema.json", true},
		{"models-unknown-key.yaml", "models.schema.json", false},
		{"policy-valid.yaml", "model-policy.schema.json", true},
		{"policy-unknown-key.yaml", "model-policy.schema.json", false},
		{"access-valid.yaml", "model-access.schema.json", true},
		{"access-unknown-key.yaml", "model-access.schema.json", false},
		{"snapshot-valid.yaml", "model-snapshot.schema.json", true},
		{"snapshot-unknown-key.yaml", "model-snapshot.schema.json", false},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			fixture, err := os.ReadFile("testdata/" + tc.fixture)
			if err != nil {
				t.Fatal(err)
			}
			var doc any
			if err := yaml.Unmarshal(fixture, &doc); err != nil {
				t.Fatal(err)
			}
			err = validate(t, tc.schema, doc)
			if tc.valid && err != nil {
				t.Fatalf("valid fixture rejected: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("invalid fixture accepted")
			}
		})
	}
}

func TestLegacyAgentCatalogStillMatchesSchemaAndProbeBudget(t *testing.T) {
	doc := readYAML(t, "spec/agent.yaml")
	if err := validate(t, "agent.schema.json", doc); err != nil {
		t.Fatalf("legacy agent spec rejected: %v", err)
	}
	budget := doc.(map[string]any)["model_probe_budget"].(map[string]any)
	if budget["max_calls_per_update"] != 20 || budget["max_output_tokens_per_call"] != 16 || budget["timeout_seconds"] != 30 {
		t.Fatalf("probe budget = %#v", budget)
	}
}

func readYAML(t *testing.T, name string) any {
	t.Helper()
	data, err := fs.ReadFile(harnez.DefaultFS, name)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func validate(t *testing.T, schemaName string, doc any) error {
	t.Helper()
	load := func(name string) (*jsonschema.Schema, error) {
		data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/"+name)
		if err != nil {
			return nil, err
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, err
		}
		return &schema, nil
	}
	root, err := load(schemaName)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := root.Resolve(&jsonschema.ResolveOptions{
		BaseURI: "https://schemas.harnez.invalid/",
		Loader: func(uri *url.URL) (*jsonschema.Schema, error) {
			name := path.Base(uri.Path)
			if strings.Contains(name, "..") {
				return nil, fs.ErrNotExist
			}
			return load(name)
		},
	})
	if err != nil {
		t.Fatalf("resolve %s: %v", schemaName, err)
	}
	return resolved.Validate(doc)
}
