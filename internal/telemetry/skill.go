package telemetry

import (
	"regexp"
	"strings"
)

// Skill rows (issue 735) use ToolName SkillToolName and one of these call
// types, so a report can tell real skill calls from inferred ones.
const (
	SkillToolName = "Skill"
	// CallTypeSkillPrompt marks a typed /name that matched an installed skill
	// or command (Claude UserPromptSubmit).
	CallTypeSkillPrompt = "prompt"
	// CallTypeSkillInferred marks a heuristic row: an agent without a skill
	// hook (Codex, AGY) read a skills/<name>/SKILL.md file. A human or agent
	// editing that skill reads it too, so these rows over-count.
	CallTypeSkillInferred = "skill-inferred"
)

var (
	skillNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*(:[A-Za-z0-9][A-Za-z0-9_.-]*)?$`)
	skillFilePattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.-])skills/([A-Za-z0-9][A-Za-z0-9_.-]*)/SKILL\.md\b`)
)

// NormalizeSkillName returns the skill name in a Skill tool input or typed
// command: the first word without a leading slash, arguments dropped, a
// plugin:skill namespace kept whole. It returns "" for anything that is not
// a plain name, such as a path.
func NormalizeSkillName(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	name := strings.TrimPrefix(fields[0], "/")
	if !skillNamePattern.MatchString(name) {
		return ""
	}
	return name
}

// SkillFromFileRef returns the skill whose skills/<name>/SKILL.md the text
// (a file path or shell command) refers to, or "".
func SkillFromFileRef(text string) string {
	if m := skillFilePattern.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}
