package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDebloatGatingMembership(t *testing.T) {
	cfg := testDebloatConfig(t)
	if !cfg.PresetDisableClaudeAiConnectors {
		t.Fatal("presets must disable account connectors")
	}
	if !reflect.DeepEqual(cfg.PresetDeniedMCPServers, []string{"harnez", "codebase-memory-mcp"}) {
		t.Fatalf("MCP membership = %v", cfg.PresetDeniedMCPServers)
	}
	for _, name := range []string{"gokay-sweep", "sitegen-sweep", "reverse-sprint", "epic", "lean-sprint-v1", "gokay", "sitegen", "publish"} {
		if cfg.PresetSkillOverrides[name] != "user-invocable-only" {
			t.Errorf("%s must be invoke-only", name)
		}
	}
	for _, name := range []string{
		"google-workspace", "xlsx", "pptx", "computer-use", "built-in-browser",
		"artifact-emulator", "deep-research", "mcp-builder", "slack-gif-creator",
		"import-memory", "plugin-authoring", "internal-comms", "canvas-design",
		"web-artifacts-builder", "chrome-browser", "codebase-memory",
		"codebase-memory-scout", "codebase-memory-auditor",
	} {
		if cfg.PresetSkillOverrides[name] != "off" {
			t.Errorf("%s must be off", name)
		}
	}
	if _, gated := cfg.PresetSkillOverrides["tool-feedback-protocol"]; gated {
		t.Fatal("tool-feedback-protocol must remain available")
	}
	for _, tool := range append(cfg.MinimalDeny, cfg.AggressiveExtraDeny...) {
		if tool == "ToolSearch" {
			t.Fatal("ToolSearch must remain available")
		}
	}
	for _, preset := range []string{DebloatPresetMinimal, DebloatPresetAggressive} {
		t.Run(preset, func(t *testing.T) {
			dir := t.TempDir()
			if err := ApplyDebloat(dir, cfg, DebloatOptions{Preset: preset}); err != nil {
				t.Fatal(err)
			}
			settings := readSettings(t, dir)
			if settings["disableClaudeAiConnectors"] != true {
				t.Fatal("connector toggle not applied")
			}
			overrides := settings["skillOverrides"].(map[string]any)
			for name, mode := range cfg.PresetSkillOverrides {
				if overrides[name] != mode {
					t.Errorf("%s override = %v, want %s", name, overrides[name], mode)
				}
			}
			entries, err := mcpServerDenials(settings)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range cfg.PresetDeniedMCPServers {
				if !hasMCPServerDenial(entries, name) {
					t.Errorf("missing server denial %s", name)
				}
			}
		})
	}
}

func TestDebloatMCPAndConnectorsRoundTrip(t *testing.T) {
	for _, initial := range []string{
		`{}`,
		`{"deniedMcpServers":[],"disableClaudeAiConnectors":false}`,
		`{"deniedMcpServers":[{"serverName":"harnez"},{"serverCommand":["custom","mcp"]},{"serverUrl":"https://example.com/*"}],"disableClaudeAiConnectors":true}`,
	} {
		t.Run(initial, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := testDebloatConfig(t)
			for pass := 0; pass < 2; pass++ {
				before, _ := os.ReadFile(path)
				recordBefore, _ := os.ReadFile(debloatRecordPath(dir))
				if err := ApplyDebloat(dir, cfg, DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
					t.Fatal(err)
				}
				if pass == 1 {
					after, _ := os.ReadFile(path)
					recordAfter, _ := os.ReadFile(debloatRecordPath(dir))
					if !bytes.Equal(before, after) || !bytes.Equal(recordBefore, recordAfter) {
						t.Fatal("repeated apply changed settings or ownership record")
					}
				}
			}
			if err := RevertDebloat(dir); err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(initial), &want); err != nil {
				t.Fatal(err)
			}
			if got := readSettings(t, dir); !reflect.DeepEqual(got, want) {
				t.Fatalf("revert = %v, want %v", got, want)
			}
		})
	}
}

func TestDebloatGatingRequiresPresetAndFollowsConfig(t *testing.T) {
	for _, preset := range []string{"", DebloatPresetMinimal} {
		dir := t.TempDir()
		cfg := testDebloatConfig(t)
		if preset != "" {
			cfg.PresetDisableClaudeAiConnectors = false
			cfg.PresetDeniedMCPServers = nil
		}
		if err := ApplyDebloat(dir, cfg, DebloatOptions{Preset: preset}); err != nil {
			t.Fatal(err)
		}
		settings := readSettings(t, dir)
		for _, key := range []string{"disableClaudeAiConnectors", "deniedMcpServers"} {
			if _, present := settings[key]; present {
				t.Errorf("%s written without preset/config opt-in", key)
			}
		}
		if preset == "" {
			if _, present := settings["skillOverrides"]; present {
				t.Fatal("plain options must not gate skills")
			}
		}
	}
	dir := t.TempDir()
	if err := ApplyDebloat(dir, DebloatConfig{}, DebloatOptions{DisableClaudeAiConnectors: true}); err != nil {
		t.Fatal(err)
	}
	if readSettings(t, dir)["disableClaudeAiConnectors"] != true {
		t.Fatal("standalone connector flag must still work")
	}
}

func TestMCPDenialRevertPreservesLaterEntries(t *testing.T) {
	settings := map[string]any{}
	rec := debloatRecord{}
	if err := applyMCPServerDenials(settings, &rec, []string{"harnez"}); err != nil {
		t.Fatal(err)
	}
	later := map[string]any{"serverName": "personal"}
	settings["deniedMcpServers"] = append(settings["deniedMcpServers"].([]any), later)
	if err := revertMCPServerDenials(settings, rec); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings["deniedMcpServers"], []any{later}) {
		t.Fatalf("lost later denial: %v", settings)
	}
}

func TestMCPDenialRejectsInvalidExistingValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	initial := []byte(`{"deniedMcpServers":"invalid"}`)
	if err := os.WriteFile(path, initial, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal}); err == nil {
		t.Fatal("expected refusal to overwrite malformed existing value")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, initial) {
		t.Fatalf("existing settings changed: %s, %v", got, err)
	}
}
