package agy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHooksDoc(t *testing.T) {
	doc := BuildHooksDoc()
	if _, ok := doc["hooks"]; ok {
		t.Fatal("BuildHooksDoc should not have redundant outer 'hooks' wrapper")
	}
	entry, ok := doc[HookName].(map[string]any)
	if !ok {
		t.Fatalf("BuildHooksDoc missing top-level %q key", HookName)
	}
	if enabled, ok := entry["enabled"].(bool); !ok || !enabled {
		t.Errorf("enabled = %v, want true", entry["enabled"])
	}
	preToolUse, ok := entry["PreToolUse"].([]map[string]any)
	if !ok || len(preToolUse) == 0 {
		t.Fatalf("PreToolUse invalid or empty: %v", entry["PreToolUse"])
	}
	if matcher, _ := preToolUse[0]["matcher"].(string); matcher != "*" {
		t.Errorf("matcher = %q, want '*'", matcher)
	}
	hooks, ok := preToolUse[0]["hooks"].([]map[string]any)
	if !ok || len(hooks) == 0 {
		t.Fatalf("hooks invalid or empty: %v", preToolUse[0]["hooks"])
	}
	if cmd, _ := hooks[0]["command"].(string); cmd != "harnez hook agy" {
		t.Errorf("command = %q, want 'harnez hook agy'", cmd)
	}
	postToolUse, ok := entry["PostToolUse"].([]map[string]any)
	if !ok || len(postToolUse) != 1 {
		t.Fatalf("PostToolUse invalid: %v", entry["PostToolUse"])
	}
	if matcher, _ := postToolUse[0]["matcher"].(string); matcher != "*" {
		t.Errorf("PostToolUse matcher = %q, want '*'", matcher)
	}
	postHooks, ok := postToolUse[0]["hooks"].([]map[string]any)
	if !ok || len(postHooks) != 1 {
		t.Fatalf("PostToolUse hooks invalid: %v", postToolUse[0]["hooks"])
	}
	if cmd, _ := postHooks[0]["command"].(string); cmd != "harnez hook agy-post" {
		t.Errorf("PostToolUse command = %q, want 'harnez hook agy-post'", cmd)
	}
}

func TestApplyCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "hooks.json")

	changed, err := Apply(path)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first apply")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse written hooks.json: %v", err)
	}
	if _, ok := raw[HookName]; !ok {
		t.Fatalf("expected top-level %q key in written file: %s", HookName, string(data))
	}
	if _, ok := raw["hooks"]; ok {
		t.Fatalf("unexpected outer 'hooks' key in written file: %s", string(data))
	}

	installed, drifted := Status(path)
	if !installed || drifted {
		t.Fatalf("Status = installed=%v drifted=%v, want true/false", installed, drifted)
	}

	// idempotent: second apply is a no-op.
	changed, err = Apply(path)
	if err != nil {
		t.Fatalf("Apply (2nd): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on repeat apply")
	}
}

func TestApplyMigratesLegacyNestedSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	legacyJSON := `{
  "hooks": {
    "harnez": {
      "enabled": true,
      "PreToolUse": [
        {
          "matcher": "run_command",
          "hooks": [
            {"type": "command", "command": "harnez agy-hooks hook"}
          ]
        }
      ]
    }
  }
}`
	if err := os.WriteFile(path, []byte(legacyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Legacy format must be detected as installed and drifted
	installed, drifted := Status(path)
	if !installed || !drifted {
		t.Fatalf("Status on legacy format = installed=%v drifted=%v, want true/true", installed, drifted)
	}

	// Apply should migrate to top-level and clean up empty "hooks" wrapper
	changed, err := Apply(path)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true when migrating legacy format")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse migrated hooks.json: %v", err)
	}
	if _, ok := raw[HookName]; !ok {
		t.Fatalf("expected top-level %q key after migration: %s", HookName, string(data))
	}
	if _, ok := raw["hooks"]; ok {
		t.Fatalf("legacy 'hooks' wrapper should be removed when empty: %s", string(data))
	}

	installed, drifted = Status(path)
	if !installed || drifted {
		t.Fatalf("Status after migration = installed=%v drifted=%v, want true/false", installed, drifted)
	}
}

func TestApplyMigratesLegacyPreservingOtherHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	initial := `{
  "hooks": {
    "someone-elses-hook": {"enabled": true, "PostToolUse": []},
    "harnez": {"enabled": true}
  },
  "unrelatedTopLevelKey": "keep-me"
}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse hooks.json: %v", err)
	}

	if _, ok := raw[HookName]; !ok {
		t.Fatalf("expected top-level %q key", HookName)
	}
	if raw["unrelatedTopLevelKey"] != "keep-me" {
		t.Errorf("unrelated top-level key not preserved: %v", raw["unrelatedTopLevelKey"])
	}
	hooks, ok := raw["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("expected other hooks preserved under 'hooks', got: %v", raw["hooks"])
	}
	if _, ok := hooks["someone-elses-hook"]; !ok {
		t.Errorf("expected someone-elses-hook preserved")
	}
	if _, ok := hooks[HookName]; ok {
		t.Errorf("expected legacy harnez removed from 'hooks' map")
	}
}

func TestStatusDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	drifted := `{"harnez": {"enabled": false}}`
	if err := os.WriteFile(path, []byte(drifted), 0644); err != nil {
		t.Fatal(err)
	}

	installed, isDrifted := Status(path)
	if !installed {
		t.Fatal("expected installed=true")
	}
	if !isDrifted {
		t.Fatal("expected drifted=true after hand-editing the harnez entry")
	}
}

func TestStatusMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist", "hooks.json")
	installed, drifted := Status(path)
	if installed || drifted {
		t.Fatalf("Status on missing file = installed=%v drifted=%v, want false/false", installed, drifted)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	initial := `{
  "harnez": {"enabled": true},
  "hooks": {
    "someone-elses-hook": {"enabled": true}
  },
  "unrelatedTopLevelKey": "keep-me"
}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := Remove(path)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	installed, _ := Status(path)
	if installed {
		t.Fatal("expected harnez entry removed")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "someone-elses-hook") || !strings.Contains(got, "unrelatedTopLevelKey") {
		t.Errorf("expected unrelated content preserved after Remove, got:\n%s", got)
	}
	if strings.Contains(got, HookName) {
		t.Errorf("expected harnez removed from file, got:\n%s", got)
	}

	// second remove is a no-op
	changed, err = Remove(path)
	if err != nil {
		t.Fatalf("Remove (2nd): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on repeat remove")
	}
}

func TestRemoveLegacyNested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	initial := `{
  "hooks": {
    "harnez": {"enabled": true}
  }
}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := Delete(path)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed once empty, stat err=%v", err)
	}
}

func TestRemoveDeletesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed once empty, stat err=%v", err)
	}
}
