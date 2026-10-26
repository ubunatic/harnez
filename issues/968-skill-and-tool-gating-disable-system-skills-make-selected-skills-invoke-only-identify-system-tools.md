# 968 — Skill and tool gating: disable system skills, make selected skills invoke-only, identify system tools

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 601 (invoke-only wrap skill), 347/348 (disableBundledSkills research)

**Goal**: Cut the Claude Code context used by unneeded skills and tools, and gate skills that should run only on explicit request. Done when the listed items are disabled or gated and `/context` shows the lower usage, or when blocked on a user decision or denied permission, then clear the goal and write a failure report.

---

## 1. Problem & Motivation
`/context` (Haiku 5.5, 200k window) shows 5.3k tokens for skills, 12.9k for system tools and 15k for deferred system tools. Many skills listed in context are not used in this repository. Some skills should run only when the user types them, so the model does not trigger them by mistake.

## 2. Technical Specification / Findings
- **Skill sources in `/context`:** "User" (`~/.claude/skills`), "Built-in" (`plugin-authoring`) and "claude.ai sync" (about 20 Anthropic skills such as `docx`, `pdf`, `pptx`, `xlsx`, `computer-use`, `built-in-browser`).
- **Invoke-only today:** only skills with `disable-model-invocation: true` in frontmatter are blocked from model invocation. Found in `~/.claude/skills`: `harnez-handoff`, `harnez-init`, `harnez-selftest`, `harnez-status`, `next`, `peer-assistant`, `respect`, `wrap`. Other "only on request" skills (`bug-hunt`, `epic`, `jam`, `new-project`, `harnez-advisor`) rely on their description text only.
- **Unverified:** whether a `claude.ai sync` or `Built-in` skill can be disabled from its own file, from settings, or only from the claude.ai account. Not yet checked.
- **Unverified:** how to identify and disable the "System tools" and "System tools (deferred)" blocks in `/context`, which hold about 28k tokens combined. Not yet checked which settings or flags control them.

## 3. Implementation & Verification Plan
1. **Disable these system skills** (source to be confirmed per item): `google-workspace`, `xlsx`, `pptx`, `computer-use`, `built-in-browser`, `artifact-emulator`, `deep-research`, `mcp-builder`, `slack-gif-creator`, `import-memory`, `plugin-authoring`.
2. **Make invoke-only** by adding `disable-model-invocation: true` to `~/.claude/skills/<name>/SKILL.md`, then `harnez apply`: `gokay-sweep`, `sitegen-sweep`, `reverse-sprint`, `epic`, `lean-sprint-v1`.
3. **Open decision for the user, not yet applied:** whether to also gate `internal-comms`, `canvas-design`, `web-artifacts-builder`, `chrome-browser`, `gokay`, `sitegen`, `publish`, `tool-feedback-protocol`, `codebase-memory`, `codebase-memory-scout`, `codebase-memory-auditor`, and all `codebase-memory` and `harnez` MCP tools.
4. **Identify and disable system tools:** find the setting or flag that removes or defers the "System tools" block in `/context`, record the exact setting, and apply it only if it is safe for this repository.

Verification: rerun `/context` before and after and compare the Skills and System tools totals. Confirm each gated skill refuses to run from a plain request without its slash command.
