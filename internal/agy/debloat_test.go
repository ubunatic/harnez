package agy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func testAgyDebloatConfig() DebloatConfig {
	return DebloatConfig{
		MinimalDeny: []string{
			"schedule",
			"generate_image",
			"ask_question",
		},
		AggressiveExtraDeny: []string{
			"read_url_content",
			"search_web",
			"define_subagent",
			"manage_subagents",
		},
	}
}

func readSettings(t *testing.T, target string) map[string]any {
	t.Helper()
	settingsPath, _ := resolvePaths(target)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read %s: %v", settingsPath, err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse %s: %v", settingsPath, err)
	}
	return m
}

func denySlice(m map[string]any) []string {
	perms, _ := m["permissions"].(map[string]any)
	raw, _ := perms["deny"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func TestApplyDebloat_MinimalOnEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg := testAgyDebloatConfig()

	changed, err := ApplyDebloat(dir, cfg, "minimal")
	if err != nil {
		t.Fatalf("ApplyDebloat minimal: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first apply")
	}

	got := denySlice(readSettings(t, dir))
	want := []string{"ask_question", "generate_image", "schedule"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deny = %v, want %v", got, want)
	}

	// Idempotent
	changed, err = ApplyDebloat(dir, cfg, "minimal")
	if err != nil {
		t.Fatalf("ApplyDebloat minimal 2nd: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on second apply")
	}
}

func TestApplyDebloat_Aggressive(t *testing.T) {
	dir := t.TempDir()
	cfg := testAgyDebloatConfig()

	changed, err := ApplyDebloat(dir, cfg, "aggressive")
	if err != nil {
		t.Fatalf("ApplyDebloat aggressive: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	got := denySlice(readSettings(t, dir))
	want := []string{"ask_question", "define_subagent", "generate_image", "manage_subagents", "read_url_content", "schedule", "search_web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deny = %v, want %v", got, want)
	}
}

func TestApplyRevertDebloat_RoundTripWithPreExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	initial := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"run_command"},
			"deny":  []string{"custom_tool"},
		},
		"otherKey": "value",
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := testAgyDebloatConfig()
	if _, err := ApplyDebloat(dir, cfg, "minimal"); err != nil {
		t.Fatalf("ApplyDebloat: %v", err)
	}

	m := readSettings(t, dir)
	if m["otherKey"] != "value" {
		t.Fatalf("otherKey lost after apply: %v", m)
	}

	reverted, err := RevertDebloat(dir)
	if err != nil || !reverted {
		t.Fatalf("RevertDebloat: reverted=%v err=%v", reverted, err)
	}

	after := readSettings(t, dir)
	if after["otherKey"] != "value" {
		t.Fatalf("otherKey lost after revert: %v", after)
	}
	gotDeny := denySlice(after)
	if !reflect.DeepEqual(gotDeny, []string{"custom_tool"}) {
		t.Fatalf("deny after revert = %v, want [custom_tool]", gotDeny)
	}

	_, recordPath := resolvePaths(dir)
	if _, err := os.Stat(recordPath); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar record removed, stat err=%v", err)
	}
}

func TestApplyDebloat_SwitchPresets(t *testing.T) {
	dir := t.TempDir()
	cfg := testAgyDebloatConfig()

	if _, err := ApplyDebloat(dir, cfg, "aggressive"); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDebloat(dir, cfg, "minimal"); err != nil {
		t.Fatal(err)
	}

	got := denySlice(readSettings(t, dir))
	want := []string{"ask_question", "generate_image", "schedule"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deny after switching to minimal = %v, want %v", got, want)
	}
}

func TestStatusDebloat_Execution(t *testing.T) {
	dir := t.TempDir()
	cfg := testAgyDebloatConfig()

	if err := StatusDebloat(dir, cfg); err != nil {
		t.Fatalf("StatusDebloat empty: %v", err)
	}
	if _, err := ApplyDebloat(dir, cfg, "minimal"); err != nil {
		t.Fatal(err)
	}
	if err := StatusDebloat(dir, cfg); err != nil {
		t.Fatalf("StatusDebloat applied: %v", err)
	}
}
