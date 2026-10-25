package modelcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromDirsWithoutMachineFilesUsesDefaults(t *testing.T) {
	stateDir := t.TempDir()
	configDir := t.TempDir()
	catalog, err := LoadFromDirs(stateDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) == 0 {
		t.Fatal("embedded defaults produced no models")
	}
	if catalog.Policy != nil {
		t.Fatal("missing policy unexpectedly produced a policy")
	}
	for _, model := range catalog.Models {
		if model.Identity.Harness == "" || model.Identity.Model == "" || !model.Enabled {
			t.Fatalf("invalid default model: %#v", model)
		}
	}
}

func TestLoadFromDirsMergesStateAndPolicy(t *testing.T) {
	stateDir := t.TempDir()
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(stateDir, "catalog.yaml"), `version: 1
models:
  - identity: {harness: codex, provider: codex, model: gpt-6-luna, version: gpt-6-luna, tier: low}
    aliases: [fast-luna]
    facts:
      capabilities:
        - value: tool use
          source: user-declared
          observed_at: "2026-10-25T10:00:00Z"
          evidence: [owner]
  - identity: {harness: custom, provider: local, model: private, version: "1"}
    aliases: [private]
    facts: {}`)
	writeCatalogFile(t, filepath.Join(stateDir, "access.yaml"), `version: 1
observed_at: "2026-10-25T10:00:00Z"
complete: false
coverage: []
observations:
  - identity: {harness: codex, provider: codex, model: gpt-6-luna, version: gpt-6-luna, tier: low}
    status: unknown
    method: not-probed
    facts: []`)
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), `version: 1
default_enabled: false
enabled: [codex:luna:low]
models:
  codex:luna:low:
    kind: patch
    fields:
      purpose: null
  local:private:
    kind: custom
    identity: {harness: custom, provider: local, model: private, version: "1"}
    enabled: true`)

	catalog, err := LoadFromDirs(stateDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	var luna *Model
	var private *Model
	for _, model := range catalog.Models {
		if model.Identity.Model == "gpt-6-luna" {
			copy := model
			luna = &copy
		}
		if model.Identity.Model == "private" {
			copy := model
			private = &copy
		}
	}
	if luna == nil || len(luna.Facts["capabilities"].([]any)) != 1 || !contains(luna.Aliases, "fast-luna") {
		t.Fatalf("state facts or aliases were not merged: %#v", luna)
	}
	if luna == nil || len(luna.Access) != 1 || luna.Access[0].Status != "unknown" || !luna.Enabled {
		t.Fatalf("unknown access changed enablement: %#v", luna)
	}
	if luna == nil || luna.Overrides["purpose"] != nil || luna.Policy == nil {
		t.Fatalf("policy patch was not retained: %#v", luna)
	}
	if private == nil || private.Policy == nil || private.Policy.Kind != "custom" || !private.Enabled {
		t.Fatalf("custom policy model missing: %#v", private)
	}
	if catalog.AccessComplete {
		t.Fatal("incomplete access data was reported complete")
	}
}

func TestLoadRejectsInvalidPolicyWithoutFallingBack(t *testing.T) {
	stateDir := t.TempDir()
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), `version: 1
default_enabled: true
enabled: []
models: {}
unknown: true`)
	_, err := LoadFromDirs(stateDir, configDir)
	if err == nil || !strings.Contains(err.Error(), "models.yaml") || !strings.Contains(err.Error(), "additional properties") {
		t.Fatalf("invalid policy error = %v", err)
	}
}

func TestLoadRejectsPolicyPinAgainstInheritedIdentity(t *testing.T) {
	stateDir := t.TempDir()
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), `version: 1
default_enabled: true
enabled: []
models:
  codex:luna:low:
    kind: patch
    version_pin: stale-version`)
	_, err := LoadFromDirs(stateDir, configDir)
	if err == nil || !strings.Contains(err.Error(), "does not match catalog identity.version") {
		t.Fatalf("inherited version pin error = %v", err)
	}
}

func TestLoadRejectsConflictingEnabledAliases(t *testing.T) {
	stateDir := t.TempDir()
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), `version: 1
default_enabled: true
enabled: []
models:
  codex:luna:low: {kind: patch, enabled: false}
  codex:gpt-6-luna:low: {kind: patch, enabled: true}`)
	_, err := LoadFromDirs(stateDir, configDir)
	if err == nil || !strings.Contains(err.Error(), "conflicting enabled values") {
		t.Fatalf("conflicting policy error = %v", err)
	}
}

func TestLoadUsesIsolatedXDGDirectories(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	stateHome := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	for _, name := range []string{"HARNEZ_MODELS", "HARNEZ_MODEL_POLICY", "HARNEZ_STATE_HOME", "HARNEZ_CONFIG_HOME"} {
		t.Setenv(name, "")
	}
	configDir := filepath.Join(configHome, "harnez")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), `version: 1
default_enabled: false
enabled: []
models: {}`)
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Policy == nil || catalog.Policy.DefaultEnabled {
		t.Fatalf("XDG_CONFIG_HOME policy not loaded: %#v", catalog.Policy)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "harnez", "models.yaml")); !os.IsNotExist(err) {
		t.Fatalf("loader consulted non-XDG config: stat error = %v", err)
	}
}

func writeCatalogFile(t *testing.T, filename, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
