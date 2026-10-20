package assess

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateRAMPGoldens = flag.Bool("update-ramp", false, "update RAMP compatibility goldens")

func TestRAMPOutputGolden(t *testing.T) {
	for _, scenario := range []string{"empty", "complete", "incoherent", "nongit"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			if scenario != "nongit" {
				initGitRepo(t, dir)
			}
			files := map[string]string{}
			if scenario == "complete" || scenario == "nongit" {
				files = map[string]string{
					"AGENTS.md": "# Agent Guidelines", ".claude/settings.json": "{}",
					"docs/AgenticLoop.md": "# Practices", ".claude/agents/helper.md": "# Helper",
					".claude/commands/check.md": "# Check", "skills/check/SKILL.md": "# Skill",
					"flows/team.yaml": "agents: []", "agent-sessions/run.jsonl": "{}",
				}
			} else if scenario == "incoherent" {
				files["flows/team.yaml"] = "agents: []"
			}
			for path, content := range files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "nongit" {
				commitAll(t, dir, "fixture")
			}
			if err := os.WriteFile(filepath.Join(dir, "agent_helper.sh"), []byte("echo helper"), 0644); err != nil {
				t.Fatal(err)
			}
			profile, err := AssessRAMPWithProjected(dir, []string{".codex/AGENTS.md"})
			if err != nil {
				t.Fatal(err)
			}
			jsonText, err := RenderRAMPJSON(profile)
			if err != nil {
				t.Fatal(err)
			}
			for format, got := range map[string]string{"text": RenderRAMPText(profile), "init": RenderRAMPInitText(profile), "json": jsonText} {
				path := filepath.Join("testdata", "ramp-"+scenario+"."+format+".golden")
				if *updateRAMPGoldens {
					if err := os.MkdirAll("testdata", 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(got), 0644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if got != string(want) {
					t.Errorf("%s output differs from %s\ngot:\n%s\nwant:\n%s", format, path, got, want)
				}
			}
		})
	}
}
