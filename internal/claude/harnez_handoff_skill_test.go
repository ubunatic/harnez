package claude

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/handoff"
)

func TestHarnezHandoffSkillInstallToEveryConfiguredTarget(t *testing.T) {
	cfg := loadTestConfig(t)
	targetDir := t.TempDir()

	var skill *Command
	for i := range cfg.Skills {
		if cfg.Skills[i].Name == "harnez-handoff" {
			skill = &cfg.Skills[i]
			break
		}
	}
	if skill == nil {
		t.Fatal("harnez-handoff skill not registered in embedded config")
	}
	if skill.File != "docs/commands/HarnezHandoff.md" {
		t.Fatalf("harnez-handoff skill file = %q, want docs/commands/HarnezHandoff.md", skill.File)
	}
	if len(skill.Resources) != 0 {
		t.Fatalf("harnez-handoff resources = %#v, want none: the spec is rendered into SKILL.md", skill.Resources)
	}
	if override := cfg.Debloat.PresetSkillOverrides["harnez-handoff"]; override != "user-invocable-only" {
		t.Errorf("PresetSkillOverrides[harnez-handoff] = %q, want user-invocable-only", override)
	}
	spec, err := handoff.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	specLines := specStrings(spec)

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
			"## 2. Profiles and Known Agents",
			"### Profile `local`",
			"### Profile `cloud`",
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
		for _, want := range specLines {
			if !strings.Contains(content, want) {
				t.Errorf("expected %s/SKILL.md to contain spec text %q", dir, want)
			}
		}
		for _, banned := range []string{"<!-- harnez:render", "handoff.yaml", "harnez read"} {
			if strings.Contains(content, banned) {
				t.Errorf("%s/SKILL.md still contains %q", dir, banned)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "handoff.yaml")); !os.IsNotExist(err) {
			t.Errorf("%s/handoff.yaml must not be installed (stat err = %v)", dir, err)
		}
	}
}

// specStrings lists every agent and profile value that must reach the skill.
func specStrings(s *handoff.Spec) []string {
	var out []string
	for _, p := range s.Profiles {
		out = append(out, p.Description)
		out = append(out, p.Rules...)
	}
	for _, a := range s.Agents {
		out = append(out, a.Name, a.Result)
		out = append(out, a.Aliases...)
		out = append(out, a.Facts...)
		out = append(out, a.Hosts...)
	}
	return out
}

// The skill source holds no hand-copied spec facts; they come only from the
// renderer, so a spec change changes the installed skill.
func TestHarnezHandoffSkillFollowsSpec(t *testing.T) {
	source, err := fs.ReadFile(harnez.DefaultFS, "docs/commands/HarnezHandoff.md")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := handoff.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range spec.Agents {
		for _, fact := range append([]string{a.Result}, a.Facts...) {
			if fact != "" && strings.Contains(string(source), fact) {
				t.Errorf("docs/commands/HarnezHandoff.md hand-copies spec fact %q", fact)
			}
		}
	}

	specData, err := fs.ReadFile(harnez.DefaultFS, "spec/handoff.yaml")
	if err != nil {
		t.Fatal(err)
	}
	const oldFact = "Runs on large Intel (non-AMD) machines."
	const newFact = "Runs on a test-only machine type."
	if !strings.Contains(string(specData), oldFact) {
		t.Fatalf("spec/handoff.yaml lacks %q; update this test", oldFact)
	}
	fsys := fstest.MapFS{
		"docs/commands/HarnezHandoff.md": {Data: source},
		"spec/handoff.yaml":              {Data: []byte(strings.Replace(string(specData), oldFact, newFact, 1))},
	}
	cmd := Command{Name: "harnez-handoff", File: "docs/commands/HarnezHandoff.md"}
	content, err := genSkillContent(cmd, fsys)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, newFact) || strings.Contains(content, oldFact) {
		t.Errorf("rendered skill does not follow the changed spec:\n%s", content)
	}
}

func TestRenderSkillBodyRejectsUnknownRenderer(t *testing.T) {
	if _, err := renderSkillBody("x", "a\n<!-- harnez:render nope -->\nb", fstest.MapFS{}); err == nil {
		t.Fatal("unknown renderer accepted")
	}
	if got, err := renderSkillBody("x", "plain", nil); err != nil || got != "plain" {
		t.Fatalf("plain body = %q, %v", got, err)
	}
}
