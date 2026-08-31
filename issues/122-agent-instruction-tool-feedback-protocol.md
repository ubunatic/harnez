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
- Wire injection into [[119]]'s `harnez hook install` flow (or `harnez
  init`'s doc-copy mechanism, per this repo's existing `apply`/`init`
  split — see `docs/CLIDesign.md`) rather than a third, separate
  mechanism. Decide which at implementation time based on where other
  per-agent instruction snippets currently live.
- Must not duplicate or conflict with this project's own
  `AGENTS.md`/`CLAUDE.md` generation conventions (`docs/Markdown.md`,
  the `apply`/`init` CLI split) — this is agent-facing instruction
  content for *harnez's own* observability feature, injected into
  *other* projects' agent configs, not into harnez's own docs.

## Acceptance Criteria

- [ ] Snippet is injected idempotently (repeated `hook install` doesn't
      duplicate the block).
- [ ] Snippet removal is covered by `harnez hook uninstall` (119).
- [ ] Token count of the injected block roughly matches the spec's
      ~25-token estimate (sanity check, not a hard gate).

## Notes

Low priority relative to 116–121: the instruction text is only useful
once `harnez rate` (117) actually exists and does something with the
calls it prompts for.
