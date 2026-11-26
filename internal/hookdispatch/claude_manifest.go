package hookdispatch

// ClaudeManifest is the set of Claude hook events harnez installs and the
// handlers each one runs, in run order. `apply` writes one settings entry per
// event (Manifest.Entries), and `harnez hook <Event>` runs the handlers bound to
// it. Tools and names are the routing; cmd/harnez/hook_dispatch.go binds the
// run functions by "Event/Name".
//
// PreToolUse has one entry for all its tools. exec is the only PreToolUse/Bash
// handler: Claude Code does not compose two hooks' updatedInput rewrites on
// the same matcher (last one to finish wins), so the distill rewrite runs
// inside exec. See docs/HookRewritePattern.md.
var ClaudeManifest = Manifest{
	"PreToolUse": {
		{Name: "exec", Kind: JSON, Tools: []string{"Bash"}},
		{Name: "read", Kind: JSON, Tools: []string{"View", "Read", "ReadMultipleFiles"}},
		{Name: "pre-edit", Kind: JSON, Tools: []string{"Edit", "Write"}},
	},
	"PostToolUse":        {{Name: "skill", Kind: Silent, Tools: []string{"Skill"}}},
	"PostToolUseFailure": {{Name: "skill", Kind: Silent, Tools: []string{"Skill"}}},
	"UserPromptSubmit":   {{Name: "skill", Kind: Silent}},
	"SessionStart":       {{Name: "claude-instructions", Kind: Text}},
	"PreCompact": {
		{Name: "codex-telemetry", Kind: Silent},
		{Name: "compact-reset", Kind: Silent},
	},
	"PostCompact": {{Name: "codex-telemetry", Kind: Silent}},
	"Stop":        {{Name: "sound", Kind: Silent}},
}
