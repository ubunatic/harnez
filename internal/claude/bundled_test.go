package claude

import (
	"strings"
	"testing"
)

// The shipped config's bundled skills resolve in the embedded FS and do
// not clash with harnez-owned skill names.
func TestEmbeddedBundledSkillsResolve(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	skills, err := BundledSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) == 0 {
		t.Fatal("no bundled skills in the embedded config")
	}
}

func TestBundledSkillNameClashIsAnError(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Skills = append(cfg.Skills, Command{Name: cfg.BundledSkills[0].Skills[0]})
	if _, err := BundledSkills(cfg); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("clash not reported: %v", err)
	}
}
