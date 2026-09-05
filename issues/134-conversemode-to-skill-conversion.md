# 134 — Add a ConciseMode trigger Skill alongside `harnez mode`'s persistence mechanism

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
2026-08-31 instruction-distribution audit synthesis (§6), this was flagged
as a candidate for replacing that disk-mutation with a Skill.

[[130]] item 7 (`claude_skills_target` write path) has since landed
(commit `96e7220`), so this is no longer blocked.

**Design decision, settled 2026-08-31 (user clarification)**: `harnez
mode`'s `AGENTS.md`-rewrite is not a pre-Skill-era workaround to be
replaced — it serves two distinct purposes, only one of which a Skill can
take over:
1. **Trigger fallback** — catching a terseness request even when the
   agent overlooks `ConciseMode.md`'s own guidance. A Skill *can* do this,
   and can do it more reliably than hoping the agent notices unprompted
   (same "trigger-tied, not standing prose" lesson as the Tool Feedback
   Protocol work in [[130]]).
2. **Cross-agent, cross-session persistence** — writing the chosen tier
   into the repo's `AGENTS.md` so that *other* agents (a different Claude
   Code session started later, a Prime Agent session, any future agent
   reading this repo) also see the current setting. A Skill is scoped to
   the session it fires in; it has no mechanism to make state visible to
   a separate agent/session/harness. This is a real, structural limit, not
   a gap to design around — Skills cannot replace this role.

**Verdict**: hybrid, not full replacement. Keep `harnez mode`'s
`AGENTS.md`-rewrite exactly as the persistence mechanism. Add a Skill
purely as an additional, more reliable *trigger* surface for role 1 — when
it fires (description: "user asks to speed up responses / reduce verbosity
/ mentions slow inference or low-TPS hardware"), it invokes or explains
`harnez mode` rather than attempting to replace what that command does.

## Scope

- Add a `concise-mode` (or similarly named) entry to `config.yaml`'s
  `skills:` list, following the same shape as the `tool-feedback-protocol`
  entry added in [[130]] — Claude-Code-only delivery via the
  `claude_skills_target` path, no new slash command needed (`/mode` stays
  as the direct manual invocation).
- Skill body: explain the tier system briefly and instruct calling
  `harnez mode <tier>` — do not duplicate the full `ConciseMode.md` tier
  table inline; point to it the same way other Skills/docs do.
- Do NOT change `harnez mode`'s implementation or its `AGENTS.md`-section
  rewrite mechanism — that's the settled, correct persistence path and is
  out of scope here.
- Out of scope: touching `Website.md` or the Tool Feedback Protocol
  Skill-conversion work — those are already done in [[130]].

## Acceptance Criteria

- [x] Design decision recorded: hybrid (Skill-trigger + `harnez
      mode`-persistence), not full Skill replacement — see Problem above.
- [ ] Skill implemented per that decision.
- [ ] `harnez apply`/`harnez diff` clean afterward.
- [ ] `go test ./...` passes.

## Notes

No longer blocked — [[130]] item 7's Skill infra and the design ambiguity
are both resolved. Ready to pick up whenever scheduled.

---

## Implementation Plan

**Important finding that changes the scope**: a `mode` Skill entry already
exists in `config.yaml`'s `skills:` list (line ~217, `file: commands/mode.md`)
and is already installed to `claude_skills_target` — it is live in Claude Code's
skill list today. So the infrastructure work is done; what is missing is purely
the *trigger surface*. Its current description —
`"Switch ConciseMode terseness level (lite, std, ultra, off) and sync AGENTS.md"` —
is command-shaped: it fires when the user types `/mode`, but not when the user
says "you're too verbose", "keep it short", or "this local model is slow".

### Design decision

**Amend the existing `mode` entry's description; do not add a second
`concise-mode` skill.** Two skills over the same mechanism compete for the same
trigger, and the loser is invisible — exactly the buried-directive failure mode
128 documents. The Skill body (`commands/mode.md`) already does the right thing:
it invokes `harnez mode <tier>` and points at `@docs/practices/ConciseMode.md`
without duplicating the tier table, satisfying this ticket's Scope bullets 2
and 3. `harnez mode`'s `AGENTS.md`/`AGENTS.local.md` rewrite stays untouched, per
the settled hybrid verdict.

### Steps

1. `config.yaml`, `skills:` → `mode` entry: rewrite `description:` to be
   trigger-shaped, in the same style as the `tool-feedback-protocol` entry
   (which names its trigger conditions explicitly). Something like:
   > Use when the user asks for shorter/faster replies, says responses are too
   > verbose, mentions slow inference or low-TPS/local hardware, or types
   > `/mode` — switch the ConciseMode terseness tier (lite, std, ultra, off) by
   > running `harnez mode <tier>`, which also persists the tier into AGENTS.md
   > so other sessions and agents inherit it.
2. `commands/mode.md`: add two lines at the top of `## Behavior` stating the
   natural-language triggers, so the loaded body reinforces the description.
   Do not expand it further — the doc's brevity is a feature.
3. `harnez apply` on a scratch HOME (or `scripts/smoke-test.sh`) and confirm
   `harnez diff` is clean afterward.
4. `go test ./...` — check whether `internal/claude/claudeskills_test.go` asserts
   on the `mode` skill's description text; update the fixture if so.
5. Tick Acceptance Criteria 2–4 and close, recording in Notes that the decision
   was "amend the existing entry", not "add a new skill".

### Risks / open questions

- A broad trigger description can cause spurious activation (any mention of
  "slow" firing the skill). Keep the trigger list to the four concrete phrasings
  above rather than a general "verbosity" concept.
- If the user genuinely wants a *separate* named skill for discoverability
  (`/concise-mode`), that is a one-line config addition later — but note it would
  need a distinct body, not a duplicate of `commands/mode.md`.

### Scope

Small (one config description, two doc lines, one possible test fixture).
