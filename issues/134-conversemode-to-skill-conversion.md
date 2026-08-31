# 134 — Convert `ConciseMode.md` from AGENTS.md-mutation to a Claude-Code Skill

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[130-instruction-distribution-audit-followups]], [[075-concisemode-promote-doc-to-real-skill]], [docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md](../docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md)

## Problem

Split out of [[130]] item 8, which originally bundled three Skill-conversion
candidates (`Website.md`, `ConciseMode.md`, Tool Feedback Protocol) behind
one blocked-on-item-7 line. `ConciseMode.md` is a distinct, more involved
case than the other two and deserves its own ticket rather than riding
along with 130's remaining scope.

Issue [[075]] already promoted `ConciseMode.md` from a passive doc into a
runtime switch (`harnez mode`) that rewrites a managed `AGENTS.md` section
on disk — a disk-mutating mechanism built before Skills were an available
option for harnez-authored content reaching Claude Code. Per the
2026-08-31 instruction-distribution audit synthesis (§6), this is a
candidate for replacing that disk-mutation with a Skill triggered by
context (e.g. "the user asks to change terseness / mentions slow
inference") instead.

## Scope

- Blocked on [[130]] item 7 (`claude_skills_target` write path in
  `apply.go`/`config.yaml`) — cannot start until that infra exists.
- Evaluate whether a Skill can fully replace `harnez mode`'s current
  mechanism, or whether the two need to coexist (e.g. Skill handles
  *detecting* the terseness request and explaining the tiers, while
  `harnez mode` still does the actual `AGENTS.md`-section rewrite for
  persistence across sessions — a Skill's on-demand loading doesn't by
  itself provide persistent state the way a written `AGENTS.md` section
  does).
- If a Skill can't cleanly replace persistence, scope this as a hybrid:
  Skill for the trigger-and-explain path, existing `harnez mode` command
  retained for the actual persistent switch.
- Out of scope: touching `Website.md` or the Tool Feedback Protocol
  Skill-conversion work — those stay in [[130]] item 8.

## Acceptance Criteria

- [ ] Design decision recorded: full Skill replacement vs. hybrid
      (Skill-trigger + `harnez mode`-persistence).
- [ ] Implemented per that decision.
- [ ] `harnez apply`/`harnez diff` clean afterward if `config.yaml` changes.
- [ ] `go test ./...` passes.

## Notes

Not scheduled ahead of [[130]]'s remaining items — file this ticket now
while the scope split is fresh, pick it up once item 7's Skill-target infra
lands and 130's Website.md/Tool-Feedback-Protocol conversions are done.
