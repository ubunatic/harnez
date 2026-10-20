package assess

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestEmbeddedRAMPSpec(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/ramp.yaml")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := parseRAMPSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if RAMPRuleSetVersion != spec.Version {
		t.Fatal("version is not consumed")
	}
	levels := []RAMPLevel{RAMPLevelL1, RAMPLevelL2, RAMPLevelL3, RAMPLevelL4}
	for i, key := range []string{"l1", "l2", "l3", "l4"} {
		if string(levels[i]) != spec.Levels[key] || LevelDescription(levels[i]) != spec.Descriptions.Levels[key] {
			t.Errorf("level %s is not consumed", key)
		}
	}
	paths := map[string]string{
		"unconfigured": "agent_helper.sh", "ai_rules": "AGENTS.md", "tool_config": ".claude/settings.json",
		"agent_standards": "docs/AgenticLoop.md", "named_agents": ".claude/agents/helper.md",
		"reusable_commands": ".claude/commands/check.md", "domain_skills": "skills/check/SKILL.md",
		"multi_agent_flows": "flows/team.yaml", "session_records": "agent-sessions/run.jsonl",
	}
	for key, path := range paths {
		art := classifyRAMPPath(path, []byte("# Skill\nagents: []"))
		cat := spec.Categories[key]
		if art == nil {
			t.Fatalf("no classification for %s", key)
		}
		if string(art.Category) != cat.ID || string(art.Level) != cat.Level || CategoryDescription(art.Category) != spec.Descriptions.Categories[key] {
			t.Errorf("category %s is not consumed: %+v", key, art)
		}
	}
	for _, key := range []string{"high", "medium", "low", "unknown"} {
		if spec.Confidence[key] == "" {
			t.Errorf("missing confidence %s", key)
		}
	}
	for key, value := range map[string]RAMPConfidence{"high": ConfidenceHigh, "unknown": ConfidenceUnknown} {
		if string(value) != spec.Confidence[key] {
			t.Errorf("confidence %s is not consumed", key)
		}
	}
	for key, value := range map[string]ArtifactStatus{"committed": StatusCommitted, "uncommitted": StatusUncommitted, "projected": StatusProjected} {
		if string(value) != spec.ArtifactStatuses[key] {
			t.Errorf("status %s is not consumed", key)
		}
	}
}

func TestRAMPSpecValidation(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/ramp.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"unknown level", func(s map[string]any) {
			s["categories"].(map[string]any)["ai_rules"].(map[string]any)["level"] = "missing"
		}, "unknown level"},
		{"duplicate category", func(s map[string]any) {
			categories := s["categories"].(map[string]any)
			categories["ai_rules"].(map[string]any)["id"] = categories["unconfigured"].(map[string]any)["id"]
		}, "duplicate"},
		{"duplicate level", func(s map[string]any) {
			levels := s["levels"].(map[string]any)
			levels["l2"] = levels["l1"]
		}, "duplicate"},
		{"duplicate confidence", func(s map[string]any) {
			confidence := s["confidence"].(map[string]any)
			confidence["low"] = confidence["high"]
		}, "duplicate"},
		{"duplicate status", func(s map[string]any) {
			statuses := s["artifact_statuses"].(map[string]any)
			statuses["projected"] = statuses["committed"]
		}, "duplicate"},
		{"missing description", func(s map[string]any) {
			delete(s["descriptions"].(map[string]any)["categories"].(map[string]any), "ai_rules")
		}, "required"},
		{"empty title", func(s map[string]any) { s["descriptions"].(map[string]any)["levels"].(map[string]any)["l1"] = "" }, "length"},
		{"missing level", func(s map[string]any) { delete(s["levels"].(map[string]any), "l1") }, "required"},
		{"missing confidence", func(s map[string]any) { delete(s["confidence"].(map[string]any), "high") }, "required"},
		{"unknown field", func(s map[string]any) { s["extra"] = true }, "additional"},
		{"wrong type", func(s map[string]any) { s["version"] = 42 }, "string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var s map[string]any
			if err := yaml.Unmarshal(data, &s); err != nil {
				t.Fatal(err)
			}
			tc.mutate(s)
			mutated, err := yaml.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseRAMPSpec(mutated); err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
	if _, err := parseRAMPSpec([]byte("version: [")); err == nil {
		t.Fatal("malformed YAML accepted")
	}
	if _, err := parseRAMPSpec(append(data, []byte("\nversion: duplicate\n")...)); err == nil {
		t.Fatal("duplicate YAML key accepted")
	}
}
