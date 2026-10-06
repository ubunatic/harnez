package claude

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestHarnezHandoffSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()

	var handoff *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == "harnez-handoff" {
			handoff = &cfg.Skills[i]
			break
		}
	}
	if handoff == nil {
		t.Fatal("harnez-handoff skill not registered in embedded config")
	}
	if handoff.File != "docs/commands/HarnezHandoff.md" {
		t.Fatalf("harnez-handoff skill file = %q, want docs/commands/HarnezHandoff.md", handoff.File)
	}
	if len(handoff.Resources) != 1 || handoff.Resources[0].Source != "spec/handoff.yaml" || handoff.Resources[0].Target != "handoff.yaml" {
		t.Fatalf("harnez-handoff resources = %#v, want spec/handoff.yaml -> handoff.yaml", handoff.Resources)
	}
	if override := cfg.Debloat.PresetSkillOverrides["harnez-handoff"]; override != "user-invocable-only" {
		t.Errorf("PresetSkillOverrides[harnez-handoff] = %q, want user-invocable-only", override)
	}
	spec, err := fs.ReadFile(harnez.DefaultFS, "spec/handoff.yaml")
	if err != nil {
		t.Fatal(err)
	}

	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.CodexHooksTarget = filepath.Join(t.TempDir(), "codex-config.toml")
	cfg.AgyHooksTarget = filepath.Join(t.TempDir(), "gemini", "config", "hooks.json")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = filepath.Join(t.TempDir(), "prime-agent")
	cfg.AgentsMD.Global.Target = filepath.Join(t.TempDir(), "AGENTS.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	for _, skillsRoot := range skillTargets(cfg) {
		dir := filepath.Join(skillsRoot, "harnez-handoff")
		data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			t.Fatalf("expected harnez-handoff skill in %s: %v", dir, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: \"harnez-handoff\"\ndescription:") {
			t.Errorf("expected skill frontmatter in %s, got:\n%s", dir, content)
		}
		for _, want := range []string{
			"disable-model-invocation: true",
			"## 1. Read the Request",
			"`this work`",
			"`explore`",
			"Ask the user **once**",
			"`local Jules`",
			"## 2. Load the Agent Facts",
			"`handoff.yaml` next to this SKILL.md",
			"harnez read spec/handoff.yaml",
			"## 3. Explore Mode",
			"if the working directory is a git repo",
			"rev-list --count <remote>/<branch>..HEAD",
			"--since=3.days",
			"`14.days`",
			"user excludes",
			"merge-conflict risk",
			"## 4. Write Each Prompt",
			"no memory of this session",
			"/goal",
			"renumbered on merge",
			"## 5. Output",
			"git -C <repo> fetch <remote>",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %s/SKILL.md to contain %q", dir, want)
			}
		}
		installed, err := os.ReadFile(filepath.Join(dir, "handoff.yaml"))
		if err != nil {
			t.Fatalf("expected handoff.yaml resource in %s: %v", dir, err)
		}
		if !bytes.Equal(installed, spec) {
			t.Errorf("%s/handoff.yaml differs from spec/handoff.yaml", dir)
		}
	}
}
