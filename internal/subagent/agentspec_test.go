package subagent

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestFlash38EscalationGuidanceAndLeanSprintDeveloperPreference(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/agent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Models map[string]ModelGuide `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	flash38 := spec.Models["agy:flash38"]
	if flash38.Cost != 4 {
		t.Errorf("agy:flash38 cost = %d, want 4 (agy quota cost remains unmeasured)", flash38.Cost)
	}
	if flash38.Roles != "reviewer, advisor" {
		t.Errorf("agy:flash38 roles = %q, want reviewer, advisor", flash38.Roles)
	}
	if !strings.Contains(flash38.Use, "escalation-only") || !strings.Contains(flash38.Use, "avoid for developer work") {
		t.Errorf("agy:flash38 use = %q, want escalation-only and avoid developer work", flash38.Use)
	}
	for name, model := range spec.Models {
		if strings.HasPrefix(name, "agy:") && strings.Contains(model.Roles, "developer") {
			t.Errorf("%s roles = %q, want no developer role", name, model.Roles)
		}
	}
	if sonnet := spec.Models["agy:sonnet"]; !strings.Contains(sonnet.Use, "reviewer when Claude quota is out") || strings.Contains(sonnet.Use, "developer") {
		t.Errorf("agy:sonnet use = %q, want reviewer-only guidance when Claude quota is out", sonnet.Use)
	}
	skill, err := fs.ReadFile(harnez.DefaultFS, "docs/commands/lean-sprint.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"Prefer `luna` first", "use `terra` when stronger judgment is needed", "avoid `agy` models for developer work"} {
		if !strings.Contains(string(skill), phrase) {
			t.Errorf("lean-sprint skill missing %q", phrase)
		}
	}
}

func TestEmbeddedAgentSpecLoadsAndResolves(t *testing.T) {
	spec, err := loadAgentSpec()
	if err != nil {
		t.Fatal(err)
	}
	if spec.DefaultModel != "codex:luna:low" {
		t.Fatalf("default=%q", spec.DefaultModel)
	}
	model, err := DefaultModel()
	if err != nil || model.Provider != "codex" {
		t.Fatalf("model=%#v err=%v", model, err)
	}
	if spec.ExternalSessions["codex"].Format != "jsonl" {
		t.Fatalf("Codex external session spec = %#v", spec.ExternalSessions["codex"])
	}
}

func TestExternalSessionSchemaDeclaresRuntimeFields(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/agent.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	type schemaNode struct {
		MinProperties        int                    `json:"minProperties"`
		Required             []string               `json:"required"`
		Properties           map[string]*schemaNode `json:"properties"`
		AdditionalProperties json.RawMessage        `json:"additionalProperties"`
	}
	var schema struct {
		Required   []string               `json:"required"`
		Properties map[string]*schemaNode `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	contains := func(values []string, target string) bool {
		for _, value := range values {
			if value == target {
				return true
			}
		}
		return false
	}
	if !contains(schema.Required, "external_sessions") {
		t.Fatal("agent schema does not require external_sessions")
	}
	external := schema.Properties["external_sessions"]
	if external == nil {
		t.Fatal("agent schema is missing external_sessions")
	}
	if external.MinProperties < 1 {
		t.Fatal("external_sessions schema must require at least one provider")
	}
	var provider schemaNode
	if err := json.Unmarshal(external.AdditionalProperties, &provider); err != nil {
		t.Fatal("external_sessions schema must define provider entries")
	}
	for _, field := range []string{"root", "pattern", "format", "record_type_path", "record_type", "max_record_bytes", "name_prefix", "available_status", "fields"} {
		if _, ok := provider.Properties[field]; !ok {
			t.Errorf("external provider schema is missing %q", field)
		}
	}
	fields := provider.Properties["fields"]
	if fields == nil {
		t.Fatal("external provider schema is missing fields")
	}
	for _, field := range []string{"id", "working_dir", "model_provider", "source", "created_at"} {
		if !contains(fields.Required, field) {
			t.Errorf("external field mapping schema does not require %q", field)
		}
	}
}

func TestParseAgentSpecRejectsBadDefault(t *testing.T) {
	for _, data := range []string{"{}", "default_model: codex:missing:low\n"} {
		if _, err := parseAgentSpec([]byte(data)); err == nil {
			t.Fatalf("parse %q unexpectedly succeeded", data)
		}
	}
}

func TestParseAgentSpecRequiresExternalSessionDiscovery(t *testing.T) {
	data := "default_model: codex:luna:low\ndefault_role: a\nroles:\n  a: {spawns: [], rules: fine}\n"
	if _, err := parseAgentSpec([]byte(data)); err == nil || !strings.Contains(err.Error(), "external_sessions") {
		t.Fatalf("parse without external_sessions = %v, want required provider error", err)
	}
}

func TestResolveModelBareAliases(t *testing.T) {
	for _, alias := range []string{"luna", "sol", "astra", "haiku", "flash37", "flash38"} {
		if _, err := ResolveModel(alias); err != nil {
			t.Errorf("alias %q: %v", alias, err)
		}
	}
	if _, err := ResolveModel("unknown"); err == nil {
		t.Fatal("unknown alias unexpectedly resolved")
	}
}

