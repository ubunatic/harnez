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
	models := catalog.Models()
	if len(models) == 0 {
		t.Fatal("embedded defaults produced no models")
	}
	if catalog.UserPolicy() != nil {
		t.Fatal("missing policy unexpectedly produced a policy")
	}
	for _, model := range models {
		if model.Identity.Harness == "" || model.Identity.Model == "" || !catalog.IsEnabled(model.Identity.Harness, model.Identity.Model, model.Identity.Tier) {
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
    observed_at: "2026-10-25T10:00:00Z"
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
	for _, model := range catalog.Models() {
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
	if luna == nil || len(luna.Access) != 1 || luna.Access[0].Status != "unknown" || !catalog.IsEnabled("codex", "gpt-6-luna", "low") {
		t.Fatalf("unknown access changed enablement: %#v", luna)
	}
	if luna == nil || luna.Overrides["purpose"] != nil || luna.Policy == nil {
		t.Fatalf("policy patch was not retained: %#v", luna)
	}
	if private == nil || private.Policy == nil || private.Policy.Kind != "custom" || !catalog.IsEnabled("local", "private", "low") {
		t.Fatalf("custom policy model missing: %#v", private)
	}
	if catalog.AccessComplete() {
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

func TestLoadRejectsTierlessTieredAliasConflict(t *testing.T) {
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), `version: 1
default_enabled: false
enabled: []
models:
  codex:luna: {kind: patch, enabled: false}
  codex:gpt-6-luna:low: {kind: patch, enabled: true}`)
	if _, err := LoadFromDirs(t.TempDir(), configDir); err == nil || !strings.Contains(err.Error(), "conflicting enabled values") {
		t.Fatalf("tierless/tiered conflict error = %v", err)
	}
}

func TestLoadUsesIsolatedXDGDirectories(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	stateHome := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
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
	if policy := catalog.UserPolicy(); policy == nil || policy.DefaultEnabled {
		t.Fatalf("XDG_CONFIG_HOME policy not loaded: %#v", policy)
	}
	writeCatalogFile(t, filepath.Join(home, ".config", "harnez", "models.yaml"), "version: 1\ndefault_enabled: true\nenabled: []\nmodels: {}\n")
	catalog, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if policy := catalog.UserPolicy(); policy == nil || policy.DefaultEnabled {
		t.Fatalf("HOME config overrode XDG_CONFIG_HOME: %#v", policy)
	}
}

func TestUnknownAccessDoesNotCreateModelOrDenyDefaultEnabled(t *testing.T) {
	stateDir, configDir := t.TempDir(), t.TempDir()
	writeCatalogFile(t, filepath.Join(stateDir, "access.yaml"), `version: 1
observed_at: "2026-10-25T10:00:00Z"
complete: true
coverage: []
observations:
  - identity: {harness: pi, provider: openai, model: Pi/Only-Unknown, version: "1", tier: low}
    status: unknown
    method: probe
    observed_at: "2026-10-25T10:01:00Z"
    facts: []
  - identity: {harness: pi, provider: openai, model: Pi/Denied, version: "1", tier: low}
    status: denied
    method: probe
    observed_at: "2026-10-25T10:02:00Z"
    facts: []`)
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), "version: 1\ndefault_enabled: true\nenabled: []\nmodels: {}\n")
	catalog, err := LoadFromDirs(stateDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	models := catalog.Models()
	for _, id := range []string{"openai:Pi/Only-Unknown", "openai:Pi/Denied"} {
		if _, ok := models[id]; ok {
			t.Fatalf("unknown access created catalog model %q", id)
		}
	}
	access := catalog.Access()
	if len(access) != 2 || access[1].Status != "denied" || !catalog.IsEnabled("pi", "Pi/Denied", "low") {
		t.Fatalf("unknown/denied observations lost or changed default policy: %#v", access)
	}
}

func TestCatalogAccessorsReturnDetachedCopies(t *testing.T) {
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), "version: 1\ndefault_enabled: true\nenabled: [codex:luna:low]\nmodels: {}\n")
	cat, err := LoadFromDirs(t.TempDir(), configDir)
	if err != nil {
		t.Fatal(err)
	}
	models := cat.Models()
	for id, model := range models {
		model.Aliases = append(model.Aliases, "mutated")
		model.Facts["nested"] = map[string]any{"value": "mutated"}
		models[id] = model
		break
	}
	for _, model := range cat.Models() {
		if contains(model.Aliases, "mutated") {
			t.Fatal("Models returned mutable catalog state")
		}
	}
	policy := cat.UserPolicy()
	policy.Enabled[0] = "changed"
	if cat.UserPolicy().Enabled[0] != "codex:luna:low" {
		t.Fatal("UserPolicy returned mutable catalog state")
	}
}

func TestLoadRejectsUnappliedPolicyField(t *testing.T) {
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), "version: 1\ndefault_enabled: true\nenabled: []\nmodels:\n  codex:luna:low:\n    kind: patch\n    fields: {capabilities: [tools]}\n")
	if _, err := LoadFromDirs(t.TempDir(), configDir); err == nil || !strings.Contains(err.Error(), "not applied") {
		t.Fatalf("unapplied policy field error = %v", err)
	}
}

func TestCustomIdentityVersionPin(t *testing.T) {
	configDir := t.TempDir()
	writeCatalogFile(t, filepath.Join(configDir, "models.yaml"), "version: 1\ndefault_enabled: true\nenabled: []\nmodels:\n  local:private:\n    kind: custom\n    identity: {harness: custom, provider: local, model: private, version: \"1\"}\n    version_pin: \"1\"\n")
	if _, err := LoadFromDirs(t.TempDir(), configDir); err != nil {
		t.Fatalf("matching custom identity pin rejected: %v", err)
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
