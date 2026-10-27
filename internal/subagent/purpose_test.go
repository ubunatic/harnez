package subagent

import (
	"strings"
	"testing"
)

func TestPurposeResolution(t *testing.T) {
	for alias, target := range map[string]string{
		"dev:lean": "terra:low", "dev:std": "terra:med", "dev:smart": "sol:med",
		"dev:tui": "sol:med", "dev:gui": "terra:low", "dev:cheap": "luna:med",
		"dev:fast": "haiku:med", "search:cheap": "luna:low", "review:smart": "sol:low",
	} {
		t.Run(alias, func(t *testing.T) {
			got, err := ResolveModel(alias)
			want, wantErr := ResolveModel(target)
			if err != nil || wantErr != nil || got != want {
				t.Fatalf("%s = %+v, %v; want %+v, %v", alias, got, err, want, wantErr)
			}
		})
	}
	if _, err := ResolveModel("dev:ai"); err == nil || !strings.Contains(err.Error(), "unset") {
		t.Fatalf("unset alias error = %v", err)
	}
	for _, name := range []string{"codex:sol:med", "sol"} {
		if _, err := ResolveModel(name); err != nil {
			t.Errorf("existing %s: %v", name, err)
		}
	}
}

func TestPurposeValidation(t *testing.T) {
	base, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(*agentSpec, map[string]modelAlias)
	}{
		{"cycle", func(s *agentSpec, _ map[string]modelAlias) {
			s.PurposeAliases["dev:lean"] = []string{"dev:std"}
			s.PurposeAliases["dev:std"] = []string{"dev:lean"}
		}},
		{"depth five", func(s *agentSpec, _ map[string]modelAlias) {
			s.PurposeAliases["dev:lean"] = []string{"dev:std"}
			s.PurposeAliases["dev:std"] = []string{"dev:smart"}
			s.PurposeAliases["dev:smart"] = []string{"dev:tui"}
			s.PurposeAliases["dev:tui"] = []string{"dev:cheap"}
		}},
		{"tier", func(s *agentSpec, _ map[string]modelAlias) { s.PurposeAliases["dev:low"] = []string{"sol:med"} }},
		{"tier synonym", func(s *agentSpec, _ map[string]modelAlias) { s.PurposeAliases["dev:medium"] = []string{"sol:med"} }},
		{"unknown", func(s *agentSpec, _ map[string]modelAlias) { s.PurposeAliases["dev:lean"] = []string{"no-such-model"} }},
		{"duplicate", func(s *agentSpec, _ map[string]modelAlias) {
			s.PurposeAliases["dev:lean"] = []string{"sol:med", "sol:med"}
		}},
		{"bad name", func(s *agentSpec, _ map[string]modelAlias) { s.PurposeAliases["dev:Bad"] = []string{"sol:med"} }},
		{"provider collision", func(_ *agentSpec, m map[string]modelAlias) {
			m["dev:foo"] = modelAlias{Model: Model{Provider: "dev", Name: "foo"}}
		}},
		{"model alias collision", func(_ *agentSpec, m map[string]modelAlias) {
			m["codex:fake"] = modelAlias{Model: Model{Provider: "codex", Name: "fake"}, Aliases: []string{"dev"}}
		}},
		{"role alias collision", func(s *agentSpec, _ map[string]modelAlias) {
			r := s.Roles["developer"]
			r.Aliases = append(r.Aliases, "dev")
			s.Roles["developer"] = r
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.PurposeAliases = make(map[string][]string)
			for k, v := range base.PurposeAliases {
				s.PurposeAliases[k] = append([]string{}, v...)
			}
			s.Roles = make(map[string]RoleSpec)
			for k, v := range base.Roles {
				v.Aliases = append([]string{}, v.Aliases...)
				s.Roles[k] = v
			}
			models := make(map[string]modelAlias)
			for k, v := range base.Models {
				models[k] = v
			}
			tc.change(&s, models)
			if err := validatePurpose(s, models); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
