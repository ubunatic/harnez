// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// testDebloatConfig loads the real embedded config.yaml so these tests
// exercise the actual spec content (docs/Spec.md: config.yaml is the single
// source of truth for debloat preset membership) instead of a duplicated
// literal list.
func testDebloatConfig(t *testing.T) DebloatConfig {
	t.Helper()
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}
	return cfg.Debloat
}

func TestDebloatCodexFeatureMembershipFromEmbeddedSpec(t *testing.T) {
	features := testDebloatConfig(t).CodexFeatures
	want := map[string]bool{"apps": false, "plugins": false}
	if !reflect.DeepEqual(features, want) {
		t.Fatalf("Codex debloat features = %v, want %v", features, want)
	}
}

func TestDebloatAgyMembershipFromEmbeddedSpec(t *testing.T) {
	agyCfg := testDebloatConfig(t).Agy
	wantMinimal := []string{"schedule", "generate_image", "ask_question"}
	if !reflect.DeepEqual(agyCfg.MinimalDeny, wantMinimal) {
		t.Fatalf("Agy minimal deny = %v, want %v", agyCfg.MinimalDeny, wantMinimal)
	}
	wantAggressive := []string{"read_url_content", "search_web", "define_subagent", "manage_subagents"}
	if !reflect.DeepEqual(agyCfg.AggressiveExtraDeny, wantAggressive) {
		t.Fatalf("Agy aggressive deny = %v, want %v", agyCfg.AggressiveExtraDeny, wantAggressive)
	}
}

func readSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	return m
}

