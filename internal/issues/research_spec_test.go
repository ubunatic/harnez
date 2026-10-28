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
	goals := document["goals"].([]any)
	goal := goals[0].(map[string]any)
	goal["prio"] = "P9"
	if err := resolved.Validate(document); err == nil {
		t.Error("accepted invalid priority")
	}
	goal["prio"] = "P1"
	delete(document, "goals")
	if err := resolved.Validate(document); err == nil {
		t.Error("accepted missing goals")
	}
}
