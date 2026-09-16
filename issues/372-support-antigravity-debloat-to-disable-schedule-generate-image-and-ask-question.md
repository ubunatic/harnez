# 372 — Support Antigravity debloat to disable schedule, generate_image, and ask_question

**Status**: Open
**Priority**: P1 (High)
**Severity**: Minor
**Category**: Feature

---

## Summary

Add Antigravity (`agy` / `~/.gemini/antigravity-cli`) support to `harnez apply --debloat`, `harnez status --debloat`, and `harnez revert --debloat`.

As identified in the context assessment (`docs/AntigravityDebloatAssessment.md`), Antigravity injects 13.9k tokens of system tool definitions into every conversation turn. The initial phase of AGY debloat will target three high-overhead / peripheral tools:
1. `schedule` (~1,450 tokens) — cron and one-shot timer management.
2. `generate_image` (~1,100 tokens) — image generation and UI mockups.
3. `ask_question` (~850 tokens) — interactive questionnaire modal (chat text remains fallback).

Combined upfront token savings: **~3,400 tokens (~24% of AGY tool surface)**.

---

## Proposed Spec & Config Changes

Update `config.yaml` to include Antigravity tool definitions in the debloat specification:

```yaml
debloat:
  # Existing Claude Code debloat settings...
  # Antigravity target debloat definitions:
  agy:
    minimal_deny:
      - schedule
      - generate_image
      - ask_question
    aggressive_extra_deny:
      - read_url_content
      - search_web
      - define_subagent
      - manage_subagents
```

---

## Target Settings Surface

Antigravity CLI configuration lives in `~/.gemini/antigravity-cli/settings.json` (or can be configured via CLI/target flags).

`harnez` should:
- Support detecting and managing `~/.gemini/antigravity-cli/settings.json` when targeting AGY.
- Maintain full rollback capability via `<target>/.harnez-debloat.json` sidecar.
- Support `harnez apply --debloat` / `harnez revert --debloat` / `harnez status --debloat` across both Claude Code and Antigravity.

---

## Acceptance Criteria

- [ ] `config.yaml` debloat spec extended with `agy` tool presets (`schedule`, `generate_image`, `ask_question`).
- [ ] Go debloat implementation handles Antigravity settings path and JSON schema.
- [ ] Sidecar recording (`.harnez-debloat.json`) ensures atomic reverts.
- [ ] `harnez status --debloat` reports AGY debloat status accurately.
- [ ] Unit tests cover AGY debloat apply, merge, status, and revert flows.
