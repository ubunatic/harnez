// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/jsonc"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "full"},
		{[]string{""}, "full"},
		{[]string{"full"}, "docs,skills,telemetry,agents,usage"},
		{[]string{"docs-only"}, "docs,skills"},
		{[]string{"telemetry-only"}, "telemetry"},
		{[]string{"agents-only"}, "agents"},
		{[]string{"docs-only,telemetry"}, "docs,skills,telemetry"},
		{[]string{"agents", " usage "}, "agents,usage"},
	}
	for _, c := range cases {
		got, err := Parse(c.in...)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		if got.String() != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	if _, err := Parse("profile"); err == nil {
		t.Error("Parse(profile): want error for unknown name")
	}
}

func TestNilSetEnablesAll(t *testing.T) {
	var s Set
	for _, c := range All {
		if !s.Has(c) {
			t.Errorf("nil set: Has(%s) = false", c)
		}
	}
	if !s.Allows([]Component{Telemetry, Agents}) {
		t.Error("nil set: Allows = false")
	}
}

func TestResolvePrecedence(t *testing.T) {
	got, _ := Resolve([]string{"agents-only"}, []string{"docs-only"}, []string{"telemetry-only"})
	if got.String() != "agents" {
		t.Errorf("flag should win, got %s", got)
	}
	got, _ = Resolve(nil, []string{"docs-only"}, []string{"telemetry-only"})
	if got.String() != "docs,skills" {
		t.Errorf("config should win over local, got %s", got)
	}
	got, _ = Resolve(nil, nil, []string{"telemetry-only"})
	if got.String() != "telemetry" {
		t.Errorf("local fallback, got %s", got)
	}
	got, _ = Resolve(nil, nil, nil)
	if got != nil {
		t.Errorf("no selection should be nil (full), got %s", got)
	}
}

func TestValidateConfigRejectsUnknownComponents(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"config", &Config{ComponentNames: []string{"telemtry"}}, "telemtry"},
		{"requirement", &Config{Skills: []Command{{Name: "broken", Requires: []string{"telemtry"}}}}, "skill \"broken\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateConfig() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestPresetNamesMatchPresets(t *testing.T) {
	names := PresetNames()
	if len(names) != len(presets) {
		t.Fatalf("PresetNames %v out of sync with presets", names)
	}
	for _, n := range names {
		if _, ok := presets[n]; !ok {
			t.Errorf("PresetNames lists %q, not in presets", n)
		}
	}
}

// setupComponentsHome isolates HOME and redirects every apply target into
// it, mirroring the internal/components package's test fixture (issue 491
// M6: these tests moved here once selection lives entirely in this
// package and internal/components was deleted).
func setupComponentsHome(t *testing.T) (target string, cfg *Config) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNEZ_DISABLE_RATE_FEEDBACK", "")
	// agy hooks are only installed when ~/.gemini exists.
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}
	cfg.PrimeAgentTarget = filepath.Join(home, ".prime", "agent")
	cfg.SkillsTarget = filepath.Join(home, ".gemini", "skills")
	cfg.CodexSkillsTarget = filepath.Join(home, ".codex", "skills")
	cfg.ClaudeSkillsTarget = filepath.Join(home, ".claude", "skills")
	return filepath.Join(home, ".claude"), cfg
}

func settingsCommands(t *testing.T, target string) (hooks []string, statusLine string) {
	t.Helper()
	doc := jsonc.Read(filepath.Join(target, "settings.json"))
	hm, _ := doc["hooks"].(map[string]any)
	for _, v := range hm {
		entries, _ := v.([]any)
		for _, e := range entries {
			m, _ := e.(map[string]any)
			list, _ := m["hooks"].([]any)
			for _, h := range list {
				hm2, _ := h.(map[string]any)
				if cmd, ok := hm2["command"].(string); ok {
					hooks = append(hooks, cmd)
				}
			}
		}
	}
	if sl, ok := doc["statusLine"].(map[string]any); ok {
		statusLine, _ = sl["command"].(string)
	}
	return hooks, statusLine
}

