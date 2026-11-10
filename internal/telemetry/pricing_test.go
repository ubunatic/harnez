package telemetry

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestLoadPricingCatalog_EmbeddedSpec(t *testing.T) {
	catalog, err := LoadPricingCatalog("")
	if err != nil {
		t.Fatalf("LoadPricingCatalog failed: %v", err)
	}

	model, ok := catalog["gpt-5"]
	if !ok {
		t.Fatalf("expected 'gpt-5' model in catalog")
	}

	if model.Model != "gpt-5" {
		t.Errorf("expected model 'gpt-5', got %q", model.Model)
	}
	// stats.go labels the pricing source with RecordedPricingRevision, so the spec must match it.
	if model.Revision != RecordedPricingRevision {
		t.Errorf("spec/pricing.yaml revision %q differs from RecordedPricingRevision %q", model.Revision, RecordedPricingRevision)
	}
	if model.Rates.CachedInputMicrosPerMillion == nil || *model.Rates.CachedInputMicrosPerMillion != 125000 {
		t.Errorf("unexpected cached input micros: %v", model.Rates.CachedInputMicrosPerMillion)
	}
	if model.Rates.UncachedInputMicrosPerMillion == nil || *model.Rates.UncachedInputMicrosPerMillion != 1250000 {
		t.Errorf("unexpected uncached input micros: %v", model.Rates.UncachedInputMicrosPerMillion)
	}
	if model.Rates.OutputMicrosPerMillion == nil || *model.Rates.OutputMicrosPerMillion != 10000000 {
		t.Errorf("unexpected output micros: %v", model.Rates.OutputMicrosPerMillion)
	}
	if model.Rates.ReasoningMicrosPerMillion == nil || *model.Rates.ReasoningMicrosPerMillion != 10000000 {
		t.Errorf("unexpected reasoning micros: %v", model.Rates.ReasoningMicrosPerMillion)
	}
}

func TestLoadPricingCatalog_EnvOverride(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "pricing.json")

	testData := []byte(`{"gpt-test":{"model":"gpt-test","revision":"test-1","rates":{"cached_input_micros_per_million":10,"uncached_input_micros_per_million":20,"output_micros_per_million":30,"reasoning_micros_per_million":40}}}`)

	if err := os.WriteFile(tempFile, testData, 0644); err != nil {
		t.Fatalf("failed to write temp pricing file: %v", err)
	}

	t.Setenv("HARNEZ_PRICING_FILE", tempFile)

	catalog, err := LoadPricingCatalog("")
	if err != nil {
		t.Fatalf("LoadPricingCatalog failed: %v", err)
	}

	model, ok := catalog["gpt-test"]
	if !ok {
		t.Fatalf("expected 'gpt-test' model in catalog")
	}

	if *model.Rates.CachedInputMicrosPerMillion != 10 {
		t.Errorf("unexpected cached input micros: %v", *model.Rates.CachedInputMicrosPerMillion)
	}
}

func TestPricingSchemaValidation(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/pricing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/pricing.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatalf("spec violates schema: %v", err)
	}

	// Bad entry: missing model field
	badEntry := []byte(`{"gpt-5":{"revision":"harnez-2026-09-18","rates":{"cached_input_micros_per_million":125000,"uncached_input_micros_per_million":1250000,"output_micros_per_million":10000000,"reasoning_micros_per_million":10000000}}}`)
	var badInst any
	if err := json.Unmarshal(badEntry, &badInst); err != nil {
		t.Fatal(err)
	}

	if err := resolved.Validate(badInst); err == nil {
		t.Errorf("expected schema to reject bad entry (missing model), but it passed")
	}
}