func denyOf(m map[string]any) []string {
	perms, _ := m["permissions"].(map[string]any)
	var out []string
	for _, v := range denySlice(perms) {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func denySlice(perms map[string]any) []string {
	raw, _ := perms["deny"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestApplyDebloat_MinimalOnEmpty(t *testing.T) {
	dir := t.TempDir()

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	got := denyOf(readSettings(t, dir))
	want := []string{"DesignSync", "PushNotification", "RemoteTrigger"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deny = %v, want %v", got, want)
	}
	settings := readSettings(t, dir)
	if settings["disableBundledSkills"] != true {
		t.Fatalf("minimal preset disableBundledSkills = %v, want true", settings["disableBundledSkills"])
	}
}

func TestApplyDebloat_PresetBundledSkillsFollowsConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := testDebloatConfig(t)
	cfg.PresetDisableBundledSkills = false

	if err := ApplyDebloat(dir, cfg, DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	if _, present := settings["disableBundledSkills"]; present {
		t.Fatalf("minimal preset wrote disableBundledSkills despite config opt-out: %v", settings["disableBundledSkills"])
	}
}

func TestApplyDebloat_MinimalSkillOverrides(t *testing.T) {
	dir := t.TempDir()

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	overrides, _ := settings["skillOverrides"].(map[string]any)
	if got, want := len(overrides), 19; got != want {
		t.Fatalf("minimal skill override count = %d, want %d: %v", got, want, overrides)
	}
	if got := overrides["commit"]; got != "user-invocable-only" {
		t.Fatalf("commit override = %v, want user-invocable-only", got)
	}
	for _, name := range []string{"domain-modeling", "evergreen", "lmcoder"} {
		if got, present := overrides[name]; present {
			t.Fatalf("minimal preset unexpectedly overrides %s = %v", name, got)
		}
	}
}

func TestApplyDebloat_AggressiveSkillOverrides(t *testing.T) {
	dir := t.TempDir()

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetAggressive}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	overrides, _ := settings["skillOverrides"].(map[string]any)
	if got, want := len(overrides), 22; got != want {
		t.Fatalf("aggressive skill override count = %d, want %d: %v", got, want, overrides)
	}
	for _, name := range []string{"domain-modeling", "evergreen", "lmcoder"} {
		if got := overrides[name]; got != "user-invocable-only" {
			t.Fatalf("%s override = %v, want user-invocable-only", name, got)
		}
	}
}

func TestApplyDebloat_RejectsInvalidSkillOverrideMode(t *testing.T) {
	dir := t.TempDir()
	cfg := testDebloatConfig(t)
	cfg.PresetSkillOverrides = map[string]string{"commit": "sometimes"}

	err := ApplyDebloat(dir, cfg, DebloatOptions{Preset: DebloatPresetMinimal})
	if err == nil {
		t.Fatal("expected invalid skill override mode to fail")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(statErr) {
		t.Fatalf("settings.json should not be written on invalid config, stat err=%v", statErr)
	}
}

func TestApplyRevertDebloat_RestoresSkillOverrides(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	initial := map[string]any{
		"skillOverrides": map[string]any{
			"commit":       "on",
			"personal-one": "off",
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetAggressive}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}
	if err := RevertDebloat(dir); err != nil {
		t.Fatalf("RevertDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	overrides, _ := settings["skillOverrides"].(map[string]any)
	if got := overrides["commit"]; got != "on" {
		t.Fatalf("commit after revert = %v, want on", got)
	}
	if got := overrides["personal-one"]; got != "off" {
		t.Fatalf("unrelated override after revert = %v, want off", got)
	}
	if _, present := overrides["discovery"]; present {
		t.Fatalf("discovery should be absent after revert, got %v", overrides["discovery"])
	}
}

func TestApplyDebloat_StandaloneBundledSkillsToggle(t *testing.T) {
	dir := t.TempDir()
	cfg := testDebloatConfig(t)
	cfg.PresetDisableBundledSkills = false

	if err := ApplyDebloat(dir, cfg, DebloatOptions{DisableBundledSkills: true}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	if settings["disableBundledSkills"] != true {
		t.Fatalf("standalone disableBundledSkills = %v, want true", settings["disableBundledSkills"])
	}
}

func TestApplyDebloat_PreservesUnrelatedData(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	initial := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Bash(git:*)"},
			"deny":  []string{"SomeOtherTool"},
		},
		"model":       "opus",
		"customField": map[string]any{"nested": true},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	if settings["model"] != "opus" {
		t.Errorf("model field lost: %+v", settings)
	}
	cf, _ := settings["customField"].(map[string]any)
	if cf["nested"] != true {
		t.Errorf("customField.nested lost: %+v", settings)
	}
	perms, _ := settings["permissions"].(map[string]any)
	allow := denySlice(map[string]any{"deny": perms["allow"]})
	if len(allow) != 1 || allow[0] != "Bash(git:*)" {
		t.Errorf("permissions.allow lost: %+v", perms["allow"])
	}

	deny := denySlice(perms)
	sort.Strings(deny)
	want := []string{"DesignSync", "PushNotification", "RemoteTrigger", "SomeOtherTool"}
	if !reflect.DeepEqual(deny, want) {
		t.Fatalf("deny = %v, want %v (union, no duplicates, pre-existing kept)", deny, want)
	}
}

func TestRevertDebloat_LeavesPreExistingDenyEntry(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	initial := map[string]any{
		"permissions": map[string]any{"deny": []string{"DesignSync"}},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}
	if err := RevertDebloat(dir); err != nil {
		t.Fatalf("RevertDebloat: %v", err)
	}

	deny := denyOf(readSettings(t, dir))
	if !reflect.DeepEqual(deny, []string{"DesignSync"}) {
		t.Fatalf("deny after revert = %v, want pre-existing [DesignSync] preserved", deny)
	}
}

func TestRevertDebloat_RestoresPriorTrueToggle(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	initial := map[string]any{"disableArtifact": true}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{DisableArtifact: true}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}
	if err := RevertDebloat(dir); err != nil {
		t.Fatalf("RevertDebloat: %v", err)
	}

	settings := readSettings(t, dir)
	if settings["disableArtifact"] != true {
		t.Fatalf("disableArtifact after revert = %v, want true restored", settings["disableArtifact"])
	}
}

func TestRevertDebloat_RemovesToggleAbsentBefore(t *testing.T) {
	dir := t.TempDir()

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{DisableWorkflows: true}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}
	settings := readSettings(t, dir)
	if settings["disableWorkflows"] != true {
		t.Fatalf("expected disableWorkflows true after apply, got %v", settings["disableWorkflows"])
	}

	if err := RevertDebloat(dir); err != nil {
		t.Fatalf("RevertDebloat: %v", err)
	}
	settings = readSettings(t, dir)
	if _, present := settings["disableWorkflows"]; present {
		t.Fatalf("disableWorkflows should be absent after revert, got %v", settings["disableWorkflows"])
	}
}

func TestApplyRevertDebloat_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	initial := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Bash(git:*)"},
			"deny":  []string{"SomeOtherTool"},
		},
		"model": "opus",
	}
	before, _ := json.Marshal(initial)
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{
		Preset:       DebloatPresetAggressive,
		NotebookEdit: true,
		Cron:         true,
	}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}
	if err := RevertDebloat(dir); err != nil {
		t.Fatalf("RevertDebloat: %v", err)
	}

	after := readSettings(t, dir)
	var beforeMap map[string]any
	if err := json.Unmarshal(before, &beforeMap); err != nil {
		t.Fatalf("unmarshal before: %v", err)
	}
	if !reflect.DeepEqual(normalizeForCompare(beforeMap), normalizeForCompare(after)) {
		t.Fatalf("round trip mismatch:\nbefore=%+v\nafter=%+v", beforeMap, after)
	}
	if _, err := os.Stat(debloatRecordPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar record removed after revert, stat err=%v", err)
	}
}

// normalizeForCompare re-marshals through JSON so both sides use identical
// dynamic types ([]any, map[string]any) regardless of how they were built.
func normalizeForCompare(m map[string]any) map[string]any {
	data, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}

func TestApplyDebloat_AggressiveRequiresExplicitPreset(t *testing.T) {
	dir := t.TempDir()

	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	deny := denyOf(readSettings(t, dir))
	for _, tool := range testDebloatConfig(t).AggressiveExtraDeny {
		for _, d := range deny {
			if d == tool {
				t.Fatalf("minimal preset must not deny %q, got deny=%v", tool, deny)
			}
		}
	}
}

func TestRevertDebloat_NoRecordErrors(t *testing.T) {
	dir := t.TempDir()
	if err := RevertDebloat(dir); err == nil {
		t.Fatal("expected error reverting with no debloat record present")
	}
}

func TestStatusDebloat_RunsWithAndWithoutRecord(t *testing.T) {
	dir := t.TempDir()
	if err := StatusDebloat(dir, testDebloatConfig(t)); err != nil {
		t.Fatalf("StatusDebloat (no record): %v", err)
	}
	if err := ApplyDebloat(dir, testDebloatConfig(t), DebloatOptions{Preset: DebloatPresetMinimal, DisableArtifact: true}); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}
	if err := StatusDebloat(dir, testDebloatConfig(t)); err != nil {
		t.Fatalf("StatusDebloat (with record): %v", err)
	}
}
