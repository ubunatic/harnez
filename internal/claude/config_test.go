// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

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
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}
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
