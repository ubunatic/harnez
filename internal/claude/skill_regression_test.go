package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func skillRegressionConfig(t *testing.T, content string) (*Config, string, []string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(t.TempDir(), "config")
	cfg := loadTestConfig(t)
	cfg.Skills = []Command{{Name: "refresh-me", Description: "generated description", Content: content}}
	cfg.SkillsTarget = filepath.Join(home, ".gemini", "skills")
	cfg.CodexSkillsTarget = filepath.Join(home, ".codex", "skills")
	cfg.ClaudeSkillsTarget = filepath.Join(home, ".claude", "skills")
	cfg.PrimeAgentTarget = filepath.Join(home, ".prime", "agent")
	cfg.PiAgentTarget = filepath.Join(home, ".pi", "agent")
	cfg.CodexHooksTarget = filepath.Join(home, ".codex", "config.toml")
	cfg.AgyHooksTarget = filepath.Join(home, ".gemini", "config", "hooks.json")
	cfg.AgentsMD.Global.Target = filepath.Join(home, ".claude", "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(home, ".pi", "agent", "extensions", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(home, ".config", "opencode", "harnez-distill.ts")
	return cfg, target, skillTargets(cfg)
}

func TestApplyRefreshesManagedSkillInEveryTarget(t *testing.T) {
	cfg, target, roots := skillRegressionConfig(t, "old source body")
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("initial ApplyAll: %v", err)
	}
	cfg.Skills[0].Content = "updated source body"
	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("refresh ApplyAll: %v", err)
	}
	for _, root := range roots {
		dir := filepath.Join(root, cfg.Skills[0].Name)
		data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		if !strings.Contains(string(data), "updated source body") || strings.Contains(string(data), "old source body") {
			t.Errorf("%s did not refresh from source:\n%s", root, data)
		}
		if _, err := os.Stat(filepath.Join(dir, ManagedSkillMarker)); err != nil {
			t.Errorf("%s missing marker: %v", root, err)
		}
	}
}

func TestApplyAdoptsLegacyHarnezSkillInEveryTarget(t *testing.T) {
	cfg, target, roots := skillRegressionConfig(t, "current source body")
	for _, root := range roots {
		dir := filepath.Join(root, cfg.Skills[0].Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		legacy := "---\nname: refresh-me\ndescription: old description\n---\nlegacy body\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(legacy), 0644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := CaptureTestStdout(func() error { return ApplyAll(target, cfg, nil, false, false) })
	if err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	for _, root := range roots {
		dir := filepath.Join(root, cfg.Skills[0].Name)
		if !strings.Contains(out, "adopted "+dir) {
			t.Errorf("apply did not report adoption of %s:\n%s", dir, out)
		}
		if _, err := os.Stat(filepath.Join(dir, ManagedSkillMarker)); err != nil {
			t.Errorf("adopted skill in %s missing marker: %v", root, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "current source body") || strings.Contains(string(data), "legacy body") {
			t.Errorf("adopted skill in %s was not refreshed:\n%s", root, data)
		}
	}
}

func TestApplyAndDiffReportUnmarkedUserSkillsInEveryTarget(t *testing.T) {
	cfg, target, roots := skillRegressionConfig(t, "new configured body")
	for i, root := range roots {
		dir := filepath.Join(root, cfg.Skills[0].Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		name := cfg.Skills[0].Name
		if i == 1 {
			name = "different-skill"
		}
		body := fmt.Sprintf("---\nname: %s\n---\nuser body\n", name)
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		if i != 1 {
			if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep me"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	applyOut, err := CaptureTestStdout(func() error { return ApplyAll(target, cfg, nil, false, false) })
	if err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	for i, root := range roots {
		path := filepath.Join(root, cfg.Skills[0].Name)
		if !strings.Contains(applyOut, path) || !strings.Contains(applyOut, "touch "+strconv.Quote(filepath.Join(path, ManagedSkillMarker))) {
			t.Errorf("apply output omitted skip reason/takeover hint for %s:\n%s", path, applyOut)
		}
		data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
		if err != nil {
			t.Errorf("apply removed user skill in %s: %v", path, err)
		} else if !strings.Contains(string(data), "user body") || strings.Contains(string(data), "new configured body") {
			t.Errorf("apply changed user skill in %s:\n%s", path, data)
		}
		if i == 1 {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, "notes.txt")); err != nil {
			t.Errorf("apply removed user file in %s: %v", path, err)
		}
	}

	var drift bool
	diffOut, err := CaptureTestStdout(func() error {
		var diffErr error
		drift, diffErr = DiffAll(target, cfg, nil)
		return diffErr
	})
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if !drift {
		t.Fatal("DiffAll did not report unmarked user-owned skill directories")
	}
	for _, root := range roots {
		path := filepath.Join(root, cfg.Skills[0].Name)
		if !strings.Contains(diffOut, path) || !strings.Contains(diffOut, "touch "+strconv.Quote(filepath.Join(path, ManagedSkillMarker))) {
			t.Errorf("diff output omitted skip reason/takeover hint for %s:\n%s", path, diffOut)
		}
	}
}
