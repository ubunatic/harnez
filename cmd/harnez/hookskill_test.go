package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

// Payloads follow the issue 735 probe (scripts/canary-735-skill-hooks.sh).
func TestClaudeSkillHookRecordsSkillCalls(t *testing.T) {
	home, cwd := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	writeSkillFile(t, filepath.Join(home, ".claude", "skills", "issue", "SKILL.md"))
	writeSkillFile(t, filepath.Join(cwd, ".claude", "commands", "local-cmd.md"))
	writeSkillFile(t, filepath.Join(home, ".claude", "plugins", "cache", "market", "plugin-dev", "1.0.0", "skills", "hook-development", "SKILL.md"))

	cases := []struct {
		name, payload, wantSkill, wantType string
	}{
		{"skill tool", `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"issue","args":"file two tickets"}}`, "issue", "hook:post"},
		{"failed skill tool", `{"hook_event_name":"PostToolUseFailure","tool_name":"Skill","tool_input":{"skill":"lean-sprint"}}`, "lean-sprint", "hook:failure"},
		{"plugin skill tool", `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"plugin-dev:hook-development"}}`, "plugin-dev:hook-development", "hook:post"},
		{"typed skill with args", `{"hook_event_name":"UserPromptSubmit","prompt":"/issue some args\n"}`, "issue", "prompt"},
		{"typed project command", `{"hook_event_name":"UserPromptSubmit","prompt":"/local-cmd"}`, "local-cmd", "prompt"},
		{"typed plugin skill", `{"hook_event_name":"UserPromptSubmit","prompt":"/plugin-dev:hook-development"}`, "plugin-dev:hook-development", "prompt"},
		{"other tool", `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`, "", ""},
		{"absolute path", `{"hook_event_name":"UserPromptSubmit","prompt":"/home/x"}`, "", ""},
		{"file path", `{"hook_event_name":"UserPromptSubmit","prompt":"/tmp/a.md what is this?"}`, "", ""},
		{"built-in clear", `{"hook_event_name":"UserPromptSubmit","prompt":"/clear"}`, "", ""},
		{"built-in compact", `{"hook_event_name":"UserPromptSubmit","prompt":"/compact"}`, "", ""},
		{"built-in model", `{"hook_event_name":"UserPromptSubmit","prompt":"/model opus"}`, "", ""},
		{"not installed", `{"hook_event_name":"UserPromptSubmit","prompt":"/zz-missing"}`, "", ""},
		{"plain prompt", `{"hook_event_name":"UserPromptSubmit","prompt":"use /issue later"}`, "", ""},
		{"not json", `nope`, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []telemetry.ToolCall
			payload := strings.Replace(tc.payload, "{", `{"session_id":"s1","cwd":"`+cwd+`",`, 1)
			runClaudeSkillHook(strings.NewReader(payload), skillHookOptions{
				DBPath: "db", Home: home,
				Insert: func(_ string, call telemetry.ToolCall) error { got = append(got, call); return nil },
			})
			if tc.wantSkill == "" {
				if len(got) != 0 {
					t.Fatalf("recorded %+v, want nothing", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("recorded %d rows, want 1", len(got))
			}
			c := got[0]
			if c.ToolName != "Skill" || c.SkillName != tc.wantSkill || c.CallType != tc.wantType || c.AgentID != "claude" || c.SessionID != "s1" || c.WorkingDir != cwd {
				t.Fatalf("row = %+v", c)
			}
		})
	}
}

func writeSkillFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
