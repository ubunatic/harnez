# 122 — Agent instruction template: Tool Feedback Protocol injection

**Status**: Closed — resolved in f158a98
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [[117-harnez-rate-command]], [[119-harnez-hook-agent-hook-management]], [[128-per-agent-full-system-prompt-self-audit-for-repetition]]

## Problem

For `harnez rate` (117) to actually get called, agents need the
instruction snippet injected into their config files (`CLAUDE.md`,
`.antigravity/instructions.md`, etc.) telling them to call it after
every internal tool use.

## Scope

- Add the ~25-token instruction block from the spec:
  ```markdown
  ### Tool Feedback Protocol
  After executing any internal tool (e.g., reading files, editing, semantic scanning), immediately record the action:
  `harnez rate <tool_name> <1-5> "<1-line outcome summary>" "<project_folder>/<ticket_name>"`
  - Score 5: Flawless result.
  - Score 3: Partial success / required adjustment.
  - Score 1: Total failure / useless output.
  ```
- Per [[119]]'s 2026-08-31 decision (no standalone `harnez hook`
  command), wire injection into `apply`'s existing per-agent config
  writes — the same managed-sections mechanism `apply` already uses to
  write `~/.claude/CLAUDE.md`, `~/.prime/agent/AGENTS.md`, etc. (see
  `docs/CLIDesign.md`'s `apply flow`) — rather than `init`'s doc-copy
  mechanism, since this snippet targets *other projects'* agent
  configs globally, not this repo's own `init`-scaffolded project docs.
- Must not duplicate or conflict with this project's own
  `AGENTS.md`/`CLAUDE.md` generation conventions (`docs/Markdown.md`,
  the `apply`/`init` CLI split) — this is agent-facing instruction
  content for *harnez's own* observability feature, injected into
  *other* projects' agent configs, not into harnez's own docs.

## Acceptance Criteria

- [x] Snippet is injected idempotently (repeated `apply` runs don't
      duplicate the block), matching `apply`'s existing managed-section
      idempotency. Verified: `TestApplyInstallsToolFeedbackProtocol`
      (`internal/claude/toolfeedback_test.go`) applies twice against
      isolated temp-dir targets and asserts exactly one managed-section
      marker pair; confirmed against the real `~/.claude/CLAUDE.md` /
      `~/.prime/agent/AGENTS.md` too — `harnez diff` reports "No
      changes." after a second real `apply`.
- [x] Snippet removal is covered by `harnez clean` (119), matching how
      other managed sections are stripped. Verified in the same test:
      `CleanAll` removes the section from both global targets.
- [x] Token count of the injected block roughly matches the spec's
      ~25-token estimate (sanity check, not a hard gate). Verified in
      `TestToolFeedbackProtocolConfigEntry` (rough word-count bound,
      15-60 words, not a real tokenizer).

## Resolution

Implemented as a pure `config.yaml` addition — a new
`agents_md.global.sections` entry named "Tool Feedback Protocol",
consumed by the exact same managed-section mechanism `apply.go` already
uses for the "Instructions Hierarchy" / "Minimal Global Docs" sections
(no new Go code path, no `apply.go` changes needed). Reaches both global
targets `apply` already writes: `~/.claude/CLAUDE.md` and
`~/.prime/agent/AGENTS.md`. `harnez status` now reports it as
`[Tool Feedback Protocol] ok` alongside the other managed sections.

## Notes

Low priority relative to 116–121: the instruction text is only useful
once `harnez rate` (117) actually exists and does something with the
calls it prompts for. This is the last ticket in the 115-122
tool-observability story — all eight are now closed.

**2026-08-31 follow-up**: despite the snippet being correctly injected
and live in `~/.claude/CLAUDE.md`, a real session still skipped it on
its first tool call — the injection working is not the same as the
directive being followed. Prompted [[128]] and a clean-session
self-audit (see
[docs/studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md](../docs/studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md))
which diagnosed why: the snippet reads as standing background policy
with no example invocation and no stated consequence for skipping,
unlike the git-commit-trailer rule which is exercised as a literal
template every commit. Worth revisiting the snippet's phrasing along
those lines if misses recur.
