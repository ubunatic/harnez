package issues

import (
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestResearchSchema(t *testing.T) {
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/research.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	const example = `# .harnez/research.yaml
subagent_model: "luna:med"
subagent_role: "advisor"
batch_size: 3

# Target environment & boundary constraints
constraints:
  profile: "RTX 3060 12GB VRAM, 48GB host RAM, Ubuntu Linux, x86_64"
  toolchain: "Go 1.26, llama.cpp"
  disallowed:
    - "Closed-source weights or proprietary APIs without self-hosting support"
  ignored_blockers:
    - "Community claims of 'does not work on AMD/Linux' (assume driver/kernel workarounds exist)"

# Prioritized research goals (P1-P3) with brief descriptions
goals:
  - id: fit
    prio: P1
    title: "Environment & Resource Fit"
    description: "Determine exact byte sizes, memory footprints, and feasibility within configured constraints."
  - id: quality
    prio: P1
    title: "Real-world Quality & Failure Modes"
    description: "Search community forums (Reddit, GitHub, X) for real user reception, hallucinations, loops, or regressions."
  - id: baselines
    prio: P2
    title: "Community Baselines"
    description: "Identify which established tools or models reviewers compare this candidate against."
  - id: identity
    prio: P3
    title: "Upstream Identity & Licensing"
    description: "Verify official repository, license terms, author/organization, and release timeline."

# Issue automation rules
actions:
  auto_close_infeasible: true
  close_reason_template: "evaluated: {verdict_summary}"
  update_section: "2. Technical Specification / Findings"
`
	var document map[string]any
	if err := yaml.Unmarshal([]byte(example), &document); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(document); err != nil {
		t.Fatalf("example: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"batch below minimum", func(d map[string]any) { d["batch_size"] = 0 }},
		{"batch above maximum", func(d map[string]any) { d["batch_size"] = 15 }},
		{"empty goals", func(d map[string]any) { d["goals"] = []any{} }},
		{"missing goals", func(d map[string]any) { delete(d, "goals") }},
		{"missing id", func(d map[string]any) { delete(d["goals"].([]any)[0].(map[string]any), "id") }},
		{"missing title", func(d map[string]any) { delete(d["goals"].([]any)[0].(map[string]any), "title") }},
		{"missing description", func(d map[string]any) { delete(d["goals"].([]any)[0].(map[string]any), "description") }},
		{"empty id", func(d map[string]any) { d["goals"].([]any)[0].(map[string]any)["id"] = "" }},
		{"empty title", func(d map[string]any) { d["goals"].([]any)[0].(map[string]any)["title"] = "" }},
		{"empty description", func(d map[string]any) { d["goals"].([]any)[0].(map[string]any)["description"] = "" }},
		{"invalid priority", func(d map[string]any) { d["goals"].([]any)[0].(map[string]any)["prio"] = "P9" }},
		{"extra root key", func(d map[string]any) { d["extra"] = true }},
		{"extra goal key", func(d map[string]any) { d["goals"].([]any)[0].(map[string]any)["extra"] = true }},
		{"extra constraints key", func(d map[string]any) { d["constraints"].(map[string]any)["extra"] = true }},
		{"extra actions key", func(d map[string]any) { d["actions"].(map[string]any)["extra"] = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var document map[string]any
			if err := yaml.Unmarshal([]byte(example), &document); err != nil {
				t.Fatal(err)
			}
			tt.mutate(document)
			if err := resolved.Validate(document); err == nil {
				t.Error("accepted invalid document")
			}
		})
	}
}
