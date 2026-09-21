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
	for _, alias := range []string{"luna", "sol", "astra", "haiku", "sonnet", "opus", "flash"} {
		if _, err := ResolveModel(alias); err != nil {
			t.Errorf("alias %q: %v", alias, err)
		}
	}
	if _, err := ResolveModel("unknown"); err == nil {
		t.Fatal("unknown alias unexpectedly resolved")
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
