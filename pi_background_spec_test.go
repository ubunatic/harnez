package harnez

import (
	"encoding/json"
	"io/fs"
	"maps"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

// The Pi background-task spec is JSON-compatible YAML: the TypeScript loader parses it with JSON.parse.
// This test validates it against its schema, so `make test-short` in CI catches a spec that the loader
// would reject only at Pi startup.
func TestPiBackgroundTasksSpecMatchesSchema(t *testing.T) {
	specData, err := fs.ReadFile(DefaultFS, "spec/pi-background-tasks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(specData, &document); err != nil {
		t.Fatalf("parse spec/pi-background-tasks.yaml: %v", err)
	}
	resolved := resolvePiBackgroundSchema(t)
	if err := resolved.Validate(document); err != nil {
		t.Fatalf("validate spec/pi-background-tasks.yaml: %v", err)
	}

	// The schema must reject what the loader rejects: a non-positive bound, an unknown key, a missing stop key.
	for name, mutate := range map[string]func(map[string]any){
		"zero output limit": func(doc map[string]any) { doc["output_limit_chars"] = 0 },
		"unknown key":       func(doc map[string]any) { doc["extra"] = 1 },
		"missing poll":      func(doc map[string]any) { delete(doc["stop"].(map[string]any), "poll_ms") },
	} {
		mutated := clonePiBackgroundSpec(document)
		mutate(mutated)
		if err := resolved.Validate(mutated); err == nil {
			t.Errorf("schema accepted an invalid spec: %s", name)
		}
	}
}

func resolvePiBackgroundSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	schemaData, err := fs.ReadFile(DefaultFS, "spec/schemas/pi-background-tasks.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatalf("parse pi-background-tasks schema: %v", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve pi-background-tasks schema: %v", err)
	}
	return resolved
}

// clonePiBackgroundSpec copies the nested maps so a mutation does not change the valid document.
func clonePiBackgroundSpec(doc map[string]any) map[string]any {
	out := maps.Clone(doc)
	out["stop"] = maps.Clone(doc["stop"].(map[string]any))
	return out
}
