package claude

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPrimeAgentTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &Config{
		SkillsTarget:      "~/.gemini/skills",
		CodexSkillsTarget: "~/.codex/skills",
		PrimeAgentTarget:  "~/.prime/agent",
	}

	wantSkills := []string{
		filepath.Join(home, ".gemini", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".prime", "agent", "skills"),
	}
	if got := skillTargets(cfg); !reflect.DeepEqual(got, wantSkills) {
		t.Fatalf("skillTargets() = %#v, want %#v", got, wantSkills)
	}

	claudeTarget := filepath.Join(home, ".claude")
	wantCommands := []string{
		filepath.Join(claudeTarget, "commands"),
		filepath.Join(home, ".prime", "agent", "prompts"),
	}
	if got := commandTargets(claudeTarget, cfg); !reflect.DeepEqual(got, wantCommands) {
		t.Fatalf("commandTargets() = %#v, want %#v", got, wantCommands)
	}
}

func TestPrimeAgentTargetCanBeDisabled(t *testing.T) {
	cfg := &Config{SkillsTarget: "/skills", PrimeAgentTarget: ""}
	if got := skillTargets(cfg); !reflect.DeepEqual(got, []string{"/skills"}) {
		t.Fatalf("skillTargets() = %#v, want only configured skills target", got)
	}
	if got := commandTargets("/claude", cfg); !reflect.DeepEqual(got, []string{"/claude/commands"}) {
		t.Fatalf("commandTargets() = %#v, want only Claude command target", got)
	}
}

func TestTargetPathsAreDeduplicated(t *testing.T) {
	cfg := &Config{
		SkillsTarget:      "/shared",
		CodexSkillsTarget: "/shared",
		PrimeAgentTarget:  "/prime",
	}
	want := []string{"/shared", "/prime/skills"}
	if got := skillTargets(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("skillTargets() = %#v, want %#v", got, want)
	}
}
