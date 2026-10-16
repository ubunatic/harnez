package claude

import (
	"strings"
	"testing"
	"testing/fstest"
)

// The shipped config's bundled skills resolve in the embedded FS and do
// not clash with harnez-owned skill names.
func TestEmbeddedBundledSkillsResolve(t *testing.T) {
	cfg := loadTestConfig(t)
	skills, err := BundledSkills(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Fatal("embedded config must not ship third-party skills")
	}
}

func bundledFixtureConfig() *Config {
	return &Config{
		BundledSkills: []BundledSkillSource{{URL: "https://example.com/skills", Commit: strings.Repeat("a", 40), Dir: "vendor", Skills: []string{"example"}}},
		FS:            fstest.MapFS{"vendor/example/SKILL.md": {Data: []byte("---\nname: example\ndescription: Example\n---\nExample body\n")}},
	}
}

func TestConfiguredBundledSkillsResolve(t *testing.T) {
	cfg := bundledFixtureConfig()
	skills, err := BundledSkills(cfg)
	if err != nil || len(skills) != 1 {
		t.Fatalf("resolve fixture: %v, %v", skills, err)
	}
	if skills[0].Name != "example" || skills[0].URL != cfg.BundledSkills[0].URL || skills[0].Commit != cfg.BundledSkills[0].Commit {
		t.Fatalf("lost bundle provenance: %+v", skills[0])
	}
}

func TestBundledSkillNameClashIsAnError(t *testing.T) {
	cfg := bundledFixtureConfig()
	cfg.Skills = append(cfg.Skills, Command{Name: cfg.BundledSkills[0].Skills[0]})
	if _, err := BundledSkills(cfg); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("clash not reported: %v", err)
	}
}