func mustApplyComponents(t *testing.T, target string, cfg *Config, docs []string, names ...string) {
	t.Helper()
	set, err := Parse(names...)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyAllVariant(target, cfg, set, docs, false, false, "", false); err != nil {
		t.Fatalf("ApplyAllVariant(%v): %v", names, err)
	}
}

func TestApply_FullMatchesPlainApply(t *testing.T) {
	target, cfg := setupComponentsHome(t)
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	want, _ := os.ReadFile(filepath.Join(target, "settings.json"))

	target2, cfg2 := setupComponentsHome(t)
	mustApplyComponents(t, target2, cfg2, nil) // no selection = full
	got, _ := os.ReadFile(filepath.Join(target2, "settings.json"))
	if string(got) != string(want) {
		t.Errorf("unfiltered selected apply settings differ from plain apply:\n%s\nvs\n%s", got, want)
	}
}

func TestApply_FullThenDocsOnly(t *testing.T) {
	target, cfg := setupComponentsHome(t)
	mustApplyComponents(t, target, cfg, nil, "full")

	hooks, statusLine := settingsCommands(t, target)
	if !slices.Contains(hooks, "harnez exec hook") || statusLine != "harnez statusline" {
		t.Fatalf("full: hooks=%v statusLine=%q", hooks, statusLine)
	}
	codexPath := filepath.Join(os.Getenv("HOME"), ".codex", "config.toml")
	if installed, _ := codex.Status(codexPath); !installed {
		t.Fatal("full: codex hook not installed")
	}
	agyPath := filepath.Join(os.Getenv("HOME"), ".gemini", "config", "hooks.json")
	if installed, _ := agy.Status(agyPath); !installed {
		t.Fatal("full: agy hook not installed")
	}
	// A user's own Codex hook on an event harnez also uses must survive.
	userHook := "\n[[hooks.PreToolUse]]\nmatcher = \"Edit\"\n\n[[hooks.PreToolUse.hooks]]\ntype = \"command\"\ncommand = \"my-lint\"\n"
	f, err := os.OpenFile(codexPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(userHook); err != nil {
		t.Fatal(err)
	}
	f.Close()
	rateSkill := filepath.Join(cfg.ClaudeSkillsTarget, "tool-feedback-protocol", "SKILL.md")
	issueSkill := filepath.Join(cfg.ClaudeSkillsTarget, "issue", "SKILL.md")
	for _, p := range []string{rateSkill, issueSkill} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("full: %s missing", p)
		}
	}

	mustApplyComponents(t, target, cfg, nil, "docs-only")

	hooks, statusLine = settingsCommands(t, target)
	for _, h := range hooks {
		if isHarnezCommand(h) {
			t.Errorf("docs-only: harnez hook %q left in settings", h)
		}
	}
	if !slices.ContainsFunc(hooks, func(h string) bool { return strings.HasPrefix(h, "ffplay") }) {
		t.Error("docs-only: non-harnez Stop hook must survive")
	}
	if statusLine != "" {
		t.Errorf("docs-only: statusLine %q left", statusLine)
	}
	codexData, _ := os.ReadFile(codexPath)
	if strings.Contains(string(codexData), "harnez ") {
		t.Errorf("docs-only: codex harnez hooks left:\n%s", codexData)
	}
	if !strings.Contains(string(codexData), "my-lint") {
		t.Errorf("docs-only: user codex hook removed:\n%s", codexData)
	}
	if installed, _ := agy.Status(agyPath); installed {
		t.Error("docs-only: agy harnez hook left")
	}
	if _, err := os.Stat(rateSkill); !os.IsNotExist(err) {
		t.Error("docs-only: tool-feedback-protocol skill left")
	}
	if _, err := os.Stat(issueSkill); err != nil {
		t.Error("docs-only: issue skill must stay")
	}

	set, _ := Parse("docs-only")
	drift, err := DiffAll(target, cfg, set)
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if drift {
		t.Error("docs-only: DiffAll under the same selection reports drift")
	}

	// Switching back reinstalls everything.
	mustApplyComponents(t, target, cfg, nil, "full")
	hooks, statusLine = settingsCommands(t, target)
	if !slices.Contains(hooks, "harnez exec hook") || statusLine == "" {
		t.Errorf("back to full: hooks=%v statusLine=%q", hooks, statusLine)
	}
	if _, err := os.Stat(rateSkill); err != nil {
		t.Error("back to full: rate skill not reinstalled")
	}
}

