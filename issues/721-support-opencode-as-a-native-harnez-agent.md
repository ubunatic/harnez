# 721 — Support OpenCode as a native Harnez agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#712 Pi CLI driver](712-add-support-for-the-pi-coding-agent-cli-1-0.md), [#716 Pi skills and hooks](716-install-harnez-skills-and-hooks-into-pi-instances.md), [#723 agent container](723-consolidated-agent-container-with-in-container-lmcoder-smoke-test.md)

---

## 1. Problem & Motivation
OpenCode works with a local LLM server, but Harnez only writes its Distill
plugin (`opencode_plugin_target`, `internal/claude/distill_adapters.go`).
There is no `harnez agent` driver and no skill target, so OpenCode cannot join
the agent container (#723) on equal terms with Pi and Codex.

/goal Make OpenCode a native Harnez agent like Pi after #712 and #716: a
`harnez agent` driver plus skills and supported hooks managed by `harnez apply`,
`status` and `revert`. Then add it to the #723 container. Stop and report if
OpenCode's CLI cannot run non-interactively or lacks a skill/hook surface.

## 2. Technical Specification / Findings
- No driver in `internal/subagent/`; `SkillTargetsByAgent` in
  `internal/claude/apply.go` has no OpenCode entry.
- lmcoder runs OpenCode in its multi-agent image with skills copied to
  `$XDG_CONFIG_HOME/opencode/skills/`; OpenCode listed all Harnez skills there
  (lmcoder issue 144). Confirm against OpenCode's own docs.

## 3. Implementation & Verification Plan
- Driver following `internal/subagent/pi.go`; apply target following #716.
- Tests as for the other agents; live check against a local model.
- Add OpenCode to the #723 container and its smoke test.
