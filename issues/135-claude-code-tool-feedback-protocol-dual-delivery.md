# 135 — Claude Code gets Tool Feedback Protocol content twice (global section + Skill)

**Status**: Closed — Split Tool Feedback Protocol into compact global guidance and full Skill detail
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

---

## Implementation Plan

Decide empirically first, then (only if warranted) differentiate. The cheap
measurement already exists — `harnez stats --overhead` reports
`instruction_bytes` for exactly the `rate_feedback: true`-gated section + skill
pair (`cmd/harnez/stats.go`'s `rateOverheadReport`, issue 142).

### Steps

1. **Measure** (no code): run `harnez stats --overhead --json` and record
   `instruction_bytes` / `estimated_instruction_tokens`. The gated skill body in
   `config.yaml` is ~2.5 KB; the gated global section is ~700 B. Note the split
   in the ticket.
2. **Check real delivery**: confirm whether Claude Code actually auto-loads the
   `tool-feedback-protocol` skill in normal sessions, or only on description
   match. Grep a few recent `~/.claude/projects/*/*.jsonl` transcripts for the
   skill name. If the skill is rarely loaded, the "always both present" framing
   is theoretical and the right outcome is *close as won't-fix with reasoning
   recorded* — that is an acceptable terminal state per the Acceptance Criteria.
3. **If differentiation is chosen**, edit only `config.yaml`:
   - Trim the `agents_md.global.sections` "Tool Feedback Protocol" `content` to a
     pointer form: the one `harnez rate` invocation line, the 5/3/1 scale, the
     `--ok` heartbeat line, plus "full worked example and the expected-failure
     rules live in the `tool-feedback-protocol` Skill (Claude Code / Codex /
     Gemini); Prime Agent has no Skill delivery, so this section stays
     self-sufficient." Keep it self-sufficient — Prime Agent
     (`prime_agent_target`) receives sections but **not** skills
     (`skillTargets` in `internal/claude/apply.go`), so the section can never
     degrade to a bare cross-reference.
   - Leave the `skills:` entry as the long form (worked example, `HARNEZ_EXPECT_FAILURE`
     guidance, rationale).
4. **Verify**: `go test ./...`, then `scripts/smoke-test.sh` (apply + idempotency
   + drift repair). `internal/claude`'s section-merge is name-keyed via
   `markdown.Clean`/`applySectionMD`, so a shrunken section rewrites cleanly in
   place; confirm a second `harnez apply` reports no changes.
5. Re-run `harnez stats --overhead` and record the delta in the ticket.

### Design decisions / tradeoffs

- The global section must stay **standalone-usable**, not a pure pointer: it is
  the only delivery path for Prime Agent. This is the constraint 130 identified;
  it bounds how much can be cut (target ~40-50% shorter, not eliminated).
- Do not add a new config mechanism (e.g. per-target section variants) here —
  that is [[149]]'s per-agent profile work. If 149 lands first, this ticket
  collapses into "give the Tool Feedback Protocol section a
  Prime-Agent-only profile and drop it from Claude/Codex/Gemini targets", which
  is strictly better. Prefer waiting for 149 over inventing a second mechanism.

### Risks / open questions

- Shortening the section risks weakening compliance; the memory note
  "harnez rate after every tool call" records that this instruction is already
  easy to miss inside a large stacked block. Measure compliance before/after via
  `harnez stats` unrated-failure counts rather than assuming.
- Whether Claude Code loads the Skill at all is unverified — step 2 is the
  load-bearing step; if it answers "rarely", stop at step 2.

### Scope

**Small** (config-only edit + verification), or **zero** if step 2 says
close-as-designed. Blocked-adjacent on [[149]]: if that mechanism is imminent,
defer rather than doing the interim trim.
