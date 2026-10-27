package subagent

import (
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