func TestCodexSolAndLunaResolveToGPT6(t *testing.T) {
	for alias, want := range map[string]string{
		"sol":            "gpt-6.1-sol",
		"codex:sol":      "gpt-6.1-sol",
		"codex:sol:low":  "gpt-6.1-sol",
		"luna":           "gpt-6-luna",
		"codex:luna":     "gpt-6-luna",
		"codex:luna:med": "gpt-6-luna",
	} {
		model, err := ResolveModel(alias)
		if err != nil {
			t.Fatalf("ResolveModel(%q): %v", alias, err)
		}
		if model.Name != want {
			t.Errorf("ResolveModel(%q).Name = %q, want %q", alias, model.Name, want)
		}
		if model.Provider != "codex" {
			t.Errorf("ResolveModel(%q).Provider = %q, want codex", alias, model.Provider)
		}
	}
}

func TestResolveModelBareSonnetOpusAmbiguous(t *testing.T) {
	// The spec selects the Claude model as preferred when provider aliases collide.
	for alias, want := range map[string]string{"sonnet": "claude:sonnet", "opus": "claude:opus"} {
		model, err := ResolveModel(alias)
		if err != nil {
			t.Errorf("alias %q: %v", alias, err)
			continue
		}
		if got := model.Provider + ":" + model.Name; got != want {
			t.Errorf("alias %q resolved to %q, want %q", alias, got, want)
		}
	}
	for _, spec := range []string{"claude:sonnet", "claude:opus", "agy:sonnet", "agy:opus"} {
		if _, err := ResolveModel(spec); err != nil {
			t.Errorf("spec %q: %v", spec, err)
		}
	}
}

func TestAgentSpecRolesAndCheckSpawn(t *testing.T) {
	if r, err := DefaultRole(); err != nil || r != "developer" {
		t.Fatalf("default role = %q, %v", r, err)
	}
	names, err := RoleNames()
	if err != nil || strings.Join(names, ",") != "advisor,developer,orchestrator,reviewer" {
		t.Fatalf("roles = %v, %v", names, err)
	}
	for _, tc := range []struct{ caller, target, want string }{
		{"", "orchestrator", ""}, {"", "developer", ""},
		{"orchestrator", "developer", ""}, {"orchestrator", "reviewer", ""}, {"orchestrator", "advisor", ""},
		{"orchestrator", "orchestrator", "may only start developer, reviewer, advisor"},
		{"developer", "developer", "the host runs live harnez agent checks after your commit"}, {"reviewer", "developer", "the host runs live harnez agent checks after your commit"}, {"advisor", "reviewer", "the host runs live harnez agent checks after your commit"},
		{"", "wizard", "unknown agent role"}, {"wizard", "developer", "unknown agent role"},
	} {
		err := CheckSpawn(tc.caller, tc.target)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Fatalf("CheckSpawn(%q, %q) = %v, want %q", tc.caller, tc.target, err, tc.want)
		}
	}
	for role, leaf := range map[string]bool{"developer": true, "reviewer": true, "advisor": true, "orchestrator": false, "": false, "wizard": false} {
		if IsLeafRole(role) != leaf {
			t.Fatalf("IsLeafRole(%q) = %v, want %v", role, !leaf, leaf)
		}
	}
	for _, role := range names {
		if rules, err := RoleRules(role); err != nil || rules == "" {
			t.Fatalf("RoleRules(%q) = %q, %v", role, rules, err)
		}
	}
}

func TestParseAgentSpecRejectsBrokenRoles(t *testing.T) {
	base := "default_model: codex:luna:low\n" + testExternalSessionSpecYAML
	for name, doc := range map[string]string{
		"default role undefined": base + "default_role: nobody\nroles:\n  a: {spawns: [], rules: x}\n",
		"spawns undefined role":  base + "default_role: a\nroles:\n  a: {spawns: [b], rules: x}\n",
		"spawns itself":          base + "default_role: a\nroles:\n  a: {spawns: [a], rules: x}\n",
		"empty rules":            base + "default_role: a\nroles:\n  a: {spawns: [], rules: ' '}\n",
	} {
		if _, err := parseAgentSpec([]byte(doc)); err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
	if _, err := parseAgentSpec([]byte(base + "default_role: a\nroles:\n  a: {spawns: [], rules: fine}\n")); err != nil {
		t.Fatal(err)
	}
}

func TestParseAgentSpecDoesNotMutateModelAliases(t *testing.T) {
	before := KnownModels()
	if _, err := parseAgentSpec([]byte("default_model: custom:x:low\ndefault_role: a\n" + testExternalSessionSpecYAML + "models:\n  custom:x: {provider: custom, name: x, tier: low}\nroles:\n  a: {spawns: [], rules: fine}\n")); err != nil {
		t.Fatal(err)
	}
	after := KnownModels()
	if len(before) != len(after) {
		t.Fatalf("parse mutated aliases: before=%d after=%d", len(before), len(after))
	}
}

const testExternalSessionSpecYAML = `external_sessions:
  fixture:
    root: "{home}/sessions"
    pattern: "*.jsonl"
    format: jsonl
    record_type_path: type
    record_type: session_meta
    max_record_bytes: 1024
    active_match_window_seconds: 120
    name_prefix: fixture
    available_status: available
    fields: {id: payload.id, working_dir: payload.cwd, model_provider: payload.model_provider, source: payload.source, created_at: payload.timestamp}
`
