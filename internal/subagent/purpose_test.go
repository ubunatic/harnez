package subagent

import (
	"fmt"
	"strings"
	"testing"
)

func TestRoleModelResolution(t *testing.T) {
	for _, tc := range []struct{ role, pick, target string }{
		{"dev", "lean", "terra:low"}, {"dev", "std", "terra:med"}, {"dev", "smart", "sol:med"},
		{"dev", "tui", "sol:med"}, {"dev", "gui", "terra:low"}, {"dev", "cheap", "luna:med"},
		{"dev", "fast", "haiku:med"}, {"review", "smart", "sol:low"}, {"review", "", "sol:low"},
		{"", "smart", "sol:med"}, {"developer", "smart", "sol:med"}, {"advisor", "cheap", "luna:low"}, {"advisor", "smart", "opus:low"},
		{"orchestrator", "", "codex:luna:high"}, {"review", "codex:sol:med", "codex:sol:med"}, {"review", "sol", "sol"},
	} {
		t.Run(tc.role+"/"+tc.pick, func(t *testing.T) {
			got, err := ResolveModelForRole(tc.role, tc.pick)
			want, wantErr := ResolveModel(tc.target)
			if err != nil || wantErr != nil || got != want {
				t.Fatalf("got %+v %v, want %+v %v", got, err, want, wantErr)
			}
		})
	}
	for _, tc := range []struct{ role, pick, want string }{
		{"review", "lean", "known traits"}, {"developer", "nonexistent", "known traits"}, {"developer", "ai", "unset"},
	} {
		if _, err := ResolveModelForRole(tc.role, tc.pick); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s/%s: %v, want %s", tc.role, tc.pick, err, tc.want)
		}
	}
}

func TestCanonicalRoleModelResolution(t *testing.T) {
	got, err := ResolveModelForCanonicalRole("developer", "smart")
	want, wantErr := ResolveModelForRole("dev", "smart")
	if err != nil || wantErr != nil || got != want {
		t.Fatalf("canonical=%+v %v alias=%+v %v", got, err, want, wantErr)
	}
	if _, err := ResolveModelForCanonicalRole("dev", "smart"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("canonical rejects alias: %v", err)
	}
}

func TestRoleSelectionOfficialModelDiagnostics(t *testing.T) {
	for _, name := range []string{"gemini-4-argon", "gemini-4-argon:low"} {
		selection, err := ResolveModelWithAvailability("advisor", name, nil, false)
		if err != nil || selection.Model.Provider != "agy" || selection.Model.Name != "gemini-4-argon" || selection.Model.Tier != "low" {
			t.Errorf("selection for %q = %+v, %v", name, selection, err)
		}
	}
	_, err := ResolveModelWithAvailability("advisor", "gemini-4-argom", nil, false)
	if err == nil || !strings.Contains(err.Error(), "known traits:") || !strings.Contains(err.Error(), "close matches: gemini-4-argon") {
		t.Fatalf("unknown model diagnostics = %v", err)
	}
	spec := agentSpec{Roles: map[string]RoleSpec{"advisor": {Models: map[string][]string{"smart": {"first"}}}}}
	aliases := map[string]modelAlias{
		"a:first":  {Model: Model{Provider: "a", Name: "shared-model", Tier: "low"}},
		"b:second": {Model: Model{Provider: "b", Name: "shared-model", Tier: "low"}},
	}
	_, err = resolveCanonicalRoleModelIn(spec, "advisor", "shared-model", aliases)
	if err == nil || !strings.Contains(err.Error(), "known traits: smart") || !strings.Contains(err.Error(), "candidates: a:first, b:second") {
		t.Fatalf("ambiguous model diagnostics = %v", err)
	}
}

func TestOfficialNameInModelPick(t *testing.T) {
	spec := agentSpec{Roles: map[string]RoleSpec{"advisor": {Models: map[string][]string{"smart": {"gemini-4-argon"}}}}}
	aliases, err := modelAliasMap()
	if err != nil {
		t.Fatal(err)
	}
	picks := modelPicksIn(spec, func(role, trait string) (Model, error) {
		selection, err := resolveRoleSelection(spec, role, trait, func(target string) (Model, error) {
			return resolveModelIn(aliases, target)
		}, func(Model) string { return "" })
		return selection.Model, err
	})
	if len(picks) != 1 || picks[0].Resolved != "agy:argon:low" || picks[0].Error != "" {
		t.Fatalf("picks = %+v", picks)
	}
}

func TestModelPickErrors(t *testing.T) {
	spec := agentSpec{Roles: map[string]RoleSpec{"developer": {Models: map[string][]string{"ai": {}, "smart": {"sol:med"}}}}}
	picks := modelPicksIn(spec, func(role, trait string) (Model, error) { return Model{}, fmt.Errorf("model disabled by user policy") })
	if len(picks) != 2 || !strings.Contains(picks[0].Error, "unset") || !strings.Contains(picks[1].Error, "disabled") {
		t.Fatalf("picks=%+v", picks)
	}
}

func TestRoleModelValidation(t *testing.T) {
	base, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*agentSpec)
	}{
		{"cycle", "cycle", func(s *agentSpec) {
			r := s.Roles["developer"]
			r.Models["lean"] = []string{"std"}
			r.Models["std"] = []string{"lean"}
		}},
		{"depth", "depth exceeds 4", func(s *agentSpec) {
			r := s.Roles["developer"]
			r.Models["lean"] = []string{"std"}
			r.Models["std"] = []string{"smart"}
			r.Models["smart"] = []string{"tui"}
			r.Models["tui"] = []string{"cheap"}
		}},
		{"tier", "invalid trait", func(s *agentSpec) { s.ModelTraits["low"] = "x" }},
		{"synonym", "invalid trait", func(s *agentSpec) { s.ModelTraits["medium"] = "x" }},
		{"model alias", "collides", func(s *agentSpec) { s.ModelTraits["sol"] = "x" }},
		{"provider", "collides", func(s *agentSpec) { s.ModelTraits["codex"] = "x" }},
		{"role", "collides", func(s *agentSpec) { s.ModelTraits["developer"] = "x" }},
		{"duplicate", "duplicate", func(s *agentSpec) { s.Roles["developer"].Models["lean"] = []string{"sol:med", "sol:med"} }},
		{"unknown", "target", func(s *agentSpec) { s.Roles["developer"].Models["lean"] = []string{"no-such-model"} }},
		{"missing default", "default_model", func(s *agentSpec) { r := s.Roles["developer"]; r.DefaultModel = "missing"; s.Roles["developer"] = r }},
		{"bad name", "invalid trait", func(s *agentSpec) { s.ModelTraits["Bad"] = "x" }},
		{"missing trait", "unknown trait", func(s *agentSpec) { s.Roles["developer"].Models["missing"] = []string{"sol:med"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.ModelTraits = map[string]string{}
			for k, v := range base.ModelTraits {
				s.ModelTraits[k] = v
			}
			s.Roles = map[string]RoleSpec{}
			for k, v := range base.Roles {
				v.Models = map[string][]string{}
				for n, targets := range base.Roles[k].Models {
					v.Models[n] = append([]string{}, targets...)
				}
				s.Roles[k] = v
			}
			tc.change(&s)
			if err := validateRoleModels(s, s.Models); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
		})
	}
}
