package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestGlobalSectionsSpec(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(document); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t)
	if len(cfg.GlobalSectionTargets) != 5 || len(cfg.GlobalSections) != 2 {
		t.Fatalf("missing global config: %+v", cfg.GlobalSections)
	}
	if !strings.Contains(cfg.GlobalSections[0].Content, "pkill -f") || !strings.Contains(cfg.GlobalSections[0].Content, "HTO=0") {
		t.Fatal("process hygiene rules missing")
	}
}

func TestApplyAndDiffGlobalSections(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("user\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t)
	cfg.GlobalSectionTargets = []string{path}
	target := filepath.Join(home, ".claude")
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatal(err)
	}
	changed, err := DiffAll(target, cfg, nil)
	if err != nil || changed {
		t.Fatalf("applied diff: %v, %v", changed, err)
	}
	cfg.GlobalSections[0].Content += "\nchanged\n"
	changed, err = DiffAll(target, cfg, nil)
	if err != nil || !changed {
		t.Fatalf("pending diff: %v, %v", changed, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(data), "changed") {
		t.Fatalf("diff wrote file: %q, %v", data, err)
	}
}

func TestGlobalSectionsLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "0")
	cfg := &Config{GlobalSections: []MDSection{{Name: "Process hygiene", Content: "first"}}}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md", "prime.md", "pi.md"} {
		path := filepath.Join(t.TempDir(), name)
		cfg.GlobalSectionTargets = append(cfg.GlobalSectionTargets, path)
		original := "User text  \r\n\nkeep this without final newline"
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(t.TempDir(), "missing.md")
	cfg.GlobalSectionTargets = append(cfg.GlobalSectionTargets, missing)
	apply := func(want bool) {
		t.Helper()
		changed, err := manageGlobalSections(cfg, false)
		if err != nil || changed != want {
			t.Fatalf("apply = %v, %v; want %v", changed, err, want)
		}
		changed, err = manageGlobalSections(cfg, true)
		if err != nil || changed {
			t.Fatalf("diff after apply = %v, %v", changed, err)
		}
	}
	apply(true)
	apply(false)
	cfg.GlobalSections[0].Content = "updated"
	if changed, err := manageGlobalSections(cfg, true); err != nil || !changed {
		t.Fatalf("pending update: %v, %v", changed, err)
	}
	apply(true)
	for _, path := range cfg.GlobalSectionTargets[:5] {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), "<!-- harnez:begin Process hygiene -->") != 1 || !strings.Contains(string(data), "updated") {
			t.Fatalf("bad block: %q", data)
		}
	}
	cfg.GlobalSections = nil
	if changed, err := manageGlobalSections(cfg, true); err != nil || !changed {
		t.Fatalf("pending removal: %v, %v", changed, err)
	}
	apply(true)
	for _, path := range cfg.GlobalSectionTargets[:5] {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "User text  \r\n\nkeep this without final newline" {
			t.Fatalf("user text changed: %q", data)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("permissions changed: %v, %v", info, err)
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing file created: %v", err)
	}
}

func TestGlobalSectionsLegacyFeedback(t *testing.T) {
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "0")
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	legacy := "before\n<!-- harnez:begin Tool Feedback Protocol -->\nold\n<!-- harnez:end Tool Feedback Protocol -->\nafter"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{GlobalSectionTargets: []string{path}, GlobalSections: []MDSection{{Name: "Tool Feedback Protocol", Content: "new", RateFeedback: true}}}
	for _, want := range []bool{true, false} {
		changed, err := manageGlobalSections(cfg, false)
		if err != nil || changed != want {
			t.Fatalf("migration = %v, %v", changed, err)
		}
	}
	data, _ := os.ReadFile(path)
	if string(data) != strings.Replace(legacy, "old", "new", 1) {
		t.Fatalf("migration duplicated or moved block: %q", data)
	}
	cfg.Feedback.DisableRateProtocol = true
	if _, err := manageGlobalSections(cfg, false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "before\nafter" {
		t.Fatalf("disabled feedback retained: %q", data)
	}
}
