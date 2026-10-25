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
		fixture     string
		schema      string
		valid       bool
		wantPath    string
		wantKeyword string
		checkPin    bool
	}{
		{fixture: "models-valid.yaml", schema: "models.schema.json", valid: true},
		{fixture: "models-unknown-key.yaml", schema: "models.schema.json", wantPath: "models.schema.json", wantKeyword: "unexpected additional properties"},
		{fixture: "models-fact-unknown-key.yaml", schema: "models.schema.json", wantPath: "models.schema.json", wantKeyword: "unexpected additional properties"},
		{fixture: "models-negative-price.yaml", schema: "models.schema.json", wantPath: "/properties/value", wantKeyword: "exclusiveMinimum"},
		{fixture: "models-negative-api-price.yaml", schema: "models.schema.json", wantPath: "/properties/input_per_million", wantKeyword: "minimum"},
		{fixture: "models-zero-relative-cost.yaml", schema: "models.schema.json", wantPath: "/properties/value", wantKeyword: "exclusiveMinimum"},
		{fixture: "models-web-without-url.yaml", schema: "models.schema.json", wantPath: "/$defs/fact/allOf/0/then", wantKeyword: "required"},
		{fixture: "models-discovered-without-method.yaml", schema: "models.schema.json", wantPath: "/$defs/fact/allOf/1/then", wantKeyword: "method"},
		{fixture: "models-discovered-without-account.yaml", schema: "models.schema.json", wantPath: "/$defs/fact/allOf/1/then", wantKeyword: "account"},
		{fixture: "models-measured-without-evidence.yaml", schema: "models.schema.json", wantPath: "/properties/evidence", wantKeyword: "minItems"},
		{fixture: "models-bad-tier.yaml", schema: "models.schema.json", wantPath: "/properties/tier", wantKeyword: "enum"},
		{fixture: "models-bad-observed-at.yaml", schema: "models.schema.json", wantPath: "/properties/observed_at", wantKeyword: "pattern"},
		{fixture: "policy-valid.yaml", schema: "model-policy.schema.json", valid: true, checkPin: true},
		{fixture: "policy-valid-pi-id.yaml", schema: "model-policy.schema.json", valid: true},
		{fixture: "policy-unknown-key.yaml", schema: "model-policy.schema.json", wantPath: "model-policy.schema.json", wantKeyword: "unexpected additional properties"},
		{fixture: "policy-custom-without-identity.yaml", schema: "model-policy.schema.json", wantPath: "/allOf/0/then", wantKeyword: "required"},
		{fixture: "policy-bad-enabled-id.yaml", schema: "model-policy.schema.json", wantPath: "/properties/enabled/items", wantKeyword: "pattern"},
		{fixture: "policy-bad-version-pin.yaml", schema: "model-policy.schema.json", wantPath: "models.custom:internal-7b.version_pin", wantKeyword: "does not match identity.version", checkPin: true},
		{fixture: "access-valid.yaml", schema: "model-access.schema.json", valid: true},
		{fixture: "access-unknown-key.yaml", schema: "model-access.schema.json", wantPath: "model-access.schema.json", wantKeyword: "unexpected additional properties"},
		{fixture: "snapshot-valid.yaml", schema: "model-snapshot.schema.json", valid: true},
		{fixture: "snapshot-unknown-key.yaml", schema: "model-snapshot.schema.json", wantPath: "model-snapshot.schema.json", wantKeyword: "unexpected additional properties"},
		{fixture: "change-report-valid.yaml", schema: "model-change-report.schema.json", valid: true},
		{fixture: "change-report-bad-category.yaml", schema: "model-change-report.schema.json", wantPath: "/properties/category", wantKeyword: "enum"},
		{fixture: "change-report-bad-source.yaml", schema: "model-change-report.schema.json", wantPath: "/properties/sources/items", wantKeyword: "enum"},
		{fixture: "policy-patch-semantics.yaml", schema: "model-policy.schema.json", valid: true},
		{fixture: "change-report-unknown-key.yaml", schema: "model-change-report.schema.json", wantPath: "/properties/rows/items", wantKeyword: "unexpected additional properties"},
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
			if err == nil && tc.checkPin {
				err = ValidatePolicyPins(doc)
			}
			if tc.valid && err != nil {
				t.Fatalf("valid fixture rejected: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("invalid fixture accepted")
			}
			if !tc.valid && (!strings.Contains(err.Error(), tc.wantPath) || !strings.Contains(err.Error(), tc.wantKeyword)) {
				t.Fatalf("rejection = %q, want path %q and keyword %q", err, tc.wantPath, tc.wantKeyword)
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
		// Schema $ids establish the reference base; this loader only maps those
		// external reference URIs back to the embedded schema files.
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
