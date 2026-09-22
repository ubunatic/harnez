package subagent

import (
	"strings"
	"testing"
)

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
}

func TestParseAgentSpecRejectsBadDefault(t *testing.T) {
	for _, data := range []string{"{}", "default_model: codex:missing:low\n"} {
		if _, err := parseAgentSpec([]byte(data)); err == nil {
			t.Fatalf("parse %q unexpectedly succeeded", data)
		}
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
		"sol":            "gpt-6-sol",
		"codex:sol":      "gpt-6-sol",
		"codex:sol:low":  "gpt-6-sol",
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
	// claude:sonnet/claude:opus and agy:sonnet/agy:opus share the same bare
	// alias, so it must require the provider prefix.
	for _, alias := range []string{"sonnet", "opus"} {
		if _, err := ResolveModel(alias); err == nil {
			t.Errorf("alias %q: expected ambiguity error, got none", alias)
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
		{"developer", "developer", "is a leaf worker"}, {"reviewer", "developer", "is a leaf worker"}, {"advisor", "reviewer", "is a leaf worker"},
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
	base := "default_model: codex:luna:low\n"
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
	if _, err := parseAgentSpec([]byte("default_model: custom:x:low\ndefault_role: a\nmodels:\n  custom:x: {provider: custom, name: x, tier: low}\nroles:\n  a: {spawns: [], rules: fine}\n")); err != nil {
		t.Fatal(err)
	}
	after := KnownModels()
	if len(before) != len(after) {
		t.Fatalf("parse mutated aliases: before=%d after=%d", len(before), len(after))
	}
}
