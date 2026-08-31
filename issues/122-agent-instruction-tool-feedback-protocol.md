# 122 — Agent instruction template: Tool Feedback Protocol injection

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [[117-harnez-rate-command]], [[119-harnez-hook-agent-hook-management]]

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

- [ ] Snippet is injected idempotently (repeated `apply` runs don't
      duplicate the block), matching `apply`'s existing managed-section
      idempotency.
- [ ] Snippet removal is covered by `harnez clean` (119), matching how
      other managed sections are stripped.
- [ ] Token count of the injected block roughly matches the spec's
      ~25-token estimate (sanity check, not a hard gate).

## Notes

Low priority relative to 116–121: the instruction text is only useful
once `harnez rate` (117) actually exists and does something with the
calls it prompts for.
