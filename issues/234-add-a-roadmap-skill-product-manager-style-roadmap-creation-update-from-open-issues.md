# 234 — Add a /roadmap skill: product-manager-style roadmap creation/update from open issues

**Status**: Closed — implemented; commands/roadmap.md + config.yaml registration, reviewed and verified
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `docs/Roadmap.md` (this session's one-off, hand-built example of the exact output this
skill should produce), `commands/harnez-sync.md` (closest existing precedent for an autonomous,
non-interactive multi-step skill that shells out to `harnez find`/`harnez issues` rather than
reimplementing logic inline), `commands/sprint.md` / `commands/fresh-sprint.md` (precedent for a
skill that dispatches a fresh subagent to do the actual work and stays responsive), `config.yaml`
(`skills:` / `commands:` registration shape — see `harnez-sync`'s dual entry for the pattern to
mirror)

---

## 1. Problem & Motivation

This session manually did exactly the workflow this ticket proposes to package: dispatched an
Opus agent to (1) discover the open backlog via `harnez find -d . issues status:open`, (2) read
each ticket's appended `## Implementation Plan` for scope/dependency signal, (3) synthesize a
`docs/Roadmap.md` grouping tickets into themes with a Now/Next/Later sequencing, explicitly
weighted toward "what most increases the tool's value to its actual usage pattern" rather than
raw ticket count or arbitrary grouping.

That worked well, but it was a one-shot, hand-composed agent prompt with no reusable packaging —
running it again next month means re-deriving the same prompt from scratch, and there's no
"update" mode: today's `docs/Roadmap.md` would just get overwritten wholesale by a re-run rather
than reconciled against what's already there (what shipped since, what got reprioritized, what's
newly blocked).

This should generalize beyond harnez: the same shape of skill is useful in any `harnez`-managed
project with an `issues/` tracker — e.g. `voxi` (dictation/ASR project) has its own backlog and
its own product-value axis (dictation quality/latency/reliability), completely different from
harnez's (agentic-workflow tooling velocity). The skill must not hardcode harnez-specific value
judgments; it needs to derive "what does this product exist to do" per-repo.

## 2. Proposed Shape

A new `commands/roadmap.md` skill (and command, mirroring `harnez-sync`'s dual `skills:` +
`commands:` registration in `config.yaml`), invoked as `/roadmap` (optionally `/roadmap <repo>`
for a non-current-directory target, matching other skills' `-d <repo>` convention).

Three-step workflow, done by a dispatched subagent (per `commands/sprint.md`'s "stay responsive,
don't block the host on the result" pattern — this is a multi-minute synthesis task, not a quick
lookup):

1. **Check open issues**: `harnez find -d <repo> issues status:open`, then read each ticket's
   metadata and (if present) its `## Implementation Plan` section for scope/dependency/blocker
   signal — don't re-derive scope from scratch if a plan already exists.
2. **Check current roadmap**: if `docs/Roadmap.md` (or wherever this repo keeps one — check for
   an existing file first, don't assume the path) already exists, read it. This is the "update"
   mode this session's one-off didn't have: reconcile against what's there rather than blindly
   regenerating — tickets that shipped since should move out, tickets that got reprioritized
   should move buckets, and the reasoning for *why* something moved should be visible in the
   diff, not silently overwritten.
3. **Create/update the roadmap**: synthesize or revise the document, acting as a product manager
   whose sequencing decisions are justified by **value delivered to the product's actual use
   cases** — not ticket volume, not "what's easiest," not an arbitrary theme taxonomy. The skill
   needs to figure out what those use cases *are* for the repo it's running in (read
   `AGENTS.md`/`README.md`/`CONTEXT.md` for product identity — e.g. harnez's use case is agentic
   developer workflows across multiple AI coding harnesses; voxi's is dictation/ASR quality) rather
   than assuming a fixed rubric. This is the part most likely to need a strong model (this
   session used Opus for the synthesis step) — the skill's dispatched subagent should probably
   request a capable model rather than the default.

## 3. Acceptance Criteria

- [ ] `commands/roadmap.md` exists, follows this repo's skill-file conventions (see
      `commands/harnez-sync.md`/`commands/issue.md` for structure/tone), and is registered in
      `config.yaml`'s `skills:` list (and `commands:` list, if this should also be directly
      invocable as `/roadmap` the way `harnez-sync` is both).
- [ ] Running it in a repo with no existing roadmap doc produces one, grouped by theme, with an
      explicit Now/Next/Later-style sequencing and a stated rationale tied to product use cases
      (not just "P2 tickets before P3 tickets").
- [ ] Running it again in a repo that already has a roadmap doc **updates** it — reflects closed
      tickets, new tickets, and reprioritizations — rather than wholesale regenerating from a
      blank slate.
- [ ] The skill does not hardcode harnez-specific value judgments (e.g. "usage --watch dashboard
      truth matters most") into the skill file itself — that kind of judgment belongs in the
      per-repo output, derived from that repo's own product identity docs, not baked into the
      skill's instructions.
- [ ] The skill does not modify any ticket, `issues/README.md`, or code — output is limited to
      the roadmap document itself.

## 4. Non-Goals

- Not a replacement for the issue tracker itself, and not a scheduling/assignment tool — it's a
  synthesis/communication artifact.
- Not proposing this skill auto-close, auto-reprioritize, or edit any `issues/*.md` file — it
  reads the tracker, it doesn't write to it.
- Not proposing a specific rubric or scoring formula for "value to use cases" — that's a judgment
  call the dispatched subagent makes per repo, informed by that repo's own docs, not a fixed
  algorithm this ticket should over-specify.

No implementation plan yet — filed to capture the idea and the concrete precedent
(`docs/Roadmap.md`, produced by hand this session) it should generalize from.
