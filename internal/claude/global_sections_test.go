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
	if !strings.Contains(cfg.GlobalSections[0].Content, "pkill -f") || !strings.Contains(cfg.GlobalSections[0].Content, "HTO=0") || !strings.Contains(cfg.GlobalSections[0].Content, "sole product") {
		t.Fatal("process hygiene rules missing")
	}
	const wantFeedback = "### Tool Feedback Protocol\n" +
		"Harnez-tracked projects only (`issues/` present); omit ticket or skip otherwise.\n" +
		"Rate a call only when it failed or produced an unexpected outcome — not routine successes:\n" +
		"`harnez rate <tool_name> <1-5> \"<1-line outcome summary>\" \"<project_folder>/<ticket_name>\"`\n" +
		"Example: `harnez rate Grep 1 \"missed target, wrong file\" harnez/117-harnez-rate-command`\n" +
		"Score 5 flawless, 3 partial, 1 useless.\n" +
		"If prompted after a long quiet stretch and nothing has failed, confirm with\n" +
		"`harnez rate --ok` instead of staying silent — no tool name, score, or ticket required.\n"
	if cfg.GlobalSections[1].Content != wantFeedback {
		t.Fatalf("Tool Feedback text changed:\nwant %q\n got %q", wantFeedback, cfg.GlobalSections[1].Content)
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

func TestCleanManagedSectionsPreservesUnknownBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "CLAUDE.md")
	unknown := "<!-- harnez:begin Instructions Hierarchy -->\nkeep hierarchy\n<!-- harnez:end Instructions Hierarchy -->\n" +
		"<!-- harnez:begin Issue Tracker Discovery -->\nkeep tracker\n<!-- harnez:end Issue Tracker Discovery -->\n" +
		"<!-- harnez:begin Agent-Filed Feedback -->\nkeep feedback\n<!-- harnez:end Agent-Filed Feedback -->\n"
	if err := os.WriteFile(path, []byte(unknown), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t)
	cfg.GlobalSectionTargets = []string{path}
	if err := ApplyAll(filepath.Join(home, ".claude"), cfg, nil, false, false); err != nil {
		t.Fatal(err)
	}
	if err := CleanAll(filepath.Join(home, ".claude"), cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != unknown {
		t.Fatalf("clean changed unknown blocks: %q, %v", data, err)
	}
}

func TestGlobalSectionsOnlyAddsProcessHygieneToNewTargets(t *testing.T) {
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "0")
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	const userText = "user instructions\n"
	if err := os.WriteFile(path, []byte(userText), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t)
	cfg.GlobalSectionTargets = []string{path}
	if _, err := manageGlobalSections(cfg, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<!-- harnez:begin Process hygiene -->") || strings.Contains(string(data), "<!-- harnez:begin Tool Feedback Protocol -->") {
		t.Fatalf("new target received unexpected managed blocks: %q", data)
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
	cfg.RetiredGlobalSections = []string{"Process hygiene"}
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

func TestGlobalSectionsKeepsFeedbackAndMigratesWithoutRewrite(t *testing.T) {
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "0")
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	feedback := strings.TrimSuffix(loadTestConfig(t).GlobalSections[1].Content, "\n")
	legacy := "before\n<!-- harnez:begin Tool Feedback Protocol -->\n" + feedback + "\n<!-- harnez:end Tool Feedback Protocol -->\nafter\n"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t)
	cfg.GlobalSectionTargets = []string{path}
	processBlock := "<!-- harnez:begin Process hygiene -->\n" + strings.TrimSuffix(cfg.GlobalSections[0].Content, "\n") + "\n<!-- harnez:end Process hygiene -->\n"
	for _, want := range []bool{true, false} {
		changed, err := manageGlobalSections(cfg, false)
		if err != nil || changed != want {
			t.Fatalf("migration = %v, %v", changed, err)
		}
	}
	data, _ := os.ReadFile(path)
	if string(data) != legacy+processBlock {
		t.Fatalf("feedback wording or placement changed: %q", data)
	}
	cfg.Feedback.DisableRateProtocol = true
	if _, err := manageGlobalSections(cfg, false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "before\nafter\n"+processBlock {
		t.Fatalf("known feedback block was not removed during opt-out: %q", data)
	}
}
