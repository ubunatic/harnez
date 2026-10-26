// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestDebloatConfigSchema(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/schemas/config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		debloat string
		valid   bool
	}{
		{"valid", `{"preset_disable_claude_ai_connectors":true,"preset_denied_mcp_servers":["harnez"],"preset_skill_overrides":{"epic":"user-invocable-only","xlsx":"off"}}`, true},
		{"connector type", `{"preset_disable_claude_ai_connectors":"true"}`, false},
		{"server type", `{"preset_denied_mcp_servers":[1]}`, false},
		{"empty server", `{"preset_denied_mcp_servers":[""]}`, false},
		{"blank server", `{"preset_denied_mcp_servers":[" "]}`, false},
		{"duplicate server", `{"preset_denied_mcp_servers":["harnez","harnez"]}`, false},
		{"override mode", `{"preset_skill_overrides":{"epic":"sometimes"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var debloat map[string]any
			if err := json.Unmarshal([]byte(tc.debloat), &debloat); err != nil {
				t.Fatal(err)
			}
			err := resolved.Validate(map[string]any{"debloat": debloat})
			if (err == nil) != tc.valid {
				t.Fatalf("validation = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}

func TestLanguageSourceFor(t *testing.T) {
	cases := []struct {
		name    string
		lang    Language
		variant string
		want    string
	}{
		{"lite set, variant lite", Language{Source: "full.md", LiteSource: "lite.md"}, "lite", "lite.md"},
		{"lite set, variant full", Language{Source: "full.md", LiteSource: "lite.md"}, "full", "full.md"},
		{"lite unset, variant lite falls back", Language{Source: "full.md"}, "lite", "full.md"},
		{"lite unset, variant full", Language{Source: "full.md"}, "full", "full.md"},
		{"lite unset, empty variant", Language{Source: "full.md"}, "", "full.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.lang.SourceFor(c.variant)
			if got != c.want {
				t.Errorf("SourceFor(%q) = %q, want %q", c.variant, got, c.want)
			}
		})
	}
}

func TestConfigComponentSelectionRoundTrip(t *testing.T) {
	want := Config{
		ComponentNames: []string{"docs-only", "telemetry"},
		Skills:         []Command{{Name: "example", Requires: []string{"agents", "telemetry"}}},
	}

	data, err := yaml.Marshal(want)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got.ComponentNames, want.ComponentNames) {
		t.Fatalf("component names = %v, want %v", got.ComponentNames, want.ComponentNames)
	}
	if !reflect.DeepEqual(got.Skills[0].Requires, want.Skills[0].Requires) {
		t.Fatalf("skill requirements = %v, want %v", got.Skills[0].Requires, want.Skills[0].Requires)
	}
}

func TestEmbeddedConfigToolFeedbackRequiresTelemetry(t *testing.T) {
	cfg := loadTestConfig(t)
	for _, skill := range cfg.Skills {
		if skill.Name == "tool-feedback-protocol" {
			if !reflect.DeepEqual(skill.Requires, []string{"telemetry"}) {
				t.Fatalf("tool-feedback-protocol requires = %v, want [telemetry]", skill.Requires)
			}
			return
		}
	}
	t.Fatal("tool-feedback-protocol skill not found")
}