func TestApply_TelemetryOnly(t *testing.T) {
	target, cfg := setupComponentsHome(t)
	mustApplyComponents(t, target, cfg, []string{"git"}, "telemetry-only")
	if entries, _ := os.ReadDir(filepath.Join(target, "docs")); len(entries) > 0 {
		t.Errorf("telemetry-only: docs installed: %v", entries)
	}

	hooks, statusLine := settingsCommands(t, target)
	if !slices.Contains(hooks, "harnez exec hook") || !slices.Contains(hooks, "harnez hook read") {
		t.Errorf("telemetry-only: hooks=%v", hooks)
	}
	if statusLine != "" {
		t.Errorf("telemetry-only: statusLine %q installed", statusLine)
	}
	if _, err := os.Stat(filepath.Join(cfg.ClaudeSkillsTarget, "issue")); !os.IsNotExist(err) {
		t.Error("telemetry-only: skills installed")
	}
}

func TestApply_PresetsComposeByUnion(t *testing.T) {
	target, cfg := setupComponentsHome(t)
	mustApplyComponents(t, target, cfg, nil, "docs-only,telemetry-only")
	hooks, _ := settingsCommands(t, target)
	if !slices.Contains(hooks, "harnez exec hook") {
		t.Error("union: telemetry hooks missing")
	}
	if _, err := os.Stat(filepath.Join(cfg.ClaudeSkillsTarget, "tool-feedback-protocol", "SKILL.md")); err != nil {
		t.Error("union: rate skill missing although skills and telemetry are enabled")
	}
}

func TestApply_AgentsOnlyAfterFull(t *testing.T) {
	target, cfg := setupComponentsHome(t)
	mustApplyComponents(t, target, cfg, nil, "full")
	mustApplyComponents(t, target, cfg, nil, "agents-only")

	hooks, statusLine := settingsCommands(t, target)
	for _, h := range hooks {
		if isHarnezCommand(h) {
			t.Errorf("agents-only: harnez hook %q left", h)
		}
	}
	if statusLine != "" {
		t.Errorf("agents-only: statusLine %q left", statusLine)
	}
	// skills component off: rate skill removed (unmet requires), others skipped, not deleted
	if _, err := os.Stat(filepath.Join(cfg.ClaudeSkillsTarget, "tool-feedback-protocol", "SKILL.md")); !os.IsNotExist(err) {
		t.Error("agents-only: rate skill left")
	}
	if _, err := os.Stat(filepath.Join(cfg.ClaudeSkillsTarget, "issue", "SKILL.md")); err != nil {
		t.Error("agents-only: skip semantics must leave the issue skill in place")
	}
}

func TestApply_OnlyHarnezHooksLeavesNoHooksKey(t *testing.T) {
	target, cfg := setupComponentsHome(t)
	cfg.Hooks = slices.DeleteFunc(slices.Clone(cfg.Hooks), func(h Hook) bool {
		return !isHarnezCommand(h.Command)
	})
	mustApplyComponents(t, target, cfg, nil, "full")
	mustApplyComponents(t, target, cfg, nil, "docs-only")
	doc := jsonc.Read(filepath.Join(target, "settings.json"))
	if _, ok := doc["hooks"]; ok {
		t.Errorf("docs-only without user hooks: stale hooks key %v", doc["hooks"])
	}
}
