package telemetry

import "testing"

func TestNormalizeSkillName(t *testing.T) {
	for in, want := range map[string]string{
		"issue":                          "issue",
		"/issue file two tickets":        "issue",
		"  lean-sprint\n":                "lean-sprint",
		"plugin-dev:hook-development":    "plugin-dev:hook-development",
		"/plugin-dev:hook-development x": "plugin-dev:hook-development",
		"/home/x":                        "",
		"/tmp/a.md":                      "",
		"":                               "",
		"/":                              "",
		"a:b:c":                          "",
		"../etc":                         "",
	} {
		if got := NormalizeSkillName(in); got != want {
			t.Errorf("NormalizeSkillName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSkillFromFileRef(t *testing.T) {
	for in, want := range map[string]string{
		"cat /home/u/.codex/skills/issue/SKILL.md":                              "issue",
		"sed -n '1,240p' /home/u/.codex/skills/tool-feedback-protocol/SKILL.md": "tool-feedback-protocol",
		"/home/u/.gemini/skills/evergreen/SKILL.md":                             "evergreen",
		"skills/x/SKILL.md":               "x",
		"cat /home/u/skills/x/README.md":  "",
		"cat /home/u/myskills/x/SKILL.md": "",
		"rg SKILL.md":                     "",
	} {
		if got := SkillFromFileRef(in); got != want {
			t.Errorf("SkillFromFileRef(%q) = %q, want %q", in, got, want)
		}
	}
}
