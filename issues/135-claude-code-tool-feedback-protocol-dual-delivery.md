# 135 — Claude Code gets Tool Feedback Protocol content twice (global section + Skill)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [130](archive/130-instruction-distribution-audit-followups.md) (archived), [docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md](../docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md)

## Problem

Found by the independent reviewer of `issues/archive/130-instruction-distribution-audit-followups.md`'s item 8b
(commit `96e7220`). `config.yaml`'s `agents_md.global.sections` "Tool
Feedback Protocol" section and the new `skills:` `tool-feedback-protocol`
entry carry near-verbatim duplicate content — same `harnez rate` command,
same worked example (`harnez rate Read 5 "found the bug in apply.go"
harnez/117-...`), same 5/3/1 scoring scale. Both reach Claude Code: the
global section as always-on prose in `CLAUDE.md`, the Skill as a
description-matched auto-load. Deliberate at the time (130's reasoning:
Prime Agent has no Skill delivery, so the global section must stay) —
but that reasoning doesn't address Claude Code itself now carrying both
copies, which is exactly the repetition shape the original
instruction-distribution audit was trying to reduce.

## Scope

- Differentiate the two copies' content instead of keeping them
  near-identical: the global section could stay a minimal pointer/reminder
  ("see the tool-feedback-protocol Skill for the worked example"), while
  the Skill carries the full worked-example content — rather than both
  independently restating the whole thing.
- Alternative: leave as-is if a future audit re-run confirms this
  dual-delivery isn't actually costing anything in practice (Claude Code
  sessions may simply not load the Skill and the prose section most of the
  time, so the "always both present" framing may be more theoretical than
  real — worth checking empirically before spending effort here).

## Acceptance Criteria

- [ ] Either the two copies are differentiated (pointer + detail split), or
      a decision to leave as-is is recorded with reasoning.

## Notes

Low priority — not a bug, a minor duplication left over from a legitimate
per-harness design tradeoff. Good candidate for the next
instruction-distribution audit re-run (see [[130]]'s notes on eventually
packaging that audit as a repeatable skill).
